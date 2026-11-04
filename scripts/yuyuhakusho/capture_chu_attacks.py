#!/usr/bin/env python3
"""Full original Chu ground/air and special-attack selector calls."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_ground_idle import actor
from capture_effect_pool import pool_state

ENTRIES = {0x17e12: 'ground', 0x17f54: 'air', 0x17fca: 'ground-special', 0x18092: 'air-special'}
RETURNS = {0x15e26, 0x19c44, 0x17f9a, 0x17fb0, 0x17fc6, 0x17ffe, 0x1801a, 0x18034, 0x1804a, 0x1807a, 0x1808e, 0x18090, 0x180aa, 0x180c4, 0x180de, 0x180f4}
CONSTRUCTORS = {0x17ec2, 0x18812, 0x18828, 0x1883e, 0x18854, 0x1886a, 0x18880, 0x188d8, 0x188ee, 0x18930, 0x18ac4, 0x18adc, 0x18ea8, 0x18ec0, 0x1992e, 0x1994e, 0x19980, 0x199be, 0x19a22, 0x19a4a, 0x19a84}
ACTOR_WRITES = {0x26, 0x59, 0x5a, 0x5b, 0x5d, 0x5f, 0x60, 0x61, 0x64, 0x6b, 0x6c, 0x6d, 0x6e, 0x77, 0xb8, 0xb9} | set(range(0x10, 0x18)) | set(range(0x4c, 0x50))


def number(raw, off, n, signed=False):
    return int.from_bytes(raw[off:off+n], 'big', signed=signed)


def snapshot(ram, address, rom):
    off = address & 0xffff
    obj = ram[off:off+256]
    ptr = number(obj, 0x90, 4)&0xffffff
    memory, index = (rom, ptr) if ptr < len(rom) else (ram, ptr&0xffff)
    return {'body': actor(obj), 'actor_bytes': obj.hex(), 'child_bytes': ram[off+256:off+512].hex(),
            'held': obj[0x71], 'pressed': obj[0x72], 'changed': obj[0x73],
            'energy': number(obj, 0xb8, 2, True), 'child_size': number(ram, off+260, 2),
            'gravity_parameter': number(memory, index+8, 4, True), 'pool': pool_state(ram),
            'throws': ram[0xa600:0xa800].hex(), 'throw_offset': number(ram, 0xa00c, 2)}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
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
    points = sorted(set(ENTRIES)|RETURNS|CONSTRUCTORS)
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, streams, errors = [], [], [], []
    pending = {}
    private = False
    label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                address = ref.get_register(13)&0xffffff
                sp = ref.get_register(15)
                key = (address, sp)
                if pc in ENTRIES:
                    assert not pending, 'Overlapping Chu selector'
                    pending[key] = {'kind': ENTRIES[pc], 'object_id': address&0xffff, 'frame': ref.frame,
                                    'before': snapshot(ref.ram_cpu(), address, rom), 'constructors': []}
                elif pc in CONSTRUCTORS:
                    for (owner, _), row in pending.items():
                        if owner == address:
                            row['constructors'].append(hex(pc))
                elif pc in RETURNS and key in pending:
                    row = pending.pop(key)
                    row.update(after=snapshot(ref.ram_cpu(), address, rom), exit=hex(pc), exit_frame=ref.frame)
                    old, new = bytes.fromhex(row['before']['actor_bytes']), bytes.fromhex(row['after']['actor_bytes'])
                    unexpected = [i for i, (a, b) in enumerate(zip(old, new)) if a != b and i not in ACTOR_WRITES]
                    assert not unexpected, unexpected
                    assert row['before']['child_bytes'] == row['after']['child_bytes']
                    if private:
                        row['name'] = label
                        isolated.append(row)
                        ref.cpu.cycles = 4000000
                    else:
                        natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 4000000
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
        assert frames == expected and not pending and not errors, (len(natural), errors, list(pending))
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def write_bytes(address, raw):
            for i, b in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def setup(seed=0, request=0, held=0, pressed=0, changed=0, facing=0, pose=0, energy=255, health=100, child=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            raw = bytes((i*73+seed*29+17)&255 for i in range(512))
            write_bytes(0xff8400, raw)
            for off, value, size in [(0x58, 9, 1), (0x5f, pose, 1), (0x61, facing, 1), (0x6b, request, 1),
                                     (0x71, held, 1), (0x72, pressed, 1), (0x73, changed, 1), (0x78, health, 2),
                                     (0xb8, energy, 2), (0x90, 0xff7000, 4), (0x40, 0x7fff1234, 4), (0x44, -0x80004321, 4)]:
                write(0xff8400+off, value, size)
            write(0xff8504, child, 2)
            write(0xff7008, [0x3600, -1, -0x80000000, 0x7fffffff][seed%4], 4)
            write_bytes(0xffa600, bytes((i*19+seed*17+91)&255 for i in range(512)))
            write(0xffa00c, (seed%16)*32, 2)
            raw = bytearray((i*37+seed*23+13)&255 for i in range(0x800))
            for i in range(16):
                raw[i*128+4:i*128+6] = (0x80 if seed%3 == 2 or seed%3 == 1 and i != 1 else 0).to_bytes(2, 'big')
            write_bytes(0xff9400, raw)
            write(0xffa036, 0xff80 if seed%2 else 0, 2)

        def invoke(kind, name):
            nonlocal label
            label = name
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff8400)
            setreg(16, next(pc for pc, value in ENTRIES.items() if value == kind))
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu()
            count = len(isolated)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (kind, name, errors, list(pending))
            after = ref.ram_cpu()
            allowed = set(range(0x8400, 0x8500)) | set(range(0x9400, 0x9c00)) | set(range(0xa600, 0xa800)) | {0xa00c, 0xa00d, 0xa036, 0xa037} | set(range(0xff80, 0xffe4))
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return isolated[-1]

        for seed, (request, pose, direction, press, facing) in enumerate(itertools.product([0, 7, 8], [0, 11], range(16), [0, 16, 64, 80], [0, 1, 3])):
            setup(seed, request=request, pose=pose, held=direction, pressed=press, facing=facing)
            invoke('ground', f'ground-{request}-{pose}-{direction}-{press}-{facing}')
        for seed, (direction, press, facing) in enumerate(itertools.product(range(16), [0, 16, 64, 80], [0, 1, 3])):
            setup(seed, held=direction, pressed=press, facing=facing)
            invoke('air', f'air-{direction}-{press}-{facing}')
        for seed, (kind, request, direction, changed, facing) in enumerate(itertools.product(['ground-special', 'air-special'], range(9), [0, 1, 4, 8, 15], [0, 16, 32, 64, 48, 80, 96, 112], [0, 1, 3])):
            setup(seed, request=request, held=direction, changed=changed, facing=facing, energy=255 if seed%2 else 0, child=0 if seed%3 else 0x100)
            invoke(kind, f'special-{kind}-{request}-{direction}-{changed}-{facing}')
        charged = [('ground-special', 1, 16, 12), ('ground-special', 2, 64, 16), ('ground-special', 3, 16, 16),
                   ('ground-special', 3, 64, 20), ('ground-special', 4, 0, 16), ('ground-special', 5, 16, 4),
                   ('ground-special', 5, 32, 4), ('ground-special', 5, 64, 4), ('ground-special', 6, 64, 200),
                   ('air-special', 1, 16, 8), ('air-special', 2, 64, 12), ('air-special', 3, 16, 16), ('air-special', 3, 64, 20)]
        for index, (kind, request, changed, cost) in enumerate(charged):
            for energy, health, child, facing in itertools.product([-32768, -1, 0, cost-1, cost, cost+1, 32767], [111, 112], [0, 0x100], [0, 1, 3]):
                setup(index, request=request, changed=changed, facing=facing, energy=energy, health=health, child=child)
                invoke(kind, f'energy-{index}-{energy}-{health}-{child}-{facing}')
        boundary_count = len(isolated)
        for kind, request, changed in [('ground-special', 3, 16), ('ground-special', 4, 0), ('ground-special', 5, 32), ('air-special', 3, 64)]:
            setup(request, request=request, changed=changed, energy=50)
            calls = [invoke(kind, f'stream-{kind}-{request}/{step}') for step in range(8)]
            streams.append({'kind': kind, 'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity
    for name, records, scope in [('chu-attacks-oracle', natural, 'Natural full selector returns'),
                                 ('chu-attacks-boundaries', isolated[:boundary_count], 'Declared isolated full selector calls'),
                                 ('chu-attacks-sequences', streams, 'Single-initial-state selector streams')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        calls = [call for stream in records for call in stream['calls']] if name.endswith('-sequences') else records
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(records),
                  'calls': len(calls), 'kind_counts': dict(Counter(r['kind'] for r in calls)),
                  'constructor_counts': dict(Counter(c for r in calls for c in r['constructors'])),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'frames_match_original_core': frames == expected,
                  'incomplete_calls': 0, 'errors': errors, 'pass': bool(records) and not errors,
                  'limits': ['Selectors and initialization only, not complete moves or matches', 'All isolated inputs replace private reference-process data']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
