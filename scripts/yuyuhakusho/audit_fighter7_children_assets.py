#!/usr/bin/env python3
"""Read-only source7 children asset audit; report metadata, never ROM/tile bytes.

Failure scenarios were written first in verification/FIGHTER7_CHILDREN_ASSET_AUDIT.md.
Only the standard library is used. No CPU, emulator, renderer or exporter runs.
"""
import argparse
import collections
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BASE = Path('games/yuyuhakusho')
TESTDATA = BASE/'engine/testdata'
STATIC = Path('docs/research/md-yuyuhakusho/static')
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
TYPED_SHA = '72d54890df9bbb66b7a70e0f026e343dc749d971b9450979d103324e65d16aec'
CHILD = (0xd92ca, 0xd9a3a)
PARENT = (0xcf99a, 0xd069a)
# AN/shape -> graphics, bank, bounded shape end.
BINDINGS = {CHILD: (0xd8f1a, 0x240000, 0xd9e4a),
            PARENT: (0xce98a, 0x1c0000, 0xd104a)}
FX_GRAPHICS = 0x27f618
ENTRIES = {'first': 0x136e6, 'second': 0x1391e, 'pan': 0x15094,
           'effect': 0x149ac, 'clock': 0x19c96, 'record': 0x1eb40,
           'creator59': 0x1be64, 'creator60': 0x1bfce, 'creator61': 0x1c032}
FIXTURE_SHA = {
    'oracle': 'f236f3885b30eefb62465e8c11a08be79a7ca1d511dadb8e11affb49bfcf1cf1',
    'boundaries': 'f2996cdb66613a0ba8a3370c85a1ad84baa635d11ca16c3a048f610f34cd68f1',
    'sequences': '76df601b11ba5118029d19e48756fafc730021e6760eb0228e30550b19749244'}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def require(condition, message):
    if not condition:
        raise ValueError(message)


def number(raw, offset, size=2, signed=False):
    require(0 <= offset and offset+size <= len(raw), f'out of bounds at {offset:#x}/{size}')
    return int.from_bytes(raw[offset:offset+size], 'big', signed=signed)


def key_of(row):
    return tuple(row['key'][field] for field in ['animation_set', 'shape_set', 'pose', 'index'])


def key_json(key):
    return {'animation_set': hex(key[0]), 'shape_set': hex(key[1]),
            'pose': key[2], 'pose_hex': hex(key[2]), 'index': key[3]}


def graphic_json(key):
    return {'graphics_set': hex(key[0]), 'tile_bank': hex(key[1]), 'parameter': key[2]}


def actor_bytes(snapshot, object_id):
    base = snapshot['object_id']
    if base <= object_id and object_id+256 <= base+1024:
        start = object_id-base
        raw = bytes.fromhex(snapshot['family'])[start:start+256]
    else:
        require(0x9400 <= object_id and object_id+128 <= 0x9c00, f'unknown actor {object_id:#x}')
        start = object_id-0x9400
        raw = bytes.fromhex(snapshot['pool']['bytes'])[start:start+128]
    require(len(raw) >= 128, 'incomplete saved actor')
    return raw


def animation_key(raw):
    return number(raw, 0x50, 4), number(raw, 0x8c, 4), raw[0x5f], raw[0x59]


def draw_from_actor(raw, object_id):
    return {'ObjectID': object_id, 'GraphicsSet': number(raw, 0x1c, 4),
            'TileBank': number(raw, 0x20, 4), 'Parameter': number(raw, 0x3c),
            'ScreenX': number(raw, 8), 'ScreenY': number(raw, 12),
            'OffsetX': number(raw, 0x28, signed=True), 'OffsetY': number(raw, 0x2a, signed=True),
            'Attributes': number(raw, 0x24), 'Depth': number(raw, 0x3e),
            'Layer': raw[0x26], 'SkipTiles': raw[0x27],
            'Cache': [number(raw, 0x2c+i*4, 4) for i in range(4)]}


def rows(groups):
    for group in groups:
        if 'calls' in group:
            for row in group['calls']:
                yield group['name'], row
        else:
            yield None, group


def opaque_pixels(raw):
    return sum((value >> 4 != 0)+(value & 15 != 0) for value in raw)


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


