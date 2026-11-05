#!/usr/bin/env python3
"""Offline ROM-bound indexed backgrounds for arena offsets2..e.

Original snapshots resolve only bytes outside each ROM resource's upload range.
No ROM, VRAM image, video or emulator code enters the compact runtime catalogs.
"""
import argparse
import gzip
import json
from pathlib import Path

from extract_hud_assets import decode_resource
from reference import ROOT, ROM_SHA256, sha

ARENAS = {
    2: (0x20ae0, 0x2175c), 4: (0x20cb2, 0x21798),
    6: (0x20e28, 0x217cc), 8: (0x20fb4, 0x21800),
    10: (0x21126, 0x21834), 12: (0x212f8, 0x21870),
    14: (0x21412, 0x218a4),
}


def require(value, message):
    if not value:
        raise ValueError(message)


def be(raw, position, size=2):
    require(0 <= position <= len(raw) - size, 'ROM source outside target')
    return int.from_bytes(raw[position:position + size], 'big')


def resource_bindings(rom, arena):
    callback, source = ARENAS[arena]
    require(be(rom, 0xf94c + arena * 2, 4) == callback and
            be(rom, 0x21700 + arena * 2, 4) == source, 'ROM arena registry differs')
    cursor, resources, decoded, facts = source, [], [], []
    while be(rom, cursor) != 0xffff:
        kind, address, destination = be(rom, cursor), be(rom, cursor + 2, 4), be(rom, cursor + 6)
        require(kind in (0, 6), 'unsupported original resource')
        entry = {'kind': kind, 'source': address, 'destination': destination}
        if kind == 6:
            entry.update({'encoded_bytes': be(rom, address),
                          'encoded_sha256': sha(rom[address:address + 2 + be(rom, address)])})
            raw, tokens = decode_resource(rom, address)
            require(destination + len(raw) <= 65536 and len(raw) % 2 == 0,
                    'original source has unsupported upload range')
            decoded.append((entry, raw))
            facts.append({'source': address, 'destination': destination, 'decoded_bytes': len(raw),
                          'decoded_sha256': sha(raw), 'token_counts': tokens,
                          'encoded_bytes': entry['encoded_bytes'], 'encoded_sha256': entry['encoded_sha256']})
        else:
            entry['length'] = be(rom, cursor + 8)
        resources.append(entry)
        cursor += 10 if kind == 0 else 8
        require(len(resources) <= 16, 'unterminated original resource list')
    return {'arena_offset': arena, 'scene_behavior': callback, 'resource_list': source,
            'list_sha256': sha(rom[source:cursor + 2]), 'resources': resources}, decoded, facts


def ranges(values):
    """Return exclusive contiguous ranges of true positions."""
    result, start = [], None
    for index, value in enumerate([*values, False]):
        if value and start is None:
            start = index
        elif not value and start is not None:
            result.append((start, index))
            start = None
    return result


def inspect_binding(rom, arena):
    binding, decoded, facts = resource_bindings(rom, arena)
    covered = bytearray(65536)
    for entry, raw in decoded:
        start = entry['destination']
        require(not any(covered[start:start + len(raw)]), 'overlapping original arena upload sources')
        covered[start:start + len(raw)] = bytes([1]) * len(raw)
    uncovered = []
    for base in (0xc000, 0xe000):
        uncovered.extend({'start': base + start, 'end': base + end}
                         for start, end in ranges([not value for value in covered[base:base + 0x1000]]))
    return {'arena_offset': arena, 'resource_binding': binding, 'decoded_resources': facts,
            'unwritten_map_ranges': uncovered,
            'loaded_original_required': True,
            'limits': ['Static resource decoding does not establish natural video or dynamic material behavior']}


def read_reference(path, arena):
    raw = gzip.decompress(path.read_bytes())
    data = json.loads(raw)
    manifest = json.loads(path.with_name(path.name.replace('.json.gz', '-manifest.json')).read_bytes())
    require(manifest.get('pass') is True and manifest.get('control_mismatches') == 0
            and manifest.get('frames_match_original_core', 0) >= 3602
            and manifest.get('fixture_sha256') == sha(raw) and manifest.get('gzip_sha256') == sha(path.read_bytes()),
            'original reference is not bound to a complete matched control')
    require(data.get('identity', {}).get('rom_sha256') == ROM_SHA256, 'reference ROM identity differs')
    require(data.get('frames') and all(frame.get('arena_offset') == arena for frame in data['frames']),
            'missing selected original arena samples')
    require(any(load.get('resource_list') == ARENAS[arena][1] for load in data.get('observed_loads', [])),
            'original resource load was not observed')
    return data, sha(raw), sha(path.read_bytes())


