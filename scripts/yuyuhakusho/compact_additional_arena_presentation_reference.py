#!/usr/bin/env python3
"""Project pinned existing captures into semantic Go presentation test inputs.

This development-only tool never starts a core and never reads a ROM. Expected
pixel bytes come directly from original f+1 video; the only exclusion is the
already independently verified opaque original-sprite mask. It does not use
the Go decoder or renderer to generate expected values.
"""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import struct

from verify_additional_arena_assets import compact_color, sprite_mask

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
REGISTRY = {2: (0x20ae0, 0x2175c), 4: (0x20cb2, 0x21798),
            6: (0x20e28, 0x217cc), 8: (0x20fb4, 0x21800),
            10: (0x21126, 0x21834), 12: (0x212f8, 0x21870),
            14: (0x21412, 0x218a4)}


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def word(raw, offset):
    return int.from_bytes(raw[offset:offset + 2], 'big')


def window(vram, registers):
    base = (registers[3] & 0x3c) << 10
    tiles, cells, unique = [], [], {}
    for index in range(64 * 32):
        attribute = word(vram, base + index * 2)
        start = (attribute & 2047) * 32
        encoded = vram[start:start + 32].hex()
        if encoded not in unique:
            unique[encoded] = len(tiles)
            tiles.append(encoded)
        cells.append({'tile': unique[encoded], 'flags': attribute & 0xf800})
    return {'tiles': tiles, 'cells': cells}