def decode_animation(rom, key):
    animation, shape, pose, index = key
    shape_limit = BINDINGS[key[:2]][2]
    directory_bytes = number(rom, animation)
    require(directory_bytes > 0 and directory_bytes % 2 == 0 and pose*2 < directory_bytes,
            f'pose outside AN directory {key}')
    pointer = animation+pose*2
    address = animation+number(rom, pointer, signed=True)+index*8
    require(animation+directory_bytes <= address and address+8 <= shape, f'AN outside bounded region {key}')
    record = rom[address:address+8]
    table_bytes = number(rom, shape+2, signed=True)
    require(table_bytes > 0 and table_bytes % 4 == 0 and record[1]*4 < table_bytes,
            f'shape directory outside declared region {key}')
    entry = shape+record[1]*4
    directory = rom[entry:entry+4]
    payload = shape+number(directory, 2, signed=True)
    starts = [shape+number(rom, shape+offset+2, signed=True) for offset in range(0, table_bytes, 4)]
    limit = min([start for start in starts if start > payload]+[shape_limit])
    require(shape+table_bytes <= payload <= limit, f'shape payload outside region {key}')
    cursor = payload

    def shape_record():
        nonlocal cursor
        require(cursor+4 <= limit, f'shape crosses next source boundary {key}')
        raw = rom[cursor:cursor+4]
        cursor += 4
        return raw

    metadata, groups = [0, 0, 0, 0], [[], [], [], []]
    if directory[0] & 0x40:
        metadata = list(shape_record())
    for group, flag in enumerate([0x40, 0x20, 0x10, 0x08]):
        if not directory[0] & flag:
            continue
        for _ in range(64):
            raw = shape_record()
            groups[group].append({'width': raw[0], 'offset_x': raw[1]-0x80,
                                  'height': raw[2], 'offset_y': raw[3]-0x80})
            if group == 3 or raw[0] & 0x80:
                break
        else:
            raise ValueError(f'unterminated shape group {key}/{group}')
    data = {'duration': record[0], 'flags': record[2], 'enabled_stages': record[3],
            'sound': number(rom, 0x218d8+record[4], 1), 'offset_x': record[6]-0x80,
            'offset_y': record[7]-0xa0, 'graphic_parameter': directory[1]*2,
            'shape_flags': directory[0], 'metadata': metadata, 'groups': groups}
    source = {'animation_address': hex(address), 'animation_bytes': record.hex(),
              'directory_bytes': directory.hex(), 'shape_address': hex(payload),
              'shape_bytes': rom[payload:cursor].hex(), 'sound_index': record[4]}
    proof = {'pose_pointer_address': hex(pointer), 'pose_pointer_sha256': sha(rom[pointer:pointer+2]),
             'animation_address': hex(address), 'animation_sha256': sha(record),
             'shape_directory_address': hex(entry), 'shape_directory_sha256': sha(directory),
             'shape_address': hex(payload), 'shape_bytes': cursor-payload,
             'shape_sha256': sha(rom[payload:cursor]), 'shape_group_records': [len(g) for g in groups],
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
    require(start < end <= limit, f'graphic source outside bounded region {graphics:#x}/{parameter}')
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


def decode_resource(rom, source):
    """Bounded offline decode of original988/9ba format, never a runtime asset."""
    cursor = source+2
    end = cursor+number(rom, source)
    require(end <= len(rom), 'compressed preload source outside ROM')
    result, tokens = bytearray(), [0]*8
    while cursor < end:
        control = rom[cursor]
        cursor += 1
        tokens[control >> 5] += 1
        if control & 128:
            require(cursor < end, 'truncated preload backreference')
            count, distance = ((control >> 2) & 31)+2, ((control << 8 | rom[cursor]) & 1023)+1
            cursor += 1
            require(distance <= len(result), 'preload backreference before output')
            for _ in range(count):
                result.append(result[-distance])
        elif control & 32:
            count = (control & 31)+2
            require(cursor < end, 'truncated preload repeat')
            value = rom[cursor]
            cursor += 1
            if control & 64:
                require(cursor+count <= end, 'truncated preload interleaved repeat')
                for _ in range(count):
                    result.extend((value, rom[cursor]))
                    cursor += 1
            else:
                result.extend(bytes([value])*count)
        elif control & 64:
            count = (control & 31)+2
            require(cursor+2 <= end, 'truncated preload word repeat')
            result.extend(rom[cursor:cursor+2]*count)
            cursor += 2
        else:
            count = (control & 31)+1
            require(cursor+count <= end, 'truncated preload literal')
            result.extend(rom[cursor:cursor+count])
            cursor += count
        require(len(result) <= 65536, 'preload decompression exceeds declared video bank')
    require(cursor == end, 'preload decompression boundary differs')
    return bytes(result), tokens


def audit():
    inputs = Inputs()
    script_raw = inputs.raw(Path(__file__).resolve().relative_to(ROOT))
    rom_path = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    require(sha(rom) == ROM_SHA, 'selected ROM identity differs')
    static = inputs.document(STATIC/'fighter7-children-analysis-manifest.json')
    require(static['rom_sha256'] == ROM_SHA and static['route'] == 'direct-ghidra-headless; REA raw68000 unsupported',
            'static source identity/route differs')
    for name, listing in static['listings'].items():
        inputs.raw(STATIC/name, listing['sha256'])
    inputs.raw(Path(static['probe']), static['probe_sha256'])
    for name in ['arena-assets-loader.txt', 'effect-motion-core.txt']:
        inputs.raw(STATIC/name)
    inputs.raw(Path('scripts/yuyuhakusho/extract_hud_assets.py'))
    binding_proofs = []
    parent_entry = 0x12732+7*2
    parent_initializer = parent_entry+number(rom, parent_entry, signed=True)
    for name, start, key in [('source7_parent', parent_initializer, PARENT),
                             ('first_child', 0x13710, CHILD), ('second_child', 0x13948, CHILD)]:
        graphics, bank, _ = BINDINGS[key]
        for delta, field, value in zip([0, 8, 16, 24], [0x1c, 0x20, 0x50, 0x8c], [graphics, bank, *key]):
            address = start+delta
            require(rom[address:address+2] == bytes.fromhex('2b7c') and number(rom, address+2, 4) == value and
                    number(rom, address+6) == field, f'initializer binding differs at {address:#x}')
        binding_proofs.append({'kind': name, 'address': hex(start), 'bytes': 32,
                               'sha256': sha(rom[start:start+32]), **graphic_json((graphics, bank, 0)),
                               'animation_set': hex(key[0]), 'shape_set': hex(key[1])})

    manifest = inputs.document(BASE/'data/world-sprites-manifest.json')
    require(manifest['pass'] and manifest['identity']['rom_sha256'] == ROM_SHA, 'world sprite manifest differs')
    catalog_path, oracle_path = BASE/'data/world-sprites.json.gz', BASE/'data/world-sprites-oracle.json.gz'
    for path, name in [(catalog_path, 'catalog'), (oracle_path, 'oracle')]:
        raw = inputs.raw(path, manifest[name+'_gzip_sha256'])
        require(sha(gzip.decompress(raw)) == manifest[name+'_sha256'], f'{name} uncompressed digest differs')
    inputs.raw(Path('scripts/yuyuhakusho/extract_world_sprites.py'), manifest['script_sha256'])
    catalog, oracle = inputs.document(catalog_path), inputs.document(oracle_path)
    require(catalog['schema'] == oracle['schema'] == 1 and catalog['rom_sha256'] == oracle['rom_sha256'] == ROM_SHA,
            'catalog/oracle schema or source differs')
    require(catalog['tile_format'] == 'md-4bpp-row-major-high-nibble-first', 'tile format differs')
    tile_bytes = bytes.fromhex(catalog['tile_bytes'])
    require(len(tile_bytes) % 32 == 0 and len(tile_bytes)//32 == manifest['tiles'], 'indexed tile bank size differs')
    frames = {(r['key']['graphics_set'], r['key']['parameter']): r for r in catalog['frames']}
    default_slot_numbers = {r['slot'] for r in catalog['default_static_slots']}
    sources = {(r['graphics_set'], r['parameter']): r for r in oracle['catalog_source_frames']}
    require(len(frames) == len(catalog['frames']) == len(sources) == len(oracle['catalog_source_frames']) == manifest['all_frames'],
            'duplicate or missing source/catalog graphics key')
    require(next(r for r in catalog['characters'] if r['character_id'] == 7)['graphics_set'] == BINDINGS[PARENT][0],
            'source7 parent catalog binding differs')

    typed_manifest = inputs.document(BASE/'data/fighter7-children-rules-manifest.json')
    typed = inputs.document(BASE/'data/fighter7-children-rules.json', TYPED_SHA)
    require(typed_manifest['pass'] and typed_manifest['source_rom_sha256'] == ROM_SHA and
            typed_manifest['output_sha256'] == TYPED_SHA and typed_manifest['frames'] == 94 and
            typed_manifest['new_frames'] == 94 and typed_manifest['matching_existing_frames'] == 0,
            'typed AN export identity/count differs')
    inputs.raw(Path('scripts/yuyuhakusho/export_fighter7_children_data.py'), typed_manifest['export_sha256'])
    inputs.raw(Path(typed_manifest['capture_frozen_source']), typed_manifest['capture_sha256'])
    for name, expected in typed_manifest['static_sha256'].items():
        inputs.raw(STATIC/name, expected)
    new_animations = {key_of(r): r for r in typed['frames']}
    require(len(new_animations) == len(typed['frames']) == 94 and
            all(key[:2] == CHILD and 0x3c <= key[2] <= 0x45 for key in new_animations), 'new child AN key range differs')
    existing_catalog = inputs.document(BASE/'data/animation-catalog.json.gz')
    require(existing_catalog['schema'] == 1 and existing_catalog['rom_sha256'] == ROM_SHA, 'existing AN catalog identity differs')
    existing = {key_of(r): r for r in existing_catalog['frames']}
    require(len(existing) == len(existing_catalog['frames']) and not set(existing).intersection(new_animations),
            'duplicate existing/new child AN key')

    effect_doc = inputs.document(BASE/'data/effect-sequences.json')
    effect_source = inputs.document(TESTDATA/'effect-motion-data-source.json')
    require(effect_doc['schema'] == 1 and effect_doc['rom_sha256'] == effect_source['rom_sha256'] == ROM_SHA,
            'generic FX source identity differs')
    definition = effect_doc['library']['types'][0x24]
    effect_binding = effect_source['source']['bindings'][0x24]
    fx_entry = 0x14a00+0x24*2
    fx_initializer = fx_entry+number(rom, fx_entry, signed=True)
    require(effect_binding['entry'] == hex(fx_entry) and effect_binding['target'] == hex(fx_initializer) and
            rom[fx_initializer:fx_initializer+30].hex() == effect_binding['bytes'], 'type24 original initializer differs')
    require(rom[fx_initializer:fx_initializer+2] == bytes.fromhex('2b7c') and
            rom[fx_initializer+6:fx_initializer+10] == bytes.fromhex('00502b7c') and
            rom[fx_initializer+14:fx_initializer+18] == bytes.fromhex('001c006d') and
            rom[fx_initializer+20:fx_initializer+30] == bytes.fromhex('00241b7c000100274e75'),
            'type24 initializer opcodes differ')
    sequence = number(rom, fx_initializer+2, 4)
    require(definition == {'sequence': sequence, 'graphic_set': number(rom, fx_initializer+10, 4),
                           'tile_base': 0, 'set_tile_base': False, 'attribute_or': number(rom, fx_initializer+18),
                           'clear_velocity': False, 'dynamic_tiles': False, 'set_skip_tiles': True,
                           'skip_tiles': 1, 'sound': 0} and definition['graphic_set'] == FX_GRAPHICS,
            'type24 semantic initializer differs')
    effect_sequence = effect_doc['library']['sequences'][str(sequence)]
    require(effect_sequence and effect_sequence[-1]['duration'] == 0 and
            all(frame['duration'] > 0 for frame in effect_sequence[:-1]), 'type24 sequence terminator differs')
    seq_raw = bytes(value for frame in effect_sequence for value in [frame['graphic'], frame['duration']])
    require(rom[sequence:sequence+len(seq_raw)] == seq_raw, 'type24 graphic/duration table differs from ROM')
    fx_parameters = {frame['graphic']*2 for frame in effect_sequence[:-1]}
    helper_types = [0x20676+i*0x4e for i in range(8)]
    require(all(rom[address:address+6] == bytes.fromhex('117c00240058') for address in helper_types),
            'healing helper allocation type differs')

    animations, an_proofs = {}, {}
    draw_counts, an_counts, effect_counts = collections.Counter(), collections.Counter(), collections.Counter()
    fixture_reports, dirty, dirty_preloads = [], [], []
    valid_child_parameters = {r['data']['graphic_parameter'] for r in new_animations.values()}
    require(0 in valid_child_parameters, 'child transparent ROM parameter is missing')
    for fixture in ['oracle', 'boundaries', 'sequences']:
        path = TESTDATA/f'fighter7-children-{fixture}.json.gz'
        capture = inputs.document(TESTDATA/f'fighter7-children-{fixture}-manifest.json')
        require(capture['pass'] and capture['identity']['rom_sha256'] == ROM_SHA and not capture['errors'] and
                capture['frames_match_original_core'] and capture['incomplete_calls'] == 0,
                f'{fixture}: CPU evidence identity/status differs')
        raw = inputs.raw(path, FIXTURE_SHA[fixture])
        require(capture['gzip_sha256'] == FIXTURE_SHA[fixture] and sha(gzip.decompress(raw)) == capture['fixture_sha256'],
                f'{fixture}: saved fixture digest differs')
        capture_name = 'capture_fighter7_children.py' if fixture == 'oracle' else 'capture_fighter7_children_private_frozen.py'
        inputs.raw(Path('scripts/yuyuhakusho')/capture_name, capture['capture_sha256'])
        for baseline in capture['baselines']:
            inputs.raw(Path(baseline['input']), baseline['input_sha256'])
            inputs.raw(Path(baseline['reference']), baseline['reference_raw_sha256'])
            inputs.raw(Path(baseline['reference_manifest']), baseline['reference_manifest_sha256'])
            require(baseline['all_ram_video_match'], f'{fixture}: baseline did not match original core')
        document = inputs.document(path)
        observed = {key_of(r): r for r in document['library']}
        require(len(observed) == len(document['library']) == capture['library_frames'], f'{fixture}: duplicate AN library key')
        for key, row in observed.items():
            require(key[:2] in BINDINGS, f'{fixture}: unknown AN binding {key}')
            expected = new_animations.get(key) if key[:2] == CHILD else existing.get(key)
            require(expected is not None and row['data'] == expected['data'], f'{fixture}: typed AN differs {key}')
            data, source, proof = decode_animation(rom, key)
            require(row['data'] == data and row['source'] == source, f'{fixture}: AN differs from exact ROM source {key}')
            require(key not in animations or animations[key] == row, f'{fixture}: contradictory captured AN {key}')
            animations[key], an_proofs[key] = row, proof
        calls = draws = covered = retained = preload_uncovered = records = 0
        kinds = collections.Counter()
        for stream, row in rows(document['calls']):
            calls += 1
            kind = row['kind']
            kinds[kind] += 1
            require(kind in ENTRIES and int(row['entry'], 16) == ENTRIES[kind], f'{fixture}: callback entry differs')
            request = row.get('draw_request')
            require(bool(row['draw']) == (request is not None), f'{fixture}: draw/request presence differs')
            require(kind in ['first', 'second', 'effect'] or request is None, f'{fixture}: nonvisual callback drew')
            before, after = [actor_bytes(row[side], row['object_id']) for side in ['before', 'after']]
            env = row['before']['environment']
            paused = bool(env['pause_flags'] & 128 or env['slow_flag'])
            if request is not None:
                draws += 1
                require(row['exit'] == '0x1580' and request == draw_from_actor(after, row['object_id']),
                        f'{fixture}: request differs from its draw-time actor at {row["sequence"]}')
                graphic = tuple(request[field] for field in ['GraphicsSet', 'TileBank', 'Parameter'])
                if kind in ['first', 'second']:
                    require(graphic[:2] == BINDINGS[CHILD][:2], f'{fixture}: child graphics set/bank differs')
                    if graphic[2] not in valid_child_parameters:
                        require(fixture == 'boundaries' and paused and number(before, 6) == 2 and
                                request == draw_from_actor(before, row['object_id']), f'{fixture}: unexplained dirty request')
                        retained += 1
                        dirty.append({'fixture': fixture, 'stream': stream, 'name': row['name'],
                                      'sequence': row['sequence'], 'kind': kind, **graphic_json(graphic),
                                      'stage': number(before, 6), 'pause_flags': env['pause_flags'],
                                      'slow_flag': env['slow_flag'], 'entire_draw_unchanged': True, 'asset_coverage': False})
                        continue
                else:
                    require(graphic[:2] == (FX_GRAPHICS, 0) and graphic[2] in fx_parameters and after[0x58] == 0x24 and
                            request['SkipTiles'] == 1 and number(after, 0x50, 4) == sequence,
                            f'{fixture}: type24 draw binding differs')
                    index = after[0x59]
                    require(index < len(effect_sequence)-1 and graphic[2] == effect_sequence[index]['graphic']*2,
                            f'{fixture}: type24 sequence consumer differs')
                    effect_counts[(index, fixture)] += 1
                    base = request['Attributes'] & 0x7ff
                    require(base == 506 or (fixture == 'sequences' and stream == '59-1-False' and
                                           env['palette_attributes'] == request['Attributes'] == 0xffff and base == 2047),
                            f'{fixture}: unknown preload slot context')
                    if base != 506:
                        frame = frames[(FX_GRAPHICS, graphic[2])]
                        required_slots = sorted({(base+p['first_slot']+i) & 2047 for p in frame['pieces']
                                                 for i in range(p['width_tiles']*p['height_tiles'])})
                        dirty_preloads.append({'fixture': fixture, 'stream': stream, 'name': row['name'],
                                               'sequence': row['sequence'], 'kind': kind, **graphic_json(graphic),
                                               'palette_attributes': env['palette_attributes'],
                                               'attributes': request['Attributes'], 'skip_tiles': request['SkipTiles'],
                                               'required_absolute_slots': required_slots, 'frame_covered': True,
                                               'missing_default_slots': [slot for slot in required_slots if slot not in default_slot_numbers],
                                               'default_preload_context_covered': False, 'asset_coverage': False,
                                               'reason': 'Private dirty palette OR changes the base to2047; original prior slots are not captured here.'})
                        preload_uncovered += 1
                draw_counts[(graphic, kind, fixture)] += 1
                covered += 1
            submitted = kind == 'record' or (request is not None and kind in ['first', 'second'] and not paused)
            if submitted:
                key = animation_key(after)
                require(key in observed, f'{fixture}: submitted AN key absent from capture {key}')
                graphics, bank, _ = BINDINGS[key[:2]]
                require((number(after, 0x1c, 4), number(after, 0x20, 4)) == (graphics, bank) and
                        number(after, 0x3c) == observed[key]['data']['graphic_parameter'],
                        f'{fixture}: AN-to-graphic mapping differs {key}')
                an_counts[(key, kind, fixture)] += 1
                records += 1
        require(calls == capture['calls'] and draws == capture['draws'] and len(document['calls']) == capture['records'] and
                dict(kinds) == capture['kind_counts'], f'{fixture}: saved call/draw counts differ')
        fixture_reports.append({'fixture': fixture, 'records': len(document['calls']), 'callbacks': calls,
                                'kind_counts': dict(sorted(kinds.items())), 'draw_requests': draws,
                                'exact_frame_covered_requests': covered,
                                'asset_source_covered_requests': covered-preload_uncovered,
                                'retained_dirty_paused_requests': retained,
                                'dirty_palette_preload_context_requests': preload_uncovered,
                                'AN_submissions': records})
    require({key for key, _, _ in an_counts} == set(animations) and
            set(new_animations) == {key for key in animations if key[:2] == CHILD}, 'a typed AN key lacks a captured submission')
    require({key[2] for key, _, _ in draw_counts if key[:2] == BINDINGS[CHILD][:2]} == valid_child_parameters,
            'a child graphic key lacks saved CPU draw consumers')
    require({index for index, _ in effect_counts} == set(range(len(effect_sequence)-1)), 'type24 sequence item lacks CPU draws')

    # The normal type24 first slot is506. Its original resource list loads the
    # compressed common atlas at0x1800; TileBank0 is deliberately never read as art.
    require(number(rom, 0x21700, 4) == 0x21720 and rom[0x21728:0x21730] == bytes.fromhex('00060027dcb41800'),
            'scene0 common preload resource binding differs')
    preloaded, token_counts = decode_resource(rom, 0x27dcb4)
    require(len(preloaded) == 10176, 'common atlas decoded length differs')
    static_slots = {r['slot']: r['tile'] for r in catalog['default_static_slots']}
    require(len(static_slots) == len(catalog['default_static_slots']), 'duplicate default static slot')
    static_proofs = []
    checkpoint_sources = [r for r in oracle['natural'] if r['frame'] == 2400]
    require(checkpoint_sources, 'original preload checkpoint is absent')
    for slot, expected_pixels in zip(range(506, 510), [1, 3, 7, 15]):
        require(slot in static_slots, f'missing default healing FX slot {slot}')
        tile_id = static_slots[slot]
        require(0 <= tile_id < len(tile_bytes)//32, f'preload tile ID outside catalog {slot}')
        tile = tile_bytes[tile_id*32:(tile_id+1)*32]
        offset = slot*32-0x1800
        require(tile == preloaded[offset:offset+32] and opaque_pixels(tile) == expected_pixels,
                f'preload ROM/catalog tile bytes or pixels differ {slot}')
        checkpoints = []
        for row in checkpoint_sources:
            for field in ['before_vram', 'uploaded_vram']:
                saved = bytes.fromhex(row[field])
                require(saved[slot*32:(slot+1)*32] == tile, f'original checkpoint preload differs {slot}')
            checkpoints.append({'scenario': row['scenario'], 'frame': row['frame'],
                                'before_vram_tile_sha256': sha(tile), 'uploaded_vram_tile_sha256': sha(tile)})
        static_proofs.append({'slot': slot, 'catalog_tile_id': tile_id, 'tile_sha256': sha(tile),
                              'opaque_pixels': expected_pixels, 'decoded_resource_offset': offset,
                              'original_checkpoint_matches': checkpoints})

    required = {(BINDINGS[key[:2]][0], BINDINGS[key[:2]][1], row['data']['graphic_parameter'])
                for key, row in animations.items()}
    required.update((FX_GRAPHICS, 0, parameter) for parameter in fx_parameters)
    proofs, unique_tiles = [], {row['catalog_tile_id'] for row in static_proofs}
    missing = []
    for graphic in sorted(required):
        graphics, bank, parameter = graphic
        frame, source = frames.get((graphics, parameter)), sources.get((graphics, parameter))
        if frame is None or frame['tile_bank'] != bank or source is None:
            missing.append({**graphic_json(graphic), 'reason': 'Exact catalog/oracle frame or bank is absent'})
            continue
        preloaded_only = graphics == FX_GRAPHICS
        require(frame['preloaded_only'] == source['preloaded_only'] == preloaded_only and frame['transfers'] and frame['pieces'],
                f'incomplete exact asset {graphic_json(graphic)}')
        limit = 0x300000 if preloaded_only else next(key[0] for key, value in BINDINGS.items() if value[:2] == (graphics, bank))
        start, payload, stamps, pieces = decode_graphic(rom, graphics, parameter, limit)
        require(source['tiles_base'] == bank and source['source_address'] == start and source['payload'] == payload.hex() and
                source['source_sha256'] == sha(payload), f'original oracle source differs {graphic_json(graphic)}')
        require(pieces == frame['pieces'] and stamps == [t['stamp'] for t in frame['transfers']],
                f'original transfer/piece source differs {graphic_json(graphic)}')
        slots, transfers, indexed = {}, [], []
        for transfer in frame['transfers']:
            stamp = transfer['stamp']
            count, destination = stamp & 255, (stamp >> 8) & 255
            if preloaded_only:
                require(transfer['tiles'] == [], 'preloaded descriptor unexpectedly contains guessed ROM tiles')
                transfers.append({'stamp': stamp, 'descriptor_only': True, 'upload_skipped': True,
                                  'source_bank_is_not_an_art_source': True, 'descriptor_tile_count': count})
                continue
            require(len(transfer['tiles']) == count, f'tile transfer length differs {graphic_json(graphic)}')
            first = bank+(stamp >> 16)*32
            require(first+count*32 <= len(rom), 'source tiles extend past selected ROM')
            for index, tile_id in enumerate(transfer['tiles']):
                require(0 <= tile_id < len(tile_bytes)//32, f'invalid catalog tile ID {graphic_json(graphic)}')
                tile = tile_bytes[tile_id*32:(tile_id+1)*32]
                require(tile == rom[first+index*32:first+(index+1)*32], f'ROM/catalog tile bytes differ {graphic_json(graphic)}/{index}')
                slots[destination+index] = (tile_id, tile)
                indexed.append(tile_id)
            transfers.append({'stamp': stamp, 'destination_first_relative_slot': destination,
                              'rom_first_tile_address': hex(first), 'tile_count': count,
                              'rom_tiles_sha256': sha(rom[first:first+count*32]), 'catalog_tile_ids': transfer['tiles']})
        if preloaded_only:
            slots = {slot-506: (row['catalog_tile_id'], tile_bytes[row['catalog_tile_id']*32:(row['catalog_tile_id']+1)*32])
                     for slot, row in zip(range(506, 510), static_proofs)}
        used, opaque, piece_proofs = set(), 0, []
        for index, piece in enumerate(pieces):
            piece_slots = list(range(piece['first_slot'], piece['first_slot']+piece['width_tiles']*piece['height_tiles']))
            for slot in piece_slots:
                require(slot in slots, f'piece tile slot unassigned {graphic_json(graphic)}/{slot}')
                used.add(slot)
                opaque += opaque_pixels(slots[slot][1])
            piece_proofs.append({'index': index, **piece, 'relative_slots': piece_slots,
                                 'absolute_slots_for_normal_preload': [506+slot for slot in piece_slots] if preloaded_only else None,
                                 'catalog_tile_ids': [slots[slot][0] for slot in piece_slots],
                                 'opaque_piece_pixels': sum(opaque_pixels(slots[slot][1]) for slot in piece_slots)})
        transparent = graphic == (*BINDINGS[CHILD][:2], 0)
        require(opaque == 0 if transparent else opaque > 0, f'original transparent/nontransparent pixels differ {graphic_json(graphic)}')
        unique_tiles.update(indexed)
        draw_consumers = [{'kind': kind, 'fixture': fixture, 'requests': count}
                          for (key, kind, fixture), count in sorted(draw_counts.items()) if key == graphic]
        typed_consumers = [{**key_json(key), 'submissions': sum(count for (value, _, _), count in an_counts.items() if value == key)}
                           for key, row in sorted(animations.items())
                           if (BINDINGS[key[:2]][0], BINDINGS[key[:2]][1], row['data']['graphic_parameter']) == graphic]
        require(draw_consumers or typed_consumers, f'asset lacks captured consumers {graphic_json(graphic)}')
        proofs.append({**graphic_json(graphic), 'covered': True, 'preloaded_only': preloaded_only,
                       'normal_preload_base_slot': 506 if preloaded_only else None,
                       'source_frame_address': hex(start), 'source_payload_bytes': len(payload),
                       'source_payload_sha256': sha(payload), 'graphic_directory_address': hex(graphics+parameter),
                       'graphic_directory_sha256': sha(rom[graphics+parameter:graphics+parameter+2]),
                       'transfers': transfers, 'pieces': piece_proofs, 'used_relative_slots': sorted(used),
                       'opaque_piece_pixels': opaque, 'draw_consumers': draw_consumers, 'typed_AN_consumers': typed_consumers})
    require(not missing, f'missing exact normal assets: {missing}')
    require(rom_path.read_bytes() == rom, 'selected ROM changed during audit')
    totals = {field: sum(report[field] for report in fixture_reports)
              for field in ['callbacks', 'draw_requests', 'exact_frame_covered_requests', 'asset_source_covered_requests',
                            'retained_dirty_paused_requests', 'dirty_palette_preload_context_requests', 'AN_submissions']}
    return {'scope': 'Source7 child136e6/1391e, derived generic149ac type24 and captured parent AN indexed-asset audit',
            'pass': True, 'direct_catalog_reuse': True, 'missing_assets': missing,
            'rom_sha256': ROM_SHA, 'rom_bytes': len(rom), 'audit_script_sha256': sha(script_raw),
            'sources': inputs.report(), 'catalog_graphic_keys': len(frames), 'catalog_tiles': len(tile_bytes)//32,
            'existing_AN_catalog_records': len(existing), 'new_child_AN_records': len(new_animations),
            'captured_AN_records': len(animations), 'required_graphic_keys': len(required),
            'required_draw_graphic_keys': len({key for key, _, _ in draw_counts}),
            'required_unique_indexed_tiles': len(unique_tiles), 'bindings': binding_proofs,
            'animations': [{**key_json(key), **graphic_json((BINDINGS[key[:2]][0], BINDINGS[key[:2]][1], row['data']['graphic_parameter'])),
                            'new_child_record': key in new_animations, 'source': an_proofs[key],
                            'consumers': [{'kind': kind, 'fixture': fixture, 'submissions': count}
                                          for (value, kind, fixture), count in sorted(an_counts.items()) if value == key]}
                           for key, row in sorted(animations.items())],
            'type24': {'type': 0x24, 'initializer_entry': hex(fx_entry), 'initializer_address': hex(fx_initializer),
                       'initializer_sha256': sha(rom[fx_initializer:fx_initializer+30]),
                       'sequence_address': hex(sequence), 'sequence_sha256': sha(seq_raw), 'sequence': effect_sequence,
                       'sequence_consumers': [{'index': index, 'fixture': fixture, 'draws': count}
                                              for (index, fixture), count in sorted(effect_counts.items())],
                       'healing_helper_type_immediates': [hex(value) for value in helper_types]},
            'normal_preload': {'resource_list_address': '0x21720', 'resource_entry_address': '0x21728',
                               'resource_entry_sha256': sha(rom[0x21728:0x21730]), 'resource_kind': 6,
                               'compressed_source_address': '0x27dcb4', 'compressed_source_bytes': number(rom, 0x27dcb4)+2,
                               'compressed_source_sha256': sha(rom[0x27dcb4:0x27dcb4+2+number(rom, 0x27dcb4)]),
                               'destination_vram_byte_address': '0x1800', 'decoded_bytes': len(preloaded),
                               'decoded_sha256': sha(preloaded), 'decompression_token_counts': token_counts,
                               'slots': static_proofs},
            'fixtures': fixture_reports, 'request_totals': totals, 'frames': proofs,
            'retained_dirty_paused_requests': dirty, 'dirty_palette_preload_context_requests': dirty_preloads,
            'limitations': [
                f'The {len(dirty)} retained dirty stage-two paused/slow requests are preserved CPU inputs, explicitly outside asset coverage.',
                f'The {len(dirty_preloads)} dirty-palette FX draws have exact catalog frames but require prior slots2047,0,1,2; default preload pixel coverage is not claimed.',
                'Normal type24 uses PreloadedOnly plus SkipTiles1. Slots506..509 match both the decoded original compressed scene0 resource and captured original VRAM.',
                'Transparent child Parameter0 is verified against its exact ROM tile; no transparent fallback is consulted.',
                'Parent AN submissions are covered as semantic resource consumers, not as observed full parent draws.',
                'This reads frozen CPU/Ghidra evidence and ROM data; it does not execute CPU code or claim REA decompilation.',
                'The result proves asset identity and required pixels within the saved scope, not palette timing, cache/global-slot persistence, full frame composition or scanline behavior.',
                'Neither ROM, raw tile bytes, CPU image nor emulator core is an output; final runtime remains independent.']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    output = args.out.resolve()
    require(output.is_relative_to(ROOT), 'audit output must remain inside this worktree')
    require(output.parent.is_dir(), 'audit output parent must already exist')
    report = audit()
    output.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'pass': report['pass'], 'direct_catalog_reuse': report['direct_catalog_reuse'],
                      'required_graphic_keys': report['required_graphic_keys'],
                      'required_unique_indexed_tiles': report['required_unique_indexed_tiles'],
                      'new_child_AN_records': report['new_child_AN_records'], 'missing_assets': report['missing_assets'],
                      'report_bytes': output.stat().st_size, 'report_sha256': sha(output.read_bytes())}, indent=2))


if __name__ == '__main__':
    main()
