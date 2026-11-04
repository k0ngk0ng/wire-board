#!/usr/bin/env python3
"""Original world clock, pause and feedback helpers with full-window evidence."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ENTRIES = {0x3a76: 'video', 0x3a86: 'logic', 0x3a9c: 'toggle', 0x3ad6: 'pause', 0x39ac: 'shake'}
EXITS = {0x3a84, 0x3a9a, 0x3aac, 0x3ad4, 0x3aec, 0x3afc, 0x3a2c}
FIELDS = {'frame': (0xf584, 2), 'frame_delay': (0xf586, 2), 'logic_frame': (0xf588, 2), 'logic_delay': (0xf58a, 2),
          'pause_flags': (0xf550, 1), 'pause_mask': (0xf551, 1), 'controller_count': (0xf552, 2), 'audio_pause': (0xf807, 1),
          'slow_flag': (0xf55a, 1), 'screen_shake': (0xf566, 2), 'duration_request': (0xf568, 2), 'amplitude_request': (0xf56a, 2),
          'amplitude': (0xf56c, 2), 'decay': (0xf56e, 2), 'pan_amplitude': (0xf570, 2), 'pan_offset': (0xf572, 2)}


def snapshot(ram):
    state = {key: int.from_bytes(ram[off:off+n], 'big') for key, (off, n) in FIELDS.items()}
    state['pressed'] = list(ram[0xf798:0xf7a0])
    return {'state': state, 'window': ram[0xf550:0xf590].hex(), 'pressed': ram[0xf798:0xf7a0].hex(), 'audio_pause': ram[0xf807]}


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', required=True, type=Path); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set(ENTRIES)|EXITS); pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, streams, errors = [], [], [], []
    pending = {}; private = False; label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                sp = ref.get_register(15)
                if pc in ENTRIES:
                    assert not pending, 'Nested clock helper'
                    pending[sp] = {'kind': ENTRIES[pc], 'frame': ref.frame, 'before': snapshot(ref.ram_cpu())}
                elif pc in EXITS and sp in pending:
                    row = pending.pop(sp); row.update(after=snapshot(ref.ram_cpu()), exit=hex(pc))
                    if private:
                        row['name'] = label; isolated.append(row); ref.cpu.cycles = 4000000
                    else:
                        natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for f in range(1, 3601):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400: checkpoint = ref.save()
        hook(None); assert frames == expected and not pending and not errors, (errors, pending)
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)
        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b
        def setup(seed=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15): setreg(reg, 0)
            for key, (off, size) in FIELDS.items(): write(0xff0000+off, (seed*37+off*13)&((1 << (size*8))-1), size)
            write(0xfff552, 8, 2)
            for i in range(8): write(0xfff798+i, (seed*19+i*31)&255)
        def invoke(kind, name):
            nonlocal label
            label = name; setreg(17, 0x2700); setreg(15, 0xffff00); write(0xffff00, 0x3d8, 4)
            setreg(16, next(pc for pc, k in ENTRIES.items() if k == kind)); ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu(); count = len(isolated); hook(observer); execute(200000); hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (kind, name, errors)
            after = ref.ram_cpu(); allowed = set(range(0xf550, 0xf590)) | {0xf807} | set(range(0xfed0, 0xff04))
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (kind, name, unexpected)
            return isolated[-1]
        for kind in ['video', 'logic']:
            for seed, (counter, delay, pause) in enumerate(itertools.product([0, 1, 32767, 32768, 65535], [0, 1, 32768, 65535], [0, 1, 127, 128, 255])):
                setup(seed); off = 0xf584 if kind == 'video' else 0xf588
                write(0xff0000+off, counter, 2); write(0xff0000+off+2, delay, 2); write(0xfff550, pause)
                invoke(kind, f'{kind}-{counter}-{delay}-{pause}')
        for seed, (flags, mask, count, slot, pressed) in enumerate(itertools.product([0, 1, 64, 65, 128, 129, 192, 193, 255], [0, 1, 3, 128, 255], [0, 1, 4, 8], [0, 3, 7], [0, 127, 128, 255])):
            setup(seed); write(0xfff550, flags); write(0xfff551, mask); write(0xfff552, count, 2)
            for i in range(8): write(0xfff798+i, pressed if i == slot else 0)
            invoke('toggle', f'toggle-{flags}-{mask}-{count}-{slot}-{pressed}')
        for flags, audio in itertools.product(range(256), [0, 1, 128, 255]):
            setup(flags); write(0xfff550, flags); write(0xfff807, audio)
            invoke('pause', f'pause-{flags}-{audio}')
        for seed, (pause, slow, request, duration, amplitude, decay, odd) in enumerate(itertools.product([0, 128], [0, 1], [0, 1, 65535], [0, 1, 3, 65535], [0, 1, 256, 65535], [0, 1, 65535], [0, 1])):
            setup(seed)
            for off, value, size in [(0xf550, pause, 1), (0xf55a, slow, 1), (0xf56a, request, 2), (0xf568, duration, 2),
                                     (0xf56c, amplitude, 2), (0xf56e, decay, 2), (0xf588, odd, 2), (0xf570, [0, 1, 511, 65535][seed%4], 2)]:
                write(0xff0000+off, value, size)
            invoke('shake', f'shake-{pause}-{slow}-{request}-{duration}-{amplitude}-{decay}-{odd}')
        boundaries = len(isolated)
        for initial in range(4):
            setup(initial); write(0xfff550, 1); write(0xfff551, 255); write(0xfff552, 8, 2)
            calls = []
            for step in range(32):
                input_state = {'pressed': [128 if i == initial and step in [5, 15, 16] else 0 for i in range(8)],
                               'duration': 8 if step == 20 else None, 'amplitude': 1024 if step == 20 else None}
                for i, value in enumerate(input_state['pressed']): write(0xfff798+i, value)
                if input_state['duration'] is not None: write(0xfff568, input_state['duration'], 2)
                if input_state['amplitude'] is not None: write(0xfff56a, input_state['amplitude'], 2)
                for kind in ['video', 'toggle', 'pause', 'logic', 'shake']:
                    call = invoke(kind, f'stream-{initial}/{step}/{kind}')
                    if kind == 'video': call['external_input'] = input_state
                    calls.append(call)
            streams.append({'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity
    for name, rows in [('world-clock-oracle', natural), ('world-clock-boundaries', isolated[:boundaries]), ('world-clock-sequences', streams)]:
        raw = (json.dumps(rows, separators=(',', ':'))+'\n').encode(); zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        calls = [call for stream in rows for call in stream['calls']] if name.endswith('-sequences') else rows
        report = {'scope': 'Complete world counter/pause/feedback helper calls only', 'identity': identity, 'records': len(rows), 'calls': len(calls),
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped),
                  'frames_match_original_core': frames == expected, 'errors': errors, 'incomplete_calls': 0, 'pass': bool(rows)and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
