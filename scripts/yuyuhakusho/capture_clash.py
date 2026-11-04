#!/usr/bin/env python3
"""Complete original attack-clash calls, with passive and isolated evidence."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

SHAPES = 0xa300
POOL = 0x9400
ALLOWED_ACTOR = {0x5b, 0x69, 0x6a, 0x6d, 0x75, 0x89} | set(range(0x94, 0x9c))


def number(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram):
    offset = number(ram, 0xa006, 2)
    assert offset <= 256 and offset % 16 == 0, offset
    owners = sorted({number(ram, SHAPES+i+12, 2) for i in range(0, offset, 16)})
    assert all(0x8000 <= owner <= 0xff00 for owner in owners), owners
    return {'offset': offset, 'shapes': ram[SHAPES:SHAPES+256].hex(),
            'actors': {str(owner): ram[owner:owner+256].hex() for owner in owners},
            'effects': ram[POOL:POOL+0x800].hex(), 'cursor': number(ram, 0xa036, 2),
            'pending_sound': ram[0xa030]}


def validate(before, after):
    assert before['offset'] == after['offset'] and before['shapes'] == after['shapes']
    assert before['actors'].keys() == after['actors'].keys()
    for key, old in before['actors'].items():
        new = bytes.fromhex(after['actors'][key])
        unexpected = [i for i, (a, b) in enumerate(zip(bytes.fromhex(old), new)) if a != b and i not in ALLOWED_ACTOR]
        assert not unexpected, (key, unexpected)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    pcs = (C.c_uint*3)(0x1f090, 0x1f11c, 0x1f21c)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows, isolated, errors = [], [], []
    pending = None
    private = False
    name = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @callback_type
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                if pc == 0x1f090:
                    assert pending is None, 'Nested clash entry'
                    pending = {'frame': ref.frame, 'before': snapshot(ref.ram_cpu()), 'contacts': 0}
                elif pc == 0x1f11c:
                    assert pending is not None
                    pending['contacts'] += 1
                elif pc == 0x1f21c:
                    assert pending is not None, 'Clash return without entry'
                    pending.update(after=snapshot(ref.ram_cpu()), exit_frame=ref.frame)
                    validate(pending['before'], pending['after'])
                    if private:
                        pending['name'] = name
                        isolated.append(pending)
                        ref.cpu.cycles = 4000000
                    else:
                        rows.append(pending)
                    pending = None
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, len(pcs))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not errors, (len(rows), errors)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value & ((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def write_bytes(address, data):
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def actor_spec(owner, index):
            return {'owner': owner, 'team': index, 'target_bit': index, 'mask': 0xffffffff,
                    'attack_value': 17+index, 'attack_flags': 0, 'frame_ticks': 0,
                    'y': 0x1400000, 'ground': 0x1400000, 'contact_flags': 0x80,
                    'cancel_ticks': 93, 'residual': -123456, 'shake': 7}

        def shape(owner, x=100, y=320, width=16, height=24):
            return {'owner': owner, 'x': x, 'y': y, 'width': width, 'height': height}

        def run_case(case_name, actors, shapes, busy=(), cursor=0, seed=0):
            nonlocal name
            name = case_name
            ref.restore(checkpoint, restore_refresh_cycles=True)
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            for reg in range(15):
                setreg(reg, 0)
            for index, actor in enumerate(actors):
                owner = actor['owner']
                raw = bytearray((i*73+seed*29+index*47+17)&255 for i in range(256))
                write_bytes(0xff0000+owner, raw)
                for field, off, size in [('team', 0x6e, 1), ('target_bit', 0x70, 1), ('mask', 0x94, 4),
                                          ('attack_value', 0x65, 1), ('attack_flags', 0x64, 1), ('frame_ticks', 0x5b, 1),
                                          ('y', 0x44, 4), ('ground', 0x9c, 4), ('contact_flags', 0x89, 1),
                                          ('cancel_ticks', 0x6d, 1), ('residual', 0x98, 4), ('shake', 0x6a, 1)]:
                    write(0xff0000+owner+off, actor[field], size)
            raw = bytearray((i*19+seed*31+91)&255 for i in range(256))
            for i, record in enumerate(shapes):
                for field, off, size in [('x', 0, 2), ('y', 4, 2), ('width', 8, 1), ('height', 9, 1), ('owner', 12, 2)]:
                    value = record[field] & ((1 << (size*8))-1)
                    raw[i*16+off:i*16+off+size] = value.to_bytes(size, 'big')
            write_bytes(0xffa300, raw)
            write(0xffa006, len(shapes)*16, 2)
            raw_pool = bytearray((i*37+seed*23+13)&255 for i in range(0x800))
            for i in range(16):
                raw_pool[i*128+4:i*128+6] = (0x80 if i in busy else 0).to_bytes(2, 'big')
            write_bytes(0xff9400, raw_pool)
            write(0xffa036, cursor, 2)
            write(0xffa030, 0xc9)
            before_ram = ref.ram_cpu()
            count = len(isolated)
            setreg(16, 0x1f090)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and pending is None and not errors, (name, errors, pending is not None)
            after_ram = ref.ram_cpu()
            allowed = set(range(0xffa300 & 0xffff, 0xa400)) | set(range(POOL, POOL+0x800)) | {0xa030, 0xa036, 0xa037} | set(range(0xffb0, 0xffe4))
            for actor in actors:
                allowed.update(range(actor['owner'], actor['owner']+256))
            unexpected = [i for i, (a, b) in enumerate(zip(before_ram, after_ram)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)

        base_actors = [actor_spec(0x8400, 0), actor_spec(0x9000, 1)]
        base_shapes = [shape(0x8400), shape(0x9000, x=112)]
        for i, (strength_a, strength_b, flying, tick) in enumerate(itertools.product(range(4), range(4), range(3), [0, 255])):
            actors = [dict(a) for a in base_actors]
            actors[0]['attack_flags'] = strength_a | (0x10 if i&1 else 0)
            actors[1]['attack_flags'] = strength_b | (0x10 if i&2 else 0)
            actors[0]['attack_value'] = [0, 128, 255][i%3]
            actors[1]['attack_value'] = [255, 128, 0][i%3]
            actors[0]['frame_ticks'] = tick
            actors[1]['frame_ticks'] = 1 if tick else 0
            if flying == 1:
                actors[0]['y'] -= 1  # Full long differs; high-word check also changes here.
            elif flying == 2:
                actors[1]['y'] += 1  # Full long differs, high word still equal.
            run_case(f'strength-{strength_a}-{strength_b}/air-{flying}/tick-{tick}', actors, base_shapes,
                     busy=range(16) if i%4 == 0 else (), cursor=0xff80 if i%5 == 0 else 0, seed=i)
        for i, (target_bit, mask) in enumerate(itertools.product([0, 1, 31, 32, 33, 255], [0, 1, 2, 0x80000000, 0xffffffff])):
            actors = [dict(a) for a in base_actors]
            actors[1]['target_bit'], actors[0]['mask'] = target_bit, mask
            run_case(f'target-bit-{target_bit}/mask-{mask:x}', actors, base_shapes, seed=i)
        for field, values in [('team', [0, 1, 255]), ('ground', [0x1400000, 0x140ffff, 0x13fffff, -0x80000000])]:
            for value in values:
                actors = [dict(a) for a in base_actors]
                actors[1][field] = value
                run_case(f'filter-{field}-{value}', actors, base_shapes)
        geometry = [(100, 132, 16, 16), (100, 133, 16, 16), (100, 100, 0, 0),
                    (100, 100, 255, 1), (100, 101, 255, 1), (0, -32768, 1, 1),
                    (32767, -32768, 4, 4), (-32768, -32767, 128, 128), (32767, 32767, 255, 255)]
        for axis in ['x', 'y']:
            for i, (a, b, wa, wb) in enumerate(geometry):
                shapes = [dict(s) for s in base_shapes]
                dimension = 'width' if axis == 'x' else 'height'
                shapes[0][axis], shapes[1][axis] = a, b
                shapes[0][dimension], shapes[1][dimension] = wa, wb
                run_case(f'geometry-{axis}-{i}', base_actors, shapes, seed=i)
        for count in [0, 1, 4, 16]:
            actors = [actor_spec(owner, i) for i, owner in enumerate([0x8400, 0x8500, 0x9000, 0x9100])]
            shapes = [shape(actors[i%4]['owner'], x=100+(i%3)*3) for i in range(count)]
            run_case(f'four-actors-records-{count}', actors, shapes, seed=count)
        for name_suffix, busy in [('all-full', range(16)), ('current-free', range(1, 16)),
                                  ('next-free', set(range(16))-{1}), ('last-free', set(range(16))-{15})]:
            run_case('pool-'+name_suffix, base_actors, base_shapes, busy=busy)
        identity = ref.identity
    for fixture_name, records, scope in [('clash-oracle', rows, 'Complete passive original calls'),
                                         ('clash-boundaries', isolated, 'Explicit isolated original CPU calls')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(fixture_name+'.json.gz')).write_bytes(compressed)
        report = {'scope': scope, 'identity': identity, 'records': len(records), 'contacts': sum(r['contacts'] for r in records),
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'frames_match_original_core': frames == expected,
                  'incomplete_calls': 0, 'errors': errors, 'pass': bool(records) and not errors,
                  'limits': ['Full clash and allocation/initialization, not effect lifetime or complete matches',
                             'Private isolated original CPU inputs are deliberately replaced; natural frames are untouched']}
        (out/(fixture_name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
