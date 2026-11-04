#!/usr/bin/env python3
"""Original pre-object frame and post-object palette stages."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_world_clock import snapshot as clock_snapshot, FIELDS


def controllers(ram):
    result = []
    for i in range(8):
        values = [ram[0xf780+i+j*8] for j in range(10)]
        result.append({name: values[j] | values[j+5]<<8 for j, name in enumerate(['current', 'previous', 'changed', 'pressed', 'released'])})
    return result


def snapshot(ram):
    word = lambda off: int.from_bytes(ram[off:off+2], 'big')
    return {'clock': clock_snapshot(ram), 'controllers': controllers(ram), 'seed': int.from_bytes(ram[0xff18:0xff1c], 'big'),
            'slow_gate': ram[0xf55b], 'reset_enabled': ram[0xff17],
            'palette': {'flags': ram[0xf500], 'status': ram[0xf501], 'mode': word(0xf502), 'step': word(0xf504), 'target': word(0xf506),
                        'current': [word(0xe000+i*2) for i in range(64)], 'primary': [word(0xe080+i*2) for i in range(64)],
                        'secondary': [word(0xe100+i*2) for i in range(64)]},
            'palette_registers': ram[0xf500:0xf508].hex(), 'palette_bytes': ram[0xe000:0xe180].hex()}


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', required=True, type=Path); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge = C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [0x3a2e, 0x3a42, 0x3a5a, 0x2300, 0x2f4a, 0x2f6e, 0x3372, 0x3390, 0x200, 0x3d8]
    pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    pinned = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    # A business call can straddle retro_run's video boundary. Stop accepting
    # new calls at3600, but drain existing calls under two extra baseline frames.
    with ReferenceSession(ROOT, out/'unobserved-tail') as control:
        expected = [control.step(f, buttons_at(events, f))['reference'] for f in range(1, 3603)]
    assert expected[:3600] == pinned
    natural, cases, streams, errors = [], [], [], []
    pending = None; private = False; label = ''; private_return = None; active_frame = None
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending, private_return, active_frame
            try:
                ram = ref.ram_cpu()
                if private and pc == 0x3d8:
                    private_return = ram; ref.cpu.cycles = 4000000; return
                if pc == 0x3a2e and not private:
                    active_frame = ref.frame if ref.frame <= 3600 else None
                    if active_frame is not None:
                        assert pending is None
                        pending = {'kind': 'begin', 'frame': active_frame, 'before': snapshot(ram), 'sampled': None, 'reset': False}
                elif pc == 0x3a42 and pending is not None and pending['kind'] == 'begin':
                    pending['sampled'] = controllers(ram)
                elif pc in [0x3a5a, 0x200] and pending is not None and pending['kind'] == 'begin':
                    pending.update(after=snapshot(ram), reset=pc == 0x200, exit_frame=ref.frame)
                    if private: pending['name'] = label; cases.append(pending); ref.cpu.cycles = 4000000
                    else: natural.append(pending)
                    pending = None
                elif pc == 0x2300 and not private and active_frame is not None:
                    sp = ref.get_register(15)&0xffff
                    caller = int.from_bytes(ram[sp:sp+4], 'big')
                    if caller in [0x2f3c, 0x3364]:
                        assert pending is None
                        pending = {'kind': 'finish', 'frame': active_frame, 'before': snapshot(ram), 'caller': hex(caller)}
                elif pc in [0x2f4a, 0x2f6e, 0x3372, 0x3390] and pending is not None and pending['kind'] == 'finish':
                    pending.update(after=snapshot(ram), exit_frame=ref.frame); natural.append(pending); pending = None; active_frame = None
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for f in range(1, 3603):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400: checkpoint = ref.save()
        hook(None); assert frames == expected and pending is None and not errors, (errors, pending)
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)
        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b
        def call(pc):
            nonlocal private_return
            private_return = None; setreg(17, 0x2700); setreg(15, 0xffff00); write(0xffff00, 0x3d8, 4)
            setreg(16, pc); ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896
            hook(observer); execute(200000); hook(None)
            assert private_return is not None and not errors, (hex(pc), errors)
        def setup(seed):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15): setreg(reg, 0)
            for name, (off, size) in FIELDS.items(): write(0xff0000+off, (seed*37+off*11)&((1 << (size*8))-1), size)
            write(0xfff552, [0, 1, 4, 8][seed%4], 2); write(0xfff55b, seed%2); write(0xffff17, seed%2)
            write(0xffff18, [0, 1, 0x12345678, 0xffffffff][seed%4], 4)
            for i in range(80): write(0xfff780+i, (seed*31+i*17)&255)
        def set_samples(samples):
            for channel, frame in enumerate(samples):
                for index, name in enumerate(['current', 'previous', 'changed', 'pressed', 'released']):
                    write(0xfff780+channel+index*8, frame[name]); write(0xfff7a8+channel+index*8, frame[name]>>8)
        def begin(name, sampled):
            nonlocal pending, label
            label = name
            row = {'kind': 'begin', 'name': name, 'before': snapshot(ref.ram_cpu()), 'sampled': None, 'reset': False}
            # Explicit private composition: original video helper followed by
            # the post-normalization block. No hardware bus/2c9e is simulated.
            call(0x3a76)
            ram = ref.ram_cpu()
            if ram[0xf55b] != 0 and ram[0xf55a] != 0:
                row['after'] = snapshot(ram); cases.append(row); return row
            set_samples(sampled); row['sampled'] = sampled; pending = row
            setreg(17, 0x2700); setreg(15, 0xffff00); write(0xffff00, 0x3d8, 4); setreg(16, 0x3a42)
            ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896; count = len(cases)
            hook(observer); execute(200000); hook(None)
            assert len(cases) == count+1 and pending is None and not errors, (name, errors)
            return cases[-1]
        def finish(name):
            row = {'kind': 'finish', 'name': name, 'before': snapshot(ref.ram_cpu())}
            call(0x2300)
            if ref.ram_cpu()[0xf500]&1: call(0x233c)
            row['after'] = snapshot(ref.ram_cpu()); cases.append(row); return row
        def samples(seed):
            result = []
            for i in range(8):
                held = [0, 128, 0x70, 0xf0][(i+seed)%4]
                previous = [0, 128, 0x70, 0xf0][(i+seed//4)%4]
                changed = held^previous
                result.append({'current': held, 'previous': previous, 'changed': changed, 'pressed': held&changed, 'released': previous&changed})
            return result
        for seed, (flags, slow, gate, reset, mask) in enumerate(itertools.product([0, 1, 65, 129, 193], [0, 1, 255], [0, 1], [0, 1], [0, 1, 15, 255])):
            setup(seed)
            for off, value in [(0xf550, flags), (0xf55a, slow), (0xf55b, gate), (0xff17, reset), (0xf551, mask)]: write(0xff0000+off, value)
            begin(f'begin-{flags}-{slow}-{gate}-{reset}-{mask}', samples(seed))
        for seed, (flags, status, mode, step, target) in enumerate(itertools.product([0, 1, 3], [0, 1, 2, 128, 129], [0, 0x100, 0x200, 0xe00, 0xf00], [0, 1, 0x100, 65535], [0, 0x300, 0xfff, 65535])):
            setup(seed)
            for off, value, size in [(0xf500, flags, 1), (0xf501, status, 1), (0xf502, mode, 2), (0xf504, step, 2), (0xf506, target, 2)]: write(0xff0000+off, value, size)
            for i in range(192): write(0xffe000+i*2, (seed*19+i*37)&0xffff, 2)
            finish(f'finish-{flags}-{status}-{mode}-{step}-{target}')
        boundary_count = len(cases)
        for stream in range(4):
            setup(stream); write(0xfff550, 1); write(0xfff552, 4, 2); write(0xfff55a, 0); write(0xfff55b, 1); write(0xffff17, 0)
            for off, value, size in [(0xf500, 1, 1), (0xf501, 128, 1), (0xf502, 0, 2), (0xf504, 0x100, 2), (0xf506, 0xfff, 2)]: write(0xff0000+off, value, size)
            calls = []
            for step in range(40):
                sample = samples(stream+step)
                if step in [10, 15]:
                    for i in range(4): sample[i] = {'current': 128, 'previous': 0, 'changed': 128, 'pressed': 128, 'released': 0}
                calls.extend([begin(f'stream-{stream}/{step}', sample), finish(f'stream-{stream}/{step}')])
            streams.append({'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity
    for name, rows in [('world-frame-oracle', natural), ('world-frame-boundaries', cases[:boundary_count]), ('world-frame-sequences', streams)]:
        raw = (json.dumps(rows, separators=(',', ':'))+'\n').encode(); zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        calls = [c for s in rows for c in s['calls']] if name.endswith('-sequences') else rows
        report = {'scope': 'Complete pre-object rules with trusted normalized-input boundary and post-object palette composition', 'identity': identity,
                  'records': len(rows), 'calls': len(calls), 'begin': sum(r['kind']=='begin' for r in calls), 'finish': sum(r['kind']=='finish' for r in calls),
                  'resets': sum(r.get('reset', False) for r in calls), 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'baseline_video_frames': len(frames), 'accepted_entry_last_frame': 3600,
                  'incomplete_calls': 0, 'errors': errors, 'pass': bool(rows)and not errors,
                  'limits': ['Hardware normalization2c9e and its F554 scratch cursor are outside this API', 'Private begins compose original video helper and post-normalization block, not hardware samples', 'Object pass/presentation/audio-driver timing are separate stages']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
