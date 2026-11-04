#!/usr/bin/env python3
"""Verify bounded original bytes and derive a static action roadmap, not rules."""
import argparse
import gzip
import hashlib
import json
import re
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
STATIC = Path(__file__).resolve().parent
ROM_SHA256 = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
TABLES = {'ground': 0x15e10, 'air': 0x15ee8,
          'ground_special': 0x15f0e, 'air_special': 0x15f34}
FRAME_TABLES = {0x1a342: 13, 0x1af28: 26, 0x1b238: 7,
                0x1b25e: 11, 0x1cb20: 8, 0x1cb80: 8}
RECOVERED = set(range(14)) | {0x0f, 0x14, 0x15, *range(0x41, 0x47)}
KNOWN_CHILDREN = {0x1313e, 0x132de, 0x1361e}
KNOWN_HELPERS = {0x15cb0, 0x1faf0, 0x1fb4c, 0x1fce8, 0x20202,
                 0x203f8, 0x20444, 0x204fa, 0x205b0}
SOURCE_FILES = [
    'games/yuyuhakusho/engine/chu_action.go',
    'games/yuyuhakusho/engine/chu_tail.go',
    'games/yuyuhakusho/engine/chu_attacks.go',
    'games/yuyuhakusho/engine/chu_special_motion.go',
    'games/yuyuhakusho/engine/chu_projectile.go',
    'games/yuyuhakusho/engine/chu_attachments.go',
    'games/yuyuhakusho/engine/throw_records.go',
    'games/yuyuhakusho/engine/ground_control.go',
    'games/yuyuhakusho/engine/reaction_motion.go',
    'games/yuyuhakusho/engine/effects.go',
    'games/yuyuhakusho/engine/character_commands.go',
    'games/yuyuhakusho/engine/fighter_ai.go',
    'games/yuyuhakusho/engine/jump.go',
    'games/yuyuhakusho/data/animation-catalog-manifest.json',
    'games/yuyuhakusho/data/animation-catalog-validation.json',
    'games/yuyuhakusho/data/command-programs.json',
    'games/yuyuhakusho/engine/testdata/character-commands-validation.json',
    'games/yuyuhakusho/engine/testdata/fighter-ai-validation.json',
    'games/yuyuhakusho/engine/testdata/chu-action-validation.json',
    'games/yuyuhakusho/engine/testdata/chu-special-motion-validation.json',
    'games/yuyuhakusho/engine/testdata/chu-projectile-validation.json',
    'games/yuyuhakusho/engine/testdata/chu-attachments-validation.json',
    'docs/research/md-yuyuhakusho/static/action-character-tables.txt',
    'docs/research/md-yuyuhakusho/static/action-dispatch-table.txt',
    'docs/research/md-yuyuhakusho/static/FighterActionRoadmapProbe.java',
    'docs/research/md-yuyuhakusho/static/derive-fighter-action-roadmap.py',
]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def binding(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path.read_bytes())}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--write', action='store_true', help='write derived research artifacts')
    args = parser.parse_args()
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert sha(rom) == ROM_SHA256
    flow_path = STATIC/'fighter-action-flow.json.gz'
    raw_flow = gzip.decompress(flow_path.read_bytes())
    flow = json.loads(raw_flow)
    instructions = {int(row['pc'], 16): row for row in flow['instructions']}
    assert len(instructions) == len(flow['instructions']) == 5454
    for pc, row in instructions.items():
        assert bytes.fromhex(row['bytes']) == rom[pc:pc+row['length']]

    def table(base, count):
        return [base+i*2+int.from_bytes(rom[base+i*2:base+i*2+2], 'big', signed=True)
                for i in range(count)]

    selectors = {mode: table(base, 11) for mode, base in TABLES.items()}
    slot_targets = table(0x15a68, 74)
    prep_targets = table(0x15c20, 11)
    indirect = []
    for pc, row in instructions.items():
        match = re.fullmatch(r'lea \(0x([0-9a-f]+),PC,D0w\*0x1\),A0', row['text'])
        if match:
            base = int(match[1], 16)
            count = 11 if base == 0x1690c else 16 if pc < 0x18440 else FRAME_TABLES[base]
            indirect.append({'lea_pc': hex(pc), 'table': hex(base), 'count': count,
                             'bytes': rom[base:base+count*2].hex(),
                             'targets': [hex(target) for target in table(base, count)],
                             'scope': 'structural entries; arbitrary input/frame indexes are not proven valid'})
    assert len(indirect) == 29
    computed = {int(row['table'], 16)-2: [int(target, 16) for target in row['targets']]
                for row in indirect}
    assert {pc for pc, row in instructions.items() if row['flow_kind'] == 'COMPUTED_JUMP'} == set(computed)

    def walk(entry):
        seen, outside, todo = set(), set(), [entry]
        while todo:
            pc = todo.pop()
            if pc in seen:
                continue
            if pc not in instructions:
                outside.add(pc)
                continue
            seen.add(pc)
            row = instructions[pc]
            todo.extend(int(target, 16) for target in row['flows'])
            if row['fallthrough'] is not None:
                todo.append(int(row['fallthrough'], 16))
            todo.extend(computed.get(pc, []))
        return seen, outside

    def action_writes(seen):
        result = []
        for pc in sorted(seen):
            text = instructions[pc]['text']
            match = re.fullmatch(r'move.b #(-?0x[0-9a-f]+),\(0x5d,A5\)', text)
            if match:
                result.append({'action': int(match[1], 16) & 255, 'pc': hex(pc)})
            elif text == 'clr.b (0x5d,A5)':
                result.append({'action': 0, 'pc': hex(pc)})
        return result

    def callback_stores(seen):
        result = []
        for pc in sorted(seen):
            match = re.fullmatch(r'move.l #0x([0-9a-f]+),\(0x0,A[01]\)', instructions[pc]['text'])
            if match:
                target = int(match[1], 16)
                result.append({'pc': hex(pc), 'target': hex(target),
                               'entry_has_recovered_chu_rule': target in KNOWN_CHILDREN,
                               'new_creation_context_cpu_verified': False})
        return result

    characters = []
    owner_map = {slot: [] for slot in range(74)}
    for cid in range(11):
        modes, selected, selector_seen = {}, set(), set()
        for mode, entries in selectors.items():
            seen, outside = walk(entries[cid])
            selector_seen |= seen
            writes = action_writes(seen)
            selected |= {row['action'] for row in writes}
            ctor_roots = sorted({int(target, 16) for pc in seen if pc < 0x18440
                                 for target in instructions[pc]['flows']
                                 if 0x18440 <= int(target, 16) < 0x19c96})
            inline = [{'pc': hex(pc), 'text': instructions[pc]['text']} for pc in sorted(seen)
                      if pc < 0x18440 and re.search(r'\(-0x59[0-9a-f]{2},A1\)', instructions[pc]['text'])]
            modes[mode] = {'entry': hex(entries[cid]), 'static_action_writes': writes,
                           'direct_constructor_roots': [hex(pc) for pc in ctor_roots],
                           'inline_throw_instructions': inline,
                           'outside_edges': [hex(pc) for pc in sorted(outside)],
                           'observation_kind': 'bounded_static_overapproximation; no new CPU calls'}
        closure, todo = set(selected) | set(range(14)), list(selected) + list(range(14))
        action_seen, helpers = set(), set()
        while todo:
            slot = todo.pop()
            assert 0 <= slot < 74
            seen, outside = walk(slot_targets[slot])
            action_seen |= seen
            helpers |= outside - {0x15afc}
            for row in action_writes(seen):
                if row['action'] not in closure:
                    closure.add(row['action'])
                    todo.append(row['action'])
        for slot in closure:
            owner_map[slot].append(cid)
        callbacks = callback_stores(selector_seen | action_seen)
        missing = sorted(closure-RECOVERED)
        constructors = sorted({int(pc, 16) for mode in modes.values()
                               for pc in mode['direct_constructor_roots']})
        next_prep = prep_targets[cid+1] if cid < 10 else 0x18440
        characters.append({'source_id': cid, 'command_preparation_entry': hex(prep_targets[cid]),
                           'attack_selectors': modes,
                           'selector_region': [hex(selectors['ground'][cid]), hex(next_prep)],
                           'static_action_closure': [f'{slot:02x}' for slot in sorted(closure)],
                           'already_recovered_same_slot': [f'{slot:02x}' for slot in sorted(closure & RECOVERED)],
                           'missing_continuation_slots': [f'{slot:02x}' for slot in missing],
                           'missing_continuation_roots': [hex(slot_targets[slot]) for slot in missing],
                           'constructor_roots': [hex(pc) for pc in constructors],
                           'child_callback_stores': callbacks,
                           'new_child_callback_entries': sorted({row['target'] for row in callbacks if not row['entry_has_recovered_chu_rule']}),
                           'outside_helpers': [{'entry': hex(pc), 'same_entry_has_recovered_helper': pc in KNOWN_HELPERS}
                                               for pc in sorted(helpers)],
                           'full_action_oracle_status': 'existing_source9_verified' if cid == 9 else 'not_yet_captured_for_this_source',
                           'new_original_cpu_calls': 0})
    assert set(int(slot, 16) for slot in characters[9]['static_action_closure']) == RECOVERED
    assert [slot for slot, owners in owner_map.items() if not owners] == [0x10]
    assert not characters[10]['child_callback_stores']
    assert characters[1]['attack_selectors']['air_special']['static_action_writes'] == []
    assert instructions[selectors['air_special'][1]]['text'] == 'rts'

    nominal_bounds = sorted(set(slot_targets)) + [0x1cc82]
    slots = []
    for slot, entry in enumerate(slot_targets):
        seen, outside = walk(entry)
        shared = [other for other, target in enumerate(slot_targets) if other != slot and target == entry]
        status = 'recovered_same_slot' if slot in RECOVERED else 'same_entry_as_0f_but_current_switch_rejects_10' if slot == 0x10 else 'missing'
        slots.append({'slot': f'{slot:02x}', 'entry': hex(entry),
                      'structural_region_end_exclusive': hex(nominal_bounds[nominal_bounds.index(entry)+1]),
                      'source_ids_static_may_use': owner_map[slot],
                      'owner_scope': 'selector + continuation closure; common 00..0d included for all; not natural execution',
                      'same_entry_slots': [f'{other:02x}' for other in shared],
                      'implementation': status, 'reachable_instruction_count_including_helpers': len(seen),
                      'next_action_writes': action_writes(seen), 'child_callback_stores': callback_stores(seen),
                      'outside_edges': [hex(pc) for pc in sorted(outside)]})

    # Prior listings are independent saved evidence. Their overlap must not drift.
    overlap, differences = 0, []
    for path in STATIC.glob('*.txt'):
        if path.name.startswith('fighter-action-'):
            continue
        for line in path.read_text().splitlines():
            match = re.fullmatch(r'([0-9a-f]{8}) (.+)', line)
            if match and int(match[1], 16) in instructions:
                overlap += 1
                if instructions[int(match[1], 16)]['text'] != match[2]:
                    differences.append([path.name, line])
    assert overlap == 1372 and not differences
    regions = [('fighter-action-selectors-focused.txt', 0x15f4a, 0x18440),
               ('fighter-action-constructors-focused.txt', 0x18440, 0x19c96),
               ('fighter-action-slots-focused.txt', 0x19cee, 0x1cc82)]
    listings = []
    for name, start, end in regions:
        lines = [f'{pc:08x} {row["text"]}' for pc, row in sorted(instructions.items()) if start <= pc < end]
        text = ('route=direct-ghidra-headless; REA raw 68000 unsupported\n'
                'source=MD Yu Yu Hakusho Japan; bounded static reachable instructions, not runtime coverage\n'
                f'scope=0x{start:x}..0x{end:x} exclusive; see fighter-action-coverage.json for roots and indirect tables\n\n'
                + '\n'.join(lines) + '\n')
        path = STATIC/name
        if args.write:
            path.write_text(text)
        else:
            assert path.read_text() == text
        listings.append(dict(binding(path), instructions=len(lines)))
    seeds = sorted(set(slot_targets) | {pc for entries in selectors.values() for pc in entries}
                   | {int(pc, 16) for row in indirect for pc in row['targets']})
    report = {'schema': 1, 'scope': 'Static roadmap for source0..10 action execution gaps; no engine implementation',
              'evidence_kind': 'Bounded static observation and explicitly overapproximated control-flow inference',
              'rom_sha256': ROM_SHA256, 'new_original_cpu_calls': 0, 'runtime_complete': False,
              'analysis_checks_pass': True,
              'flow': dict(binding(flow_path), raw_sha256=sha(raw_flow), instructions=len(instructions),
                           instruction_bytes_verified=24744, prior_listing_instructions_equal=overlap,
                           computed_jumps_bound=29),
              'ghidra_seed_arguments': ','.join(f'{pc:x}' for pc in seeds),
              'dispatcher': {'entry': '0x15a18', 'command_preparation_dispatch': '0x15c10',
                             'shared_input_preparation': '0x15c36', 'table': '0x15a68', 'slots': 74,
                             'unique_targets': 73, 'shared_tail': '0x15afc', 'return': '0x15b82',
                             'table_bytes': rom[0x15a68:0x15afc].hex(),
                             'arbitrary_runtime_index_range_established': False},
              'selector_tables': {mode: {'table': hex(base), 'wrapper': wrapper,
                                        'bytes': rom[base:base+22].hex()}
                                  for (mode, base), wrapper in zip(TABLES.items(), ['0x15de6', '0x15e28', '0x15efe', '0x15f24'])},
              'indirect_tables': indirect,
              'current_recovered_slots': [f'{slot:02x}' for slot in sorted(RECOVERED)],
              'characters': characters, 'slots': slots, 'focused_listings': listings,
              'sources': [binding(ROOT/path) for path in SOURCE_FILES],
              'recommended_next_source': 10,
              'recommendation_basis': 'Five missing continuations; no new character child callback store; existing 0x20202 helper; source8/2 need one/two new child lifecycles',
              'known_new_source10_state': [{'offset': '0x48', 'width': 'byte', 'use': 'cleared by 0x19bf0, incremented/read by action49; current Chu action state has no named owner'}],
              'unknowns': ['No new character selector, constructor or continuation CPU return is claimed here',
                           'Static graph follows both sides of conditions and direct-call fallthrough; it is an overapproximation',
                           'Source2 request1 direction table0x1690c has 11 structural entries; held nibble11..15 reads adjacent code, behavior unverified',
                           'Slot10 aliases slot0f at0x1ca9c but no in-scope selector/common path writes10; validity and reachability remain unknown',
                           'Child callback entries are known stores; newly needed callback extents, initialization resources and full lifetimes are not recovered here',
                           'Animation, commands and AI coverage do not prove attacks or full matches playable',
                           'Collision-selected states outside selector/continuation closure may add requirements and need integrated CPU evidence']}
    path = STATIC/'fighter-action-coverage.json'
    if args.write:
        path.write_text(json.dumps(report, indent=2)+'\n')
    else:
        assert json.loads(path.read_text()) == report
    print(json.dumps({'analysis_checks': 'pass', 'instructions': len(instructions),
                      'prior_listing_equal': overlap, 'slots': len(slots), 'selectors': 44,
                      'new_original_cpu_calls': 0, 'source10_missing': characters[10]['missing_continuation_slots'],
                      'source10_new_callbacks': characters[10]['new_child_callback_entries'],
                      'manifest': binding(path)}, indent=2))


if __name__ == '__main__':
    main()
