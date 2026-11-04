#!/usr/bin/env python3
"""Independent HUD indexed assets and bounded original-call evidence.

The published asset is derived from fixed ROM resources, never from a captured
window. Reference CPU state and video remain development-only oracle inputs.
"""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, ROM_SHA256, buttons_at, sha


def be(raw, address, size=2):
    return int.from_bytes(raw[address:address + size], 'big')


def decode_resource(rom, source):
    """0x988/0x9ba byte stream; used only by this offline asset producer."""
    pointer = source + 2
    end = pointer + be(rom, source)
    output = bytearray()
    tokens = [0] * 8
    while pointer < end:
        control = rom[pointer]
        pointer += 1
        tokens[control >> 5] += 1
        if control & 128:
            count = ((control >> 2) & 31) + 2
            distance = ((control << 8 | rom[pointer]) & 1023) + 1
            pointer += 1
            assert distance <= len(output)
            for _ in range(count):
                output.append(output[-distance])
        elif control & 32:
            count = (control & 31) + 2
            value = rom[pointer]
            pointer += 1
            if control & 64:
                for _ in range(count):
                    output.extend((value, rom[pointer]))
                    pointer += 1
            else:
                output.extend(bytes([value]) * count)
        elif control & 64:
            count = (control & 31) + 2
            output.extend(rom[pointer:pointer + 2] * count)
            pointer += 2
        else:
            count = (control & 31) + 1
            output.extend(rom[pointer:pointer + count])
            pointer += count
        assert pointer <= end and len(output) <= 65536
    assert pointer == end
    return bytes(output), tokens


