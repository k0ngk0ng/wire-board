#!/usr/bin/env python3
"""Read-only Fighter4 dragon asset audit; emits metadata, never tile/ROM payloads.

Failure cases and scope were fixed first in
verification/FIGHTER4_DRAGON_ASSET_AUDIT.md. Uses only the Python standard library.
"""
import argparse
import collections
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
GRAPHICS, BANK, ANIMATION, SHAPES = 0xd8f1a, 0x240000, 0xd92ca, 0xd9a3a
HEAD = {14: 84, 15: 82, 16: 76, 17: 78, 18: 80}
BOOST = [92, 94, 96, 98]
BODY = {86, 88, 90}
PARAMETERS = {0, *HEAD.values(), *BOOST, *BODY}
FIXTURES = ['boundaries', 'sequences', 'oracle']


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def require(condition, message):
    if not condition:
        raise ValueError(message)


def read_json(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def number(raw, offset, size=2, signed=False):
    require(0 <= offset and offset+size <= len(raw), f'out of bounds at {offset:#x}')
    return int.from_bytes(raw[offset:offset+size], 'big', signed=signed)


def actor_bytes(snapshot, object_id):
    base = snapshot['object_id']
    if base <= object_id < base+1024:
        start = object_id-base
        return bytes.fromhex(snapshot['family'])[start:start+256]
    require(0x9400 <= object_id < 0x9c00, f'unknown actor {object_id:#x}')
    start = object_id-0x9400
    return bytes.fromhex(snapshot['pool']['bytes'])[start:start+128]


def draw_from_snapshot(snapshot, object_id):
    raw = actor_bytes(snapshot, object_id)
    return {'ObjectID': object_id, 'GraphicsSet': number(raw, 0x1c, 4),
            'TileBank': number(raw, 0x20, 4), 'Parameter': number(raw, 0x3c),
            'ScreenX': number(raw, 8), 'ScreenY': number(raw, 12),
            'OffsetX': number(raw, 0x28, signed=True), 'OffsetY': number(raw, 0x2a, signed=True),
            'Attributes': number(raw, 0x24), 'Depth': number(raw, 0x3e),
            'Layer': raw[0x26], 'SkipTiles': raw[0x27],
            'Cache': [number(raw, 0x2c+i*4, 4) for i in range(4)]}


def decode_source(rom, parameter):
    offset = number(rom, GRAPHICS+parameter, signed=True)
    require(offset >= number(rom, GRAPHICS), f'invalid graphics offset {parameter}')
    start, cursor = GRAPHICS+offset, GRAPHICS+offset
    descriptors = []
    for _ in range(4):
        word = number(rom, cursor, 4)
        cursor += 4
        descriptors.append(word & 0x7fffffff)
        if word & 0x80000000:
            break
    else:
        raise ValueError(f'unterminated transfer list {parameter}')
    pieces = []
    for _ in range(80):
        word = number(rom, cursor, 4)
        cursor += 4
        pieces.append({'center_x': ((word >> 24) & 127)-64,
                       'center_y': ((word >> 16) & 127)-64,
                       'width_tiles': ((word >> 10) & 3)+1,
                       'height_tiles': ((word >> 8) & 3)+1,
                       'first_slot': word & 255, 'xor_attributes': (word & 0xf000) >> 1})
        if word & 0x80000000:
            break
    else:
        raise ValueError(f'unterminated sprite piece list {parameter}')
    require(cursor <= ANIMATION, f'frame extends beyond graphics table {parameter}')
    return start, rom[start:cursor], descriptors, pieces


def rows(value):
    for row in value:
        if 'calls' in row:
            yield from rows(row['calls'])
        else:
            yield row


def audit():
    rom_path = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    catalog_path = ROOT/'games/yuyuhakusho/data/world-sprites.json.gz'
    manifest_path = ROOT/'games/yuyuhakusho/data/world-sprites-manifest.json'
    oracle_path = ROOT/'games/yuyuhakusho/data/world-sprites-oracle.json.gz'
    rules_path = ROOT/'games/yuyuhakusho/engine/testdata/fighter4-projectile-rules.json'
    rom = rom_path.read_bytes()
    require(sha(rom) == ROM_SHA, 'selected ROM identity differs')
    manifest, catalog, oracle, rules = [read_json(p) for p in [manifest_path, catalog_path, oracle_path, rules_path]]
    require(manifest['pass'] and manifest['identity']['rom_sha256'] == ROM_SHA, 'manifest identity/pass differs')
    for path, key in [(catalog_path, 'catalog'), (oracle_path, 'oracle')]:
        require(sha(path.read_bytes()) == manifest[key+'_gzip_sha256'], f'{key} compressed digest differs')
        require(sha(gzip.decompress(path.read_bytes())) == manifest[key+'_sha256'], f'{key} digest differs')
    require(catalog['schema'] == oracle['schema'] == 1, 'unsupported catalog/oracle schema')
    require(catalog['rom_sha256'] == oracle['rom_sha256'] == ROM_SHA, 'catalog/oracle source identity differs')
    require(catalog['tile_format'] == 'md-4bpp-row-major-high-nibble-first', 'tile format differs')
    tile_bytes = bytes.fromhex(catalog['tile_bytes'])
    require(len(tile_bytes) % 32 == 0 and len(tile_bytes)//32 == manifest['tiles'], 'tile bank size differs')
    frames = {(f['key']['graphics_set'], f['key']['parameter']): f for f in catalog['frames']}
    require(len(frames) == len(catalog['frames']) == manifest['all_frames'], 'duplicate or missing catalog frame')
    library = {(f['graphics_set'], f['parameter']): f for f in oracle['library']}
    require(len(library) == len(oracle['library']), 'duplicate oracle frame')
    expected_animations = {(ANIMATION, SHAPES, pose, 0): value for pose, value in HEAD.items()}
    expected_animations.update({(ANIMATION, SHAPES, 19, index): value for index, value in enumerate(BOOST)})
    animations = {(a['key']['animation_set'], a['key']['shape_set'], a['key']['pose'], a['key']['index']): a['data']
                  for a in rules['animations']}
    require(len(animations) == len(rules['animations']) == 9, 'expected nine unique AN records')
    require({key: value['graphic_parameter'] for key, value in animations.items()} == expected_animations,
            'AN graph or graphic parameter differs')
    body_table = list(rom[0x13f76:0x13f7e])
    require(body_table == [90, 88, 86, 88, 90, 88, 86, 88], 'body-angle direct parameter table differs')
    sources = [catalog_path, manifest_path, oracle_path, rules_path]
    counts = collections.Counter()
    fixture_summaries, dirty = [], []
    for fixture in FIXTURES:
        path = ROOT/f'games/yuyuhakusho/engine/testdata/fighter4-projectile-{fixture}.json.gz'
        sources.append(path)
        document = read_json(path)
        observed_library = {(a['key']['animation_set'], a['key']['shape_set'], a['key']['pose'], a['key']['index']): a['data']
                            for a in document['library']}
        require(observed_library == animations, f'{fixture} captured AN data differs from semantic rules')
        calls, draws, retained = 0, 0, 0
        for row in rows(document['calls']):
            calls += 1
            request = row.get('draw_request')
            require(bool(row['draw']) == (request is not None), f'{fixture}: draw request presence differs')
            if request is None:
                continue
            draws += 1
            require(request == draw_from_snapshot(row['after'], row['object_id']),
                    f'{fixture}: callback draw-time actor differs at {row.get("name", row["sequence"])}')
            require(request['GraphicsSet'] == GRAPHICS and request['TileBank'] == BANK,
                    f'{fixture}: unexpected graphics set or tile bank')
            parameter = request['Parameter']
            if parameter not in PARAMETERS:
                before = draw_from_snapshot(row['before'], row['object_id'])
                environment = row['before']['environment']
                stage = number(actor_bytes(row['before'], row['object_id']), 6)
                require(fixture == 'boundaries' and stage == 2 and
                        (environment['pause_flags'] & 128 or environment['slow_flag'] != 0) and request == before,
                        f'{fixture}: unexpected generated graphic parameter {parameter}')
                retained += 1
                dirty.append({'name': row['name'], 'kind': row['kind'], 'parameter': parameter,
                              'stage': stage, 'pause_flags': environment['pause_flags'],
                              'slow_flag': environment['slow_flag'], 'entire_draw_unchanged': True,
                              'asset_coverage': False})
                continue
            allowed = {'head': {0, *HEAD.values()}, 'trail': {0, *BODY}, 'boost': {0, *BOOST}}
            require(parameter in allowed[row['kind']], f'unexpected kind/parameter {row["kind"]}/{parameter}')
            counts[(parameter, row['kind'], fixture)] += 1
        fixture_summaries.append({'fixture': fixture, 'callbacks': calls, 'draw_requests': draws,
                                  'covered_draw_requests': draws-retained,
                                  'retained_dirty_paused_requests': retained})
    require({key[0] for key in counts} == PARAMETERS, 'a required key is not exercised by saved CPU requests')
    required = []
    unique_tiles = set()
    for parameter in sorted(PARAMETERS):
        frame = frames.get((GRAPHICS, parameter))
        require(frame is not None, f'missing graphics key {GRAPHICS:#x}/{parameter}')
        require(frame['tile_bank'] == BANK and not frame['preloaded_only'] and frame['transfers'] and frame['pieces'],
                f'incomplete exact asset key {parameter}')
        source = library.get((GRAPHICS, parameter))
        require(source is not None, f'missing saved source frame {parameter}')
        start, payload, stamps, pieces = decode_source(rom, parameter)
        require(source['tiles_base'] == BANK and not source['preloaded_only'] and source['source_address'] == start and
                source['payload'] == payload.hex() and source['source_sha256'] == sha(payload),
                f'original oracle source differs {parameter}')
        require(pieces == frame['pieces'] and stamps == [t['stamp'] for t in frame['transfers']],
                f'decoded frame differs {parameter}')
        slots, transfers, indexed_tiles = {}, [], []
        for transfer in frame['transfers']:
            stamp = transfer['stamp']
            count, destination = stamp & 255, (stamp >> 8) & 255
            require(len(transfer['tiles']) == count, f'tile transfer length differs {parameter}')
            first = BANK+(stamp >> 16)*32
            for index, tile_id in enumerate(transfer['tiles']):
                require(0 <= tile_id < len(tile_bytes)//32, f'tile ID outside catalog {parameter}')
                tile = tile_bytes[tile_id*32:(tile_id+1)*32]
                require(tile == rom[first+index*32:first+(index+1)*32], f'tile source bytes differ {parameter}/{index}')
                slots[destination+index] = tile
                indexed_tiles.append(tile_id)
            transfers.append({'stamp': stamp, 'rom_first_tile_address': hex(first), 'tile_count': count,
                              'rom_tiles_sha256': sha(rom[first:first+count*32]), 'catalog_tile_ids': transfer['tiles']})
        used = set()
        opaque_pixels = 0
        for piece in pieces:
            for index in range(piece['width_tiles']*piece['height_tiles']):
                slot = piece['first_slot']+index
                require(slot in slots, f'piece tile slot unassigned {parameter}/{slot}')
                used.add(slot)
                opaque_pixels += sum((byte >> 4 != 0)+(byte & 15 != 0) for byte in slots[slot])
        require(parameter == 0 or opaque_pixels > 0, f'opaque graphic became transparent {parameter}')
        unique_tiles.update(indexed_tiles)
        consumers = [{'kind': kind, 'fixture': fixture, 'draw_requests': count}
                     for (value, kind, fixture), count in sorted(counts.items()) if value == parameter]
        required.append({'graphics_set': hex(GRAPHICS), 'tile_bank': hex(BANK), 'parameter': parameter,
                         'covered': True, 'preloaded_only': False, 'source_frame_address': hex(start),
                         'source_payload_sha256': sha(payload), 'piece_count': len(pieces),
                         'used_relative_slots': sorted(used), 'opaque_piece_pixels': opaque_pixels,
                         'transfers': transfers, 'consumers': consumers})
    return {'scope': 'Fighter4 head13bb6/trail13e9a/boost13fc8 saved indexed-asset coverage; callback draw-time boundary',
            'pass': True, 'direct_catalog_reuse': True, 'missing_assets': [],
            'rom_sha256': ROM_SHA, 'audit_script_sha256': sha(Path(__file__).read_bytes()),
            'sources': [{'path': str(path.relative_to(ROOT)), 'sha256': sha(path.read_bytes()),
                         'bytes': path.stat().st_size} for path in sources],
            'catalog_graphic_keys': len(frames), 'catalog_tiles': len(tile_bytes)//32,
            'required_graphic_keys': len(required), 'required_unique_indexed_tiles': len(unique_tiles),
            'animations': [{'animation_set': hex(key[0]), 'shape_set': hex(key[1]), 'pose': key[2],
                            'index': key[3], 'parameter': value} for key, value in sorted(expected_animations.items())],
            'body_angle_table': {'source_address': '0x13f76', 'direct_parameters': body_table,
                                 'parameter_scaling': 1},
            'fixtures': fixture_summaries, 'frames': required,
            'retained_dirty_paused_requests': dirty,
            'limitations': [f'The {len(dirty)} retained dirty stage-two paused requests are explicitly outside asset coverage, not missing runtime assets.',
                           'Parameter0 is a legitimate transparent ROM frame used after initialization under pause; no fallback is consulted.',
                           'This audit validates saved descriptors, pieces, indexed tiles, and original callback requests; it does not execute CPU code or claim decompilation.',
                           'Presentation-owned cache/skip/global slot persistence, palette timing, full natural frame composition, and scanline behavior require their own acceptance evidence.',
                           'The raw 68000 ROM is development-only; neither ROM nor emulator core is an output of this audit.']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    output = args.out.resolve()
    require(output.is_relative_to(ROOT), 'audit output must remain inside this worktree')
    report = audit()
    require(output.parent.is_dir(), 'audit output parent must already exist')
    output.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'pass': report['pass'], 'direct_catalog_reuse': report['direct_catalog_reuse'],
                      'required_graphic_keys': report['required_graphic_keys'],
                      'required_unique_indexed_tiles': report['required_unique_indexed_tiles'],
                      'missing_assets': report['missing_assets'], 'report_sha256': sha(output.read_bytes())}, indent=2))


if __name__ == '__main__':
    main()
