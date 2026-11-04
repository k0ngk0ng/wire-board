#!/usr/bin/env python3
"""Original shared fighter input preparation and attack-value RNG tail."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ENTRIES = {0x15c36: 'input', 0x15bfe: 'random'}
EXITS = {0x15c9e: 'input', 0x15ca6: 'input', 0x15cae: 'input', 0x15c0e: 'random'}


def number(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, offset=0x8400):
    obj = ram[offset:offset+256]
    return {'input': {'energy': number(obj, 0xb8, 4, True), 'held': obj[0x71], 'pressed': obj[0x72],
                      'extended_pressed': obj[0xc1], 'facing': obj[0x61], 'charge_ticks': obj[0xc7], 'request': obj[0x6b]},
            'random': {'seed': number(ram, 0xff18, 4), 'scale': number(obj, 0xc4, 2), 'value': obj[0x65]},
            'actor_bytes': obj.hex()}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = list(ENTRIES)+list(EXITS)
    pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, streams, errors = [], [], [], []
    pending, private, sample = {}, False, None
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal sample
            try:
                actor, sp = ref.get_register(13)&0xffffff, ref.get_register(15)
                if private:
                    if pc in EXITS and sp == 0xffffe0:
                        assert sample is None
                        sample = (ref.ram_cpu(), pc)
                        ref.cpu.cycles = 2000000
                    return
                label = ENTRIES.get(pc, EXITS.get(pc))
                key = (label, actor, sp)
                ram = ref.ram_cpu()
                if pc in ENTRIES:
                    assert key not in pending
                    pending[key] = {'kind': label, 'frame': ref.frame, 'actor': actor, 'before': snapshot(ram, actor&0xffff)}
                else:
                    row = pending.pop(key)
                    row.update(after=snapshot(ram, actor&0xffff), exit=pc, exit_frame=ref.frame)
                    old = bytes.fromhex(row['before']['actor_bytes'])
                    new = bytes.fromhex(row['after']['actor_bytes'])
                    allowed = {0x6b, 0xc7} if label == 'input' else {0x65}
                    unknown = [i for i, (a, b) in enumerate(zip(old, new)) if a != b and i not in allowed]
                    assert not unknown, (label, unknown)
                    if label == 'input':
                        assert row['before']['random'] == row['after']['random']
                    else:
                        assert row['before']['input'] == row['after']['input']
                    natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 2000000
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for f in range(1, 3601):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not errors, (len(natural), errors, len(pending))
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def setup(seed=0x12345678):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for i in range(256):
                write(0xff8400+i, (i*73+17)&255)
            write(0xffff18, seed, 4)

        def input_fields(energy=222 << 16, held=0, pressed=0, extended_pressed=0, facing=0):
            values = {'energy': energy, 'held': held, 'pressed': pressed, 'extended_pressed': extended_pressed, 'facing': facing}
            for field, off, size in [('energy', 0xb8, 4), ('held', 0x71, 1), ('pressed', 0x72, 1), ('extended_pressed', 0xc1, 1), ('facing', 0x61, 1)]:
                write(0xff8400+off, values[field], size)
            return values

        def invoke(label, name):
            nonlocal sample
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff8400)
            setreg(16, 0x15c36 if label == 'input' else 0x15bfe)
            before = ref.ram_cpu()
            sample = None
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            hook(observer)
            execute(1000000)
            hook(None)
            assert sample is not None and not errors, (label, name, errors)
            after, pc = sample
            assert EXITS[pc] == label
            allowed = {0x846b, 0x84c7} if label == 'input' else {0x8465, 0xff18, 0xff19, 0xff1a, 0xff1b}
            unknown = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed and not 0xffb0 <= i < 0xffe4]
            assert not unknown, (label, unknown)
            return {'kind': label, 'name': name, 'before': snapshot(before), 'after': snapshot(after), 'exit': pc}

        for high, fraction, charge, held in itertools.product([-32768, -1, 0, 222, 223, 224, 32767], [0, 65535], [0, 1, 63, 64, 65, 127, 255], [0, 0x10, 0x40, 0x50, 0x20, 0x30, 0x60, 0x70, 255]):
            setup()
            input_fields((high << 16)|fraction, held, 0x10, 0, 2)
            write(0xff84c7, charge)
            write(0xff846b, 99)
            isolated.append(invoke('input', f'charge-{high}-{fraction}-{charge}-{held}'))
        for facing, extended, held, pressed in itertools.product([0, 1, 2, 3, 128, 255], range(256), [0, 0x20], [0, 0x10]):
            setup()
            input_fields(222 << 16, held, pressed, extended, facing)
            write(0xff84c7, 64)
            write(0xff846b, [0, 7, 8, 99][extended%4])
            isolated.append(invoke('input', f'request-{facing}-{extended}-{held}-{pressed}'))
        for held, pressed, facing in itertools.product(range(256), [0, 0x10, 0x40, 255], [0, 1]):
            setup()
            input_fields(222 << 16, held, pressed, 0, facing)
            write(0xff84c7, 65)
            write(0xff846b, 99)
            isolated.append(invoke('input', f'base-held-{held}-{pressed}-{facing}'))
        seeds = [0, 1, 0xffffffff, 0x12345678, 0x80000000, 0x7fffffff, 0xffff0000, 0xffff, 0x2a6d365a]
        scales = [0, 1, 2, 255, 256, 257, 0x7fff, 0x8000, 0xfffe, 0xffff]
        for seed, scale, value in itertools.product(seeds, scales, [0, 1, 127, 128, 254, 255]):
            setup(seed)
            write(0xff84c4, scale, 2)
            write(0xff8465, value)
            isolated.append(invoke('random', f'rng-{seed:x}-{scale}-{value}'))
        for scenario in range(3):
            setup()
            write(0xff84c7, 0)
            write(0xff846b, 99)
            initial = snapshot(ref.ram_cpu())
            calls = []
            for step in range(128):
                if scenario == 0:
                    inputs = input_fields(222 << 16, 0x10 if step < 100 else 0, 0, 0, 0)
                elif scenario == 1:
                    inputs = input_fields(((222+step%2) << 16)|0xffff, 0x40, 0, 0, 2)
                else:
                    inputs = input_fields(-1, 0x30, 0x10 if step%3 else 0, [0, 1, 4, 5][step%4], [0, 1, 2, 3, 128, 255][step%6])
                row = invoke('input', f'input-stream-{scenario}-{step}')
                row['external_input'] = inputs
                calls.append(row)
            streams.append({'kind': 'input', 'initial': initial, 'calls': calls})
        for seed in [0, 1, 0xffffffff, 0x80000000]:
            setup(seed)
            write(0xff8465, 255)
            initial = snapshot(ref.ram_cpu())
            calls = []
            for step in range(128):
                scale = scales[step%len(scales)]
                write(0xff84c4, scale, 2)
                row = invoke('random', f'random-stream-{seed:x}-{step}')
                row['external_scale'] = scale
                calls.append(row)
            streams.append({'kind': 'random', 'initial': initial, 'calls': calls})
        identity = ref.identity
    for name, records, scope in [('fighter-preparation-oracle', natural, 'Passive complete shared preparation and random-value calls'),
                                  ('fighter-preparation-boundaries', isolated, 'Declared original CPU input/randomization matrix'),
                                  ('fighter-preparation-sequences', streams, 'Single-initial-state original input and randomization streams')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(bridge_path.read_bytes()), 'records': len(records), 'fixture_sha256': sha(raw),
                  'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'incomplete_calls': 0, 'errors': errors, 'pass': bool(records) and not errors,
                  'kind_counts': dict(Counter(r['kind'] for r in records))}
        if name.endswith('-sequences'):
            report['step_counts'] = [len(r['calls']) for r in records]
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
