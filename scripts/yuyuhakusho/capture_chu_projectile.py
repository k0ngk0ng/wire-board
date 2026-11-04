#!/usr/bin/env python3
"""Original Chu projectile rules through draw request or completed deletion."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data


def num(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, address):
    off = address&0xffff
    return {'actor_bytes': ram[off:off+256].hex(), 'parent_bytes': ram[off-256:off].hex(),
            'pool': {'cursor': num(ram, 0xa036, 2), 'bytes': ram[0x9400:0x9c00].hex()},
            'buffers': ram[0xa200:0xa600].hex(), 'offsets': [num(ram, 0xa004+i*2, 2) for i in range(4)],
            'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]),
            'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': num(ram, 0xa03e, 2),
                            'arena_left': num(ram, 0xa010, 2, True), 'arena_right': num(ram, 0xa012, 2, True),
                            'camera_x': num(ram, 0xa014, 2, True), 'camera_y': num(ram, 0xa016, 2, True), 'shake': num(ram, 0xf566, 2, True)}}


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
    points = [0x1313e, 0x13250, 0x3556, 0x3574, 0x1eb40, 0x37c56, 0x37cac]
    pcs = (C.c_uint*len(points))(*points)
    input_path = ROOT/'games/yuyuhakusho/engine/testdata/chu-special-motion-inputs.json'
    events = json.loads(input_path.read_text())
    with ReferenceSession(ROOT, out/'unobserved') as ref:
        expected = [ref.step(f, buttons_at(events, f))['reference'] for f in range(1, 3601)]
        unobserved_identity = ref.identity
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    natural, isolated, sequences, errors = [], [], [], []
    pending, library = {}, {}
    private, label = False, ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                address = ref.get_register(13)&0xffffff
                key = (address, ref.get_register(15))
                if pc == 0x1313e:
                    assert not pending, 'Overlapping projectile callback'
                    pending[key] = {'frame': ref.frame, 'object_id': address&0xffff, 'before': snapshot(ref.ram_cpu(), address),
                                    'external_queue_clears': [], 'deleted': False}
                elif pc in (0x37c56, 0x37cac):
                    for row in pending.values():
                        a = ref.get_register(9)&0xffff
                        row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                elif pc == 0x1eb40:
                    for (owner, _), row in pending.items():
                        if owner == address:
                            ram = ref.ram_cpu()
                            frame_key, data, source = selected_data(rom, ram, ram[address&0xffff:(address&0xffff)+256])
                            identity = json.dumps(frame_key, sort_keys=True)
                            assert identity not in library or library[identity]['data'] == data
                            library[identity] = {'key': frame_key, 'data': data, 'source': source}
                            row['animation_key'] = frame_key
                elif pc == 0x3556 and key in pending:
                    pending[key]['deleted'] = True
                    obj = bytes.fromhex(pending[key]['before']['actor_bytes'])
                    size = num(obj, 4, 2)
                    cleared = ((size if size else 64)>>4)*16
                    assert 0 < cleared <= 256, ('Unmodeled deletion range', size)
                    pending[key]['cleared_bytes'] = cleared
                elif key in pending and (pc == 0x13250 or pc == 0x3574 and pending[key]['deleted']):
                    row = pending.pop(key)
                    row.update(after=snapshot(ref.ram_cpu(), address), draw=pc == 0x13250,
                               cleared_bytes=row.get('cleared_bytes', 0), exit=hex(pc), exit_frame=ref.frame)
                    assert row['before']['parent_bytes'] == row['after']['parent_bytes']
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
        assert frames == expected and natural and not pending and not errors, (len(natural), errors)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def fill(address, length, seed):
            for i in range(length):
                write(address+i, (i*73+seed*29+17)&255)

        def setup(seed=0, **changes):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            fill(0xff8500, 256, seed)
            fill(0xff9400, 0x800, seed+1)
            fill(0xffa200, 1024, seed+2)
            values = {'stage': 2, 'size': 256, 'power': 0, 'pose': 33, 'index': 0, 'duration': 2, 'ticks': 0, 'flags': 0x10,
                      'x': (100<<16)|0xabcd, 'y': (260<<16)|0x5678, 'ground': (320<<16)|0x1234, 'vx': 0x60000, 'vy': -0x10000,
                      'facing': 0, 'phase': 0, 'contact': 0, 'pause': 0, 'slow': 0, 'pool': 'empty', 'left': 0, 'right': 600,
                      'camera_x': 31, 'camera_y': 23, 'shake': 0, 'offset_x': -3, 'offset_y': 4}
            values.update(changes)
            fields = {'stage': (6, 2), 'size': (4, 2), 'power': (0x7c, 2), 'pose': (0x5f, 1), 'index': (0x59, 1), 'duration': (0x5a, 1),
                      'ticks': (0x5b, 1), 'flags': (0x5c, 1), 'x': (0x40, 4), 'y': (0x44, 4), 'ground': (0x9c, 4), 'vx': (0x10, 4),
                      'vy': (0x14, 4), 'facing': (0x61, 1), 'phase': (0x48, 1), 'contact': (0x89, 1), 'offset_x': (0x54, 2), 'offset_y': (0x56, 2)}
            for name, (off, n) in fields.items():
                write(0xff8500+off, values[name], n)
            write(0xff8500, 0x1313e, 4)
            for off, value in [(0x1c, 0xd8f1a), (0x20, 0x240000), (0x50, 0xd92ca), (0x8c, 0xd9a3a)]:
                write(0xff8500+off, value, 4)
            for i in range(16):
                busy = values['pool'] == 'full' or values['pool'] == 'single' and i != 1
                write(0xff9404+i*128, 128 if busy else 0, 2)
            for i in range(4):
                write(0xffa004+i*2, [0, 240, 256][(i+seed)%3], 2)
            write(0xffa036, 0xff80 if seed%2 else 0, 2)
            write(0xffa03e, [0, 0x2000, 0xffff][seed%3], 2)
            for a, key in [(0xffa010, 'left'), (0xffa012, 'right'), (0xffa014, 'camera_x'), (0xffa016, 'camera_y'), (0xfff566, 'shake')]:
                write(a, values[key], 2)
            write(0xfff550, values['pause'])
            write(0xfff55a, values['slow'])
            write(0xffa030, 0x97 if seed%2 else 0)
            for i, value in enumerate([0x80, 0x81, 0x82, 0x83] if seed%2 else [0, 0x80, 0, 0x81]):
                write(0xfff80a+i, value)

        def invoke(name):
            nonlocal label
            label = name
            for reg in range(8):
                setreg(reg, 0)
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff8500)
            setreg(16, 0x1313e)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu()
            count = len(isolated)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (name, errors, list(pending))
            after = ref.ram_cpu()
            allowed = set(range(0x8500, 0x8600)) | set(range(0x9400, 0x9c00)) | set(range(0xa200, 0xa600)) | set(range(0xa004, 0xa00c)) | {0xa030, 0xa036, 0xa037} | set(range(0xf80a, 0xf80e)) | set(range(0xff80, 0xffe4))
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return isolated[-1]

        poses = [33, 33, 34, 35, 4, 5, 6, 7]
        for seed, (stage, power, index, ticks, paused, pool, facing) in enumerate(itertools.product([0, 2], range(8), [0, 1], [0, 1, 255], range(3), ['empty', 'single', 'full'], [0, 1, 3])):
            setup(seed, stage=stage, power=power, pose=poses[power], index=index, flags=0x10|(0x80 if index else 0), ticks=ticks,
                  pause=0x80 if paused == 1 else 0, slow=255 if paused == 2 else 0, pool=pool, facing=facing, phase=3)
            invoke(f'matrix-{stage}-{power}-{index}-{ticks}-{paused}-{pool}-{facing}')
        geometry = [(-32768, -32768, -1), (-1, 260, -1), (31, 319, 0), (100, 319, 0), (100, 320, 0), (100, 321, 0), (632, 260, -1), (633, 260, -1), (32767, 32767, -1), (700, 320, 0), (700, 321, 1)]
        for seed, (power, (x, y, vy), contact, pool) in enumerate(itertools.product(range(8), geometry, [0, 4, 8, 16, 24], ['empty', 'full'])):
            setup(seed, power=power, pose=poses[power], x=(x<<16)|0xabcd, y=(y<<16)|0x5678, vy=vy, vx=0, contact=contact, pool=pool, phase=255)
            invoke(f'geometry-{power}-{x}-{y}-{vy}-{contact}-{pool}')
        for seed, (power, phase, y, ground, facing) in enumerate(itertools.product([5, 6, 7, 8, 65535], [0, 1, 2, 3, 255], [255, 256, 257, -32768, 32767], [320, -32768], [0, 1, 3])):
            setup(seed, power=power, pose=7, phase=phase, y=(y<<16)|0x5678, ground=(ground<<16)|0x1234, facing=facing, vx=0, vy=-1)
            invoke(f'trail-{power}-{phase}-{y}-{ground}-{facing}')
        for seed, (size, cause, power) in enumerate(itertools.product([0, 31, 271, *range(16, 257, 16)], ['bound', 'contact', 'land'], range(8))):
            setup(seed, size=size, power=power, pose=poses[power], x=700<<16 if cause == 'bound' else 100<<16,
                  y=321<<16 if cause == 'land' else 260<<16, vy=0 if cause == 'land' else -1, contact=8 if cause == 'contact' else 0)
            invoke(f'clear-{size}-{cause}-{power}')
        boundary_count = len(isolated)
        for power, facing, terminal in itertools.product([0, 3, 7], [0, 1], ['bound', 'land', 'contact']):
            setup(power+facing, stage=0, power=power, pose=poses[power], facing=facing, vx=0x60000 if facing == 0 else -0x60000,
                  x=100<<16 if facing == 0 else 500<<16, y=260<<16, vy=0x10000 if terminal == 'land' else -1, phase=3)
            initial = snapshot(ref.ram_cpu(), 0xff8500)
            calls = []
            for step in range(128):
                external = {}
                if terminal == 'contact' and step == 8:
                    write(0xff8589, 8)
                    external = {'contact_flags': 8}
                row = invoke(f'sequence-{power}-{facing}-{terminal}/{step}')
                row['external_input'] = external
                calls.append(row)
                if row['deleted']:
                    break
            assert calls[-1]['deleted'], (power, facing, terminal)
            sequences.append({'initial': initial, 'calls': calls})
        identity = ref.identity
    for name, records, scope in [('chu-projectile-oracle', natural, 'Natural projectile rule phases with declared original input pulses'),
                                 ('chu-projectile-boundaries', isolated[:boundary_count], 'Isolated original rule phases and deletion boundaries'),
                                 ('chu-projectile-sequences', sequences, 'Single-initial-state rule trajectories through deletion')]:
        raw = (json.dumps({'library': list(library.values()), 'calls': records}, separators=(',', ':'))+'\n').encode()
        z = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(z)
        calls = [c for s in records for c in s['calls']] if name.endswith('-sequences') else records
        report = {'scope': scope, 'identity': identity, 'unobserved_identity': unobserved_identity, 'inputs_sha256': sha(input_path.read_bytes()),
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()), 'records': len(records), 'calls': len(calls),
                  'draws': sum(c['draw'] for c in calls), 'deletions': sum(c['deleted'] for c in calls), 'library_frames': len(library),
                  'external_queue_calls': sum(bool(c['external_queue_clears']) for c in calls), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(z),
                  'frames_match_original_core': frames == expected, 'errors': errors, 'incomplete_calls': 0, 'pass': bool(records) and not errors,
                  'limits': ['Rules stop at draw request before 0x1580; hardware sprite emission is separate', 'Deletion tested for logical slot prefixes up to256 bytes, not corrupt address aliases', 'Impact table bounded to creator-derived power0..7']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
