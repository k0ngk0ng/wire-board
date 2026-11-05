#!/usr/bin/env python3
"""Read-only source5 projectile asset audit; emits metadata, never ROM/tile bytes.

Failure cases were fixed first in verification/FIGHTER5_PROJECTILE_ASSET_AUDIT.md.
Only the Python standard library is needed; no CPU/core or renderer is executed.
"""
import argparse
import collections
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
BASE = Path('games/yuyuhakusho')
TESTDATA = BASE/'engine/testdata'
CHILD = (0xd92ca, 0xd9a3a)
PARENT = (0xca6da, 0xcb38a)
# Each binding is independently checked against original initializer instructions
# and against actual actor fields at captured AN submission boundaries.
BINDINGS = {CHILD: (0xd8f1a, 0x240000, 0xd9e4a),
            PARENT: (0xc97aa, 0x180000, 0xcbdba)}
ENTRIES = {'linear': 0x1414e, 'steered': 0x143e4, 'follower': 0x146ba,
           'linear_stop': 0x142b4, 'steered_stop': 0x1458a}
EXPECTED = {}
for pose, parameters in {0x14: [4, 6], 0x15: [4, 6], 0x16: [4, 6],
                         0x17: [4, 6, 14, 16], 0x18: [14, 16, 14, 16], 0x19: [14, 16]}.items():
    EXPECTED.update({(*CHILD, pose, index): value for index, value in enumerate(parameters)})
for pose, parameters in {0: [2], 7: [72], 0xa0: [200, 212, 218, 216, 218, 220],
                         0xa3: [252, 254, 256], 0xac: [222, 224, 226, 228, 106, 224]}.items():
    EXPECTED.update({(*PARENT, pose, index): value for index, value in enumerate(parameters)})


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def require(condition, message):
    if not condition:
        raise ValueError(message)


def number(raw, offset, size=2, signed=False):
    require(0 <= offset and offset+size <= len(raw), f'out of bounds at {offset:#x}/{size}')
    return int.from_bytes(raw[offset:offset+size], 'big', signed=signed)


def key_of(row):
    key = row['key']
    return tuple(key[name] for name in ['animation_set', 'shape_set', 'pose', 'index'])


def animation_key(actor):
    return (number(actor, 0x50, 4), number(actor, 0x8c, 4), actor[0x5f], actor[0x59])


def key_json(key):
    return {'animation_set': hex(key[0]), 'shape_set': hex(key[1]),
            'pose': key[2], 'pose_hex': hex(key[2]), 'index': key[3]}


def graphic_json(key):
    return {'graphics_set': hex(key[0]), 'tile_bank': hex(key[1]), 'parameter': key[2]}


def actor_bytes(snapshot, object_id):
    base = snapshot['object_id']
    if base <= object_id and object_id+256 <= base+1024:
        start = object_id-base
        actor = bytes.fromhex(snapshot['family'])[start:start+256]
    else:
        require(0x9400 <= object_id and object_id+128 <= 0x9c00, f'unknown actor {object_id:#x}')
        start = object_id-0x9400
        actor = bytes.fromhex(snapshot['pool']['bytes'])[start:start+128]
    require(len(actor) >= 128, 'incomplete saved actor')
    return actor


def draw_from_actor(actor, object_id):
    return {'ObjectID': object_id, 'GraphicsSet': number(actor, 0x1c, 4),
            'TileBank': number(actor, 0x20, 4), 'Parameter': number(actor, 0x3c),
            'ScreenX': number(actor, 8), 'ScreenY': number(actor, 12),
            'OffsetX': number(actor, 0x28, signed=True), 'OffsetY': number(actor, 0x2a, signed=True),
            'Attributes': number(actor, 0x24), 'Depth': number(actor, 0x3e),
            'Layer': actor[0x26], 'SkipTiles': actor[0x27],
            'Cache': [number(actor, 0x2c+i*4, 4) for i in range(4)]}


def rows(groups):
    for group in groups:
        if 'calls' in group:
            for row in group['calls']:
                yield group['name'], row
        else:
            yield None, group


