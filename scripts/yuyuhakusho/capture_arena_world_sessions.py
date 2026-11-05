#!/usr/bin/env python3
"""Cold whole-world passes against the seven existing unique original controls.

Only the hook core is executed. This producer never creates or replaces an
original control, restores a checkpoint, writes emulated RAM, or invokes a
private callback. All output stays in the selected worktree.
"""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_world import number, snapshot as base_snapshot

CALLBACKS = {2: 0x20ae0, 4: 0x20cb2, 6: 0x20e28, 8: 0x20fb4,
             10: 0x21126, 12: 0x212f8, 14: 0x21412}
ORIGINAL_SHA = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
HOOK_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'


def binding(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path.read_bytes())}


def snapshot(ram):
    result = base_snapshot(ram)
    result['backdrop_color'] = ram[0xf6df]
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--arena', type=lambda value: int(value, 16), required=True, choices=CALLBACKS)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    label = f'{args.arena:x}'
    ip = ROOT/f'docs/research/md-yuyuhakusho/arena-{label}-inputs.json'
    rp = ROOT/f'games/yuyuhakusho/engine/testdata/arena-{label}-reference-frames.json'
    mp = ROOT/f'games/yuyuhakusho/engine/testdata/arena-{label}-reference-manifest.json'
    manifest = json.loads(mp.read_bytes())
    assert manifest['identity']['core_sha256'] == ORIGINAL_SHA
    assert manifest['input'] == binding(ip) and manifest['output'] == binding(rp)
    assert manifest['frames'] == 3602 and manifest['cold_replay_verified']
    events, expected = json.loads(ip.read_bytes()), json.loads(rp.read_bytes())
    assert len(expected) == 3602
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge = C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = [0x3592, 0x35a2, 0x35a4, 0x3a2e, 0x3a6a, 0x37c56, 0x37cac, 0x3982, 0x14d0]
    pcs = (C.c_uint*len(points))(*points)
    pending, rows, errors, driver_entries = None, [], [], []
    with ReferenceSession(ROOT, out/'observed-core'/f'arena-{label}', core=core) as ref:
        assert ref.identity['core_sha256'] == HOOK_SHA

        @cbtype
        def observe(kind, width, raw_pc, value):
            nonlocal pending
            try:
                pc = raw_pc & 0xffffff
                if kind != 1 or not 2700 <= ref.frame <= 3602:
                    return
                sp = ref.get_register(15)
                if pc == 0x3a2e:
                    if ref.frame <= 3600:
                        driver_entries.append(ref.frame)
                elif pc == 0x3592 and ref.frame <= 3600:
                    assert pending is None, 'overlapping object pass'
                    ram = ref.ram_cpu()
                    assert (number(ram, 0x8000, 4), number(ram, 0x8006, 2)) == (0xa01e, 32)
                    assert number(ram, 0x8040, 4) == 0xf2a4 and number(ram, 0x8046, 2) == 14
                    assert number(ram, 0xa060, 2) == args.arena
                    assert number(ram, 0x8180, 4) == CALLBACKS[args.arena]
                    pending = {'frame': ref.frame, 'calls': [], 'rng_calls': 0,
                               'queue_clears': [], 'raw_arena_sprites': [], 'stack': sp,
                               'driver_entries': list(driver_entries), 'before': snapshot(ram)}
                    driver_entries.clear()
                elif pending is not None and pc == 0x35a2:
                    ram = ref.ram_cpu()
                    address = ref.get_register(13) & 65535
                    call = {'object_id': address, 'behavior': number(ram, address, 4),
                            'size': number(ram, address+4, 2), 'stage': number(ram, address+6, 2),
                            'saved_stride': number(ram, sp & 65535, 2)}
                    assert call['saved_stride'] == call['size']
                    pending['calls'].append(call)
                elif pending is not None and pc == 0x3982:
                    pending['rng_calls'] += 1
                elif pending is not None and pc in (0x37c56, 0x37cac):
                    address = ref.get_register(9) & 65535
                    pending['queue_clears'].append({'pc': hex(pc), 'address': address, 'before': ref.ram_cpu()[address]})
                elif pending is not None and pc == 0x14d0 and ref.get_register(13) & 65535 == 0x9e00:
                    ram = ref.ram_cpu()
                    address = ref.get_register(8) & 65535
                    assert ram[address+8:address+10] == b'\xff\xff'
                    pending['raw_arena_sprites'].append(dict(zip(('Y', 'SizeLink', 'Tile', 'X'),
                        [number(ram, address+i, 2) for i in range(0, 8, 2)])))
                elif pending is not None and pc == 0x3a6a:
                    assert sp == pending['stack']+4, 'whole-pass return stack mismatch'
                    ram = ref.ram_cpu()
                    pending['after'] = snapshot(ram)
                    pending['cleared_world'] = number(ram, 0x8040, 4) == 0
                    assert not pending['cleared_world']
                    assert len(pending['raw_arena_sprites']) == (6 if args.arena == 10 else 0)
                    rows.append(pending)
                    pending = None
            except Exception as exc:
                errors.append(str(exc))

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        for frame in range(1, 3603):
            actual = ref.step(frame, buttons_at(events, frame))['reference']
            assert actual == expected[frame-1], ('original control diverged', frame)
        hook(None)
        identity = ref.identity
    assert rows and rows[0]['frame'] == 2700 and rows[-1]['frame'] == 3600
    assert not pending and not errors, (errors[:10], bool(pending))
    raw = (json.dumps(rows, separators=(',', ':'))+'\n').encode()
    packed = gzip.compress(raw, mtime=0)
    name = f'arena-world-{label}'
    (out/(name+'.json.gz')).write_bytes(packed)
    counts = Counter(call['behavior'] for row in rows for call in row['calls'])
    report = {'scope': 'Cold actual 3592 practice-ready whole-object passes against unique supplied original3602 control',
              'arena': args.arena, 'identity': identity, 'capture': binding(Path(__file__).resolve()),
              'inputs': binding(ip), 'control': binding(rp), 'control_manifest': binding(mp),
              'frames_checked': 3602, 'frames_match_original_control': True,
              'checkpoint_restore': False, 'emulated_ram_writes': False, 'direct_cpu_calls': False,
              'entry_frame_limit': 3600, 'drain_frames': 2, 'first_frame': rows[0]['frame'],
              'last_frame': rows[-1]['frame'], 'records': len(rows),
              'object_calls': sum(counts.values()), 'callbacks': {str(k): v for k, v in sorted(counts.items())},
              'raw_arena_sprites': sum(len(row['raw_arena_sprites']) for row in rows),
              'fixture_sha256': sha(raw), 'gzip_sha256': sha(packed), 'incomplete_calls': 0,
              'errors': errors, 'pass': True}
    (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2), flush=True)


if __name__ == '__main__':
    main()