def write_gzip(out, name, data, key, facts):
    raw = (json.dumps(data, separators=(',', ':')) + '\n').encode()
    compressed = gzip.compress(raw, mtime=0)
    (out / f'{name}.json.gz').write_bytes(compressed)
    report = {'pass': True, key: sha(raw), 'gzip_sha256': sha(compressed),
              'producer_sha256': sha(Path(__file__).read_bytes()), **facts}
    (out / f'{name}-manifest.json').write_text(json.dumps(report, indent=2) + '\n')
    return report


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert sha(rom) == ROM_SHA256
    resource = 0x21720
    assert be(rom, resource) == 6 and be(rom, resource + 2, 4) == 0x27db80 and be(rom, resource + 6) == 0
    base, tokens = decode_resource(rom, 0x27db80)
    assert len(base) == 512
    names = []
    glyph_codes = set()
    for cid in range(11):
        start = be(rom, 0x109ca + cid * 4, 4)
        assert be(rom, start + 6) == 65535
        name = [be(rom, start + i * 2) for i in range(3)]
        names.append(name)
        glyph_codes.update(name)
    glyphs = []
    for code in sorted(glyph_codes):
        start = 0x24b9e0 + code * 128
        glyphs.append({'code': code, 'tiles': [rom[start + i * 32:start + (i + 1) * 32].hex() for i in range(4)]})
    portraits = []
    for cid in range(1, 12):
        start = 0xdabca + be(rom, 0xdabca + cid * 2)
        transfer, piece = be(rom, start, 4), be(rom, start + 4, 4)
        assert transfer == (0x80000010 | ((cid - 1) * 16) << 16) and piece == 0xc0400f00
        source = 0x260000 + ((transfer & 0x7fffffff) >> 16) * 32
        portraits.append({'parameter': cid * 2, 'tiles': [rom[source + i * 32:source + (i + 1) * 32].hex() for i in range(16)]})
    # The fixed top-window stripe is a direct immediate in30fc, emitted forty
    # times by3102..310e. No captured window cells enter this catalog.
    assert be(rom, 0x30fe, 4) == 0xe00de00d
    asset = {'schema': 1, 'rom_sha256': ROM_SHA256, 'resource_list': resource,
             'base_tiles': [base[i * 32:(i + 1) * 32].hex() for i in range(16)],
             'bar_tiles': [rom[0x271a00 + i * 32:0x271a00 + (i + 1) * 32].hex() for i in range(32)],
             'glyphs': glyphs, 'portraits': portraits, 'window_stripe': {'row': 1, 'columns': 40, 'attributes': 0xe00d}}
    original = json.loads(gzip.decompress((ROOT / 'games/yuyuhakusho/data/arena-assets-reference.json.gz').read_bytes()))
    assert all(bytes.fromhex(frame['active_vram'])[:512] == base for frame in original['frames'])
    asset_report = write_gzip(out, 'hud-assets', asset, 'catalog_sha256',
        {'rom_sha256': ROM_SHA256, 'scope': 'ROM-derived base, bars, name glyphs, portraits and fixed stripe only',
         'base_tiles': 16, 'bar_patterns': 32, 'name_glyphs': len(glyphs), 'portraits': 11,
         'resource_source': 0x27db80, 'compressed_source_sha256': sha(rom[0x27db80:0x27db80 + 2 + be(rom, 0x27db80)]),
         'base_decompression_token_counts': tokens, 'base_matches_original_loaded_frames': len(original['frames']),
         'font_source': 0x24b9e0, 'portrait_tiles_source': 0x260000, 'bar_patterns_source': 0x271a00,
         'reference_window_used_as_asset': False})

    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = sorted({0x3436, 0x800, 0xfe68, 0xfe76, 0xfdbe, 0xfdfa, 0xfdfc, 0xfe66, 0x20dc, 0x15772, 0x158ae})
    pcs = (C.c_uint * len(points))(*points)
    events, calls, checks, errors = [], [], [], []
    pending, hud = {}, {}
    active, private, label = False, False, ''
    selected = {frame['frame'] for frame in original['frames']}
    inputs = json.loads((ROOT / 'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT / 'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    with ReferenceSession(ROOT, out, core=core) as ref:
        def emit(event):
            events.append({'Frame': ref.frame, 'Event': event})

        def capture(kind, event):
            ram = ref.ram_cpu()
            assert kind not in pending
            pending[kind] = {'Name': label or f'natural-{kind}-{len(calls)}', 'Event': event,
                             'Before': ram[0xe600:0xe800].hex(), 'next_name_tile': be(ram, 0xf5a8) >> 5,
                             'sp': ref.get_register(15)}

        def finish(kind):
            row = pending.pop(kind)
            assert row.pop('sp') == ref.get_register(15)
            row['After'] = ref.ram_cpu()[0xe600:0xe800].hex()
            calls.append(row)
            if not private:
                emit(row['Event'])
            else:
                ref.cpu.cycles = 8000000

        @cbtype
        def observe(kind, width, pc, value):
            nonlocal active
            try:
                if pc == 0x3436 and not private and be(ref.ram_cpu(), 0x8040, 4) == 0xf2a4:
                    active = True
                    emit({'Kind': 'start', 'Start': {'kind': 'reset'}})
                elif pc == 0x800 and active and ref.get_register(8) & 0xffffff == resource and not private:
                    emit({'Kind': 'start', 'Start': {'kind': 'resource', 'source': resource}})
                elif pc == 0xfe68 and (active or private):
                    source = ref.get_register(9) & 0xffffff
                    words = []
                    while be(rom, source) != 65535:
                        words.append(be(rom, source))
                        source += 2
                        assert len(words) <= 16
                    capture('words', {'Kind': 'start', 'Start': {'kind': 'words', 'offset': ref.get_register(0) & 65535, 'words': words}})
                elif pc == 0xfe76 and 'words' in pending:
                    finish('words')
                elif pc == 0xfdbe and (active or private):
                    ram = ref.ram_cpu()
                    owner = ref.get_register(8) & 65535
                    cid = ram[owner + 0x58]
                    capture('name', {'Kind': 'start', 'Start': {'kind': 'name', 'object_id': owner, 'offset': be(ram, owner + 0x8a), 'glyphs': names[cid]}})
                elif pc == 0xfdfa and 'name' in pending:
                    finish('name')
                elif pc == 0xfdfc and (active or private):
                    capture('portrait', {'Kind': 'start', 'Start': {'kind': 'portrait'}})
                elif pc == 0x20dc and 'portrait' in pending:
                    ram = ref.ram_cpu()
                    owner = ref.get_register(8) & 65535
                    pending['portrait']['Event']['Start'].update(object_id=owner, Attributes=be(ram, 0x8064), X=be(ram, 0x8048), Y=be(ram, 0x804c), Parameter=be(ram, 0x807c))
                elif pc == 0xfe66 and 'portrait' in pending:
                    finish('portrait')
                elif pc == 0x15772 and active and not private:
                    ram = ref.ram_cpu()
                    owner, sp = ref.get_register(13) & 65535, ref.get_register(15)
                    key = (owner, sp)
                    assert key not in hud
                    hud[key] = (be(ram, owner + 0x7a), be(ram, owner + 0xbc), ram[owner + 0x70])
                elif pc == 0x158ae and active and not private:
                    ram = ref.ram_cpu()
                    owner, sp = ref.get_register(13) & 65535, ref.get_register(15)
                    old_health, old_energy, target = hud.pop((owner, sp))
                    for old, shown, tile in [(old_health, be(ram, owner + 0x7a), 16 + (target >> 2)), (old_energy, be(ram, owner + 0xbc), 20 + (target >> 2))]:
                        if shown != old:
                            emit({'Kind': 'hud', 'ObjectID': owner, 'HUD': {'Pattern': shown & 31, 'TargetTile': tile}})
            except Exception as error:
                errors.append((hex(pc), repr(error)))
                if private:
                    ref.cpu.cycles = 8000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(inputs, frame))['reference'])
            if frame in selected:
                checks.append({'Frame': frame, 'Cursor': len(events), 'Cells': ref.ram_cpu()[0xe600:0xe800].hex()})
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not hud and not errors, (pending, hud, errors[:5])
        natural_calls, natural_events = len(calls), len(events)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, raw):
            for i, byte in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer + (((address + i) & 65535) ^ 1)).value = byte

        for cid in range(11):
            for slot in [-1, 0, 1, 2, 3]:
                ref.restore(checkpoint, restore_refresh_cycles=True)
                for register in range(15):
                    setreg(register, 0)
                write(0xe600, bytes((i * 73 + cid * 11 + slot * 29) & 255 for i in range(512)))
                write(0x8400, bytes(256))
                write(0x8040, bytes(64))
                write(0x8458, bytes([cid]))
                write(0x8470, bytes([max(slot, 0) * 4]))
                write(0x848a, (16).to_bytes(2, 'big'))
                write(0x8424, [0x2200, 0x4280, 0x6300, 0x6380][max(slot, 0)].to_bytes(2, 'big'))
                write(0xf518, (0xd400).to_bytes(2, 'big'))
                write(0xf5a8, (0x400).to_bytes(2, 'big'))
                setreg(8, 0xffff8400)
                setreg(13, 0xffff8040)
                setreg(17, 0x2700)
                setreg(15, 0xffffff00)
                write(0xff00, (0x3d8).to_bytes(4, 'big'))
                label = f'character-{cid}-' + ('name' if slot < 0 else f'portrait-slot-{slot}')
                setreg(16, 0xfdbe if slot < 0 else 0xfdfc)
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                count = len(calls)
                hook(observer)
                execute(6000000)
                hook(None)
                assert len(calls) == count + 1 and not pending and not errors, (label, len(calls), pending, errors[:5])
        identity = ref.identity
    reference_report = write_gzip(out, 'hud-reference', {'Events': events, 'Calls': calls, 'Checks': checks}, 'fixture_sha256',
        {'identity': identity, 'frames_match_original_core': frames == expected,
         'natural_calls': natural_calls, 'isolated_calls': len(calls) - natural_calls,
         'natural_events': natural_events, 'selected_world_cell_inputs': len(checks),
         'bridge_sha256': sha(bridge_path.read_bytes()), 'scope': 'Ordered original start/bar emissions and CPU call before/after512-byte cells, no reference window asset',
         'errors': errors})
    print(json.dumps({'assets': asset_report, 'reference': reference_report}, indent=2))


if __name__ == '__main__':
    main()