def decode_animation(rom, key):
    anim, shape, pose, index = key
    _, _, shape_limit = BINDINGS[key[:2]]
    pointer_address = anim+pose*2
    record_address = anim+number(rom, pointer_address, signed=True)+index*8
    require(anim <= record_address and record_address+8 <= shape, f'AN record outside region {key}')
    record = rom[record_address:record_address+8]
    table_bytes = number(rom, shape+2, signed=True)
    require(table_bytes > 0 and table_bytes % 4 == 0 and record[1]*4 < table_bytes,
            f'shape directory outside declared region {key}')
    directory_address = shape+record[1]*4
    directory = rom[directory_address:directory_address+4]
    payload_address = shape+number(directory, 2, signed=True)
    starts = [shape+number(rom, shape+offset+2, signed=True) for offset in range(0, table_bytes, 4)]
    limit = min([start for start in starts if start > payload_address]+[shape_limit])
    require(shape+table_bytes <= payload_address <= limit, f'shape payload outside region {key}')
    cursor = payload_address

    def read_shape():
        nonlocal cursor
        require(cursor+4 <= limit, f'shape record crosses next source boundary {key}')
        row = rom[cursor:cursor+4]
        cursor += 4
        return row

    metadata, groups = [0, 0, 0, 0], [[], [], [], []]
    if directory[0] & 0x40:
        metadata = list(read_shape())
    for group, flag in enumerate([0x40, 0x20, 0x10, 0x08]):
        if not directory[0] & flag:
            continue
        for _ in range(64):
            row = read_shape()
            groups[group].append({'width': row[0], 'offset_x': row[1]-0x80,
                                  'height': row[2], 'offset_y': row[3]-0x80})
            if group == 3 or row[0] & 0x80:
                break
        else:
            raise ValueError(f'unterminated shape group {key}/{group}')
    data = {'duration': record[0], 'flags': record[2], 'enabled_stages': record[3],
            'sound': number(rom, 0x218d8+record[4], 1), 'offset_x': record[6]-0x80,
            'offset_y': record[7]-0xa0, 'graphic_parameter': directory[1]*2,
            'shape_flags': directory[0], 'metadata': metadata, 'groups': groups}
    source = {'animation_address': hex(record_address), 'animation_bytes': record.hex(),
              'directory_bytes': directory.hex(), 'shape_address': hex(payload_address),
              'shape_bytes': rom[payload_address:cursor].hex(), 'sound_index': record[4]}
    proof = {'pose_pointer_address': hex(pointer_address), 'pose_pointer_sha256': sha(rom[pointer_address:pointer_address+2]),
             'animation_address': hex(record_address), 'animation_sha256': sha(record),
             'shape_directory_address': hex(directory_address), 'shape_directory_sha256': sha(directory),
             'shape_address': hex(payload_address), 'shape_bytes': cursor-payload_address,
             'shape_sha256': sha(rom[payload_address:cursor]), 'shape_group_records': [len(g) for g in groups],
             'sound_index': record[4], 'sound': data['sound']}
    return data, source, proof


def decode_graphic(rom, graphics, parameter, limit):
    table_bytes = number(rom, graphics)
    require(table_bytes > 0 and table_bytes % 2 == 0 and parameter % 2 == 0 and parameter < table_bytes,
            f'graphic parameter outside directory {graphics:#x}/{parameter}')
    offset = number(rom, graphics+parameter, signed=True)
    require(offset >= table_bytes, f'graphic offset points into directory {graphics:#x}/{parameter}')
    start = graphics+offset
    offsets = [number(rom, graphics+index, signed=True) for index in range(0, table_bytes, 2)]
    end = min([graphics+value for value in offsets if value > offset]+[limit])
    require(start < end <= limit, f'graphic source outside region {graphics:#x}/{parameter}')
    cursor, stamps, pieces = start, [], []

    def word():
        nonlocal cursor
        require(cursor+4 <= end, f'graphic payload crosses next source boundary {graphics:#x}/{parameter}')
        value = number(rom, cursor, 4)
        cursor += 4
        return value

    for _ in range(4):
        value = word()
        stamps.append(value & 0x7fffffff)
        if value & 0x80000000:
            break
    else:
        raise ValueError(f'unterminated transfer list {graphics:#x}/{parameter}')
    for _ in range(80):
        value = word()
        pieces.append({'center_x': ((value >> 24) & 127)-64, 'center_y': ((value >> 16) & 127)-64,
                       'width_tiles': ((value >> 10) & 3)+1, 'height_tiles': ((value >> 8) & 3)+1,
                       'first_slot': value & 255, 'xor_attributes': (value & 0xf000) >> 1})
        if value & 0x80000000:
            break
    else:
        raise ValueError(f'unterminated sprite piece list {graphics:#x}/{parameter}')
    return start, rom[start:cursor], stamps, pieces


