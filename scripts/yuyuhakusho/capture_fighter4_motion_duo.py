#!/usr/bin/env python3
"""Observe source4's real9000 swallow/boost phases against the shared5502 control."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_ground_idle import actor as body_state
from capture_effect_pool import slot_index

ENTRIES = {0x1b3f0: 0x2a, 0x1b48a: 0x2b}
INPUT = 'docs/research/md-yuyuhakusho/source4-special-discovery-duo-inputs.json'
CONTROL = 'games/yuyuhakusho/engine/testdata/fighter4-attacks-duo-reference-frames.json'
CONTROL_MANIFEST = 'games/yuyuhakusho/engine/testdata/fighter4-attacks-duo-reference-manifest.json'


def n(raw, offset, size=2, signed=False):
    return int.from_bytes(raw[offset:offset+size], 'big', signed=signed)


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--out', type=Path, required=True); args = parser.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    events_raw = (ROOT/INPUT).read_bytes(); control_raw = (ROOT/CONTROL).read_bytes(); manifest_raw = (ROOT/CONTROL_MANIFEST).read_bytes()
    manifest = json.loads(manifest_raw); expected = json.loads(control_raw); events = json.loads(events_raw)
    assert sha(events_raw) == 'b303ca8622f90747052902f81b2b7f2bdde39f59e47feb6f36a6bf8287a6ac34'
    assert sha(control_raw) == '9f282f53a016d184d45052b30eaf8c482d4b568132f7979f7eebff4a7faa5917'
    assert manifest['input']['path'] == INPUT and manifest['input']['sha256'] == sha(events_raw)
    assert manifest['output']['path'] == CONTROL and manifest['output']['sha256'] == sha(control_raw)
    assert manifest['observer_attached'] is False and manifest['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    assert manifest['frames'] == len(expected) == 5502 and manifest['entry_frame_limit'] == 5500 and manifest['drain_frames'] == 2
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x15afc, 0x1fae6, 0x1faee, 0x3982, 0x37c56, 0x37cac, 0x13fc8]
    pcs = (C.c_uint*len(points))(*points)
    calls, errors, scene_runs = [], [], []; pending = None; child_calls = 0; child_first = None; child_last = None

    def snapshot(ram):
        obj = ram[0x9000:0x9100]; source = n(obj, 0x90, 4); pointer = source&0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer&65535)
        return {'object_id': 0x9000, 'character_id': obj[0x58], 'input_channel': obj[0x76],
                'body': body_state(obj), 'actor_bytes': obj.hex(),
                'controls': {'held': obj[0x71]|obj[0xc0]<<8, 'pressed': obj[0x72]|obj[0xc1]<<8,
                             'changed': obj[0x73]|obj[0xc2]<<8, 'raw_held': obj[0x74], 'retain_ticks': obj[0x75]},
                'energy': n(obj, 0xb8, 4, True), 'power': n(obj, 0x7c, 2, True),
                'alternate_velocity': n(obj, 0x48, 4, True), 'contact_flags': obj[0x89],
                'children': ram[0x9100:0x9400].hex(), 'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'movement_source': source, 'dragon_bank_cursor': n(ram, 0xa038), 'pending_sound': ram[0xa030],
                'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'animation_set': n(obj, 0x50, 4), 'shape_set': n(obj, 0x8c, 4),
                'parameters': {key: n(memory, index+delta, 4, True) for key, delta in
                               [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]}}

    with ReferenceSession(ROOT, out/'observed', core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending, child_calls, child_first, child_last
            try:
                if pc == 0x3982:
                    if pending is not None: pending['rng_calls'] += 1
                    return
                if pc in [0x37c56, 0x37cac]:
                    if pending is not None:
                        address = ref.get_register(9)&65535
                        pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(address), 'before': ref.ram_cpu()[address]})
                    return
                address = ref.get_register(13)&0xffffff
                if pc == 0x13fc8:
                    if address == 0xff9100:
                        child_calls += 1; child_last = ref.frame
                        if child_first is None: child_first = ref.frame
                    return
                if pc in ENTRIES and address == 0xff9000 and ref.frame <= 5500:
                    ram = ref.ram_cpu(); assert ram[0x9058] == 4 and ram[0x9076] == 1
                    assert pending is None and ref.get_register(13) == 0xffff9000
                    pending = {'kind': ENTRIES[pc], 'frame': ref.frame, 'scenario': Path(INPUT).name,
                               'source_register': hex(ref.get_register(13)), 'entry_sp': ref.get_register(15),
                               'before': snapshot(ram), 'allocations': [], 'rng_calls': 0, 'external_queue_clears': [], '_ram': ram}
                elif pending is not None:
                    if pc in [0x1fae6, 0x1faee]: pending['allocations'].append(slot_index(ref.get_register(8)))
                    elif pc == 0x15afc:
                        assert ref.get_register(13) == 0xffff9000 and ref.get_register(15) == pending['entry_sp']
                        row = pending; pending = None; before = row.pop('_ram'); after = ref.ram_cpu()
                        row.update(after=snapshot(after), exit='0x15afc', exit_frame=ref.frame)
                        low_sp = row['entry_sp']&65535
                        allowed = set(range(0x9000, 0x9c00))|{0xa036, 0xa037}|set(range(max(0, low_sp-256), min(65536, low_sp+4)))
                        allowed |= {int(clear['address'], 16) for clear in row['external_queue_clears']}
                        row['unexpected_ram_changes'] = [{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
                        row['history_before_sha256'], row['history_after_sha256'] = sha(before[0x7000:0x7800]), sha(after[0x7000:0x7800])
                        row['before_ram_sha256'], row['after_ram_sha256'] = sha(before), sha(after)
                        assert row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                        assert row['before']['pending_sound'] == row['after']['pending_sound'] and row['before']['dragon_bank_cursor'] == row['after']['dragon_bank_cursor']
                        if row['before']['sound_queue'] != row['after']['sound_queue']: assert row['external_queue_clears']
                        assert row['history_before_sha256'] == row['history_after_sha256']
                        calls.append(row)
            except Exception as error:
                errors.append(repr(error))
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p)); actual = []
        for frame in range(1, 5503):
            actual.append(ref.step(frame, buttons_at(events, frame))['reference'])
            ram = ref.ram_cpu()
            if frame == 2400:
                assert ram[0x8458] == 3 and ram[0x8476] == 0 and ram[0x9058] == 4 and ram[0x9076] == 1
            if ram[0x9058] == 4 and n(ram, 0x9000, 4) == 0x12668:
                state = {'action': ram[0x905d], 'pose': ram[0x905f], 'movement_source': n(ram, 0x9090, 4),
                         'energy_high': n(ram, 0x90b8, 2, True), 'health': n(ram, 0x9078, 2, True),
                         'child_behavior': n(ram, 0x9100, 4), 'child_size': n(ram, 0x9104)}
                if scene_runs and scene_runs[-1]['state'] == state: scene_runs[-1]['last_frame'] = frame
                else: scene_runs.append({'first_frame': frame, 'last_frame': frame, 'state': state})
        hook(None); assert actual == expected and pending is None and not errors, (errors, pending is not None)
        final = snapshot(ref.ram_cpu()); identity = ref.identity
    assert final['movement_source'] == 0x12578 and final['energy']>>16 == -1 and final['body']['action_kind'] == 0
    assert n(bytes.fromhex(final['children']), 4) == 0 and child_first == 3598
    assert any(row['kind'] == 0x2b for row in calls)
    unknown = sum(len(row['unexpected_ram_changes']) for row in calls)
    raw = (json.dumps({'calls': calls, 'scene_runs': scene_runs, 'final': final,
                       'child13fc8': {'first_frame': child_first, 'last_frame': child_last, 'calls': child_calls}}, separators=(',', ':'))+'\n').encode()
    compressed = gzip.compress(raw, mtime=0); name = 'fighter4-motion-duo-natural'
    (out/(name+'.json.gz')).write_bytes(compressed)
    report = {'pass': unknown == 0, 'scope': 'Actual two-human9000 source4 swallow/boost MAIN pre-tail evidence; no independent Go acceptance claim',
              'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
              'input': {'path': INPUT, 'sha256': sha(events_raw)}, 'reference': {'path': CONTROL, 'sha256': sha(control_raw),
                  'manifest': CONTROL_MANIFEST, 'manifest_sha256': sha(manifest_raw), 'new_unobserved_control_captured': False},
              'frames': 5502, 'entry_frame_limit': 5500, 'drain_frames': 2, 'frames_match_original_core': True,
              'actor': {'object_id': 0x9000, 'source_register': '0xffff9000', 'source_tag': 4, 'input_channel': 1, 'child_id': 0x9100,
                        'other_object_id': 0x8400, 'other_source_tag': 3, 'other_input_channel': 0},
              'records': len(calls), 'calls': len(calls), 'kind_counts': dict(Counter(str(row['kind']) for row in calls)),
              'allocation_attempts': sum(len(row['allocations']) for row in calls), 'rng_calls': sum(row['rng_calls'] for row in calls),
              'external_queue_calls': sum(bool(row['external_queue_clears']) for row in calls), 'unexpected_ram_changes': unknown,
              'history_phase_writes': 0, 'SP_restored': True, 'incomplete_calls': 0, 'errors': errors,
              'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed),
              'final_original_scene': {'movement_source': final['movement_source'], 'energy_high': final['energy']>>16,
                                       'action': final['body']['action_kind'], 'child_size': n(bytes.fromhex(final['children']), 4)},
              'child13fc8_first_frame': child_first,
              'limits': ['Actual9000 values are retained; original frozen8400 MAIN tests do not consume this fixture.',
                         'Whole-world/composition Duo E2E owns Go integration acceptance,child lifetimes and full boost expiry.',
                         'Final5502 scalar recovery is original CPU observation,not a MAIN-only Go replay.']}
    (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'name': name, 'records': len(calls), 'kinds': report['kind_counts'], 'pass': report['pass'], 'unexpected_ram_changes': unknown,
                      'child_first': child_first, 'final': report['final_original_scene']}))
    assert report['pass'], [(row['frame'], row['unexpected_ram_changes']) for row in calls if row['unexpected_ram_changes']]


if __name__ == '__main__': main()
