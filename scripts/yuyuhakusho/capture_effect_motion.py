#!/usr/bin/env python3
"""Complete generic FX rules, all37 type bindings and21 finite sequences."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def num(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def extract_library(rom):
    definitions, sources, sequences = [], [], {}
    for kind in range(37):
        entry = 0x14a00+kind*2
        target = entry+num(rom, entry, 2, True)
        pointer, sound = target, 0
        if rom[pointer:pointer+2] == bytes.fromhex('103c'):
            sound = rom[pointer+3]
            assert rom[pointer+4:pointer+10] == bytes.fromhex('4eb900021960')
            pointer += 10
        assert rom[pointer:pointer+2] == bytes.fromhex('2b7c') and num(rom, pointer+6, 2) == 0x50
        sequence = num(rom, pointer+2, 4)
        if kind < 16:
            assert rom[pointer+8:pointer+10] == bytes.fromhex('002d') and num(rom, pointer+12, 2) == 0x24
            attr = rom[pointer+11] << 8
            definition = {'sequence': sequence, 'graphic_set': num(rom, 0x14ba2, 4), 'tile_base': num(rom, 0x14baa, 4),
                          'set_tile_base': True, 'attribute_or': attr, 'clear_velocity': True, 'dynamic_tiles': True,
                          'set_skip_tiles': False, 'skip_tiles': 0, 'sound': sound}
            end = pointer+14
        else:
            assert rom[pointer+8:pointer+10] == bytes.fromhex('2b7c') and num(rom, pointer+14, 2) == 0x1c
            assert rom[pointer+16:pointer+18] == bytes.fromhex('006d') and num(rom, pointer+20, 2) == 0x24
            assert rom[pointer+22:pointer+28] == bytes.fromhex('1b7c00010027')
            assert rom[pointer+28:pointer+30] == bytes.fromhex('4e75')
            definition = {'sequence': sequence, 'graphic_set': num(rom, pointer+10, 4), 'tile_base': 0,
                          'set_tile_base': False, 'attribute_or': num(rom, pointer+18, 2), 'clear_velocity': False, 'dynamic_tiles': False,
                          'set_skip_tiles': True, 'skip_tiles': 1, 'sound': sound}
            end = pointer+30
        definitions.append(definition)
        sources.append({'type': kind, 'entry': hex(entry), 'target': hex(target), 'bytes': rom[target:end].hex()})
    pointers = sorted({d['sequence'] for d in definitions})
    assert len(pointers) == 21
    for i, pointer in enumerate(pointers):
        end = pointers[i+1] if i+1 < len(pointers) else 0x14ec6
        frames = []
        for index in range(256):
            a = pointer+index*2
            assert a+2 <= end
            frames.append({'graphic': rom[a], 'duration': rom[a+1]})
            if frames[-1]['duration'] == 0:
                break
        assert frames[-1]['duration'] == 0
        sequences[str(pointer)] = frames
    return {'types': definitions, 'sequences': sequences}, {'bindings': sources, 'common_initializer': {'address': '0x14b98', 'bytes': rom[0x14b98:0x14bd2].hex()}}


def snapshot(ram, address, owner_ids=None):
    off = address&0xffff
    owner = num(ram, off+0x48, 2)
    if owner_ids is None:
        owner_ids = [owner] if owner and owner != off and 0x8000 <= owner <= 0xff80 else []
    owners = {str(i): ram[i:i+128].hex() for i in owner_ids}
    return {'actor_bytes': ram[off:off+128].hex(), 'pool_bytes': ram[0x9400:0x9c00].hex(), 'owner_bytes': owners,
            'world': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': num(ram, 0xa03e, 2),
                      'tile_cursor': ram[0xa031], 'sound_queue': list(ram[0xf80a:0xf80e]),
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
    returns = {0x14bd0, 0x14bee, 0x14c16, 0x14c3e, 0x14c66, 0x14c8e, 0x14cb6, 0x14cde, 0x14d06, 0x14d2e, 0x14d4c, 0x14d6a, 0x14a64}
    points = [0x149ac, 0x14ac8, 0x3556, 0x3574, 0x37c56, 0x37cac, *sorted(returns)]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    library, source = extract_library(rom)
    (out/'effect-sequences.json').write_text(json.dumps({'schema': 1, 'rom_sha256': sha(rom), 'library': library, 'source': source}, indent=2)+'\n')
    input_path = ROOT/'games/yuyuhakusho/engine/testdata/chu-special-motion-inputs.json'
    events = json.loads(input_path.read_text())
    with ReferenceSession(ROOT, out/'unobserved') as ref:
        expected = [ref.step(f, buttons_at(events, f))['reference'] for f in range(1, 3601)]
        unobserved_identity = ref.identity
    natural, isolated, sequences, errors = [], [], [], []
    pending = {}
    private, label = False, ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                address = ref.get_register(13)&0xffffff
                key = (address, ref.get_register(15))
                if pc == 0x149ac:
                    assert not pending, 'Overlapping generic FX callback'
                    pending[key] = {'frame': ref.frame, 'object_id': address&0xffff, 'before': snapshot(ref.ram_cpu(), address),
                                    'deleted': False, 'external_queue_clears': []}
                elif pc in (0x37c56, 0x37cac):
                    for row in pending.values():
                        a = ref.get_register(9)&0xffff
                        row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                elif pc == 0x3556 and key in pending:
                    row = pending[key]
                    row['deleted'] = True
                    size = num(bytes.fromhex(row['before']['actor_bytes']), 4, 2)
                    count = ((size if size else 64)>>4)*16
                    assert 0 < count <= 128
                    row['cleared_bytes'] = count
                elif key in pending and (pc in returns or pc == 0x14ac8 or pc == 0x3574 and pending[key]['deleted']):
                    row = pending.pop(key)
                    owner_ids = [int(i) for i in row['before']['owner_bytes']]
                    row.update(after=snapshot(ref.ram_cpu(), address, owner_ids), draw=pc == 0x14ac8,
                               cleared_bytes=row.get('cleared_bytes', 0), exit=hex(pc), exit_frame=ref.frame)
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
        assert natural and frames == expected and not pending and not errors, (len(natural), errors, list(pending))
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def fill(address, n, seed):
            for i in range(n):
                write(address+i, (i*73+seed*29+17)&255)

        def setup(seed=0, **changes):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            fill(0xff9400, 2048, seed)
            values = {'stage': 2, 'size': 128, 'type': 16, 'index': 255, 'delay': 0, 'ticks': 1, 'flags': 0x81,
                      'owner': 0, 'x': 0x7fffabcd, 'y': -0x7fff1235, 'vx': 0x7fffffff, 'vy': -0x7fffffff,
                      'attributes': 0x98ef, 'pause': 0, 'slow': 0, 'cursor': 0, 'palette': 0x2000}
            values.update(changes)
            fields = {'stage': (6, 2), 'size': (4, 2), 'type': (0x58, 1), 'index': (0x59, 1), 'delay': (0x5a, 1), 'ticks': (0x5b, 1),
                      'flags': (0x5c, 1), 'owner': (0x48, 2), 'x': (0x40, 4), 'y': (0x44, 4), 'vx': (0x10, 4), 'vy': (0x14, 4), 'attributes': (0x24, 2)}
            for key, (off, n) in fields.items():
                write(0xff9400+off, values[key], n)
            write(0xff9400, 0x149ac, 4)
            pointer = values.get('sequence', library['types'][values['type']]['sequence'])
            write(0xff9450, pointer, 4)
            write(0xfff550, values['pause'])
            write(0xfff55a, values['slow'])
            write(0xffa031, values['cursor'])
            write(0xffa03e, values['palette'], 2)
            for a, v in [(0xffa014, -32768 if seed%2 else 31), (0xffa016, 32767 if seed%2 else -23), (0xfff566, -1 if seed%2 else 0)]:
                write(a, v, 2)
            for owner in [0x8400, 0x9000]:
                write(0xff0000+owner+0x40, 0x80001234 if seed%2 else 0x00319876, 4)
                write(0xff0000+owner+0x44, 0x7fff5678 if seed%2 else 0xffff1357, 4)
            queue = ([0x80, 0x81, 0x82, 0x83], [0, 0x80, 0, 0x81], [0x80, 0xa5, 0x82, 0x83])[seed%3]
            for i, value in enumerate(queue):
                write(0xfff80a+i, value)

        def invoke(name):
            nonlocal label
            label = name
            for reg in range(8):
                setreg(reg, 0)
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff9400)
            setreg(16, 0x149ac)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu()
            count = len(isolated)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (name, errors, list(pending))
            after = ref.ram_cpu()
            allowed = set(range(0x9400, 0x9480)) | {0xa031} | set(range(0xf80a, 0xf80e)) | set(range(0xff80, 0xffe4))
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return isolated[-1]

        for seed, (kind, attributes, cursor, paused) in enumerate(itertools.product(range(37), [0, 0x98ef, 0xffff], [0, 16, 48, 63, 64, 127, 128, 240, 255], range(3))):
            setup(seed, stage=0, type=kind, attributes=attributes, cursor=cursor, pause=0x80 if paused == 1 else 0,
                  slow=255 if paused == 2 else 0, delay=[0, 1, 255][seed%3], owner=[0, 0x8400, 0x9400][seed%3], palette=[0, 0x2000, 0xffff][seed%3])
            invoke(f'init-{kind}-{attributes}-{cursor}-{paused}')
        sequence_items = list(library['sequences'].items())
        for seed, ((pointer, frames_data), ticks, delay, paused, owner) in enumerate(itertools.product(sequence_items, [0, 1, 2, 255], [0, 1, 255], range(3), [0, 0x8400, 0x9400])):
            index = [255, 0, len(frames_data)-2][seed%3]
            setup(seed, sequence=int(pointer), ticks=ticks, delay=delay, pause=0x80 if paused == 1 else 0, slow=1 if paused == 2 else 0, owner=owner, index=index)
            invoke(f'clock-{pointer}-{index}-{ticks}-{delay}-{paused}-{owner}')
        for seed, (owner, x, y, vx, vy) in enumerate(itertools.product([0, 0x8400, 0x9000, 0x9400], [0x7fffffff, -0x80000000, -1, 0x10001], [0x7fffffff, -0x80000000, -1, 0x10001], [1, -1], [1, -1])):
            setup(seed, ticks=2, owner=owner, x=x, y=y, vx=vx, vy=vy, index=0)
            invoke(f'motion-{owner}-{x}-{y}-{vx}-{vy}')
        for seed, ((pointer, frames_data), size) in enumerate(itertools.product(sequence_items, [0, 31, 143, *range(16, 129, 16)])):
            setup(seed, sequence=int(pointer), ticks=1, index=len(frames_data)-2, size=size, owner=0x8400)
            invoke(f'clear-{pointer}-{size}')
        boundary_count = len(isolated)
        for kind, owner in itertools.product(range(37), [0, 0x8400, 0x9400]):
            setup(kind, stage=0, type=kind, index=42, delay=kind%3, owner=owner, x=(100<<16)|0xabcd, y=(200<<16)|0x5678, vx=0x10001, vy=-0x10001)
            initial = snapshot(ref.ram_cpu(), 0xff9400)
            calls = []
            for step in range(512):
                external = {}
                if owner == 0x8400 and step > 0:
                    x, y = ((100+step)<<16)|0x1234, ((200-step)<<16)|0x9876
                    write(0xff8440, x, 4)
                    write(0xff8444, y, 4)
                    external = {'owner': owner, 'x': x, 'y': y}
                row = invoke(f'sequence-{kind}-{owner}/{step}')
                row['external_input'] = external
                calls.append(row)
                if row['deleted']:
                    break
            assert calls[-1]['deleted'], (kind, owner)
            sequences.append({'initial': initial, 'calls': calls})
        identity = ref.identity
    for name, records, scope in [('effect-motion-oracle', natural, 'Natural complete generic FX rule phases'),
                                 ('effect-motion-boundaries', isolated[:boundary_count], 'Original CPU initialization, clock, motion and deletion boundaries'),
                                 ('effect-motion-sequences', sequences, 'Single-initial-state complete FX lifetimes to deletion')]:
        data = {'library': library, 'calls': records}
        raw = (json.dumps(data, separators=(',', ':'))+'\n').encode()
        z = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(z)
        calls = [c for s in records for c in s['calls']] if name.endswith('-sequences') else records
        report = {'scope': scope, 'identity': identity, 'unobserved_identity': unobserved_identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(bridge_path.read_bytes()), 'inputs_sha256': sha(input_path.read_bytes()), 'records': len(records), 'calls': len(calls),
                  'type_counts': dict(Counter(str(bytes.fromhex(c['before']['actor_bytes'])[0x58]) for c in calls)),
                  'draws': sum(c['draw'] for c in calls), 'deletions': sum(c['deleted'] for c in calls), 'direct_returns': sum(not c['draw'] and not c['deleted'] for c in calls),
                  'types': len(library['types']), 'sequences': len(library['sequences']), 'external_queue_calls': sum(bool(c['external_queue_clears']) for c in calls),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(z), 'frames_match_original_core': frames == expected, 'errors': errors, 'incomplete_calls': 0,
                  'pass': bool(records) and not errors, 'limits': ['Generic149ac callback only, not14930/14ec6 or other effect behaviors', 'Rules stop at draw request before1580 or original direct return/deletion', 'No hardware sprite encoding or whole world/effect-pool scheduling']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