def project(sample, arena, callback):
    require(sample['arena_offset'] == arena and sample['width'] == 320
            and sample['height'] == 224 and sample['pixel_format'] == 2,
            'wrong original sample identity or video geometry')
    require(sample['video_next_frame'] == sample['frame'] + 1
            and not sample['active_background_vram_changed']
            and not sample['active_vram_changes'], 'unbound original video timing or static planes')
    ram, vram = bytes.fromhex(sample['ram']), bytes.fromhex(sample['active_vram'])
    require(len(ram) == len(vram) == 65536 and word(ram, 0xa060) == arena
            and int.from_bytes(ram[0x8180:0x8184], 'big') == callback,
            'original RAM does not bind selected scene')
    palette = [compact_color(word(ram, 0xe000 + i * 2)) for i in range(64)]
    require(palette[0] == compact_color(word(ram, 0xf140)), 'unbound raster backdrop upload')
    horizontal, vertical, row_flags, visible_palette, differences = [], [], [], bytearray(), []
    require(len(sample['active_lines']) == 224, 'missing original scanlines')
    for y, line in enumerate(sample['active_lines']):
        regs = line['registers']
        require(line['line'] == y and len(regs) == 32 and len(line['cram']) == 64
                and regs[12] & 0x0e == 0 and regs[12] & 1 and regs[11] & 4 == 0
                and regs[16] == 1 and regs[17] == 0
                and regs[3] == sample['registers'][3], 'unsupported original scanline format')
        horizontal.append(list(struct.unpack('>HH', bytes.fromhex(line['hscroll']))))
        vertical.append([value & 1023 for value in struct.unpack('>HH', bytes.fromhex(line['vsram']))])
        flag = int(bool(line['draw'])) | (int(bool(regs[18] & 128) == (y >= (regs[18] & 31) * 8)) << 1)
        flag |= int(regs[0] & 4 == 0) << 2
        row_flags.append(flag)
        require(regs[7] & 63 == ram[0xf6df] & 63, 'backdrop index changes within sample')
        if line['draw']:
            visible_palette.extend(struct.pack('>64H', *line['cram']))
        elif y >= 208:
            changed = [i for i in range(1, 64) if palette[i] != line['cram'][i]]
            require(not changed or arena == 4 and set(changed) <= {9, 10, 11},
                    'unexplained blank palette phase difference')
            if changed:
                differences.append({'row': y, 'indices': changed,
                                    'observed': [line['cram'][i] for i in changed],
                                    'frame_end': [palette[i] for i in changed]})
    mask = sprite_mask(sample, vram)
    packed_mask = bytearray((320 * 224 + 7) // 8)
    video = bytes.fromhex(sample['video_next'])
    require(len(video) == 320 * 224 * 2, 'wrong original video byte count')
    expected, count = bytearray(), 0
    for i, excluded in enumerate(mask):
        if excluded:
            packed_mask[i // 8] |= 1 << (i & 7)
        else:
            expected.extend(video[i * 2:i * 2 + 2])
            count += 1
    return {'arena': arena, 'frame': sample['frame'], 'palette': palette,
            'backdrop': ram[0xf6df] & 63, 'horizontal': horizontal, 'vertical': vertical,
            'window': window(vram, sample['registers']), 'mask': packed_mask.hex(),
            'pixels_sha256': sha(expected), 'pixels': count, 'row_flags': row_flags,
            'visible_palette_sha256': sha(visible_palette),
            'blank_unused_palette_differences': differences}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--references', type=Path, default=ROOT / '.local/md-yuyuhakusho/additional-arena-assets/references')
    parser.add_argument('--out', type=Path, default=ROOT / 'games/yuyuhakusho/presentation/testdata')
    args = parser.parse_args()
    output, sources = [], {}
    for arena, (callback, resource) in REGISTRY.items():
        source_path = args.references / f'arena-assets-{arena:x}-reference.json.gz'
        manifest_path = ROOT / f'games/yuyuhakusho/data/arena-assets-{arena:x}-reference-manifest.json'
        compressed, manifest_bytes = source_path.read_bytes(), manifest_path.read_bytes()
        raw, manifest = gzip.decompress(compressed), json.loads(manifest_bytes)
        require(manifest['pass'] and manifest['frames_match_original_core'] == 3602
                and manifest['control_mismatches'] == 0 and manifest['records'] == 20
                and manifest['gzip_sha256'] == sha(compressed)
                and manifest['fixture_sha256'] == sha(raw), 'unbound original fixture manifest')
        reference = json.loads(raw)
        require(reference['identity']['rom_sha256'] == ROM_SHA
                and reference['selected_resource_load']['resource_list'] == resource,
                'wrong ROM or resource in original fixture')
        require(len(reference['frames']) == 20, 'incomplete original samples')
        output.extend(project(frame, arena, callback) for frame in reference['frames'])
        sources[f'{arena:x}'] = {'reference_gzip_sha256': sha(compressed),
                               'reference_sha256': sha(raw), 'manifest_sha256': sha(manifest_bytes),
                               'control': manifest['original_control']}
    raw = (json.dumps({'schema': 1, 'rom_sha256': ROM_SHA, 'frames': output}, separators=(',', ':')) + '\n').encode()
    compressed = gzip.compress(raw, compresslevel=9, mtime=0)
    pixels = sum(frame['pixels'] for frame in output)
    require(len(output) == 140 and pixels == 9535530, 'incomplete original pixel coverage')
    args.out.mkdir(parents=True, exist_ok=True)
    fixture = args.out / 'additional-arena-presentation-reference.json.gz'
    fixture.write_bytes(compressed)
    manifest = {'pass': True, 'scope': 'Existing original samples projected to semantic test-only inputs; no new CPU run',
                'frames': len(output), 'pixels': pixels, 'fixture_sha256': sha(raw),
                'gzip_sha256': sha(compressed), 'gzip_bytes': len(compressed), 'sources': sources,
                'producer_sha256': sha(Path(__file__).read_bytes()),
                'sprite_mask_source_sha256': sha((Path(__file__).parent / 'verify_additional_arena_assets.py').read_bytes()),
                'blank_unused_palette_difference_rows': sum(len(frame['blank_unused_palette_differences']) for frame in output),
                'limitations': ['Original opaque sprites excluded; final independent sprite composition is separate',
                                'Original indexed window is a reference adapter; independent HUD already separately verified',
                                'ID4 blank unused9..11 differ from frame-end palette; visible colors and final pixels are exact']}
    (args.out / 'additional-arena-presentation-reference-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps({k: v for k, v in manifest.items() if k != 'sources'}, indent=2))


if __name__ == '__main__':
    main()