def build_catalog(rom, arena, reference):
    binding, decoded, facts = resource_bindings(rom, arena)
    first = reference['frames'][0]
    vram = bytes.fromhex(first['active_vram'])
    require(len(vram) == 65536, 'missing original loaded video memory')
    covered = bytearray(65536)
    material = bytearray(vram)
    for entry, raw in decoded:
        start = entry['destination']
        require(vram[start:start + len(raw)] == raw, f'ROM source {entry["source"]:x} differs from loaded original')
        material[start:start + len(raw)] = raw
        covered[start:start + len(raw)] = bytes([1]) * len(raw)
    palette = binding['resources'][-1]
    require(palette['kind'] == 0 and palette['destination'] == 0xe080 and palette['length'] == 32,
            'unexpected initial scene palette')
    words = [be(rom, palette['source'] + i * 2) for i in range(16)]
    result = {'schema': 1, 'rom_sha256': ROM_SHA256, 'arena_offset': arena,
              'scene_behavior': ARENAS[arena][0], 'resource_binding': binding,
              'packed_4bpp_tiles': [], 'planes': [],
              'initial_scene_palette_0': [(value >> 1 & 7) | (value >> 2 & 0x38) | (value >> 3 & 0x1c0) for value in words],
              'runtime_inputs': ['horizontal scroll rows for A/B', 'vertical scroll for A/B', 'visible/blank rows',
                                 'current palette including raster backdrop changes', 'separate window/HUD plane'],
              'scope': f'Arena offset{arena:x} ROM-bound static A/B indexed material; scene rules and HUD are separate'}
    local_tiles, unresolved = {}, []
    registers = first['registers']
    require(len(registers) == 32 and registers[16] == 1, 'unsupported original plane geometry')
    for name, base in zip(('A', 'B'), ((registers[2] & 0x38) << 10, (registers[4] & 7) << 13)):
        plane = {'name': name, 'columns': 64, 'rows': 32, 'cells': []}
        for index in range(2048):
            attribute = be(material, base + index * 2)
            start = (attribute & 0x7ff) * 32
            require(all(covered[start:start + 32]), f'background tile {start:x} has no confirmed ROM resource source')
            packed = material[start:start + 32].hex()
            if packed not in local_tiles:
                local_tiles[packed] = len(result['packed_4bpp_tiles'])
                result['packed_4bpp_tiles'].append(packed)
            plane['cells'].append({'tile': local_tiles[packed], 'flags': attribute & 0xf800})
        result['planes'].append(plane)
        for start, end in ranges([not byte for byte in covered[base:base + 0x1000]]):
            raw = vram[base + start:base + end]
            unresolved.append({'plane': name, 'start': base + start, 'end': base + end,
                               'loaded_sha256': sha(raw), 'loaded_words': sorted({be(raw, i) for i in range(0, len(raw), 2)}),
                               'source': 'Observed loaded original; outside this arena resource list upload ranges'})
    initial_palette = reference.get('initial_scene_palette_observation', {})
    palette_seen = (initial_palette.get('pc') == 0x83c and initial_palette.get('frame', 0) > 0
                    and initial_palette.get('bytes') == rom[palette['source']:palette['source'] + 32].hex())
    frame_evidence = []
    for frame in reference['frames']:
        require(frame['arena_offset'] == arena and frame.get('video_next_frame') == frame['frame'] + 1,
                'unbound original frame identity or video timing')
        require(not frame.get('active_background_vram_changed'), 'static catalog cannot cover active background VRAM changes')
        loaded = bytes.fromhex(frame['active_vram'])
        regs = frame['registers']
        require(len(loaded) == 65536 and regs[16] == 1, 'unsupported original frame geometry')
        for plane, base in zip(result['planes'], ((regs[2] & 0x38) << 10, (regs[4] & 7) << 13)):
            for index, cell in enumerate(plane['cells']):
                attribute = be(loaded, base + index * 2)
                start = (attribute & 0x7ff) * 32
                require(cell['flags'] == attribute & 0xf800 and result['packed_4bpp_tiles'][cell['tile']] == loaded[start:start + 32].hex(),
                        f'compact static material differs at original frame{frame["frame"]} {plane["name"]} cell{index}')
        ram = bytes.fromhex(frame['ram'])
        palette_seen |= all(be(ram, 0xe080 + i * 2) == value for i, value in enumerate(words))
        frame_evidence.append({'frame': frame['frame'], 'video_next_frame': frame['video_next_frame'],
                               'active_vram_sha256': sha(loaded), 'ram_sha256': sha(ram),
                               'video_next_sha256': sha(bytes.fromhex(frame['video_next']))})
    require(palette_seen, 'initial ROM scene palette was not observed in loaded original')
    return result, {'resource_binding': binding, 'decoded_resources': facts,
                    'unwritten_map_ranges': unresolved, 'loaded_original_frames': frame_evidence,
                    'initial_scene_palette_observed': palette_seen,
                    'initial_scene_palette_observation': initial_palette}