class Inputs:
    def __init__(self):
        self.saved = {}

    def raw(self, relative, expected=None):
        path = ROOT/relative
        require(path.resolve().is_relative_to(ROOT), 'input must remain inside this worktree')
        if relative not in self.saved:
            self.saved[relative] = path.read_bytes()
        raw = self.saved[relative]
        if expected is not None:
            require(sha(raw) == expected, f'input digest differs: {relative}')
        return raw

    def document(self, relative, expected=None):
        raw = self.raw(relative, expected)
        return json.loads(gzip.decompress(raw) if str(relative).endswith('.gz') else raw)

    def report(self):
        result = []
        for path, raw in sorted(self.saved.items(), key=lambda value: str(value[0])):
            require((ROOT/path).read_bytes() == raw, f'input changed during audit: {path}')
            result.append({'path': str(path), 'bytes': len(raw), 'sha256': sha(raw)})
        return result


def audit():
    inputs = Inputs()
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    require(sha(rom) == ROM_SHA, 'selected ROM identity differs')
    frozen = inputs.document(TESTDATA/'fighter5-projectile-test-first.json')
    require(frozen['frozen_before_production'] and frozen['own_symbol_red_confirmed'], 'missing original test-first binding')
    for name, expected in frozen['files'].items():
        inputs.raw(Path(name), expected)
    static_path = Path('docs/research/md-yuyuhakusho/static/fighter5-projectile-manifest.json')
    static = inputs.document(static_path)
    require(static['rom_sha256'] == ROM_SHA and static['route'] == 'direct-ghidra-headless', 'static evidence identity differs')
    for name, expected in static['files'].items():
        inputs.raw(Path(name), expected)

    # Source5's initializer; exact move.l immediate and destination fields.
    initializer_entry = 0x12732+5*2
    initializer = initializer_entry+number(rom, initializer_entry, signed=True)
    require(initializer == 0x128ba, 'source5 initializer branch differs')
    parent_values = [BINDINGS[PARENT][0], BINDINGS[PARENT][1], *PARENT]
    for offset, field, value in zip([0, 8, 16, 24], [0x1c, 0x20, 0x50, 0x8c], parent_values):
        address = initializer+offset
        require(rom[address:address+2] == bytes.fromhex('2b7c') and number(rom, address+2, 4) == value and
                number(rom, address+6) == field, f'source5 initializer binding differs at {address:#x}')
    binding_proofs = [{'kind': 'source5_actor', 'initializer': hex(initializer),
                       'initializer_bytes': 32, 'initializer_sha256': sha(rom[initializer:initializer+32])}]
    for kind, address in [('linear', 0x14178), ('steered', 0x1440e), ('follower', 0x146ec)]:
        for delta, value, field in [(0, BINDINGS[CHILD][0], 0x1c), (8, BINDINGS[CHILD][1], 0x20)]:
            require(rom[address+delta:address+delta+2] == bytes.fromhex('2b7c') and
                    number(rom, address+delta+2, 4) == value and number(rom, address+delta+6) == field,
                    f'child binding differs: {kind}')
        binding_proofs.append({'kind': kind, 'address': hex(address), 'source_bytes': 16,
                               'sha256': sha(rom[address:address+16])})
    require(rom[0x14746:0x1474c] == bytes.fromhex('e3493b41003c'), 'follower parameter scaling instruction differs')

    manifest = inputs.document(BASE/'data/world-sprites-manifest.json')
    require(manifest['pass'] and manifest['identity']['rom_sha256'] == ROM_SHA, 'world sprite manifest differs')
    catalog_path, oracle_path = BASE/'data/world-sprites.json.gz', BASE/'data/world-sprites-oracle.json.gz'
    for path, name in [(catalog_path, 'catalog'), (oracle_path, 'oracle')]:
        raw = inputs.raw(path, manifest[name+'_gzip_sha256'])
        require(sha(gzip.decompress(raw)) == manifest[name+'_sha256'], f'{name} uncompressed digest differs')
    catalog, oracle = inputs.document(catalog_path), inputs.document(oracle_path)
    require(catalog['schema'] == oracle['schema'] == 1, 'unsupported world sprite schema')
    require(catalog['rom_sha256'] == oracle['rom_sha256'] == ROM_SHA, 'catalog/oracle ROM identity differs')
    require(catalog['tile_format'] == 'md-4bpp-row-major-high-nibble-first', 'tile format differs')
    tile_bytes = bytes.fromhex(catalog['tile_bytes'])
    require(len(tile_bytes) % 32 == 0 and len(tile_bytes)//32 == manifest['tiles'], 'indexed tile bank size differs')
    frames = {(r['key']['graphics_set'], r['key']['parameter']): r for r in catalog['frames']}
    sources = {(r['graphics_set'], r['parameter']): r for r in oracle['catalog_source_frames']}
    require(len(frames) == len(catalog['frames']) == len(sources) == len(oracle['catalog_source_frames']) == manifest['all_frames'],
            'duplicate or missing source/catalog graphics key')
    character = next(r for r in catalog['characters'] if r['character_id'] == 5)
    require(character['graphics_set'] == BINDINGS[PARENT][0] and
            next(r for r in catalog['characters'] if r['character_id'] == 6)['graphics_set'] == BINDINGS[PARENT][2],
            'source5 catalog character binding differs')

    typed = inputs.document(TESTDATA/'fighter5-projectile-data.json')
    require(typed['source_sha256'] == ROM_SHA, 'typed AN source identity differs')
    animations = {key_of(row): row for row in typed['frames']}
    require(len(animations) == len(typed['frames']) == 33 and
            {key: row['data']['graphic_parameter'] for key, row in animations.items()} == EXPECTED,
            'typed AN key/parameter graph differs; poses are hexadecimal 0x14..0x19')
    animation_proofs = {}
    for key, row in sorted(animations.items()):
        data, source, proof = decode_animation(rom, key)
        require(row['data'] == data and row['source'] == source, f'typed AN differs from exact ROM source {key}')
        animation_proofs[key] = proof
    follower = typed['follower']
    table = [[value, 4] for value in range(1, 9)]+[[0, 0]]
    table_raw = bytes(value for row in table for value in row)
    require(follower == {'address': '0x1479c', 'bytes': table_raw.hex(), 'values': table} and
            rom[0x1479c:0x147ae] == table_raw, 'follower type/delay table differs')
    child_parameters = {0, *(row[0]*2 for row in table if row[1])}
    require(child_parameters == set(range(0, 18, 2)), 'follower parameter graph differs')
    required = {(BINDINGS[key[:2]][0], BINDINGS[key[:2]][1], parameter) for key, parameter in EXPECTED.items()}
    required.update((BINDINGS[CHILD][0], BINDINGS[CHILD][1], parameter) for parameter in child_parameters)

    draw_counts, an_counts, table_counts = collections.Counter(), collections.Counter(), collections.Counter()
    dirty_paused, dirty_clock, fixture_reports = [], [], []
    for fixture in ['oracle', 'boundaries', 'sequences']:
        path = TESTDATA/f'fighter5-projectile-{fixture}.json.gz'
        capture = inputs.document(TESTDATA/f'fighter5-projectile-{fixture}-manifest.json')
        require(capture['pass'] and capture['identity']['rom_sha256'] == ROM_SHA and not capture['errors'] and
                capture['frames_match_original_core'] and capture['binding']['source_tag'] == 5,
                f'{fixture}: CPU evidence identity/status differs')
        raw = inputs.raw(path, capture['gzip_sha256'])
        require(sha(gzip.decompress(raw)) == capture['fixture_sha256'], f'{fixture}: raw fixture digest differs')
        inputs.raw(Path('scripts/yuyuhakusho/capture_fighter5_projectile.py'), capture['capture_sha256'])
        for baseline in capture['baselines']:
            inputs.raw(Path(baseline['input']), baseline['input_sha256'])
            inputs.raw(Path(baseline['reference']), baseline['reference_raw_sha256'])
            inputs.raw(Path(baseline['reference_manifest']), baseline['reference_manifest_sha256'])
            require(baseline['all_ram_video_match'], f'{fixture}: baseline did not match original core')
        document = inputs.document(path)
        observed = {key_of(row): row for row in document['library']}
        require(len(observed) == len(document['library']) and observed == animations, f'{fixture}: captured AN library differs')
        calls, draws, covered, paused_count, clock_count, records = 0, 0, 0, 0, 0, 0
        kinds = collections.Counter()
        for stream, row in rows(document['calls']):
            calls += 1
            kind = row['kind']
            kinds[kind] += 1
            request = row.get('draw_request')
            require(bool(row['draw']) == (request is not None), f'{fixture}: draw/request presence differs')
            before, after = (actor_bytes(row[side], row['object_id']) for side in ['before', 'after'])
            stage = number(before, 6)
            env = row['before']['environment']
            paused = bool(env['pause_flags'] & 128 or env['slow_flag'])
            if kind in ENTRIES:
                require(int(row['entry'], 16) == ENTRIES[kind], f'{fixture}: callback entry differs')
            if kind == 'follower' and not paused and (stage == 0 or before[0x5b] == 1):
                table_index = 0 if stage == 0 else (before[0x59]+1) & 255
                require(0 <= table_index < len(table), f'{fixture}: follower selects outside nine-item table')
                require(stage == 0 or number(before, 0x50, 4) == 0x1479c, 'follower table pointer differs')
                table_counts[(table_index, fixture)] += 1
                if table[table_index][1] == 0:
                    require(request is None and row['cleared_bytes'], 'follower terminator produced a draw')
            if request is not None:
                draws += 1
                require(kind in ENTRIES and row['exit'] == '0x1580' and
                        request == draw_from_actor(after, row['object_id']),
                        f'{fixture}: request differs from exact draw-time actor at {row["sequence"]}')
                graphic = (request['GraphicsSet'], request['TileBank'], request['Parameter'])
                require(graphic[:2] == BINDINGS[CHILD][:2], f'{fixture}: child graphics set/bank differs')
                if request['Parameter'] not in child_parameters:
                    old = draw_from_actor(before, row['object_id'])
                    require(fixture == 'boundaries' and stage == 2 and
                            (old['GraphicsSet'], old['TileBank'], old['Parameter']) == graphic,
                            f'{fixture}: unexpected generated graphic key {graphic}')
                    retained = {'fixture': fixture, 'stream': stream, 'name': row['name'], 'sequence': row['sequence'],
                                'kind': kind, **graphic_json(graphic), 'stage': stage,
                                'pause_flags': env['pause_flags'], 'slow_flag': env['slow_flag'],
                                'ticks_before': before[0x5b], 'ticks_after': after[0x5b],
                                'key_unchanged': True, 'entire_draw_unchanged': request == old,
                                'asset_coverage': False}
                    if paused:
                        require(request == old, 'paused dirty request was not retained exactly')
                        dirty_paused.append(retained)
                        paused_count += 1
                    else:
                        require(kind == 'follower' and (before[0x5b]-1) & 255 != 0 and
                                after[0x5b] == (before[0x5b]-1) & 255,
                                'unexpected unpaused dirty graphic parameter')
                        dirty_clock.append(retained)
                        clock_count += 1
                    continue
                covered += 1
                draw_counts[(graphic, kind, fixture)] += 1
                if kind == 'follower' and not paused and (stage == 0 or before[0x5b] == 1):
                    require(request['Parameter'] == table[table_index][0]*2, 'selected follower parameter differs')
            submitted = kind == 'record' or (request is not None and not paused and kind in ['linear', 'steered', 'linear_stop', 'steered_stop'])
            if submitted:
                key = animation_key(after)
                require(key in animations, f'{fixture}: submitted AN key is absent {key}')
                graphics, bank, _ = BINDINGS[key[:2]]
                require((number(after, 0x1c, 4), number(after, 0x20, 4)) == (graphics, bank) and
                        number(after, 0x3c) == animations[key]['data']['graphic_parameter'],
                        f'{fixture}: AN-to-graphic mapping differs {key}')
                an_counts[(key, kind, fixture)] += 1
                records += 1
        require(calls == capture['calls'] and draws == capture['draws'] and len(document['calls']) == capture['records'] and
                dict(kinds) == capture['kind_counts'], f'{fixture}: saved call/draw counts differ')
        fixture_reports.append({'fixture': fixture, 'records': len(document['calls']), 'callbacks': calls,
                                'kind_counts': dict(sorted(kinds.items())), 'draw_requests': draws,
                                'covered_draw_requests': covered, 'retained_dirty_paused_requests': paused_count,
                                'retained_dirty_unpaused_follower_requests': clock_count, 'AN_submissions': records})
    require({key for key, _, _ in an_counts} == set(animations), 'a typed AN key lacks captured submission consumers')
    require({key[2] for key, _, _ in draw_counts} == child_parameters, 'a child graphic key lacks saved CPU draw consumers')
    require({index for index, _ in table_counts} == set(range(9)), 'a follower table item lacks CPU selection evidence')

    proofs, unique_tiles = [], set()
    for graphic in sorted(required):
        graphics, bank, parameter = graphic
        frame = frames.get((graphics, parameter))
        require(frame is not None, f'missing exact asset {graphic_json(graphic)}')
        require(frame['tile_bank'] == bank and not frame['preloaded_only'] and frame['transfers'] and frame['pieces'],
                f'incomplete exact asset {graphic_json(graphic)}')
        source = sources.get((graphics, parameter))
        require(source is not None, f'missing saved ROM source frame {graphic_json(graphic)}')
        limit = CHILD[0] if graphics == BINDINGS[CHILD][0] else PARENT[0]
        start, payload, stamps, pieces = decode_graphic(rom, graphics, parameter, limit)
        require(source['tiles_base'] == bank and not source['preloaded_only'] and source['source_address'] == start and
                source['payload'] == payload.hex() and source['source_sha256'] == sha(payload),
                f'original oracle source differs {graphic_json(graphic)}')
        require(pieces == frame['pieces'] and stamps == [t['stamp'] for t in frame['transfers']],
                f'decoded transfer/piece source differs {graphic_json(graphic)}')
        slots, transfers, indexed = {}, [], []
        for transfer in frame['transfers']:
            stamp = transfer['stamp']
            count, destination = stamp & 255, (stamp >> 8) & 255
            require(len(transfer['tiles']) == count, f'transfer count differs {graphic_json(graphic)}')
            first = bank+(stamp >> 16)*32
            require(first+count*32 <= len(rom), 'source tiles extend past selected ROM')
            for index, tile_id in enumerate(transfer['tiles']):
                require(0 <= tile_id < len(tile_bytes)//32, f'invalid catalog tile ID {graphic_json(graphic)}')
                tile = tile_bytes[tile_id*32:(tile_id+1)*32]
                require(tile == rom[first+index*32:first+(index+1)*32], f'ROM/catalog tile bytes differ {graphic_json(graphic)}/{index}')
                slots[destination+index] = (tile_id, tile)
                indexed.append(tile_id)
            transfers.append({'stamp': stamp, 'destination_first_slot': destination,
                              'rom_first_tile_address': hex(first), 'tile_count': count,
                              'rom_tiles_sha256': sha(rom[first:first+count*32]), 'catalog_tile_ids': transfer['tiles']})
        used, opaque, piece_proofs = set(), 0, []
        for index, piece in enumerate(pieces):
            piece_slots = list(range(piece['first_slot'], piece['first_slot']+piece['width_tiles']*piece['height_tiles']))
            for slot in piece_slots:
                require(slot in slots, f'piece slot unassigned {graphic_json(graphic)}/{slot}')
                used.add(slot)
                opaque += sum((value >> 4 != 0)+(value & 15 != 0) for value in slots[slot][1])
            piece_proofs.append({'index': index, **piece, 'slots': piece_slots,
                                 'catalog_tile_ids': [slots[slot][0] for slot in piece_slots]})
        require(graphic == (*BINDINGS[CHILD][:2], 0) or opaque > 0, 'nontransparent source became transparent')
        if graphic == (*BINDINGS[CHILD][:2], 0):
            require(opaque == 0, 'initialization parameter0 no longer matches transparent ROM source')
        unique_tiles.update(indexed)
        draw_consumers = [{'kind': kind, 'fixture': fixture, 'requests': count}
                          for (key, kind, fixture), count in sorted(draw_counts.items()) if key == graphic]
        typed_consumers = [{**key_json(key), 'submissions': sum(count for (value, _, _), count in an_counts.items() if value == key)}
                           for key, value in sorted(EXPECTED.items()) if (BINDINGS[key[:2]][0], BINDINGS[key[:2]][1], value) == graphic]
        require(draw_consumers or typed_consumers, f'asset lacks captured consumers {graphic_json(graphic)}')
        proofs.append({**graphic_json(graphic), 'covered': True, 'preloaded_only': False,
                       'source_frame_address': hex(start), 'source_payload_bytes': len(payload),
                       'source_payload_sha256': sha(payload), 'graphic_directory_address': hex(graphics+parameter),
                       'graphic_directory_sha256': sha(rom[graphics+parameter:graphics+parameter+2]),
                       'transfers': transfers, 'pieces': piece_proofs, 'used_relative_slots': sorted(used),
                       'opaque_piece_pixels': opaque, 'draw_consumers': draw_consumers,
                       'typed_AN_consumers': typed_consumers})
    animation_report = []
    for key in sorted(animations):
        graphics, bank, _ = BINDINGS[key[:2]]
        animation_report.append({**key_json(key), **graphic_json((graphics, bank, EXPECTED[key])),
                                 'covered': True, 'source': animation_proofs[key],
                                 'consumers': [{'kind': kind, 'fixture': fixture, 'submissions': count}
                                               for (value, kind, fixture), count in sorted(an_counts.items()) if value == key]})
    return {'scope': 'Source5 child1414e/143e4/follower146ba and saved hit-stop142b4/1458a indexed-asset coverage',
            'pass': True, 'direct_catalog_reuse': True, 'missing_assets': [], 'renderer_fallback_used': False,
            'rom_sha256': ROM_SHA, 'audit_script_sha256': sha(Path(__file__).read_bytes()),
            'sources': inputs.report(), 'catalog_graphic_keys': len(frames), 'catalog_tiles': len(tile_bytes)//32,
            'required_graphic_keys': len(required), 'required_child_graphic_keys': len(child_parameters),
            'required_parent_graphic_keys': len(required)-len(child_parameters),
            'required_unique_indexed_tiles': len(unique_tiles), 'typed_AN_records': len(animations),
            'initializer_binding_evidence': binding_proofs,
            'follower_table': {'source_address': '0x1479c', 'source_bytes': len(table_raw), 'sha256': sha(table_raw),
                               'parameter_scaling': 2, 'scaling_instruction_address': '0x14746',
                               'scaling_instruction_sha256': sha(rom[0x14746:0x1474c]),
                               'items': [{'index': index, 'type': row[0], 'delay': row[1], 'parameter': row[0]*2,
                                          'terminal': row[1] == 0,
                                          'original_CPU_selection_calls': [{'fixture': fixture, 'calls': count}
                                                                           for (value, fixture), count in sorted(table_counts.items()) if value == index]}
                                         for index, row in enumerate(table)]},
            'fixtures': fixture_reports, 'animations': animation_report, 'frames': proofs,
            'retained_dirty_paused_requests': dirty_paused, 'retained_dirty_unpaused_follower_requests': dirty_clock,
            'limitations': ['All retained dirty requests are explicit private boundary inputs outside asset coverage; no transparent fallback is consulted.',
                            'The unpaused retained follower parameters precede a timer transition; only their graphics key stays unchanged while projection runs.',
                            'Parameter 0 is a verified transparent ROM frame used after initialization under pause.',
                            'Typed AN checks cover all 33 saved actual reads, including source5 parent submissions; reachability beyond saved inputs is not established.',
                            'Parent assets have captured AN-submission consumers here; full parent draws and later draws of emitted shared effects belong to world/presentation acceptance.',
                            'This audit reads saved original CPU/Ghidra evidence and ROM bytes; it neither executes original CPU code nor claims REA decompilation.',
                            'Runtime rule-bundle integration, presentation cache/skip/global slots, palette timing, full natural composition and scanline behavior require separate acceptance evidence.',
                            'The raw ROM and emulator core are development-only and are not outputs of this audit.']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    output = args.out.resolve()
    require(output.is_relative_to(ROOT) and output.parent.is_dir(), 'output parent must already exist inside this worktree')
    report = audit()
    output.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({name: report[name] for name in ['pass', 'direct_catalog_reuse', 'required_graphic_keys',
                                                    'required_unique_indexed_tiles', 'typed_AN_records', 'missing_assets']}
                     | {'retained_dirty_paused_requests': len(report['retained_dirty_paused_requests']),
                        'retained_dirty_unpaused_follower_requests': len(report['retained_dirty_unpaused_follower_requests']),
                        'report_sha256': sha(output.read_bytes())}, indent=2))


if __name__ == '__main__':
    main()
