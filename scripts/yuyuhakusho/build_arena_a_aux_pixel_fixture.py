#!/usr/bin/env python3
"""Build a compact final-pixel oracle from existing arena-A captures only.

Expected colors are original f+1 video bytes. SAT is used only in this offline
reference adapter to bind the six CPU submissions and exclude actual opaque
non-A sprite pixels; it is never an input to the independent game renderer.
"""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def read_bound(path, manifest_path):
    compressed, mb = path.read_bytes(), manifest_path.read_bytes()
    raw, manifest = gzip.decompress(compressed), json.loads(mb)
    require(manifest['pass'] and manifest['gzip_sha256'] == sha(compressed)
            and manifest['fixture_sha256'] == sha(raw), 'unbound original source: ' + str(path))
    return json.loads(raw), manifest, {'path': str(path.relative_to(ROOT)), 'gzip_sha256': sha(compressed),
                                     'fixture_sha256': sha(raw), 'manifest_sha256': sha(mb)}


def word(raw, offset):
    return int.from_bytes(raw[offset:offset + 2], 'big')


def pixel(vram, attribute, x, y):
    if attribute & 0x800:
        x = 7 - x
    if attribute & 0x1000:
        y = 7 - y
    value = vram[(attribute & 2047) * 32 + y * 4 + x // 2]
    return (value >> (4 if x & 1 == 0 else 0)) & 15


def color565(compact, limited):
    require(0 <= compact <= 511, 'invalid original color')
    if limited:
        compact &= 0x49
    r, g, b = (compact & 7) * 2, ((compact >> 3) & 7) * 2, ((compact >> 6) & 7) * 2
    return r << 12 | r >> 3 << 11 | g << 7 | g >> 2 << 5 | b << 1 | b >> 3


def background(sample, vram):
    colors, high, opaque = [0] * (320 * 224), bytearray(320 * 224), bytearray(320 * 224)
    for y, row in enumerate(sample['active_lines']):
        regs, palette = row['registers'], row['cram']
        require(row['line'] == y and len(regs) == 32 and len(palette) == 64
                and regs[16] == 1 and regs[17] == 0 and regs[11] & 4 == 0
                and regs[12] & 0x0e == 0 and regs[12] & 1, 'unsupported original background mode')
        horizontal = struct.unpack('>HH', bytes.fromhex(row['hscroll']))
        vertical = [n & 1023 for n in struct.unpack('>HH', bytes.fromhex(row['vsram']))]
        window = bool(regs[18] & 128) == (y >= (regs[18] & 31) * 8)
        base_a, base_b, base_window = (regs[2] & 0x38) << 10, (regs[4] & 7) << 13, (regs[3] & 0x3c) << 10
        for x in range(320):
            index, attribute, nonzero = regs[7] & 63, 0, False
            if row['draw']:
                attrs, values = [], []
                for plane, base in [(0, base_a), (1, base_b)]:
                    u, v = (x - horizontal[plane]) & 511, (y + vertical[plane]) & 255
                    if plane == 0 and window:
                        u, v, base = x, y, base_window
                    a = word(vram, base + ((v // 8) * 64 + u // 8) * 2)
                    attrs.append(a)
                    values.append(pixel(vram, a, u & 7, v & 7))
                if values[1]:
                    attribute, nonzero = attrs[1], True
                    index = ((attribute >> 9) & 0x30) | values[1]
                if values[0] and (not values[1] or attrs[0] & 0x8000 or attrs[1] & 0x8000 == 0):
                    attribute, nonzero = attrs[0], True
                    index = ((attribute >> 9) & 0x30) | values[0]
            pos = y * 320 + x
            colors[pos] = color565(palette[index], regs[0] & 4 == 0)
            opaque[pos], high[pos] = int(nonzero), int(nonzero and bool(attribute & 0x8000))
    return colors, opaque, high


def chain(sample, vram):
    base, index, seen, entries = (sample['registers'][5] & 0x7e) << 9, 0, set(), []
    for _ in range(80):
        require(index < 80 and index not in seen, 'invalid original SAT chain')
        seen.add(index)
        entry = vram[base + index * 8:base + index * 8 + 8]
        require(len(entry) == 8, 'truncated original SAT entry')
        entries.append((index, entry))
        index = entry[3] & 127
        if index == 0:
            return entries
    raise ValueError('unterminated original SAT chain')


def sprite_pixels(entry, vram):
    y0, x0 = (word(entry, 0) & 1023) - 128, (word(entry, 6) & 511) - 128
    attribute, width, height = word(entry, 4), (entry[2] >> 2 & 3) + 1, (entry[2] & 3) + 1
    for dy in range(height * 8):
        y = y0 + dy
        if not 0 <= y < 224:
            continue
        v = height * 8 - 1 - dy if attribute & 0x1000 else dy
        for dx in range(width * 8):
            x = x0 + dx
            if not 0 <= x < 320:
                continue
            u = width * 8 - 1 - dx if attribute & 0x800 else dx
            tile = ((attribute & 2047) + (u // 8) * height + v // 8) & 2047
            value = pixel(vram, tile, u & 7, v & 7)
            if value:
                yield y * 320 + x, ((attribute >> 9) & 0x30) | value, bool(attribute & 0x8000)


def pack_mask(mask):
    packed = bytearray((len(mask) + 7) // 8)
    for i, value in enumerate(mask):
        if value:
            packed[i // 8] |= 1 << (i & 7)
    return packed.hex()


def project(sample, call):
    require(sample['arena_offset'] == 10 and sample['width'] == 320 and sample['height'] == 224
            and sample['pixel_format'] == 2 and sample['video_next_frame'] == sample['frame'] + 1
            and not sample['active_background_vram_changed'] and not sample['active_vram_changes'],
            'unbound original A sample')
    require(call['kind'] == 'child' and call['object_id'] == 0x9e00 and call['frame'] == sample['frame']
            and len(call['draws']) == 6, 'missing natural A child raw submissions')
    vram, video = bytes.fromhex(sample['active_vram']), bytes.fromhex(sample['video_next'])
    require(len(vram) == 65536 and len(video) == 320 * 224 * 2, 'incomplete original image')
    entries = chain(sample, vram)
    tail, draws = entries[-6:], []
    require(len(entries) >= 6, 'missing A SAT tail')
    for (index, entry), draw in zip(tail, call['draws']):
        descriptor = bytes.fromhex(draw['descriptor'])
        require(draw['accepted'] and len(descriptor) == 10 and word(descriptor, 8) == 0xffff
                and word(descriptor, 2) == 0x0ffc and word(descriptor, 4) in (0x5d0, 0x5e0, 0x5f0)
                and index == draw['count'] and draw['after_count'] == draw['count'] + 1
                and word(entry, 0) == word(descriptor, 0) and entry[2] == descriptor[2]
                and word(entry, 4) == word(descriptor, 4) and word(entry, 6) == word(descriptor, 6),
                'CPU raw does not match actual original SAT tail')
        draws.append({'Y': word(descriptor, 0), 'SizeLink': word(descriptor, 2),
                      'Tile': word(descriptor, 4), 'X': word(descriptor, 6)})
    first = call['draws'][0]['count']
    require(2 <= first <= 74 and [i for i, _ in tail] == list(range(first, first + 6)),
            'unexpected A sprite count sequence')
    bg, bg_opaque, bg_high = background(sample, vram)
    non_aux, aux_opaque = bytearray(320 * 224), bytearray(320 * 224)
    aux_colors, aux_high = [0] * (320 * 224), bytearray(320 * 224)
    aux_indices = {index for index, _ in tail}
    for index, entry in entries:
        is_aux = index in aux_indices
        for pos, color, high in sprite_pixels(entry, vram):
            if not is_aux:
                non_aux[pos] = 1
            elif not aux_opaque[pos]:
                aux_opaque[pos] = 1
                row = sample['active_lines'][pos // 320]
                aux_colors[pos] = color565(row['cram'][color], row['registers'][0] & 4 == 0)
                aux_high[pos] = int(high)
    aux_visible, expected = bytearray(320 * 224), bytearray()
    compared = aux_pixels = aux_distinct = mismatches = 0
    for pos in range(320 * 224):
        if non_aux[pos]:
            continue
        want = int.from_bytes(video[pos * 2:pos * 2 + 2], 'little')
        color = bg[pos]
        if aux_opaque[pos] and sample['active_lines'][pos // 320]['draw'] and (aux_high[pos] or not bg_opaque[pos] or not bg_high[pos]):
            color = aux_colors[pos]
            aux_visible[pos] = 1
            aux_pixels += 1
            aux_distinct += color != bg[pos]
        mismatches += color != want
        expected.extend(video[pos * 2:pos * 2 + 2])
        compared += 1
    require(mismatches == 0, f'Independent raw-VRAM reference adapter differs from video: {mismatches}')
    require(compared > 0 and aux_pixels > 0 and aux_distinct > 0, 'A contribution was entirely masked or invisible')
    return {'frame': sample['frame'], 'first_count': first, 'draws': draws,
            'non_aux_mask': pack_mask(non_aux), 'aux_visible_mask': pack_mask(aux_visible),
            'expected_rgb565': expected.hex(), 'expected_sha256': sha(expected),
            'compared_pixels': compared, 'aux_visible_pixels': aux_pixels,
            'aux_distinct_from_background': aux_distinct, 'non_aux_excluded_pixels': sum(non_aux),
            'sat_tail_indices': [i for i, _ in tail]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, default=ROOT / 'games/yuyuhakusho/presentation/testdata')
    args = parser.parse_args()
    source, sm, source_binding = read_bound(
        ROOT / '.local/md-yuyuhakusho/additional-arena-assets/references/arena-assets-a-reference.json.gz',
        ROOT / 'games/yuyuhakusho/data/arena-assets-a-reference-manifest.json')
    natural, nm, natural_binding = read_bound(ROOT / 'games/yuyuhakusho/engine/testdata/arena-a-natural.json.gz',
                                              ROOT / 'games/yuyuhakusho/engine/testdata/arena-a-natural-manifest.json')
    require(sm['frames_match_original_core'] == 3602 and sm['control_mismatches'] == 0
            and nm['frames_match_original_control'] and nm['full_ram_timeline_replayed']
            and nm['unknown_delta'] == 0 and nm['legal_arena_a_natural'], 'incomplete original baseline')
    rows_path = ROOT / 'games/yuyuhakusho/presentation/testdata/additional-arena-presentation-reference.json.gz'
    rows_manifest = ROOT / 'games/yuyuhakusho/presentation/testdata/additional-arena-presentation-reference-manifest.json'
    rows, _, row_binding = read_bound(rows_path, rows_manifest)
    require(source['identity']['rom_sha256'] == nm['identity']['rom_sha256'] == rows['rom_sha256'] == ROM_SHA,
            'different original ROM identity')
    calls = {call['frame']: call for call in natural['calls'] if call['kind'] == 'child'}
    row_frames = {frame['frame'] for frame in rows['frames'] if frame['arena'] == 10}
    frames = [project(frame, calls[frame['frame']]) for frame in source['frames']]
    require(len(frames) == 20 and {f['frame'] for f in frames} == row_frames, 'incomplete A sample coverage')
    raw = (json.dumps({'schema': 1, 'rom_sha256': ROM_SHA, 'arena': 10,
                       'rows_gzip_sha256': row_binding['gzip_sha256'], 'frames': frames}, separators=(',', ':')) + '\n').encode()
    compressed = gzip.compress(raw, compresslevel=9, mtime=0)
    args.out.mkdir(parents=True, exist_ok=True)
    path = args.out / 'arena-a-aux-pixel-reference.json.gz'
    path.write_bytes(compressed)
    report = {'pass': True, 'scope': 'Original A six-raw final pixels; only opaque non-A sprites excluded',
              'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'gzip_bytes': len(compressed),
              'frames': len(frames), 'sources': [source_binding, natural_binding, row_binding],
              'producer_sha256': sha(Path(__file__).read_bytes()),
              'compared_pixels': sum(f['compared_pixels'] for f in frames),
              'aux_visible_pixels': sum(f['aux_visible_pixels'] for f in frames),
              'aux_distinct_from_background': sum(f['aux_distinct_from_background'] for f in frames),
              'non_aux_excluded_pixels': sum(f['non_aux_excluded_pixels'] for f in frames),
              'mismatches': 0, 'every_sample_has_visible_distinct_aux_pixels': True,
              'mask_boundary': 'Only actual nonzero pixels of original SAT entries before the six CPU-bound tail entries; no rectangles and no auxiliary pixels added',
              'limits': ['Original indexed window remains reference adapter; independent HUD events are separately verified',
                         'Original non-A sprite pixels are excluded even if hidden by a high-priority plane',
                         'No sprite overflow, arbitrary inputs, world dispatch or browser assertion']}
    (args.out / 'arena-a-aux-pixel-reference-manifest.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({k: v for k, v in report.items() if k != 'sources'}, indent=2))


if __name__ == '__main__':
    main()
