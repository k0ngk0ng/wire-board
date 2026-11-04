#!/usr/bin/env python3
"""Original-CPU sprite submission fixtures and raw presentation source spans.

This research producer does not provide a production renderer. Graphics are
selected from the already verified animation keys; original output comes from
the ROM's own 0x1580 routine, not from a second implementation of that routine.
"""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, ROM_SHA256, buttons_at, sha
from extract_animation_catalog import initializer_bindings, be


def raw_frame(rom, character, parameter):
    table = character['graphics_set']
    start = table+be(rom, table+parameter, 2, True)
    limit = character['animation_set']
    assert table <= start < limit
    cursor = start
    for _ in range(4):
        value = be(rom, cursor, 4)
        cursor += 4
        if value&0x80000000:
            break
    for _ in range(80):
        value = be(rom, cursor, 4)
        cursor += 4
        if value&0x80000000:
            break
    else:
        raise ValueError('No piece termination within original sprite budget')
    assert cursor <= limit
    return {'graphics_set': table, 'parameter': parameter, 'tiles_base': character['tiles_base'],
            'character_id': character['character_id'], 'source_address': start,
            'payload': rom[start:cursor].hex(), 'source_sha256': sha(rom[start:cursor])}


def snapshot(ram, actor=0x8400):
    return {'actor': ram[actor:actor+256].hex(), 'dma': ram[0xd400:0xe000].hex(),
            'sprites': ram[0xe000:0xf000].hex(), 'layers': ram[0xf300:0xf400].hex(),
            'dma_cursor': be(ram, 0xf518, 2), 'count': be(ram, 0xf578, 2),
            'sprite_cursor': be(ram, 0xf57c, 2), 'flags': ram[0xf54f]}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--limit', type=int, default=0)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert sha(rom) == ROM_SHA256
    characters = initializer_bindings(rom)
    catalog = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/animation-catalog.json.gz').read_bytes()))
    keys = sorted({(frame['character_id'], frame['data']['graphic_parameter']) for frame in catalog['frames']})
    library = [raw_frame(rom, characters[character], parameter) for character, parameter in keys]
    selected = library if args.limit == 0 else library[:args.limit]
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = (C.c_uint*1)(0x3d8)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    baseline = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    cases, pixels, errors = [], [], []
    sample = None
    with ReferenceSession(ROOT, out, core=core) as ref:
        frames = []
        vram = (C.c_uint8*65536).in_dll(ref.lib, 'vram')
        cram = (C.c_uint16*64).in_dll(ref.lib, 'cram')
        regs = (C.c_uint8*32).in_dll(ref.lib, 'reg')
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if pixels and pixels[-1]['frame'] == frame-1:
                pixels[-1]['video_next'] = ref.image.hex()
                pixels[-1]['video_next_frame'] = frame
            if frame == 2400:
                checkpoint = ref.save()
            if frame in [1800, 1855, 1955, 2005, 2055, 2105, 2155, 2255, 2355, 2455, 2505, 2555, 2605, 2655, 2705, 2805, 2905, 2955, 3055, 3155]:
                native = bytes(vram)
                cpu_vram = bytearray(65536)
                cpu_vram[0::2], cpu_vram[1::2] = native[1::2], native[0::2]
                ram = ref.ram_cpu()
                sources = []
                for owner in [0x8400, 0x9000]:
                    for slot in range(4):
                        descriptor = be(ram, owner+0x2c+slot*4, 4)
                        length = (descriptor&255)*32
                        if length == 0:
                            continue
                        source = be(ram, owner+0x20, 4)+(descriptor>>16)*32
                        destination = ((descriptor&0xff00)>>3)+(be(ram, owner+0x24, 2)&0x7ff)*32
                        sources.append({'owner': owner, 'slot': slot, 'source': source, 'destination': destination,
                                        'bytes': rom[source:source+length].hex()})
                pixels.append({'frame': frame, 'ram': ram.hex(), 'vram': cpu_vram.hex(), 'rom_sources': sources,
                               'cram_compact': list(cram), 'vdp_registers': list(regs), 'pixel_format': ref.pixel_format,
                               'width': ref.width, 'height': ref.height, 'video': ref.image.hex()})
        assert frames == baseline
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        @cbtype
        def observe(kind, width, pc, value):
            nonlocal sample
            if sample is not None:
                errors.append('Multiple original renderer returns')
            sample = ref.ram_cpu()
            ref.cpu.cycles = 2000000
        assert bridge.yyh_set_observer(observe, points, 1)
        for index, frame in enumerate(selected):
            for variant in range(22 if frame['parameter'] == 2 else 8):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                setreg(17, 0x2700)
                setreg(15, 0xffffe0)
                write(0xffffe0, 0x3d8, 4)
                setreg(13, 0xff8400)
                for reg in range(8):
                    setreg(reg, 0)
                depth = [0x100, 0xd8, 0, 0xffff, 0x80, 0x7fff][0 if variant < 4 else 1 if variant < 8 else min(variant-6, 5)]
                for off, value, size in [(0x1c, frame['graphics_set'], 4), (0x20, frame['tiles_base'], 4),
                                         (0x3c, frame['parameter'], 2), (8, 0x180, 2), (12, 0x140, 2),
                                         (0x28, -7 if variant&1 else 9, 2), (0x2a, 3 if variant&2 else -5, 2),
                                         (0x24, 0x8200 | ((variant&3)<<11), 2), (0x26, 0x40, 1), (0x27, int(variant == 17), 1),
                                         (0x3e, depth, 2)]:
                    write(0xff8400+off, value, size)
                for i in range(4):
                    write(0xff842c+i*4, 0x11223344 if variant&1 else 0, 4)
                if variant == 21:
                    payload = bytes.fromhex(frame['payload'])
                    for i in range(4):
                        value = be(payload, i*4, 4)
                        write(0xff842c+i*4, value&0x7fffffff, 4)
                        if value&0x80000000:
                            for remaining in range(i+1, 4):
                                write(0xff842c+remaining*4, 0, 4)
                            break
                if variant in (13, 14):
                    write(0xff8408, (0x40 if variant == 13 else 0x200)-(9 if not variant&1 else -7), 2)
                if variant in (15, 16):
                    write(0xff840c, (0x40 if variant == 15 else 0x1a0)-(3 if variant&2 else -5), 2)
                write(0xfff54f, 0 if variant == 12 else 0x80)
                write(0xfff518, 0xd400, 2)
                write(0xfff578, {18: 79, 19: 80, 20: 255}.get(variant, 0), 2)
                write(0xfff57c, 0xe600, 2)
                for i in range(0, 256, 4):
                    write(0xfff302+i, 0xe600, 2)
                before_ram = ref.ram_cpu()
                before = snapshot(before_ram)
                sample = None
                setreg(16, 0x1580)
                ref.cpu.cycles = 0
                ref.cpu.refresh_cycles = 896
                hook(observer)
                execute(1000000)
                hook(None)
                assert sample is not None and not errors, errors
                permitted = set(range(0x8400, 0x8500)) | set(range(0xd400, 0xf000)) | set(range(0xf300, 0xf400)) | {0xf518, 0xf519, 0xf54f, 0xf578, 0xf579, 0xf57c, 0xf57d} | set(range(0xff80, 0xffe4))
                unexpected = [i for i, (a, b) in enumerate(zip(before_ram, sample)) if a != b and i not in permitted]
                assert not unexpected, unexpected
                cases.append({'graphics_set': frame['graphics_set'], 'parameter': frame['parameter'], 'variant': variant,
                              'before': before, 'after': snapshot(sample), 'return': '0x3d8'})
        identity = ref.identity
    data = {'schema': 1, 'rom_sha256': ROM_SHA256, 'library': library, 'calls': cases}
    raw = (json.dumps(data, separators=(',', ':'))+'\n').encode()
    zipped = gzip.compress(raw, mtime=0)
    (out/'sprite-oracle.json.gz').write_bytes(zipped)
    pixel_raw = (json.dumps(pixels, separators=(',', ':'))+'\n').encode()
    (out/'sprite-pixel-reference.json.gz').write_bytes(gzip.compress(pixel_raw, mtime=0))
    report = {'scope': 'Isolated original sprite submission; raw natural VRAM/video references retained separately',
              'identity': identity, 'producer_sha256': sha(Path(__file__).read_bytes()), 'library_frames': len(library),
              'checked_frames': len(selected), 'original_cpu_calls': len(cases), 'all_library_frames_checked': len(selected) == len(library),
              'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'pixel_reference_sha256': sha(pixel_raw),
              'frames_match_original_core': frames == baseline, 'natural_pixel_frames': len(pixels), 'errors': errors, 'pass': not errors,
              'limits': ['Pixel comparison remains a separate check', 'Isolated renderer state is not a complete natural presentation timeline',
                         'Raw graphics frames do not establish animation reachability or a production asset format']}
    (out/'sprite-oracle-manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
