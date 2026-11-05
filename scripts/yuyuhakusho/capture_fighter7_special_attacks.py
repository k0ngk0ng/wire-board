#!/usr/bin/env python3
"""Passive source7 special-only evidence from Root's fixed 5502-frame control."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import hashlib
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_fighter7_attacks import (
    ACTOR_WRITES, CONSTRUCTORS, DEPENDENCIES, ENTRIES, HELPERS, IRQ_ENTRY,
    IRQ_EXITS, NATURAL_RETURNS, Rows, SOUND_CONSUMERS, audit_ram, byte_changes,
    snapshot,
)

INPUT_SHA = '5354fe1e66a6875483398763ce7644c363682e864537f827786fd42b0fab05fd'
BASELINE_SHA = '98a17dffa4906b63a6ad2e309c7cfb402ca38e0a46fdfce7a5b0bd81456d55dd'
ORIGINAL_CORE_SHA = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
HOOK_CORE_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
FRAME_COUNT, ENTRY_LIMIT = 5502, 5500
FROZEN = ROOT/'games/yuyuhakusho/engine/testdata/fighter7-attacks-test-first-freeze.json'
ROOTS = {
    'ground-special/r2/c': (0x196d6, 16, 0x3b, 0xa6, 1893),
    'ground-special/r2/b': (0x196ee, 16, 0x3b, 0xa6, 2007),
    'ground-special/r2/a': (0x19706, 16, 0x3b, 0xa6, 2070),
    'ground-special/r1/b': (0x195fa, 16, 0x39, 0xa0, 2157),
    'ground-special/r1/a': (0x19624, 16, 0x39, 0xa0, 2226),
    'ground-special/r3/c': (0x19676, 16, 0x3a, 0xa4, 2326),
    'ground-special/r3/b': (0x1968e, 16, 0x3a, 0xa4, 2409),
    'ground-special/r3/a': (0x196a6, 16, 0x3a, 0xa4, 2492),
    'air-special/r2/b': (0x19736, 16, 0x45, 0xae, 2669),
    'air-special/r2/a': (0x1975e, 20, 0x45, 0xaf, 2720),
    'air-special/r3/c': (0x19676, 16, 0x3a, 0xa4, 2781),
    'air-special/r3/b': (0x1968e, 16, 0x3a, 0xa4, 2876),
    'air-special/r3/a': (0x196a6, 16, 0x3a, 0xa4, 3028),
    'ground-special/r6': (0x19798, 200, 0x3d, 0xaa, 4038),
}


def verify_frozen():
    frozen = json.loads(FROZEN.read_text())
    assert len(frozen['files']) == 14
    for binding in frozen['files']:
        assert sha((ROOT/binding['path']).read_bytes()) == binding['sha256'], binding['path']
    return sha(FROZEN.read_bytes())


def successful_root(row):
    """A reached constructor alone never establishes successful payment."""
    before, after = row['before'], row['after']
    body, result = before['body'], after['body']
    request, changed = body['request'], before['changed']
    kind = row['kind']
    if kind not in ['ground-special', 'air-special']:
        return None
    if kind == 'ground-special' and request in [4, 6]:
        assert request != 4 or '0x19798' not in row['constructors'], 'R4 is not a player command'
        label = f'{kind}/r{request}'
    elif request in [1, 2, 3]:
        if request == 1 or kind == 'air-special' and request == 2:
            button = 'b' if changed & 16 else 'a' if changed & 64 else None
        else:
            button = 'c' if changed & 32 else 'b' if changed & 16 else 'a' if changed & 64 else None
        label = f'{kind}/r{request}/{button}'
    else:
        return None
    if label not in ROOTS:
        return None
    constructor, cost, action, pose, discovery_frame = ROOTS[label]
    if hex(constructor) not in row['constructors']:
        return None
    spent = before['energy']-after['energy']
    if spent == 0:
        return None
    assert spent == cost and result['action_kind'] == action and result['pose'] == pose, (label, spent, result)
    assert result['request'] == 0 and result['enabled_stages'] == 0
    assert before['energy_fraction'] == after['energy_fraction']
    assert row['object_id'] == 0x8400 and row['source_character_id'] == 7
    world = bytes.fromhex(before['world_bytes'])
    assert world[0x58] == 7 and world[0xc58] == 9
    if label == 'ground-special/r6':
        assert body['health'] <= 111 and before['held'] & 2 and before['energy'] >= 200
    elif request == 1:
        vx, vy = (0x40000, -0x20000) if label.endswith('/b') else (0x60000, -0x30000)
        if body['facing'] & 1:
            vx = -vx
        assert result['velocity_x'] == vx and result['vertical_velocity'] == vy and result['gravity'] == 0x3800
        assert after['aux48'] == before['aux48'] & 0xffffff
    elif request == 2 and kind == 'ground-special':
        high = {'c': 0x30, 'b': 0x70, 'a': 0xb0}[label[-1]]
        assert before['child_size'] == 0
        assert result['velocity_x'] == high << 16 | body['velocity_x'] & 0xffff
        assert after['aux48'] == before['aux48']
    elif request == 2:
        vx, vy = (0x40000, 0x40000) if label.endswith('/b') else (0x80000, 0x30000)
        facing = 0 if before['held'] & 8 else 1 if before['held'] & 4 else body['facing']
        assert result['velocity_x'] == vx and result['vertical_velocity'] == vy and result['facing'] == facing
    elif request == 3:
        high = {'c': 2, 'b': 1, 'a': 0}[label[-1]]
        assert after['aux48'] == high << 24 | before['aux48'] & 0xffffff
        assert result['velocity_x'] == body['velocity_x'] and result['gravity'] == body['gravity']
    return label, {
        'frame': row['frame'], 'exit_frame': row['exit_frame'], 'constructor': hex(constructor),
        'discovery_frame': discovery_frame, 'discovery_frame_matches': row['frame'] == discovery_frame,
        'object_id': row['object_id'], 'request': request, 'held': before['held'], 'changed': changed,
        'before_energy': before['energy'], 'after_energy': after['energy'], 'actual_cost': spent,
        'energy_fraction': before['energy_fraction'], 'health': body['health'],
        'facing': body['facing'], 'before_child_size': before['child_size'],
        'action': result['action_kind'], 'pose': result['pose'],
        'entry_sp': row['entry_sp'], 'exit_sp': row['exit_sp'],
        'ram_before_sha256': row['ram_audit']['before_sha256'],
        'ram_after_sha256': row['ram_audit']['after_sha256'],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--reference-manifest', required=True, type=Path)
    args = parser.parse_args()
    out, control_path = args.out.resolve(), args.reference_manifest.resolve()
    assert out.is_relative_to(ROOT) and control_path.is_relative_to(ROOT)
    frozen_sha = verify_frozen()
    control = json.loads(control_path.read_text())
    assert control['observer_attached'] is False and control['frames'] == FRAME_COUNT
    assert control['entry_frame_limit'] == ENTRY_LIMIT and control['drain_frames'] == 2
    assert control['identity']['core_sha256'] == ORIGINAL_CORE_SHA
    inputs, baseline = ROOT/control['input']['path'], ROOT/control['output']['path']
    assert inputs.resolve().is_relative_to(ROOT) and baseline.resolve().is_relative_to(ROOT)
    assert sha(inputs.read_bytes()) == control['input']['sha256'] == INPUT_SHA
    assert sha(baseline.read_bytes()) == control['output']['sha256'] == BASELINE_SHA
    events, expected = json.loads(inputs.read_text()), json.loads(baseline.read_text())
    assert len(expected) == FRAME_COUNT
    out.mkdir(parents=True, exist_ok=True)
    rows = Rows(out/'special-natural.jsonl')
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set(ENTRIES) | CONSTRUCTORS | NATURAL_RETURNS | HELPERS |
                    IRQ_EXITS | SOUND_CONSUMERS | {IRQ_ENTRY, 0x3982} |
                    {pc | 0xff000000 for pc in IRQ_EXITS | {IRQ_ENTRY}})
    assert len(points) <= 64
    pcs = (C.c_uint*len(points))(*points)
    pending, errors = None, []
    successes = {label: [] for label in ROOTS}
    irq_count = sound_count = rng_count = 0
    with ReferenceSession(ROOT, out/'observed', core=core) as ref:
        assert ref.identity['core_sha256'] == HOOK_CORE_SHA

        @callback_type
        def observe(kind, width, raw_pc, value):
            nonlocal pending, irq_count, sound_count, rng_count
            try:
                pc = raw_pc & 0xffffff
                address, sp = ref.get_register(13) & 0xffffff, ref.get_register(15)
                if pending is not None:
                    pending['min_sp'] = min(pending['min_sp'], sp)
                    if pc == IRQ_ENTRY:
                        assert '_irq_before' not in pending
                        pending['_irq_before'] = ref.ram_cpu()
                        pending['_irq_sp'], pending['_irq_pc'] = sp, raw_pc
                        return
                    if pc in IRQ_EXITS:
                        assert '_irq_before' in pending
                        pending['external_events'].append({'kind': 'irq', 'entry_pc': hex(IRQ_ENTRY),
                            'entry_raw_pc': hex(pending.pop('_irq_pc')), 'exit_raw_pc': hex(raw_pc),
                            'exit_pc': hex(pc), 'entry_sp': pending.pop('_irq_sp'), 'exit_sp': sp,
                            'changes': byte_changes(pending.pop('_irq_before'), ref.ram_cpu())})
                        irq_count += 1
                        return
                    if pc in SOUND_CONSUMERS:
                        offset = ref.get_register(9) & 65535
                        pending['external_events'].append({'kind': 'sound-consumer', 'pc': hex(pc),
                            'address': offset, 'before': ref.ram_cpu()[offset], 'after': 0, 'sp': sp,
                            'sr': ref.get_register(17)})
                        sound_count += 1
                        return
                if pc in ENTRIES:
                    if ref.frame > ENTRY_LIMIT:
                        return
                    assert pending is None, 'overlapping source7 selector'
                    ram = ref.ram_cpu()
                    assert address & 65535 == 0x8400 and ram[0x8458] == 7
                    ret = int.from_bytes(ram[sp & 65535:(sp & 65535)+4], 'big')
                    assert ret in NATURAL_RETURNS, hex(ret)
                    pending = {'kind': ENTRIES[pc], 'name': f'natural-frame-{ref.frame}',
                        'object_id': address & 65535, 'source_character_id': 7,
                        'source_register': hex(ref.get_register(13)), 'entry_pc': hex(pc),
                        'entry_sp': sp, 'return_pc': hex(ret), 'frame': ref.frame,
                        'before': snapshot(ram, address, rom), '_ram_before': ram,
                        'entry_sr': ref.get_register(17), 'min_sp': sp, 'external_events': [],
                        'constructors': [], 'rng_calls': 0, 'context': 'natural-real-menu-special'}
                elif pending is not None and pc == 0x3982:
                    pending['rng_calls'] += 1
                    rng_count += 1
                elif pending is not None and pc in CONSTRUCTORS:
                    pending['constructors'].append(hex(pc))
                elif pending is not None and pc == int(pending['return_pc'], 16):
                    assert sp == pending['entry_sp']+4 and address & 65535 == pending['object_id']
                    assert '_irq_before' not in pending
                    ram = ref.ram_cpu()
                    pending.update(after=snapshot(ram, address, rom), exit=hex(pc), exit_sp=sp, exit_frame=ref.frame)
                    pending['ram_audit'] = audit_ram(pending, pending.pop('_ram_before'), ram)
                    before, after = bytes.fromhex(pending['before']['actor_bytes']), bytes.fromhex(pending['after']['actor_bytes'])
                    assert not [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in ACTOR_WRITES]
                    assert pending['before']['children_bytes'] == pending['after']['children_bytes']
                    assert pending['before']['energy_fraction'] == pending['after']['energy_fraction']
                    assert pending['before']['random_seed'] == pending['after']['random_seed']
                    assert pending['rng_calls'] == 0
                    result = successful_root(pending)
                    if result:
                        label, detail = result
                        pending['successful_root'] = label
                        successes[label].append(detail)
                    rows.add(pending)
                    pending = None
            except Exception as exc:
                if len(errors) < 10:
                    errors.append({'frame': ref.frame, 'raw_pc': hex(raw_pc), 'error': repr(exc)})

        rom = ref.rom_path.read_bytes()
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        frames = [ref.step(frame, buttons_at(events, frame))['reference'] for frame in range(1, FRAME_COUNT+1)]
        hook(None)
        identity = ref.identity
    rows.close()
    diagnostic = {'frames_match': frames == expected, 'pending': bool(pending), 'errors': errors,
                  'success_counts': {label: len(values) for label, values in successes.items()}}
    (out/'capture-diagnostic.json').write_text(json.dumps(diagnostic, indent=2)+'\n')
    assert frames == expected and pending is None and not errors, diagnostic
    assert all(successes.values()), diagnostic
    assert all(any(detail['discovery_frame_matches'] for detail in values) for values in successes.values())
    assert verify_frozen() == frozen_sha
    name = 'fighter7-attacks-special-natural'
    fixture, digest = out/(name+'.json.gz'), hashlib.sha256()
    with fixture.open('wb') as file:
        with gzip.GzipFile(fileobj=file, mode='wb', mtime=0) as zipped:
            def emit(value):
                digest.update(value)
                zipped.write(value)
            emit(b'{"calls":[')
            with rows.path.open('rb') as source:
                for index, line in enumerate(source):
                    if index:
                        emit(b',')
                    emit(line.rstrip(b'\n'))
            emit(b']}\n')
    dependencies = ['scripts/yuyuhakusho/capture_fighter7_attacks.py', *DEPENDENCIES]
    report = {
        'scope': 'Cold-hook natural complete source7 selectors from Root-owned 5502-frame original control',
        'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
        'capture_dependencies': [{'path': path, 'sha256': sha((ROOT/path).read_bytes())} for path in dependencies],
        'reference_manifest': {'path': str(control_path.relative_to(ROOT)), 'sha256': sha(control_path.read_bytes())},
        'input': control['input'], 'baseline': control['output'] | {'frames': FRAME_COUNT},
        'frames_match_original_core': True, 'natural_entry_limit': ENTRY_LIMIT, 'drain_frames': 2,
        'records': rows.count, 'calls': rows.count, 'kind_counts': dict(rows.kinds),
        'constructor_counts': dict(rows.constructors), 'source_objects': dict(rows.sources),
        'successful_root_counts': {label: len(values) for label, values in successes.items()},
        'successful_roots': successes, 'player_roots_verified': 14, 'r4_natural_claimed': False,
        'irq_spans': irq_count, 'sound_consumer_markers': sound_count, 'rng_calls': rng_count,
        'full_cpu_ram_audited': True, 'cpu_ram_bytes': 65536, 'unexplained_ram_writes': 0,
        'fixture_sha256': digest.hexdigest(), 'gzip_sha256': sha(fixture.read_bytes()),
        'frozen_old_evidence_sha256': frozen_sha, 'frozen_old_files_unchanged': True,
        'incomplete_calls': 0, 'errors': [], 'pass': True,
        'limits': ['Selector returns only; children, healing, motion, collision and World have separate acceptance',
                   'R4 has no player command binding and is not claimed as natural coverage',
                   'No game RAM/register/cycle writes, private state, restore or alternate original control']}
    (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({key: value for key, value in report.items() if key not in ['capture_dependencies', 'successful_roots', 'limits']}, indent=2))


if __name__ == '__main__':
    main()
