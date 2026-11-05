#!/usr/bin/env python3
"""Test-first verifier for the seven additional compact arena catalogs.

This development-only verifier derives expected cells from pinned original
references and independently decodes each declared ROM compression source.
It is not imported by the game or by the offline asset producer.
"""
import argparse
import copy
import gzip
import hashlib
import json
from pathlib import Path
import struct
import zlib

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
REGISTRY = {
    2: (0x20ae0, 0x2175c), 4: (0x20cb2, 0x21798),
    6: (0x20e28, 0x217cc), 8: (0x20fb4, 0x21800),
    10: (0x21126, 0x21834), 12: (0x212f8, 0x21870),
    14: (0x21412, 0x218a4),
}


def require(value, message):
    if not value:
        raise ValueError(message)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def read_json(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def word(raw, position, length=2):
    require(0 <= position <= len(raw) - length, 'source read outside original ROM')
    return int.from_bytes(raw[position:position + length], 'big')


def original_stream(rom, source):
    """Independent token-group decoder for original988/9ba (offline only)."""
    cursor, output, counts = source + 2, bytearray(), [0] * 8
    end = cursor + word(rom, source)
    require(end <= len(rom), 'encoded source beyond original ROM')

    def take(length):
        nonlocal cursor
        require(cursor + length <= end, 'truncated original compression token')
        result = rom[cursor:cursor + length]
        cursor += length
        return result

    while cursor < end:
        token = take(1)[0]
        group = token >> 5
        counts[group] += 1
        if group >= 4:
            length = ((token >> 2) & 31) + 2
            distance = (((token & 3) << 8) | take(1)[0]) + 1
            require(distance <= len(output), 'original backreference before output')
            for _ in range(length):
                output.append(output[-distance])
        elif group == 0:
            output.extend(take((token & 31) + 1))
        elif group == 1:
            output.extend(take(1) * ((token & 31) + 2))
        elif group == 2:
            output.extend(take(2) * ((token & 31) + 2))
        else:
            length, repeated = (token & 31) + 2, take(1)[0]
            for value in take(length):
                output.extend((repeated, value))
        require(len(output) <= 65536, 'original decode exceeds VRAM')
    require(cursor == end, 'original compression boundary mismatch')
    return bytes(output), counts


def original_binding(rom, arena):
    callback, start = REGISTRY[arena]
    require(word(rom, 0xf94c + arena * 2, 4) == callback, 'wrong callback registry')
    require(word(rom, 0x21700 + arena * 2, 4) == start, 'wrong resource-list registry')
    cursor, resources, decoded = start, [], []
    while word(rom, cursor) != 0xffff:
        kind, source, destination = word(rom, cursor), word(rom, cursor + 2, 4), word(rom, cursor + 6)
        require(kind in (0, 6), 'unexpected original resource kind')
        row = {'kind': kind, 'source': source, 'destination': destination}
        if kind == 6:
            row['encoded_bytes'] = word(rom, source)
            row['encoded_sha256'] = digest(rom[source:source + 2 + row['encoded_bytes']])
            raw, tokens = original_stream(rom, source)
            decoded.append({'source': source, 'destination': destination, 'bytes': raw, 'tokens': tokens})
        else:
            row['length'] = word(rom, cursor + 8)
        resources.append(row)
        cursor += 10 if kind == 0 else 8
        require(len(resources) <= 16, 'unterminated original resource list')
    return {'arena_offset': arena, 'scene_behavior': callback, 'resource_list': start,
            'list_sha256': digest(rom[start:cursor + 2]), 'resources': resources}, decoded


def compact_color(value):
    return (value >> 1 & 7) | (value >> 2 & 0x38) | (value >> 3 & 0x1c0)


def verify_catalog(rom, arena, catalog, manifest):
    binding, decoded = original_binding(rom, arena)
    require(catalog.get('schema') == 1 and catalog.get('rom_sha256') == ROM_SHA,
            'invalid compact arena schema/ROM identity')
    require(catalog.get('arena_offset') == arena and catalog.get('scene_behavior') == REGISTRY[arena][0],
            'invalid compact arena selection')
    require(catalog.get('resource_binding') == binding, 'compact arena binding differs from original ROM')
    require(set(catalog) <= {'schema', 'rom_sha256', 'arena_offset', 'scene_behavior', 'resource_binding',
                             'packed_4bpp_tiles', 'initial_scene_palette_0', 'planes', 'runtime_inputs', 'scope'},
            'unexpected runtime payload in compact arena')
    palette = binding['resources'][-1]
    require(palette['kind'] == 0 and palette['destination'] == 0xe080 and palette['length'] == 32,
            'wrong original scene palette binding')
    expected_palette = [compact_color(word(rom, palette['source'] + i * 2)) for i in range(16)]
    require(catalog.get('initial_scene_palette_0') == expected_palette, 'wrong initial scene palette')
    tiles = catalog.get('packed_4bpp_tiles', [])
    require(0 < len(tiles) <= 2048 and len(tiles) == len(set(tiles)), 'invalid or duplicate compact tiles')
    bank = []
    for encoded in tiles:
        require(isinstance(encoded, str) and len(encoded) == 64 and encoded == encoded.lower(),
                'invalid packed 4bpp tile encoding')
        bank.append(bytes.fromhex(encoded))
    planes = catalog.get('planes', [])
    require(len(planes) == 2, 'missing complete A/B planes')
    used = set()
    for index, plane in enumerate(planes):
        require(plane.get('name') == ('A', 'B')[index] and plane.get('columns') == 64 and plane.get('rows') == 32
                and len(plane.get('cells', [])) == 2048, 'invalid complete arena map geometry')
        for cell in plane['cells']:
            require(set(cell) == {'tile', 'flags'} and isinstance(cell['tile'], int)
                    and 0 <= cell['tile'] < len(bank) and isinstance(cell['flags'], int)
                    and 0 <= cell['flags'] <= 0xf800 and cell['flags'] & 0x7ff == 0,
                    'invalid arena cell tile/flags')
            used.add(cell['tile'])
    require(used == set(range(len(bank))), 'unreferenced compact tile payload')
    require(manifest.get('rom_sha256') == ROM_SHA and manifest.get('arena_offset') == arena
            and manifest.get('resource_binding') == binding, 'manifest identity differs')
    original_sources = manifest.get('decoded_resources', [])
    require(len(original_sources) == len(decoded), 'decoded original source evidence missing')
    for expected, actual in zip(decoded, original_sources):
        require(actual.get('source') == expected['source'] and actual.get('destination') == expected['destination']
                and actual.get('decoded_bytes') == len(expected['bytes'])
                and actual.get('decoded_sha256') == digest(expected['bytes'])
                and actual.get('token_counts') == expected['tokens'], 'original source decode differs')
    material, covered = bytearray(65536), bytearray(65536)
    for source in decoded:
        start, raw = source['destination'], source['bytes']
        require(start + len(raw) <= 65536, 'original resource upload outside VRAM')
        material[start:start + len(raw)] = raw
        covered[start:start + len(raw)] = bytes([1]) * len(raw)
    original_tiles = {bytes(material[start:start + 32]) for start in range(0, 65536, 32)
                      if all(covered[start:start + 32])}
    require(all(tile in original_tiles for tile in bank), 'compact tile has no original ROM source')
    for plane, base in zip(planes, (0xc000, 0xe000)):
        for index, cell in enumerate(plane['cells']):
            address = base + index * 2
            if all(covered[address:address + 2]):
                attribute = word(material, address)
                start = (attribute & 0x7ff) * 32
                require(all(covered[start:start + 32]) and cell['flags'] == attribute & 0xf800
                        and bank[cell['tile']] == material[start:start + 32],
                        'compact map or indexed tile differs from original ROM resource')
    return binding, decoded, bank


def verify_loaded(rom, catalog, decoded, bank, reference):
    arena = catalog['arena_offset']
    require(reference.get('identity', {}).get('rom_sha256') == ROM_SHA, 'wrong original reference identity')
    samples = reference.get('frames', [])
    require(samples and all(row.get('arena_offset') == arena for row in samples), 'missing selected original frames')
    require(any(row.get('resource_list') == REGISTRY[arena][1] for row in reference.get('observed_loads', [])),
            'original resource list load was not observed')
    observation = reference.get('initial_scene_palette_observation', {})
    palette = catalog['resource_binding']['resources'][-1]
    raw_palette = rom[palette['source']:palette['source'] + 32]
    require(observation.get('pc') == 0x83c and observation.get('frame', 0) > 0
            and observation.get('ram_address') == 0xe080 and observation.get('bytes') == raw_palette.hex()
            and observation.get('sha256') == digest(raw_palette), 'initial original scene palette was not observed')
    evidence = []
    for sample in samples:
        require(sample.get('video_next_frame') == sample['frame'] + 1, 'wrong original video timing')
        vram = bytes.fromhex(sample['active_vram'])
        require(len(vram) == 65536 and not sample.get('active_background_vram_changed'),
                'static compact arena cannot cover active VRAM changes')
        registers = sample['registers']
        require(len(registers) == 32 and registers[16] == 1, 'unsupported original arena geometry')
        for plane, base in zip(catalog['planes'], ((registers[2] & 0x38) << 10, (registers[4] & 7) << 13)):
            for index, cell in enumerate(plane['cells']):
                attribute = word(vram, base + index * 2)
                require(cell['flags'] == attribute & 0xf800, 'compact cell attributes differ from loaded original')
                start = (attribute & 0x7ff) * 32
                require(bank[cell['tile']] == vram[start:start + 32], 'compact tile differs from loaded original')
        evidence.append({'frame': sample['frame'], 'active_vram_sha256': digest(vram)})
    first = bytes.fromhex(samples[0]['active_vram'])
    for row in decoded:
        start, raw = row['destination'], row['bytes']
        require(first[start:start + len(raw)] == raw, 'ROM decode differs from original loaded resource')
    return evidence


def pixel(vram, attribute, x, y):
    if attribute & 0x800:
        x = 7 - x
    if attribute & 0x1000:
        y = 7 - y
    value = vram[(attribute & 0x7ff) * 32 + y * 4 + x // 2]
    return (value >> (4 if x % 2 == 0 else 0)) & 15


def color565(value, limited):
    require(0 <= value <= 511, 'invalid original compact CRAM value')
    if limited:
        value &= 0x49
    red, green, blue = (value & 7) * 2, (value >> 3 & 7) * 2, (value >> 6 & 7) * 2
    return (red << 12) | (red >> 3 << 11) | (green << 7) | (green >> 2 << 5) | (blue << 1) | (blue >> 3)


def sprite_mask(sample, vram):
    """Exclude every original opaque sprite, retaining an explicit count."""
    mask = bytearray(320 * 224)
    base, seen, index = (sample['registers'][5] & 0x7e) << 9, set(), 0
    for _ in range(80):
        require(index < 80 and index not in seen, 'invalid original sprite exclusion chain')
        seen.add(index)
        start = (base + index * 8) & 65535
        entry = vram[start:start + 8]
        require(len(entry) == 8, 'truncated original sprite exclusion data')
        x0, y0 = (word(entry, 6) & 511) - 128, (word(entry, 0) & 1023) - 128
        attribute = word(entry, 4)
        width, height = (entry[2] >> 2 & 3) + 1, (entry[2] & 3) + 1
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
                tile = ((attribute & 0x7ff) + (u // 8) * height + v // 8) & 2047
                if pixel(vram, tile, u & 7, v & 7):
                    mask[y * 320 + x] = 1
        index = entry[3] & 127
        if index == 0:
            return mask
    raise ValueError('unterminated original sprite exclusion chain')


def render_pixels(catalog, sample):
    """Compose catalog A/B plus observed runtime raster/window inputs."""
    require(sample.get('width') == 320 and sample.get('height') == 224 and sample.get('pixel_format') == 2,
            'unsupported original video geometry or encoding')
    require(sample.get('video_next_frame') == sample['frame'] + 1, 'wrong original video timing')
    require(not sample.get('active_background_vram_changed') and not sample.get('active_vram_changes'),
            'static pixel comparison cannot silently accept active VRAM changes')
    vram = bytes.fromhex(sample['active_vram'])
    require(len(vram) == 65536 and len(sample.get('active_lines', [])) == 224,
            'missing original per-line VDP evidence')
    tiles = [bytes.fromhex(encoded) for encoded in catalog['packed_4bpp_tiles']]
    colors, sources, draw_rows = [0] * (320 * 224), bytearray(320 * 224), []
    for y, row in enumerate(sample['active_lines']):
        regs, palette = row['registers'], row['cram']
        require(row.get('line') == y and len(regs) == 32 and len(palette) == 64,
                'unbound original raster row')
        require(regs[12] & 0x0e == 0 and regs[12] & 1 and regs[11] & 4 == 0
                and regs[16] == 1 and regs[17] == 0, 'unsupported original raster profile')
        display_colors = [color565(value, regs[0] & 4 == 0) for value in palette]
        if not row['draw']:
            colors[y * 320:(y + 1) * 320] = [display_colors[regs[7] & 63]] * 320
            continue
        draw_rows.append(y)
        vsram, scroll = bytes.fromhex(row['vsram']), bytes.fromhex(row['hscroll'])
        require(len(vsram) == 4 and len(scroll) == 4, 'missing original raster scroll')
        vertical = (word(vsram, 0) & 1023, word(vsram, 2) & 1023)
        horizontal = (word(scroll, 0), word(scroll, 2))
        window = bool(regs[18] & 128) == (y >= (regs[18] & 31) * 8)
        base_window = (regs[3] & 0x3c) << 10
        for x in range(320):
            values, flags = [0, 0], [0, 0]
            for plane in (0, 1):
                u, v = (x - horizontal[plane]) & 511, (y + vertical[plane]) & 255
                if plane == 0 and window:
                    attribute = word(vram, base_window + ((y // 8) * 64 + x // 8) * 2)
                    flags[plane], values[plane] = attribute, pixel(vram, attribute, x & 7, y & 7)
                else:
                    cell = catalog['planes'][plane]['cells'][(v // 8) * 64 + u // 8]
                    flag, tx, ty = cell['flags'], u & 7, v & 7
                    if flag & 0x800:
                        tx = 7 - tx
                    if flag & 0x1000:
                        ty = 7 - ty
                    packed = tiles[cell['tile']][ty * 4 + tx // 2]
                    values[plane] = (packed >> (4 if tx % 2 == 0 else 0)) & 15
                    flags[plane] = flag
            color_index, source = regs[7] & 63, 0
            if values[1]:
                color_index, source = ((flags[1] >> 9) & 0x30) | values[1], 2
            if values[0] and (not values[1] or flags[0] & 0x8000 or flags[1] & 0x8000 == 0):
                color_index, source = ((flags[0] >> 9) & 0x30) | values[0], 3 if window else 1
            position = y * 320 + x
            colors[position], sources[position] = display_colors[color_index], source
    return colors, sources, draw_rows, sprite_mask(sample, vram)


def verify_pixels(catalog, reference):
    report = {'frames': len(reference['frames']), 'compared_pixels': 0, 'nonblack_pixels': 0,
              'sprite_pixels_excluded': 0, 'sources': [0] * 4, 'mismatches': 0, 'frame_profiles': []}
    for sample in reference['frames']:
        require(sample['arena_offset'] == catalog['arena_offset'], 'wrong pixel-reference arena')
        ram = bytes.fromhex(sample['ram'])
        require(len(ram) == 65536 and word(ram, 0xa060) == catalog['arena_offset']
                and word(ram, 0x8180, 4) == catalog['scene_behavior'], 'wrong original scene object identity')
        colors, sources, draw_rows, excluded = render_pixels(catalog, sample)
        video = bytes.fromhex(sample['video_next'])
        require(len(video) == 320 * 224 * 2, 'truncated original comparison video')
        for position, value in enumerate(colors):
            if excluded[position]:
                report['sprite_pixels_excluded'] += 1
                continue
            report['compared_pixels'] += 1
            report['nonblack_pixels'] += value != 0
            report['sources'][sources[position]] += 1
            if value != video[position * 2] | (video[position * 2 + 1] << 8):
                report['mismatches'] += 1
                report.setdefault('first_mismatch', {'frame': sample['frame'], 'video_frame': sample['video_next_frame'],
                                                     'x': position % 320, 'y': position // 320,
                                                     'source': sources[position], 'decoded': value,
                                                     'original': video[position * 2] | video[position * 2 + 1] << 8})
        report['frame_profiles'].append({'frame': sample['frame'], 'video_next_frame': sample['video_next_frame'],
                                          'draw_rows': draw_rows,
                                          'window_ranges': sorted({(row['registers'][17] << 8) | row['registers'][18]
                                                                   for row in sample['active_lines']}),
                                          'raster_register_profiles': sorted({tuple(row['registers']) for row in sample['active_lines']})})
    report['pass'] = bool(report['frames'] and report['compared_pixels'] and report['nonblack_pixels']
                          and report['sources'][1] and report['sources'][2] and report['mismatches'] == 0)
    return report


def analyze_runtime_rows(reference):
    """Check the display profile without erasing unrendered palette changes."""
    result = {'rows': 0, 'geometry_matches_visible14_207_window_lt48': True,
              'visible_palette_matches_world': True, 'backdrop_bands_match_world_f140': True,
              'blank_palette_state_differences': []}
    for sample in reference['frames']:
        ram = bytes.fromhex(sample['ram'])
        world_palette = [compact_color(word(ram, 0xe000 + index * 2)) for index in range(64)]
        result['backdrop_bands_match_world_f140'] &= word(ram, 0xe000) == word(ram, 0xf140)
        for y, row in enumerate(sample['active_lines']):
            result['rows'] += 1
            regs = row['registers']
            window = bool(regs[18] & 128) == (y >= (regs[18] & 31) * 8)
            visible = 14 <= y < 208
            result['geometry_matches_visible14_207_window_lt48'] &= (row['draw'] == visible and window == (y < 48)
                                                                     and regs[17] == 0 and regs[7] & 63 == 0
                                                                     and bool(regs[0] & 4))
            expected = world_palette.copy()
            if y < 16 or y >= 208:
                expected[0] = 0
            result['backdrop_bands_match_world_f140'] &= row['cram'][0] == expected[0]
            differences = [{'index': index, 'world': want, 'raster': got}
                           for index, (want, got) in enumerate(zip(expected, row['cram'])) if want != got]
            if visible and differences:
                result['visible_palette_matches_world'] = False
            elif differences:
                result['blank_palette_state_differences'].append({'frame': sample['frame'], 'line': y, 'colors': differences})
    result['pass'] = bool(result['rows'] and result['geometry_matches_visible14_207_window_lt48']
                          and result['visible_palette_matches_world'] and result['backdrop_bands_match_world_f140'])
    result['limits'] = ['Unrendered colors may advance during blank rows; these differences remain explicit']
    return result


def negative_checks(rom, arena, catalog, manifest, reference):
    checks = []
    for kind in ('identity', 'binding', 'palette', 'tile', 'cell', 'extra_tile', 'payload'):
        bad, info = copy.deepcopy(catalog), copy.deepcopy(manifest)
        if kind == 'identity':
            bad['arena_offset'] = 0
        elif kind == 'binding':
            bad['resource_binding']['resource_list'] = 0x21720
        elif kind == 'palette':
            bad['initial_scene_palette_0'][0] ^= 1
        elif kind == 'tile':
            bad['packed_4bpp_tiles'][0] = 'ab' * 32
        elif kind == 'cell':
            bad['planes'][0]['cells'][0]['flags'] |= 1
        elif kind == 'extra_tile':
            bad['packed_4bpp_tiles'].append('ab' * 32)
        else:
            bad['vram'] = '00'
        try:
            verify_catalog(rom, arena, bad, info)
        except (ValueError, KeyError):
            checks.append(kind)
        else:
            raise ValueError(f'negative catalog mutation accepted: {kind}')
    if reference:
        # Use the brightest original sample so palette/scroll corruption must
        # change real pixels; retain each sample's actual blank/window rows.
        selected = max(reference['frames'], key=lambda sample: sum(sum(row['cram']) for row in sample['active_lines']))
        for kind in ('timing', 'raster_palette', 'raster_scroll', 'active_vram', 'raster_mode', 'video'):
            sample = copy.deepcopy(selected)
            if kind == 'timing':
                sample['video_next_frame'] = sample['frame']
            elif kind == 'raster_palette':
                for row in sample['active_lines']:
                    row['cram'] = [0] * 64
            elif kind == 'raster_scroll':
                for row in sample['active_lines']:
                    row['hscroll'] = '00000000'
            elif kind == 'active_vram':
                sample['active_background_vram_changed'] = True
            elif kind == 'raster_mode':
                sample['active_lines'][80]['registers'][11] |= 4
            else:
                sample['video_next'] = '00' * (320 * 224 * 2)
            try:
                report = verify_pixels(catalog, {'frames': [sample]})
            except (ValueError, KeyError):
                pass
            else:
                require(not report['pass'], f'negative pixel mutation accepted: {kind}')
            checks.append(kind)
    return checks


def write_preview(path, colors):
    """Standalone lossless PNG of565 comparison pixels, with no dependencies."""
    pixels = bytearray()
    for y in range(224):
        pixels.append(0)
        for value in colors[y * 320:(y + 1) * 320]:
            pixels.extend(((value >> 11 & 31) * 255 // 31,
                           (value >> 5 & 63) * 255 // 63, (value & 31) * 255 // 31))

    def chunk(kind, raw):
        return struct.pack('>I', len(raw)) + kind + raw + struct.pack('>I', zlib.crc32(kind + raw))

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', 320, 224, 8, 2, 0, 0, 0))
                     + chunk(b'IDAT', zlib.compress(pixels)) + chunk(b'IEND', b''))


def verify_auxiliary(rom, assets, references=None):
    path = assets / 'arena-a-aux-assets.json.gz'
    require(path.exists(), 'missing compact arena auxiliary resource: arena-a-aux-assets.json.gz')
    data = read_json(path)
    manifest = read_json(assets / 'arena-a-aux-assets-manifest.json')
    require(set(data) == {'schema', 'rom_sha256', 'arena_offset', 'scene_behavior', 'callback',
                          'resource_source', 'packed_4bpp_tiles', 'scope'}, 'unexpected auxiliary runtime payload')
    require(data.get('schema') == 1 and data.get('rom_sha256') == ROM_SHA and data.get('arena_offset') == 10
            and data.get('scene_behavior') == 0x21126 and data.get('callback') == 0x21258
            and data.get('resource_source') == 0x297056, 'wrong auxiliary typed identity')
    binding, resources = original_binding(rom, 10)
    source = next(row for row in resources if row['source'] == 0x297056)
    require(source['destination'] == 0xba00 and len(source['bytes']) == 48 * 32, 'wrong original auxiliary source size')
    expected = [source['bytes'][index:index + 32].hex() for index in range(0, 48 * 32, 32)]
    require(data.get('packed_4bpp_tiles') == expected, 'auxiliary48 indexed tiles differ from original source order')
    require(manifest.get('resource_binding') == binding and manifest.get('decoded_sha256') == digest(source['bytes'])
            and manifest.get('destination') == 0xba00 and manifest.get('first_hardware_tile') == 0x5d0
            and manifest.get('catalog_sha256') == digest(gzip.decompress(path.read_bytes()))
            and manifest.get('gzip_sha256') == digest(path.read_bytes()), 'auxiliary original source proof differs')
    evidence = []
    if references:
        reference_path = references / 'arena-assets-a-reference.json.gz'
        reference = read_json(reference_path)
        original = read_json(references / 'arena-assets-a-reference-manifest.json')
        require(original.get('pass') is True and original.get('control_mismatches') == 0
                and original.get('frames_match_original_core', 0) >= 3602
                and original.get('fixture_sha256') == digest(gzip.decompress(reference_path.read_bytes())),
                'auxiliary reference is not bound to complete original control')
        require(reference['frames'] and any(row.get('resource_list') == 0x21834 for row in reference['observed_loads']),
                'auxiliary original load was not observed')
        for sample in reference['frames']:
            require(sample['arena_offset'] == 10, 'wrong original auxiliary scene')
            vram = bytes.fromhex(sample['active_vram'])
            require(len(vram) == 65536 and vram[0xba00:0xc000] == source['bytes'],
                    'auxiliary source differs from original loaded VRAM')
            evidence.append(sample['frame'])
    return {'pass': True, 'catalog_sha256': digest(gzip.decompress(path.read_bytes())),
            'gzip_sha256': digest(path.read_bytes()), 'tiles': 48, 'loaded_original_frames': evidence,
            'limits': ['This checks auxiliary indexed source, not sprite/plane final composition']}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--assets', type=Path, default=ROOT / 'games/yuyuhakusho/data')
    parser.add_argument('--references', type=Path)
    parser.add_argument('--negative-checks', action='store_true')
    parser.add_argument('--previews', type=Path)
    parser.add_argument('--arena', action='append', type=lambda value: int(value, 16))
    parser.add_argument('--aux-only', action='store_true')
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    require(digest(rom) == ROM_SHA, 'pinned original ROM differs')
    if args.aux_only:
        try:
            result = verify_auxiliary(rom, args.assets, args.references)
        except (ValueError, KeyError, OSError) as error:
            result = {'pass': False, 'error': str(error)}
        report = {'scope': 'Arena a auxiliary48 indexed tiles source and loaded original verification',
                  'verifier_sha256': digest(Path(__file__).read_bytes()), 'rom_sha256': ROM_SHA, **result}
        raw = json.dumps(report, indent=2) + '\n'
        if args.report:
            target = args.report.resolve()
            require(target.is_relative_to(ROOT), 'report must remain inside worktree')
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(raw)
        print(raw, end='')
        raise SystemExit(0 if report['pass'] else 1)
    selected = sorted(set(args.arena or REGISTRY))
    require(selected and all(arena in REGISTRY for arena in selected), 'unknown selected arena')
    results = []
    for arena in selected:
        row = {'arena_offset': arena}
        try:
            path = args.assets / f'arena-assets-{arena:x}.json.gz'
            require(path.exists(), f'missing compact arena resource: {path.name}')
            catalog = read_json(path)
            manifest = read_json(args.assets / f'arena-assets-{arena:x}-manifest.json')
            require(manifest.get('catalog_sha256') == digest(gzip.decompress(path.read_bytes()))
                    and manifest.get('gzip_sha256') == digest(path.read_bytes()), 'compact catalog digest differs from manifest')
            _, decoded, bank = verify_catalog(rom, arena, catalog, manifest)
            row.update({'catalog_sha256': digest(gzip.decompress(path.read_bytes())),
                        'gzip_sha256': digest(path.read_bytes()), 'tiles': len(bank)})
            if args.references:
                reference_path = args.references / f'arena-assets-{arena:x}-reference.json.gz'
                reference = read_json(reference_path)
                source_manifest = read_json(args.references / f'arena-assets-{arena:x}-reference-manifest.json')
                require(source_manifest.get('pass') is True and source_manifest.get('control_mismatches') == 0
                        and source_manifest.get('frames_match_original_core', 0) >= 3602
                        and source_manifest.get('fixture_sha256') == digest(gzip.decompress(reference_path.read_bytes()))
                        and source_manifest.get('gzip_sha256') == digest(reference_path.read_bytes()),
                        'original reference is not bound to a complete matched control')
                row['loaded_original_frames'] = verify_loaded(rom, catalog, decoded, bank, reference)
                row['pixels'] = verify_pixels(catalog, reference)
                require(row['pixels']['pass'], 'independent compact arena pixels differ from original video')
                row['runtime_row_analysis'] = analyze_runtime_rows(reference)
                require(row['runtime_row_analysis']['pass'], 'original raster profile differs in a visible or backdrop input')
                if args.previews:
                    directory = args.previews.resolve()
                    require(directory.is_relative_to(ROOT), 'previews must remain inside worktree')
                    sample = reference['frames'][1]
                    colors, _, _, _ = render_pixels(catalog, sample)
                    write_preview(directory / f'arena-{arena:x}-background-{sample["video_next_frame"]}.png', colors)
                    video = bytes.fromhex(sample['video_next'])
                    write_preview(directory / f'arena-{arena:x}-original-{sample["video_next_frame"]}.png',
                                  [video[position] | video[position + 1] << 8 for position in range(0, len(video), 2)])
            else:
                reference = None
            if args.negative_checks:
                row['negative_checks'] = negative_checks(rom, arena, catalog, manifest, reference)
            row['pass'] = True
        except (ValueError, KeyError, OSError) as error:
            row.update({'pass': False, 'error': str(error)})
        results.append(row)
    report = {'scope': 'Seven additional arenas: strict source, compact schema and optional loaded original verification',
              'verifier_sha256': digest(Path(__file__).read_bytes()), 'rom_sha256': ROM_SHA,
              'results': results, 'pass': all(row['pass'] for row in results)}
    raw = json.dumps(report, indent=2) + '\n'
    if args.report:
        target = args.report.resolve()
        require(target.is_relative_to(ROOT), 'report must remain inside worktree')
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(raw)
    print(raw, end='')
    raise SystemExit(0 if report['pass'] else 1)


if __name__ == '__main__':
    main()
