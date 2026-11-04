#!/usr/bin/env python3
"""Complete original Chu action routine, not a stitched per-stage replay."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_tail import snapshot as tail_snapshot, number
from capture_animation_records import selected_data

ACTION_POSES = {0: 0, 1: 5, 2: 6, 3: 0x80, 4: 0x20, 5: 0x25, 6: 0x28, 7: 0x26, 8: 0x25,
                9: 0x12, 10: 15, 11: 16, 12: 0x2b, 13: 0x13, 15: 11, 20: 0xb1, 21: 0xa0,
                65: 0xa2, 66: 0xa6, 67: 0xa7, 68: 0xa4, 69: 0xae, 70: 0xaa}


def snapshot(ram, address, rom):
    off = address&0xffff
    obj = ram[off:off+256]
    tail = tail_snapshot(ram, address, rom)
    ptr = number(obj, 0x90, 4)&0xffffff
    mem, index = (rom, ptr) if ptr < len(rom) else (ram, ptr&0xffff)
    return {'tail': tail, 'children': ram[off+256:off+1024].hex(), 'walk_speed': number(mem, index, 4, True),
            'left': number(ram, 0xa010, 2, True), 'right': number(ram, 0xa012, 2, True),
            'global_f568': number(ram, 0xf568, 2), 'global_f56a': number(ram, 0xf56a, 2)}


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
    points = [0x15a18, 0x15b78, 0x15b82, 0x37c56, 0x37cac, 0x3982]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    paths = [ROOT/'docs/research/md-yuyuhakusho/inputs.json', ROOT/'games/yuyuhakusho/engine/testdata/chu-special-motion-inputs.json']
    natural, isolated, sequences, errors, baselines = [], [], [], [], []
    pending, library = {}, {}
    private, label, scenario = False, '', ''
    checkpoints = []

    for input_index, input_path in enumerate(paths):
        scenario = input_path.name
        events = json.loads(input_path.read_text())
        if input_index == 0:
            expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
        else:
            with ReferenceSession(ROOT, out/'unobserved-special') as original:
                expected = [original.step(f, buttons_at(events, f))['reference'] for f in range(1, 3601)]
        with ReferenceSession(ROOT, out/('observed-'+str(input_index)), core=core) as ref:
            @cbtype
            def observe(kind, width, pc, value):
                try:
                    address = ref.get_register(13)&0xffffff
                    sp = ref.get_register(15)
                    key = (address, sp)
                    if pc in (0x37c56, 0x37cac):
                        for row in pending.values():
                            a = ref.get_register(9)&0xffff
                            row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a], 'sr': ref.get_register(17), 'sp': sp})
                    elif pc == 0x3982:
                        for row in pending.values():
                            row['rng_calls'] += 1
                    elif pc == 0x15a18:
                        assert not pending, 'Nested original action routine'
                        ram = ref.ram_cpu()
                        assert ram[(address&0xffff)+0x58] == 9
                        pending[key] = {'object_id': address&0xffff, 'frame': ref.frame, 'scenario': scenario,
                                        'before': snapshot(ram, address, rom), 'external_queue_clears': [], 'rng_calls': 0}
                    elif pc == 0x15b78 and key in pending:
                        ram = ref.ram_cpu()
                        frame_key, data, source = selected_data(rom, ram, ram[address&0xffff:(address&0xffff)+256])
                        identity = json.dumps(frame_key, sort_keys=True)
                        assert identity not in library or library[identity]['data'] == data
                        library[identity] = {'key': frame_key, 'data': data, 'source': source}
                        pending[key]['animation_key'] = frame_key
                    elif pc == 0x15b82 and key in pending:
                        row = pending.pop(key)
                        assert row['rng_calls'] == 1 and 'animation_key' in row
                        row.update(after=snapshot(ref.ram_cpu(), address, rom), exit_frame=ref.frame)
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
                    checkpoint, base_ram = ref.save(), ref.ram_cpu()
            hook(None)
            assert frames == expected and not pending and not errors, (scenario, errors, list(pending))
            baselines.append({'inputs': str(input_path.relative_to(ROOT)), 'sha256': sha(input_path.read_bytes()),
                              'frames': 3600, 'match': frames == expected, 'reference_sha256': sha(json.dumps(expected, separators=(',', ':')).encode())})
            if input_index == 0:
                identity = ref.identity
                continue
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            execute = ref.api('m68k_run', None, C.c_uint)

            def write(address, value, size=1):
                for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

            def write_bytes(address, raw):
                for i, b in enumerate(raw):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

            def setup(seed, action, facing=0, held=0, pressed=0, changed=0, raw_held=0, stop=0, completed=0, edge=False):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                for reg in range(15):
                    setreg(reg, 0)
                write_bytes(0xff8400, bytes((j*73+seed*11+17)&255 for j in range(1024)))
                write_bytes(0xff8450, base_ram[0x8450:0x8454])
                write_bytes(0xff848c, base_ram[0x848c:0x8490])
                pose = ACTION_POSES[action]
                obj = bytearray(ref.ram_cpu()[0x8400:0x8500])
                obj[0x5f], obj[0x59] = pose, 0
                _, frame, _ = selected_data(rom, ref.ram_cpu(), obj)
                for off, value, size in [(0x58, 9, 1), (0x59, 0, 1), (0x5a, frame['duration'], 1), (0x5b, frame['duration']-1 if edge else 0, 1),
                                         (0x5c, frame['flags'], 1), (0x5d, action, 1), (0x5f, pose, 1), (0x60, completed, 1), (0x61, facing, 1),
                                         (0x69, stop, 1), (0x6a, seed%4, 1), (0x6b, seed%9, 1), (0x6c, 0x1f if seed%2 else 0, 1), (0x6d, 0, 1), (0x6e, 0, 1), (0x6f, seed%3, 1),
                                         (0x71, held, 1), (0x72, pressed, 1), (0x73, changed, 1), (0x74, raw_held, 1), (0x75, 0, 1),
                                         (0xc0, 0, 1), (0xc1, seed%8, 1), (0xc2, 0, 1), (0xc7, seed%66, 1),
                                         (0x78, [-1, 0, 100][seed%3], 2), (0x7c, seed%256, 2), (0x7e, -1, 2), (0x80, 100, 2), (0x84, [0, 1, 129, 321][seed%4], 2),
                                         (0x86, seed%3, 1), (0x87, seed%2, 1), (0x88, 0, 1), (0x89, [0, 4, 8, 16][seed%4], 1), (0xc8, seed%3, 1),
                                         (0x40, 100<<16|0x1234, 4), (0x44, [0x1405678, 0x13f0000, 0x13fffff][seed%3], 4), (0x9c, 0x1405678, 4),
                                         (0x10, [0, 0x18000, -0x18000][seed%3], 4), (0x14, [-0x1000, 0, 0x18000][seed%3], 4), (0x4c, 0x3600, 4),
                                         (0x90, 0xff7000, 4), (0xb8, ([0, 50, 223][seed%3]<<16)|0x5678, 4), (0x98, [0, 1, 0x4800][seed%3], 4), (0xc4, seed%65536, 2)]:
                    write(0xff8400+off, value, size)
                for child in range(3):
                    write(0xff8504+child*256, 0 if seed%2 else 256, 2)
                for off, value in [(0, [0x18000, -1, 0x7fffffff][seed%3]), (4, -0x58003), (8, 0x3601), (12, 0x1c000), (24, 0xc000)]:
                    write(0xff7000+off, value, 4)
                write(0xff701c, [-1, 0, 10][seed%3], 2)
                for i in range(16):
                    write(0xff84a8+i, 0 if i >= 8 else seed%16)
                write_bytes(0xffa200, bytes((i*19+seed*31+91)&255 for i in range(0x600)))
                for i in range(4):
                    write(0xffa004+i*2, [0, 240, 256][(seed+i)%3], 2)
                write(0xffa00c, 0, 2)
                raw = bytearray((i*37+seed*23+13)&255 for i in range(0x800))
                for i in range(16):
                    raw[i*128+4:i*128+6] = (0x80 if seed%3 == 2 else 0).to_bytes(2, 'big')
                write_bytes(0xff9400, raw)
                write(0xffa036, 0xff80 if seed%2 else 0, 2)
                write(0xffa03a, 0, 2)
                write(0xffa042, 0xa0)
                write(0xffa030, 0x97 if seed%2 else 0)
                write(0xffa010, 32, 2)
                write(0xffa012, 600, 2)
                write_bytes(0xfff80a, bytes([0x80, 0x81, 0x82, 0x83] if seed%2 else [0, 0x80, 0, 0x81]))
                write(0xffff18, [0, 1, 0x12345678, 0xffffffff][seed%4], 4)

            def invoke(name):
                nonlocal_label = name
                # label belongs to the outer module invocation, not CPU state.
                nonlocal_set_label(nonlocal_label)
                setreg(17, 0x2700)
                setreg(15, 0xffffe0)
                write(0xffffe0, 0x3d8, 4)
                setreg(13, 0xff8400)
                setreg(16, 0x15a18)
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                before = ref.ram_cpu()
                count = len(isolated)
                hook(observer)
                execute(2000000)
                hook(None)
                assert len(isolated) == count+1 and not pending and not errors, (name, errors, list(pending))
                after = ref.ram_cpu()
                allowed = set(range(0x8400, 0x8800)) | set(range(0x9400, 0x9c00)) | set(range(0xa200, 0xa800)) | set(range(0xa004, 0xa00e)) | {0xa030, 0xa036, 0xa037, 0xa042} | set(range(0xf568, 0xf56c)) | set(range(0xf80a, 0xf80e)) | set(range(0xff18, 0xff1c)) | set(range(0xff80, 0xffe4))
                for owner in [0x8800, 0x8c00, 0x9000]:
                    allowed.update([owner+0xb8, owner+0xb9])
                unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
                assert not unexpected, (name, unexpected)
                return isolated[-1]

            def nonlocal_set_label(name):
                nonlocal label
                label = name

            profiles = [(0, 0, 0, 0), (8, 16, 16, 24), (4, 64, 64, 68), (1, 1, 1, 1), (0, 80, 112, 80), (0x20, 0, 1, 0x20)]
            for seed, (action, facing, profile, stop, completed, edge) in enumerate(itertools.product(ACTION_POSES, [0, 1, 3], range(len(profiles)), [0, 1], [0, 1], [False, True])):
                held, pressed, changed, raw = profiles[profile]
                setup(seed, action, facing, held, pressed, changed, raw, stop, completed, edge)
                invoke(f'action-{action:x}/facing-{facing}/input-{profile}/stop-{stop}/complete-{completed}/edge-{edge}')
            boundary_count = len(isolated)
            for stream_index, (initial, duration) in enumerate([(0, 90), (2, 90), (10, 90), (5, 90), (15, 90), (65, 90), (66, 160), (67, 90)]):
                setup(stream_index, initial)
                calls, previous = [], 0
                for step in range(duration):
                    held = 8 if step < 12 else 16 if 20 <= step < 22 else 0
                    changed = previous^held
                    external = {'held': held, 'pressed': held&changed, 'changed': changed, 'raw_held': held}
                    for field, off in [('held', 0x71), ('pressed', 0x72), ('changed', 0x73), ('raw_held', 0x74)]:
                        write(0xff8400+off, external[field])
                    call = invoke(f'stream-{stream_index}/{step}')
                    call['external_input'] = external
                    calls.append(call)
                    previous = held
                sequences.append({'initial': calls[0]['before'], 'calls': calls})
    for name, calls, scope in [('chu-action-oracle', natural, 'Full natural action-routine returns across two original baselines'),
                               ('chu-action-boundaries', isolated[:boundary_count], 'Declared isolated full action routines'),
                               ('chu-action-sequences', sequences, 'Single-initial-state complete action-routine streams')]:
        data = {'library': list(library.values()), 'calls': calls}
        raw = (json.dumps(data, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        rows = [call for stream in calls for call in stream['calls']] if name.endswith('-sequences') else calls
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(calls), 'calls': len(rows),
                  'actions': dict(Counter(str(r['before']['tail']['body']['action_kind'])for r in rows)), 'library_frames': len(library), 'baselines': baselines,
                  'external_queue_calls': sum(bool(r['external_queue_clears'])for r in rows), 'one_rng_call_each': all(r['rng_calls'] == 1 for r in rows),
                  'frames_match_original_core': all(b['match']for b in baselines), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed),
                  'incomplete_calls': 0, 'errors': errors, 'pass': bool(calls)and not errors,
                  'limits': ['Complete15a18 rule routine only, not outer fighter/world/collision scheduling or matches', 'External queue consumer calls are preserved as independent evidence']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
