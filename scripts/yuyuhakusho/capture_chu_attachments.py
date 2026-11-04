#!/usr/bin/env python3
"""Original Chu charge/release attachment rules, before presentation dispatch."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

ENTRIES = {0x132de: 'charge', 0x1361e: 'release'}


def num(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, address, kind):
    off = address&0xffff
    parent = off-(0x200 if kind == 'charge' else 0x300)
    return {'object_id': off, 'actor_bytes': ram[off:off+256].hex(), 'parent_bytes': ram[parent:parent+256].hex(),
            'buffers': ram[0xa200:0xa600].hex(), 'offsets': [num(ram, 0xa004+i*2, 2) for i in range(4)],
            'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]),
            'world': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'attribute_mask': num(ram, 0xa03e, 2),
                      'camera_x': num(ram, 0xa014, 2, True), 'camera_y': num(ram, 0xa016, 2, True), 'shake': num(ram, 0xf566, 2, True)}}


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
    points = list(ENTRIES)+[0x1580, 0x3556, 0x3574, 0x136c6, 0x37c56, 0x37cac]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    input_path = ROOT/'games/yuyuhakusho/engine/testdata/chu-special-motion-inputs.json'
    events = json.loads(input_path.read_text())
    with ReferenceSession(ROOT, out/'unobserved') as ref:
        expected = [ref.step(f, buttons_at(events, f))['reference'] for f in range(1, 3601)]
        unobserved_identity = ref.identity
    natural, isolated, streams, errors = [], [], [], []
    pending, animation_library = {}, {}
    sequence_library = []
    for index in range(8):
        address = 0x13492+index*6
        raw = rom[address:address+6]
        frames = [{'parameter': raw[i]*2, 'duration': raw[i+1]} for i in range(0, len(raw), 2)]
        assert frames[-1]['duration'] == 0 and all(f['duration'] for f in frames[:-1])
        sequence_library.append({'id': address, 'frames': frames, 'source': raw.hex(), 'sha256': sha(raw)})
    private, label = False, ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                address = ref.get_register(13)&0xffffff
                key = (address, ref.get_register(15))
                if pc in ENTRIES:
                    assert not pending, 'Nested attachment callback'
                    pending[key] = {'kind': ENTRIES[pc], 'frame': ref.frame, 'before': snapshot(ref.ram_cpu(), address, ENTRIES[pc]),
                                    'external_queue_clears': [], 'cleared_bytes': 0}
                elif pc in (0x37c56, 0x37cac):
                    for row in pending.values():
                        a = ref.get_register(9)&0xffff
                        row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a], 'sr': ref.get_register(17), 'sp': ref.get_register(15)})
                elif key in pending:
                    row = pending[key]
                    ram = ref.ram_cpu()
                    if pc == 0x136c6:
                        k, data, source = selected_data(rom, ram, ram[address&0xffff:(address&0xffff)+256])
                        identity = json.dumps(k, sort_keys=True)
                        assert identity not in animation_library or animation_library[identity]['data'] == data
                        animation_library[identity] = {'key': k, 'data': data, 'source': source}
                    elif pc == 0x3556:
                        size = num(ram, (address&0xffff)+4, 2)
                        assert size == 0 or 16 <= size <= 256, size
                        row['cleared_bytes'] = (size if size else 64)//16*16
                    if pc == 0x1580 or pc == 0x3574 and row['cleared_bytes']:
                        row.update(after=snapshot(ram, address, row['kind']), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame)
                        assert row['before']['parent_bytes'] == row['after']['parent_bytes']
                        pending.pop(key)
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
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not errors and natural, (len(natural), errors, list(pending))
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def fill(address, size, seed):
            for i in range(size):
                write(address+i, (i*73+seed*29+17)&255)

        def setup(kind, seed=0, **changes):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            off = 0x8600 if kind == 'charge' else 0x8700
            address = 0xff0000+off
            fill(0xff8400, 256, seed+1)
            fill(address, 256, seed)
            fields = {0:(0x132de if kind == 'charge' else 0x1361e, 4), 4:(256, 2), 6:(2, 2), 8:(0x180, 2), 12:(0x140, 2),
                      0x10:(0x0014abcd, 4), 0x14:(-0x0027edcc, 4), 0x1c:(0xd8f1a, 4), 0x20:(0x240000, 4), 0x24:(0xbfff, 2),
                      0x28:(-3, 2), 0x2a:(7, 2), 0x3c:(2, 2), 0x50:(0x13492 if kind == 'charge' else 0xd92ca, 4),
                      0x54:(-13, 2), 0x56:(17, 2), 0x58:(9, 1), 0x59:(0, 1), 0x5a:(4, 1), 0x5b:(1, 1), 0x5c:(0, 1),
                      0x5f:(26, 1), 0x60:(0, 1), 0x61:(seed%4, 1), 0x6d:(0, 1), 0x78:(123, 2), 0x7c:(0, 2), 0x8c:(0xd9a3a, 4)}
            aliases = {'stage':6, 'size':4, 'power':0x7c, 'index':0x59, 'ticks':0x5b, 'duration':0x5a, 'flags':0x5c, 'completed':0x60, 'pose':0x5f}
            for key, value in changes.items():
                if key in aliases:
                    field = aliases[key]
                    fields[field] = (value, fields[field][1])
            for field, (value, size) in fields.items():
                write(address+field, value, size)
            for field, value, size in [(0x58, changes.get('parent_character', 9), 1), (0x59, changes.get('parent_index', 4), 1),
                                       (0x5f, changes.get('parent_pose', 0xa6 if kind == 'charge' else 0xa7), 1),
                                       (0x7c, changes.get('parent_power', 255), 2), (0x40, 0x7fff1234, 4), (0x44, -0x80004321, 4)]:
                write(0xff8400+field, value, size)
            for i in range(0x400):
                write(0xffa200+i, (i*19+seed*31+91)&255)
            for i in range(4):
                write(0xffa004+i*2, [0, 240, 256][(seed+i)%3], 2)
            write(0xffa030, 0x97 if seed%2 else 0)
            for i, value in enumerate([0x80, 0x81, 0x82, 0x83] if seed%2 else [0, 0x80, 0, 0x81]):
                write(0xfff80a+i, value)
            for field, value, size in [(0xf550, changes.get('pause', 0), 1), (0xf55a, changes.get('slow', 0), 1),
                                       (0xa03e, [0, 0x2000, 0xffff][seed%3], 2), (0xa014, [0, 0x7fff, -32768, -1][seed%4], 2),
                                       (0xa016, [-32768, -1, 0, 32767][seed%4], 2), (0xf566, [-32768, -1, 0, 32767][(seed+1)%4], 2)]:
                write(0xff0000+field, value, size)

        def invoke(kind, name):
            nonlocal label
            label = name
            off = 0x8600 if kind == 'charge' else 0x8700
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff0000+off)
            for reg in range(8):
                setreg(reg, 0)
            setreg(16, 0x132de if kind == 'charge' else 0x1361e)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu()
            count = len(isolated)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (name, errors, list(pending))
            after = ref.ram_cpu()
            allowed = set(range(off, off+256)) | set(range(0xa200, 0xa600)) | set(range(0xa004, 0xa00c)) | {0xa030} | set(range(0xf80a, 0xf80e)) | set(range(0xff80, 0xffe4))
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return isolated[-1]

        powers = [-32768, -1, 0, 31, 32, 63, 64, 127, 128, 223, 224, 255, 256, 8192, 8224, 32767]
        for seed, (stage, pause, slow, power, parent_power, parent_character) in enumerate(itertools.product([0, 2], [0, 128], [0, 1], powers, [-32768, -1, 0, 15, 16, 31, 255, 32767], [0, 3, 9, 1])):
            setup('charge', seed, stage=stage, pause=pause, slow=slow, power=power, parent_power=parent_power, parent_character=parent_character)
            invoke('charge', f'charge-{stage}-{pause}-{slow}-{power}-{parent_power}-{parent_character}')
        for seed, (character, parent_index, parent_pose, size, stage) in enumerate(itertools.product([0, 3, 9, 1], [0, 3, 4, 5, 255], [0xa6, 0xa7, 0], [0, 16, 31, 64, 128, 256], [1, 65535])):
            setup('charge', seed, stage=stage, parent_character=character, parent_index=parent_index, parent_pose=parent_pose, size=size)
            invoke('charge', f'charge-lifetime-{character}-{parent_index}-{parent_pose}-{size}-{stage}')
        for seed, (index, ticks) in enumerate([(0, 0), (0, 1), (0, 2), (1, 1), (1, 255), (2, 2), (255, 1)]):
            setup('charge', seed, index=index, ticks=ticks, duration=255, power=0, parent_power=0)
            invoke('charge', f'charge-clock-{index}-{ticks}')
        configs = [(0, 0, 4, 0), (0, 3, 4, 0), (0, 255, 8, 0), (1, 3, 4, 0), (2, 3, 4, 128), (1, 0, 255, 0)]
        for seed, (stage, pause, slow, completed, parent_pose, pose, config) in enumerate(itertools.product([0, 2, 65535], [0, 128], [0, 1], [0, 1, 2, 255], [0xa7, 0xa6, 0], range(26, 33), configs)):
            index, ticks, duration, flags = config
            setup('release', seed, stage=stage, pause=pause, slow=slow, completed=completed, parent_pose=parent_pose, pose=pose, index=index, ticks=ticks, duration=duration, flags=flags)
            invoke('release', f'release-{stage}-{pause}-{slow}-{completed}-{parent_pose}-{pose}-{config}')
        for seed, size in enumerate([0, 16, 31, 64, 128, 256]):
            setup('release', seed, size=size, parent_pose=0)
            invoke('release', f'release-clear-{size}')
        boundary_count = len(isolated)
        for kind in ['charge', 'release']:
            for variant in range(4):
                setup(kind, variant, stage=0, pose=26+variant, parent_power=0)
                initial = snapshot(ref.ram_cpu(), 0xff8600 if kind == 'charge' else 0xff8700, kind)
                calls = []
                for step in range(80):
                    external = {'x': ((100+step)<<16)|0xabcd, 'y': (320<<16)|0x1234,
                                'parent_power': min(step*8, 255), 'parent_pose': (0xa7 if step >= 40 else 0xa6) if kind == 'charge' else 0xa7,
                                'pause': 128 if variant&1 and step%4 == 1 else 0, 'slow': 1 if variant&2 and step%4 == 2 else 0}
                    for field, value, size in [(0x40, external['x'], 4), (0x44, external['y'], 4), (0x7c, external['parent_power'], 2), (0x5f, external['parent_pose'], 1)]:
                        write(0xff8400+field, value, size)
                    write(0xfff550, external['pause'])
                    write(0xfff55a, external['slow'])
                    row = invoke(kind, f'stream-{kind}-{variant}/{step}')
                    row['external'] = external
                    calls.append(row)
                    if row['cleared_bytes']:
                        break
                assert step < 79, (kind, variant)
                streams.append({'kind': kind, 'initial': initial, 'calls': calls})
        identity = ref.identity
    for name, calls, scope in [('chu-attachments-oracle', natural, 'Natural complete attachment rules before renderer dispatch'),
                               ('chu-attachments-boundaries', isolated[:boundary_count], 'Explicit original CPU attachment rule cases'),
                               ('chu-attachments-sequences', streams, 'Single-initial-state attachment lifetimes with declared parent/world inputs')]:
        payload = {'sequences': sequence_library, 'animations': list(animation_library.values()), 'calls': calls}
        raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        rows = [call for stream in calls for call in stream['calls']] if name.endswith('-sequences') else calls
        report = {'scope': scope, 'identity': identity, 'unobserved_identity': unobserved_identity,
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()), 'inputs_sha256': sha(input_path.read_bytes()),
                  'records': len(calls), 'calls': len(rows), 'kind_counts': dict(Counter(r['kind'] for r in rows)),
                  'draw_calls': sum(r['draw'] for r in rows), 'delete_calls': sum(bool(r['cleared_bytes']) for r in rows),
                  'external_queue_calls': sum(bool(r['external_queue_clears']) for r in rows), 'animation_frames': len(animation_library), 'charge_sequences': len(sequence_library),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'frames_match_original_core': frames == expected,
                  'incomplete_calls': 0, 'errors': errors, 'pass': bool(calls) and not errors,
                  'limits': ['Rules end before0x1580 sprite submission or after0x3556 clear; not full presentation callback/world step',
                             'Original clear sizes1..15 imply large wrapped DBF writes and are outside validated storage bounds',
                             'Declared parent/world changes are external inputs, never expected child-state injection']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
