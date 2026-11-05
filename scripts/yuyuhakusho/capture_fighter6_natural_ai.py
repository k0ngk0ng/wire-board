#!/usr/bin/env python3
"""Passive real-menu CPU6 complete AI and air-special returns; no state edits."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_ai import snapshot, ALLOWED, BEHAVIORS
from capture_fighter_ai import LEAVES


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--inputs', required=True, type=Path)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--frames', type=int, default=3602)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and args.inputs.resolve().is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    events = json.loads(args.inputs.read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    returns = {0x15b18, 0x15b24, 0x15b30, 0x15b3c}
    points = sorted(set(BEHAVIORS+LEAVES+[0x1cc8a, 0x156ee, 0x3982, 0x1dc50, 0x176be, 0x194aa]) | returns)
    assert len(points) <= 64
    pcs = (C.c_uint*len(points))(*points)
    calls, air_calls, errors, frames = [], [], [], []
    pending, air_pending = None, None
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending, air_pending
            try:
                address = ref.get_register(13)&0xffff
                ram = ref.ram_cpu()
                computer6 = address in [0x8400, 0x8800, 0x8c00, 0x9000] and ram[address+0x58] == 6 and ram[address+0x76] == 255
                sp = ref.get_register(15)
                if pc == 0x1cc8a and computer6 and ref.frame <= args.frames-2:
                    assert pending is None
                    pending = {'character_id': 6, 'object_id': address, 'input_channel': 255,
                               'source_register': hex(ref.get_register(13)), 'frame': ref.frame,
                               'entry_sp': sp, 'before': snapshot(ram, address), 'paths': [],
                               'rng_calls': 0, 'odd_ccr': []}
                elif pending is not None:
                    if pc == 0x3982:
                        pending['rng_calls'] += 1
                    elif pc == 0x156ee:
                        assert computer6 and address == pending['object_id'] and sp == pending['entry_sp']+4
                        pending.update(after=snapshot(ram, address), exit=hex(pc), exit_sp=sp, exit_frame=ref.frame)
                        old, new = bytes.fromhex(pending['before']['actor']), bytes.fromhex(pending['after']['actor'])
                        assert not [i for i, (a, b) in enumerate(zip(old, new)) if a != b and i not in ALLOWED]
                        world = bytearray.fromhex(pending['before']['world'])
                        off = address-0x8400
                        world[off:off+256] = new
                        assert world.hex() == pending['after']['world']
                        calls.append(pending)
                        pending = None
                    else:
                        pending['paths'].append(hex(pc))
                if pc == 0x176be and computer6 and ref.frame <= args.frames-2:
                    assert air_pending is None
                    ret = int.from_bytes(ram[sp&65535:(sp&65535)+4], 'big')
                    assert ret in returns
                    air_pending = {'frame': ref.frame, 'object_id': address, 'entry_sp': sp,
                                   'return_pc': hex(ret), 'before': snapshot(ram, address), 'constructors': []}
                elif air_pending is not None:
                    if pc == 0x194aa:
                        air_pending['constructors'].append(hex(pc))
                    elif pc == int(air_pending['return_pc'], 16):
                        assert computer6 and address == air_pending['object_id'] and sp == air_pending['entry_sp']+4
                        air_pending.update(after=snapshot(ram, address), exit_sp=sp, exit_frame=ref.frame)
                        air_calls.append(air_pending)
                        air_pending = None
            except Exception as exc:
                errors.append(f'{ref.frame}/{pc:x}: {exc!r}')
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        action_counts, channel_counts = Counter(), Counter()
        for frame in range(1, args.frames+1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            ram = ref.ram_cpu()
            for a in [0x8400, 0x8800, 0x8c00, 0x9000]:
                if int.from_bytes(ram[a+4:a+6], 'big') == 256 and ram[a+0x58] == 6 and ram[a+0x76] == 255:
                    action_counts[hex(ram[a+0x5d])] += 1
                    channel_counts[hex(a)] += 1
        hook(None)
        identity = ref.identity
    assert not errors and pending is None and air_pending is None, errors[:5]
    outputs = []
    for name, rows in [('ai-calls', calls), ('air-special-calls', air_calls)]:
        raw = (json.dumps({'calls': rows}, separators=(',', ':'))+'\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        outputs.append({'name': name, 'records': len(rows), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped)})
    raw = (json.dumps(frames, separators=(',', ':'))+'\n').encode()
    (out/'observed-frames.json').write_bytes(raw)
    success = [r for r in air_calls if r['constructors'] and bytes.fromhex(r['after']['actor'])[0x5d] == 0x34]
    report = {'scope': 'Cold original physical practice menu CPU6; passive full AI and air-special SP+4 returns',
              'identity': identity, 'input': {'path': str(args.inputs.resolve().relative_to(ROOT)), 'sha256': sha(args.inputs.read_bytes())},
              'capture': {'path': str(Path(__file__).resolve().relative_to(ROOT)), 'sha256': sha(Path(__file__).read_bytes())},
              'frames': len(frames), 'entry_frame_limit': args.frames-2, 'drain_frames': 2,
              'complete_ai_returns': len(calls), 'complete_air_special_returns': len(air_calls),
              'source6_character_decisions': sum('0x1dc50' in r['paths'] for r in calls),
              'ai_rng_calls': sum(r['rng_calls'] for r in calls), 'computer6_objects': dict(channel_counts),
              'computer6_actions': dict(action_counts), 'natural34_complete_selections': len(success),
              'natural34_frames': [r['frame'] for r in success], 'outputs': outputs,
              'observed_frames_sha256': sha(raw), 'incomplete_calls': 0, 'errors': errors,
              'rom_ram_request_writes': False, 'restore_or_seed_injection': False,
              'unobserved_control_generated': False, 'pass': bool(calls) and not errors}
    (out/'report.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2), flush=True)


if __name__ == '__main__':
    main()
