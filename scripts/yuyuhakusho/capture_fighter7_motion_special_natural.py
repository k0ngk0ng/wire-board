#!/usr/bin/env python3
"""Additional natural MAIN evidence under the single frozen 5502-frame control."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path

from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_ground_idle import actor as body_state
from capture_effect_pool import slot_index
from capture_fighter7_motion import ENTRIES, PARENTS, n, changes

INPUT_SHA = '5354fe1e66a6875483398763ce7644c363682e864537f827786fd42b0fab05fd'
REFERENCE_SHA = '98a17dffa4906b63a6ad2e309c7cfb402ca38e0a46fdfce7a5b0bd81456d55dd'
HOOK_CORE_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, default=ROOT / 'games/yuyuhakusho/engine/testdata')
    parser.add_argument('--inputs', type=Path, default=ROOT / 'docs/research/md-yuyuhakusho/source7-special-discovery-inputs.json')
    parser.add_argument('--reference', type=Path, default=ROOT / 'games/yuyuhakusho/engine/testdata/fighter7-attacks-special-reference-frames.json')
    parser.add_argument('--reference-manifest', type=Path, default=ROOT / 'games/yuyuhakusho/engine/testdata/fighter7-attacks-special-reference-manifest.json')
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    input_raw, reference_raw, control_raw = args.inputs.read_bytes(), args.reference.read_bytes(), args.reference_manifest.read_bytes()
    events, expected, control = json.loads(input_raw), json.loads(reference_raw), json.loads(control_raw)
    assert sha(input_raw) == INPUT_SHA and sha(reference_raw) == REFERENCE_SHA
    assert control['input']['sha256'] == INPUT_SHA and control['output']['sha256'] == REFERENCE_SHA
    assert control['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    assert not control['observer_attached'] and len(expected) == 5502 and control['entry_frame_limit'] == 5500
    engine = ROOT / 'games/yuyuhakusho/engine'
    old_freeze_path = engine / 'testdata/fighter7-motion-test-first.json'
    old_freeze_raw = old_freeze_path.read_bytes()
    frozen = json.loads(old_freeze_raw)
    for path, digest in frozen['files'].items():
        assert sha((ROOT / path).read_bytes()) == digest, path
    original_go_sha = sha((engine / 'fighter7_motion.go').read_bytes())
    assert original_go_sha == 'cfb8334a40adb24ceaf5232f09cf06a14fdc948c83f3e71308cf0cf746c26004'
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x15afc, 0x1fae6, 0x1faee, 0x3982, 0x37c56, 0x37cac,
              0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint * len(points))(*points)
    rows, errors, pending, frame2400 = [], [], None, None

    def snapshot(ram, parent):
        obj = ram[parent:parent + 256]
        pointer = n(obj, 0x90, 4) & 0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer & 65535)
        return {'object_id': parent, 'character_id': obj[0x58], 'body': body_state(obj), 'actor_bytes': obj.hex(),
                'controls': {'held': obj[0x71] | obj[0xc0] << 8, 'pressed': obj[0x72] | obj[0xc1] << 8,
                             'changed': obj[0x73] | obj[0xc2] << 8, 'raw_held': obj[0x74], 'retain_ticks': obj[0x75]},
                'energy': n(obj, 0xb8, 4, True), 'power': n(obj, 0x7c, 2, True),
                'alternate_velocity': n(obj, 0x48, 4, True), 'contact_flags': obj[0x89],
                'children': ram[parent + 256:parent + 1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()}, 'left': n(ram, 0xa010, 2, True),
                'others': {str(p): ram[p:p + 256].hex() for p in PARENTS if p != parent},
                'movement_source': n(obj, 0x90, 4), 'animation_set': n(obj, 0x50, 4), 'shape_set': n(obj, 0x8c, 4),
                'parameters': {key: n(memory, index + delta, 4, True) for key, delta in
                               [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'random_seed': n(ram, 0xff18, 4), 'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e])}

    with ReferenceSession(ROOT, ROOT / '.local/md-yuyuhakusho/source7-motion-special-observed', core=core) as ref:
        assert ref.identity['core_sha256'] == HOOK_CORE_SHA

        @cbtype
        def observe(kind, width, raw_pc, value):
            nonlocal pending
            try:
                pc = raw_pc & 0xffffff
                if pc == 0xfff000:
                    if pending is not None:
                        irq = {'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()}
                        if not pending['_irq']:
                            pending['_events'].append(irq)
                        pending['_irq'].append(irq)
                    return
                if pc in (0xfff09a, 0xfff0a8, 0xfff13a):
                    if pending is not None and pending['_irq']:
                        irq = pending['_irq'].pop()
                        irq.update(exit=hex(pc), changes=changes(irq.pop('_ram'), ref.ram_cpu()))
                        pending['interrupts'].append(irq)
                    return
                if pc == 0x3982:
                    if pending is not None:
                        pending['rng_calls'] += 1
                    return
                if pc in (0x37c56, 0x37cac):
                    if pending is not None:
                        address = ref.get_register(9) & 65535
                        event = {'pc': hex(pc), 'address': address, 'before': ref.ram_cpu()[address], 'after': 0}
                        pending['external_queue_clears'].append(event)
                        if not pending['_irq']:
                            pending['_events'].append({'kind': 'audio', 'changes': [{'offset': address, 'before': event['before'], 'after': 0}]})
                    return
                parent, sp = ref.get_register(13) & 65535, ref.get_register(15)
                ram, role = ref.ram_cpu(), ENTRIES.get(pc)
                if role and parent in PARENTS and ram[parent + 0x58] == 7 and ref.frame <= 5500:
                    assert pending is None and ram[parent + 0x5d] == int(role, 16)
                    pending = {'kind': role, 'frame': ref.frame, 'object_id': parent, 'entry': hex(pc), 'entry_sp': sp,
                               'source_register': hex(ref.get_register(13)), 'before': snapshot(ram, parent), 'allocations': [],
                               'rng_calls': 0, 'interrupts': [], 'external_queue_clears': [], '_irq': [], '_events': [], '_ram': ram}
                elif pending is not None:
                    if pc in (0x1fae6, 0x1faee):
                        pending['allocations'].append(slot_index(ref.get_register(8)))
                    if pc == 0x15afc:
                        row, pending = pending, None
                        assert parent == row['object_id'] and sp == row['entry_sp'] and not row.pop('_irq')
                        row.update(after=snapshot(ram, parent), exit=hex(pc), exit_frame=ref.frame, return_sp=sp)
                        row['ram_changes'] = changes(row.pop('_ram'), ram)
                        owned = (set(range(parent, parent + 1024)) | set(range(0x9400, 0x9c00)) |
                                 {0xa036, 0xa037} | {(sp - i) & 65535 for i in range(0x180)})
                        if role == '3d' or row['kind'] == '3d':
                            owned |= {p + 0x69 for p in PARENTS if p != parent}
                        row['external_changes'] = [d for d in row['ram_changes'] if d['offset'] not in owned]
                        row['unexplained_changes'] = []
                        external_events = row.pop('_events')
                        row['external_event_chain'] = external_events
                        for delta in row['external_changes']:
                            current, count = delta['before'], 0
                            for event in external_events:
                                for change in event['changes']:
                                    if change['offset'] == delta['offset']:
                                        assert current == change['before'], ('broken external RAM chain', delta, event)
                                        current, count = change['after'], count + 1
                            if count == 0 or current != delta['after']:
                                row['unexplained_changes'].append(delta)
                        assert not row['unexplained_changes'], row['unexplained_changes'][:8]
                        assert row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                        rows.append(row)
            except Exception as exc:
                errors.append(repr(exc))

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        frames = []
        for frame in range(1, 5503):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                ram = ref.ram_cpu()
                assert ram[0x8458] == 7
                frame2400 = {'source_tag': ram[0x8458], 'object_id': 0x8400, 'animation_set': n(ram, 0x8450, 4),
                             'shape_set': n(ram, 0x848c, 4), 'movement_source': n(ram, 0x8490, 4),
                             'header_behavior': hex(n(ram, 0x8000, 4)), 'header_stage': n(ram, 0x8006)}
        hook(None)
        assert frames == expected and pending is None and not errors, (errors[:8], len(rows))
        identity = ref.identity

    assert old_freeze_path.read_bytes() == old_freeze_raw
    for path, digest in frozen['files'].items():
        assert sha((ROOT / path).read_bytes()) == digest, path
    assert sha((engine / 'fighter7_motion.go').read_bytes()) == original_go_sha
    raw = (json.dumps({'library': [], 'calls': rows}, separators=(',', ':')) + '\n').encode()
    zipped = gzip.compress(raw, mtime=0)
    name = 'fighter7-motion-special-oracle'
    (out / (name + '.json.gz')).write_bytes(zipped)
    kinds = Counter(row['kind'] for row in rows)
    report = {'pass': bool(rows) and not errors, 'scope': 'Additional natural source7 MAIN pre-tail phases; raw CPU evidence, not a new Go unit-test acceptance',
              'identity': identity, 'binding': frame2400, 'capture_sha256': sha(Path(__file__).read_bytes()),
              'frozen_original_capture_sha256': sha((ROOT / 'scripts/yuyuhakusho/capture_fighter7_motion.py').read_bytes()),
              'filter_sha256': sha(bridge_path.read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped),
              'records': len(rows), 'calls': len(rows), 'kind_counts': dict(kinds),
              'kind_windows': {kind: {'first': min(row['frame'] for row in rows if row['kind'] == kind),
                                     'last': max(row['frame'] for row in rows if row['kind'] == kind)} for kind in kinds},
              'frames_match_original_core': frames == expected, 'incomplete_calls': 0, 'errors': errors,
              'rng_calls': sum(row['rng_calls'] for row in rows),
              'external_queue_calls': sum(bool(row['external_queue_clears']) for row in rows),
              'external_change_calls': sum(bool(row['external_changes']) for row in rows),
              'unexplained_change_calls': sum(bool(row['unexplained_changes']) for row in rows),
              'interrupt_calls': sum(len(row['interrupts']) for row in rows), 'full_ram_delta_calls': len(rows),
              'original_frozen_files_unchanged': True, 'original_go_sha256': original_go_sha,
              'original_test_freeze_sha256': sha(old_freeze_raw),
              'baselines': [{'input': str(args.inputs.relative_to(ROOT)), 'input_sha256': INPUT_SHA,
                             'reference': str(args.reference.relative_to(ROOT)), 'reference_raw_sha256': REFERENCE_SHA,
                             'reference_manifest': str(args.reference_manifest.relative_to(ROOT)), 'reference_manifest_sha256': sha(control_raw),
                             'video_steps': 5502, 'entry_frame_limit': 5500, 'all_ram_video_match': True}],
              'limits': ['No shared tail, active children/FX or complete actor/world execution is reproduced by this phase collector.',
                         'Natural source7 integration is validated by the separate complete composition/world scope.',
                         'No frozen MAIN implementation, test, old collector or existing fixture was changed.',
                         'IRQ and audio changes retain exact ordered event chains; history RAM has no exemption.']}
    (out / (name + '-manifest.json')).write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2), flush=True)


if __name__ == '__main__':
    main()