def export_auxiliary(rom, reference, reference_sha, reference_gzip, out):
    binding, decoded, facts = resource_bindings(rom, 10)
    entry, raw = next((entry, raw) for entry, raw in decoded if entry['source'] == 0x297056)
    require(entry['destination'] == 0xba00 and len(raw) == 48 * 32, 'unexpected original arena auxiliary upload')
    observed = []
    for sample in reference['frames']:
        vram = bytes.fromhex(sample['active_vram'])
        require(sample['arena_offset'] == 10 and len(vram) == 65536 and vram[0xba00:0xc000] == raw,
                'auxiliary material differs from original loaded upload')
        for row in sample.get('active_vram_changes', []):
            for change in row['ranges']:
                start, end = change['address'], change['address'] + len(bytes.fromhex(change['bytes']))
                require(end <= 0xba00 or start >= 0xc000, 'unsupported active auxiliary indexed-material change')
        observed.append({'frame': sample['frame'], 'loaded_sha256': sha(vram[0xba00:0xc000])})
    catalog = {'schema': 1, 'rom_sha256': ROM_SHA256, 'arena_offset': 10, 'scene_behavior': 0x21126,
               'callback': 0x21258, 'resource_source': 0x297056,
               'packed_4bpp_tiles': [raw[index:index + 32].hex() for index in range(0, len(raw), 32)],
               'scope': 'Arena offseta six raw21258 sprites indexed material; positions and palette are current world inputs'}
    canonical = (json.dumps(catalog, separators=(',', ':')) + '\n').encode()
    compressed = gzip.compress(canonical, mtime=0)
    (out / 'arena-a-aux-assets.json.gz').write_bytes(compressed)
    source_fact = next(row for row in facts if row['source'] == 0x297056)
    manifest = {'pass': True, 'scope': catalog['scope'], 'rom_sha256': ROM_SHA256,
                'arena_offset': 10, 'callback': 0x21258, 'resource_binding': binding, **source_fact,
                'producer_sha256': sha(Path(__file__).read_bytes()), 'first_hardware_tile': 0x5d0,
                'tiles': 48, 'tile_order': 'Original sequential48 tiles; runtime local starts0/16/32',
                'reference_sha256': reference_sha, 'reference_gzip_sha256': reference_gzip,
                'catalog_sha256': sha(canonical), 'gzip_sha256': sha(compressed), 'gzip_bytes': len(compressed),
                'loaded_original_frames': observed,
                'limits': ['Indexed auxiliary source only; six draw intents and final composition are independent checks']}
    (out / 'arena-a-aux-assets-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    return manifest


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--references', type=Path)
    parser.add_argument('--inspect-only', action='store_true')
    parser.add_argument('--arena', action='append', type=lambda value: int(value, 16))
    args = parser.parse_args()
    out = args.out.resolve()
    require(out.is_relative_to(ROOT), 'outputs must remain inside this worktree')
    out.mkdir(parents=True, exist_ok=True)
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    require(sha(rom) == ROM_SHA256, 'pinned ROM identity differs')
    selected = sorted(set(args.arena or ARENAS))
    require(selected and all(arena in ARENAS for arena in selected), 'unknown selected arena')
    analysis = {'rom_sha256': ROM_SHA256, 'producer_sha256': sha(Path(__file__).read_bytes()),
                'scope': 'Original static source binding inventory; no natural pixel acceptance implied',
                'arenas': [inspect_binding(rom, arena) for arena in selected]}
    (out / 'additional-arena-source-analysis.json').write_text(json.dumps(analysis, indent=2) + '\n')
    if args.inspect_only:
        print(json.dumps({'pass': True, 'arena_sources': len(analysis['arenas']),
                          'scope': analysis['scope'], 'out': str(out)}))
        return
    require(args.references is not None, 'loaded original references are required before compact export')
    results = []
    for arena in selected:
        path = args.references / f'arena-assets-{arena:x}-reference.json.gz'
        reference, reference_sha, reference_gzip = read_reference(path, arena)
        catalog, evidence = build_catalog(rom, arena, reference)
        raw = (json.dumps(catalog, separators=(',', ':')) + '\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        name = f'arena-assets-{arena:x}'
        (out / f'{name}.json.gz').write_bytes(compressed)
        report = {'pass': True, 'scope': catalog['scope'], 'rom_sha256': ROM_SHA256, 'arena_offset': arena,
                  'producer_sha256': sha(Path(__file__).read_bytes()),
                  'compression_decoder_sha256': sha((ROOT / 'scripts/yuyuhakusho/extract_hud_assets.py').read_bytes()),
                  'reference_sha256': reference_sha, 'reference_gzip_sha256': reference_gzip,
                  'catalog_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'gzip_bytes': len(compressed),
                  'tiles': len(catalog['packed_4bpp_tiles']), 'plane_cells': 4096,
                  'limits': ['Selected original static A/B material only',
                             'Natural pixel validation is reported separately',
                             'Camera, dynamic palettes and HUD are independent runtime inputs'], **evidence}
        (out / f'{name}-manifest.json').write_text(json.dumps(report, indent=2) + '\n')
        if arena == 10:
            export_auxiliary(rom, reference, reference_sha, reference_gzip, out)
        results.append({'arena_offset': arena, 'tiles': report['tiles'], 'gzip_bytes': len(compressed), 'pass': True})
    print(json.dumps({'pass': True, 'results': results}, indent=2))


if __name__ == '__main__':
    main()
