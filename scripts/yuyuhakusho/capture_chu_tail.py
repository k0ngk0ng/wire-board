#!/usr/bin/env python3
"""Original complete normal/hit-stop shared tail for the observed Chu actors."""
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
from capture_animation_records import selected_data

FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]


def number(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, address, rom):
    off = address&0xffff
    obj = ram[off:off+256]
    ptr = number(obj, 0x90, 4)&0xffffff
    memory, index = (rom, ptr) if ptr < len(rom) else (ram, ptr&0xffff)
    return {'actor_bytes': obj.hex(), 'body': actor(obj), 'child_bytes': ram[off+256:off+512].hex(),
            'other_actors': {str(i): ram[i:i+256].hex() for i in FIGHTERS if i != off},
            'pool': pool_state(ram), 'occupied': ram[0xa042], 'buffers': ram[0xa200:0xa600].hex(),
            'offsets': [number(ram, 0xa004+i*2, 2) for i in range(4)], 'throws': ram[0xa600:0xa800].hex(),
            'throw_offset': number(ram, 0xa00c, 2), 'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]),
            'random_seed': number(ram, 0xff18, 4), 'battle_state': number(ram, 0xa03a, 2, True),
            'parameters': {key: number(memory, index+delta, size, True) for key, delta, size in
                           [('vertical_velocity', 4, 4), ('gravity', 8, 4), ('horizontal_velocity', 12, 4), ('burst_speed', 24, 4), ('energy_drain', 28, 2)]}}


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
    points = [0x15afc, 0x15b40, 0x15b78, 0x15b82, 0x37c56, 0x37cac]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, sequences, errors = [], [], [], []
    pending, library = {}, {}
    private, label = False, ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                address = ref.get_register(13)&0xffffff
                sp = ref.get_register(15)
                key = (address, sp)
                if pc in (0x37c56, 0x37cac):
                    for row in pending.values():
                        a = ref.get_register(9)&0xffff
                        row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a],
                                                             'sr': ref.get_register(17), 'sp': sp})
                elif pc in (0x15afc, 0x15b40) and key not in pending:
                    assert not pending, 'Nested shared tail'
                    obj = ref.ram_cpu()[address&0xffff:(address&0xffff)+256]
                    assert obj[0x58] == 9, ('Non-Chu natural actor', obj[0x58])
                    pending[key] = {'object_id': address&0xffff, 'frame': ref.frame, 'mode': 'normal' if pc == 0x15afc else 'hit-stop',
                                    'before': snapshot(ref.ram_cpu(), address, rom), 'external_queue_clears': []}
                elif pc == 0x15b78 and key in pending:
                    ram = ref.ram_cpu()
                    frame_key, data, source = selected_data(rom, ram, ram[address&0xffff:(address&0xffff)+256])
                    identity = json.dumps(frame_key, sort_keys=True)
                    assert identity not in library or library[identity]['data'] == data
                    library[identity] = {'key': frame_key, 'data': data, 'source': source}
                    pending[key]['animation_key'] = frame_key
                elif pc == 0x15b82 and key in pending:
                    row = pending.pop(key)
                    assert 'animation_key' in row
                    row.update(after=snapshot(ref.ram_cpu(), address, rom), exit_frame=ref.frame)
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
                base_ram = ref.ram_cpu()
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

        profiles = [(0, 0, 0, 0, 0, 0), (8, 16, 16, 0, 0, 0), (4, 64, 64, 0, 0, 0),
                    (1, 0, 1, 0, 0, 0), (0, 80, 112, 0, 0, 0), (0, 64, 64, 0, 6, 0),
                    (0, 16, 16, 0, 1, 25), (0, 0, 0, 0, 8, 0), (0x20, 0, 1, 2, 0, 0)]

        def setup(seed=0, flags=0, profile=0, pose=0, facing=0, health=100, battle=0, energy=255, extra=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            for i, owner in enumerate(FIGHTERS):
                write_bytes(0xff0000+owner, bytes((j*73+i*47+seed*11+17)&255 for j in range(256)))
                write(0xff0000+owner+4, [0, 256, 256, 256][(i+seed)%4], 2)
                write(0xff0000+owner+0x6e, (i+seed)%3)
                write(0xff0000+owner+0x78, [-1, 0, 223][(i+seed)%3], 2)
                write(0xff0000+owner+0xb8, [0, 1, 10, 32767][(i+seed)%4], 2)
            write_bytes(0xff8450, base_ram[0x8450:0x8454])
            write_bytes(0xff848c, base_ram[0x848c:0x8490])
            held, pressed, changed, extended, request, charge = profiles[profile]
            for off, value, size in [(0x58, 9, 1), (0x59, 0, 1), (0x5a, 8, 1), (0x5b, seed%2, 1), (0x5c, 0, 1), (0x5f, pose, 1),
                                     (0x61, facing, 1), (0x6b, request, 1), (0x6c, flags, 1), (0x6d, 0, 1), (0x6e, 0, 1), (0x6f, seed%3, 1),
                                     (0x71, held, 1), (0x72, pressed, 1), (0x73, changed, 1), (0xc0, 0, 1), (0xc1, extended, 1), (0xc2, 0, 1),
                                     (0x78, health, 2), (0x87, extra, 1), (0x89, 0xa5, 1), (0x90, 0xff7000, 4), (0xb8, energy, 2),
                                     (0xc4, [0, 1, 255, 65535][seed%4], 2), (0xc7, charge, 1),
                                     (0x40, 0x7fff1234, 4), (0x44, 0x1405678, 4), (0x9c, 0x140abcd if seed%2 else 0x1405678, 4)]:
                write(0xff8400+off, value, size)
            write(0xff8504, 0 if seed%2 else 256, 2)
            for off, value in [(4, [-0x58000, -0x58003, -1, 1, -0x80000000, 0x7fffffff][seed%6]), (8, 0x3601), (12, 0x1c000), (24, [-1, 0xc000, 0x7fffffff, -0x80000000][seed%4])]:
                write(0xff7000+off, value, 4)
            write(0xff701c, [-32768, -1, 0, 10, 32767][seed%5], 2)
            write_bytes(0xffa200, bytes((i*19+seed*31+91)&255 for i in range(0x600)))
            for i in range(4):
                write(0xffa004+i*2, [0, 240, 256][(seed+i)%3], 2)
            write(0xffa00c, 0, 2)
            raw = bytearray((i*37+seed*23+13)&255 for i in range(0x800))
            for i in range(16):
                raw[i*128+4:i*128+6] = (0x80 if seed%3 == 2 or seed%3 == 1 and i != 1 else 0).to_bytes(2, 'big')
            write_bytes(0xff9400, raw)
            write(0xffa036, 0xff80 if seed%2 else 0, 2)
            write(0xffa03a, battle, 2)
            write(0xffa042, 0xa0)
            write(0xffa030, 0x97 if seed%2 else 0)
            write_bytes(0xfff80a, bytes([0x80, 0x81, 0x82, 0x83] if seed%2 else [0, 0x80, 0, 0x81]))
            write(0xffff18, [0, 1, 0x12345678, 0xffffffff][seed%4], 4)

        def invoke(mode, name):
            nonlocal label
            label = name
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff8400)
            setreg(16, 0x15afc if mode == 'normal' else 0x15b40)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu()
            count = len(isolated)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (name, errors, list(pending))
            after = ref.ram_cpu()
            allowed = set(range(0x8400, 0x8500)) | set(range(0x9400, 0x9c00)) | set(range(0xa200, 0xa800)) | set(range(0xa004, 0xa00e)) | {0xa030, 0xa036, 0xa037, 0xa042} | set(range(0xf80a, 0xf80e)) | set(range(0xff18, 0xff1c)) | set(range(0xff80, 0xffe4))
            for owner in FIGHTERS[1:]:
                allowed.update([owner+0xb8, owner+0xb9])
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return isolated[-1]

        for seed, (mode, flags, profile) in enumerate(itertools.product(['normal', 'hit-stop'], range(32), range(len(profiles)))):
            setup(seed, flags=flags, profile=profile, pose=[0, 9, 10, 17, 44, 45][seed%6], facing=seed%3, extra=[0, 1, 255][seed%3])
            invoke(mode, f'flags-{mode}-{flags:x}/input-{profile}')
        for seed, (pose, health, battle, airborne) in enumerate(itertools.product([0, 17], [-1, 0, 1], [0, 255, 256, 32767, -32768], [0, 1])):
            setup(seed, pose=pose, health=health, battle=battle, flags=0x1b)
            if airborne:
                write(0xff8444, 0x13fffff, 4)
            invoke('normal', f'battle-{pose}-{health}-{battle}-{airborne}')
        for seed, (direction, facing, extra, pressed, changed) in enumerate(itertools.product([0, 1, 5, 9, 15], [0, 1, 3], [0, 1, 255], [0, 1], [0, 1])):
            setup(seed, flags=4, facing=facing, extra=extra)
            write(0xff8471, direction)
            write(0xff8472, pressed)
            write(0xff8473, changed)
            invoke('normal', f'double-jump-{direction}-{facing}-{extra}-{pressed}-{changed}')
        boundary_count = len(isolated)
        for profile in [0, 1, 2, 4, 7]:
            setup(profile, flags=0x1f, profile=profile, energy=255)
            calls = [invoke('normal', f'stream-{profile}/{step}') for step in range(6)]
            sequences.append({'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity
    for name, calls, scope in [('chu-tail-oracle', natural, 'Natural complete shared-tail returns'),
                               ('chu-tail-boundaries', isolated[:boundary_count], 'Declared isolated full tails'),
                               ('chu-tail-sequences', sequences, 'Single-initial-state shared-tail streams')]:
        data = {'library': list(library.values()), 'calls': calls}
        raw = (json.dumps(data, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        rows = [call for stream in calls for call in stream['calls']] if name.endswith('-sequences') else calls
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(calls), 'calls': len(rows),
                  'modes': dict(Counter(r['mode'] for r in rows)), 'library_frames': len(library),
                  'external_queue_calls': sum(bool(r['external_queue_clears']) for r in rows), 'frames_match_original_core': frames == expected,
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'incomplete_calls': 0, 'errors': errors, 'pass': bool(calls) and not errors,
                  'limits': ['Complete shared-tail functions, not actor/world steps or matches', 'Natural external queue events are retained without rewriting expected output']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
