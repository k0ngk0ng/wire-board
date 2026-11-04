#!/usr/bin/env python3
"""All eleven complete original character command-preparation routines."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_commands import state


def extract(rom):
    characters, library = [], {}
    for character_id in range(11):
        table_entry = 0x15c20+character_id*2
        entry = table_entry+int.from_bytes(rom[table_entry:table_entry+2], 'big', signed=True)
        cursor, bindings = entry, []
        while rom[cursor:cursor+2] != bytes.fromhex('4e75'):
            assert cursor-entry < 8*18
            assert rom[cursor:cursor+2] == bytes.fromhex('227c')
            assert rom[cursor+6:cursor+8] == bytes.fromhex('247c')
            assert rom[cursor+12:cursor+14] == bytes.fromhex('4eb9')
            slot = int.from_bytes(rom[cursor+2:cursor+6], 'big')
            pointer = int.from_bytes(rom[cursor+8:cursor+12], 'big')
            helper = int.from_bytes(rom[cursor+14:cursor+18], 'big')
            assert slot < 8 and helper in [0x1e8d0, 0x1e972]
            kind = 'direction' if helper == 0x1e8d0 else 'buttons'
            bindings.append({'slot': slot, 'kind': kind, 'program': pointer})
            if str(pointer) not in library:
                for n in range(1, 257):
                    if rom[pointer+n] == 255:
                        library[str(pointer)] = {'window': rom[pointer], 'steps': list(rom[pointer+1:pointer+n+1])}
                        break
                else:
                    raise ValueError('Unterminated command program')
            cursor += 18
        assert len({b['slot'] for b in bindings}) == len(bindings)
        characters.append({'character_id': character_id, 'entry': entry, 'exit': cursor, 'bindings': bindings,
                           'instructions_sha256': sha(rom[entry:cursor+2])})
    return characters, library


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    characters, library = extract(rom)
    entries = {c['entry']: c['character_id'] for c in characters}
    exits = {c['exit']: c['character_id'] for c in characters}
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = list(entries)+list(exits)
    pcs = (C.c_uint*len(points))(*points)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, sequences, errors = [], [], [], []
    pending, private, sample = {}, False, None
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal sample
            try:
                address, sp = ref.get_register(13)&0xffffff, ref.get_register(15)
                if private:
                    if pc in exits and sp == 0xffffe0:
                        assert sample is None, 'Multiple private preparation returns'
                        sample = (ref.ram_cpu(), pc)
                        ref.cpu.cycles = 2000000
                    return
                character_id = entries.get(pc, exits.get(pc))
                key = (address, sp)
                off = address&0xffff
                obj = ref.ram_cpu()[off:off+256]
                if pc in entries:
                    assert key not in pending and obj[0x58] == character_id
                    pending[key] = {'character_id': character_id, 'frame': ref.frame, 'actor': address,
                                    'before': state(obj), 'actor_before': obj.hex()}
                else:
                    row = pending.pop(key)
                    assert row['character_id'] == character_id
                    unknown = [i for i, (a, b) in enumerate(zip(bytes.fromhex(row['actor_before']), obj))
                               if a != b and i != 0x6b and not 0xa8 <= i < 0xb8]
                    assert not unknown, unknown
                    row.update(after=state(obj), actor_after=obj.hex(), exit=pc, exit_frame=ref.frame)
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
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not errors, (errors[:5], len(pending))
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            data = (value&((1 << (size*8))-1)).to_bytes(size, 'big')
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def setup(character_id, facing, raw, request=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for i in range(256):
                write(0xff8400+i, (i*73+character_id*37+19)&255)
            write(0xff8458, character_id)
            write(0xff8461, facing)
            write(0xff8474, raw)
            write(0xff846b, request)
            for slot in range(8):
                write(0xff84a8+slot, 0x80+slot)
                write(0xff84b0+slot, 0x90+slot)
            for binding in characters[character_id]['bindings']:
                slot = binding['slot']
                write(0xff84a8+slot, 0)
                write(0xff84b0+slot, 0)

        def invoke(character_id, name):
            nonlocal sample
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff8400)
            setreg(16, characters[character_id]['entry'])
            before_ram = ref.ram_cpu()
            before = before_ram[0x8400:0x8500]
            sample = None
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            hook(observer)
            execute(1000000)
            hook(None)
            assert sample is not None and not errors, (character_id, name, errors)
            after_ram, pc = sample
            assert pc == characters[character_id]['exit']
            after = after_ram[0x8400:0x8500]
            unknown = [i for i, (a, b) in enumerate(zip(before_ram, after_ram))
                       if a != b and i != 0x846b and not 0x84a8 <= i < 0x84b8 and not 0xffb0 <= i < 0xffe4]
            assert not unknown, unknown
            return {'character_id': character_id, 'name': name, 'before': state(before), 'after': state(after),
                    'actor_before': before.hex(), 'actor_after': after.hex(), 'exit': pc}

        for character_id in range(11):
            bindings = characters[character_id]['bindings']
            for mask, facing, raw in itertools.product(range(256), [0, 3], [0, 0x50]):
                setup(character_id, facing, raw, [0, 1, 8, 99][mask%4])
                for b in bindings:
                    slot, data = b['slot'], library[str(b['program'])]
                    write(0xff84a8+slot, data['window']-1)
                    write(0xff84b0+slot, len(data['steps'])-1 if mask&(1 << slot) else 0)
                row = invoke(character_id, f'complete-mask-{mask:02x}/facing-{facing}/held-{raw}')
                row['completed_slot_mask'] = mask
                isolated.append(row)
            # Generic helpers already have exhaustive boundary matrices. These
            # combined states exercise all raw bytes with varied valid indices,
            # byte timeout wrap and requests through each character's real order.
            for raw in range(256):
                facing = [0, 1, 128][raw%3]
                setup(character_id, facing, raw, raw%10)
                for j, b in enumerate(bindings):
                    data, slot = library[str(b['program'])], b['slot']
                    write(0xff84a8+slot, [0, data['window']-1, 255][(raw+j)%3])
                    write(0xff84b0+slot, (raw*7+j)%len(data['steps']))
                isolated.append(invoke(character_id, f'mixed-held-{raw}/facing-{facing}'))
            for facing in [0, 1, 128]:
                setup(character_id, facing, 0)
                initial = state(ref.ram_cpu()[0x8400:0x8500])
                calls = []
                for b in bindings:
                    data = library[str(b['program'])]
                    stream = list(data['steps'][:-1])+[0]*(data['window']+2)
                    if b['kind'] == 'direction' and facing != 0:
                        stream = [(x&~12) | (x >> 1)&4 | (x << 1)&8 for x in stream]
                    for raw in stream:
                        write(0xff8474, raw)
                        row = invoke(character_id, f'continuous/{len(calls)}')
                        row['raw_input'] = raw
                        calls.append(row)
                sequences.append({'character_id': character_id, 'initial': initial, 'calls': calls})
        identity = ref.identity
    for name, calls, scope in [('character-commands-oracle', natural, 'Passive complete character-preparation calls'),
                               ('character-commands-boundaries', isolated, 'Declared isolated complete character-preparation calls'),
                               ('character-commands-sequences', sequences, 'Single-initial-state original character-preparation streams')]:
        payload = {'library': library, 'characters': characters, 'calls': calls}
        raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        report = {'scope': scope, 'identity': identity, 'records': len(calls), 'characters': 11, 'programs': len(library),
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'errors': errors, 'incomplete_calls': 0, 'pass': bool(calls) and not errors}
        if name.endswith('-sequences'):
            report['step_counts'] = [len(s['calls']) for s in calls]
        else:
            report['character_counts'] = dict(Counter(r['character_id'] for r in calls))
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))
    semantic = {'schema': 1, 'rom_sha256': identity['rom_sha256'], 'characters': characters, 'library': library,
                'scope': 'Original semantic command programs and preparation order only; no ROM execution'}
    (out/'command-programs.json').write_text(json.dumps(semantic, indent=2)+'\n')


if __name__ == '__main__':
    main()
