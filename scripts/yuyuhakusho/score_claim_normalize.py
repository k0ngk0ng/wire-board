"""Research-only canonical score oracle; no native program executes here."""
from pathlib import Path
import argparse
import copy
import gzip
import itertools
import json

from reference import ROOT, sha
from score_claim_capture import SemanticView, binding, saved, ym_effective

EXCLUSIONS = [
    'source PC and sequence address', 'source shared loop-counter/return words',
    'duration multiplier resolved into typed note duration',
    'last duration used only by source cleanup diagnostic outcomes',
    'root priority, queue and selected sound owned by external dispatcher',
    'current traversal phase derived within each driver call',
    'loaded feedback belongs to chip state and is unused by score arithmetic',
    'unused source padding and source pointers',
    'disabled pan automation parameters overwritten before the next enable',
]
MUSIC = {130, 131, 132, 133, 134, 135, 136, 138, 141, 149, 143}


def load(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def envelope_map(data, kind, definition):
    position, positions, ticks, seen = 0, [], [], {}
    terminal = None
    while position not in seen:
        seen[position] = len(ticks)
        positions.append(position)
        visited, scale = set(), 0
        while True:
            if position in visited or not 0 <= position < len(data):
                raise ValueError('source envelope has an invalid same-tick transition')
            visited.add(position)
            at, value = position, data[position]
            position = (position + 1) & 255
            if value == 128:
                position = 0
                continue
            if value == 130:
                position = data[at + 1]
                continue
            if kind == 'pitch':
                if value == 129:
                    position = (position - 2) & 255
                    continue
                if value == 132:
                    scale = (scale + data[at + 1]) & 255
                    position = (position + 1) & 255
                    continue
                signed_scale = scale - 256 if scale >= 128 else scale
                if value == 131:
                    ticks.append({'offset': None, 'scale_add': signed_scale,
                                  'key_off': True, 'fixed_pitch_word': 131})
                    terminal = position
                    break
                ticks.append({'offset': value - 256 if value >= 128 else value,
                              'scale_add': signed_scale, 'key_off': False, 'fixed_pitch_word': None})
            elif value in (129, 131):
                position = (position - 2) & 255
                ticks.append({'level': None, 'writes_level': False, 'key_off': value == 131})
            else:
                ticks.append({'level': value, 'writes_level': True, 'key_off': False})
            break
        if terminal is not None:
            break
    first = len(ticks) if terminal is not None else seen[position]
    assert ticks[:first] == definition['prefix'] and ticks[first:] == definition['cycle']
    result = {}
    for index, raw_position in enumerate(positions):
        result[raw_position] = {'phase': 'prefix' if index < first else 'cycle',
                                'index': index if index < first else index - first}
    if terminal is not None:
        result[terminal] = {'phase': 'ended', 'index': 0}
    return result


def build_cursor_maps(rom, library):
    result = {'pitch': {}, 'volume': {}}
    for kind, table, end, key in (
            ('pitch', 0x3a256, 0x3a31a, 'pitch_envelopes'),
            ('volume', 0x3a31a, 0x3a492, 'psg_volume_envelopes')):
        for definition in library[key]:
            envelope_id = definition['id']
            at = table + (envelope_id - 1) * 4
            begin = int.from_bytes(rom[at:at + 4], 'big')
            result[kind][envelope_id] = envelope_map(rom[begin:end], kind, definition)
    return result


def canonical_state(source, cursor_maps):
    result = copy.deepcopy(source)
    for name in ('priority', 'selected_sound', 'queue', 'phase'):
        del result[name]
    for voice in result['voices']:
        for name in ('duration_multiplier', 'last_duration', 'loaded_feedback'):
            del voice[name]
        volume_position = voice.pop('volume_envelope_position')
        pitch_position = voice.pop('pitch_envelope_position')
        volume_id = voice['tone'] if '.psg' in voice['slot'] else 0
        pitch_id = voice['pitch_control']['envelope_id']
        voice['volume_envelope_cursor'] = copy.deepcopy(
            cursor_maps['volume'][volume_id][volume_position] if volume_id
            else {'phase': 'disabled', 'index': 0})
        voice['pitch_envelope_cursor'] = copy.deepcopy(
            cursor_maps['pitch'][pitch_id][pitch_position] if pitch_id
            else {'phase': 'disabled', 'index': 0})
    return result


def score_intents(intents):
    score, external = [], []
    for intent in intents:
        if intent['kind'] == 'pcm-command':
            external.append(intent)
        else:
            score.append({**intent, 'order': len(score)})
    return score, external


def dispatch_and_clear(events):
    headers = [event.get('sound_id', event.get('code')) for event in events if event['kind'] == 'header']
    assert len(headers) <= 1
    dispatch = headers[0] if headers else 0
    clear = dispatch in MUSIC or any(event['pc'] == 0x3918e and 0xfa20 <= event['row'] < 0xfb40 for event in events)
    return dispatch, clear


def json_line(stream, value):
    stream.write(json.dumps(value, separators=(',', ':')) + '\n')


def write_mixed(index, out, maps):
    source_index = load(index)
    results = []
    for case in source_index['cases']:
        packet = load(ROOT / case['files']['portable']['path'])
        events = load(ROOT / case['files']['provenance']['path'])['events']
        by_step = {step: list(group) for step, group in itertools.groupby(events, key=lambda event: event['step'])}
        path = out / (case['id'] + '.jsonl.gz')
        count = 0
        with gzip.open(path, 'wt', compresslevel=5) as stream:
            json_line(stream, {'kind': 'initial', 'id': case['id'], 'commands': packet['commands'],
                               'state': canonical_state(packet['initial'], maps)})
            for frame in packet['frames']:
                dispatch, clear = dispatch_and_clear(by_step.get(frame['step'], []))
                intents, external = score_intents(frame['intents'])
                json_line(stream, {'kind': 'frame', **frame, 'state': canonical_state(frame['state'], maps),
                                   'dispatch_sound_id': dispatch, 'clear_primary_priority': clear,
                                   'intents': intents, 'external_intents': external})
                count += len(intents)
        results.append({'id': case['id'], 'steps': case['steps'], 'intents': count,
                        'file': binding(path), 'input_index': binding(index)})
        print(json.dumps({'normalized': case['id'], 'steps': case['steps'], 'intents': count}), flush=True)
    return results


def native_intents(events, view):
    result, routing = [], []
    for event in events:
        kind = event['kind']
        if kind == 'ym-request':
            result.append({'order': len(result), 'kind': 'ym', 'bank': event['port'],
                           'register': event['register'], 'value': ym_effective(event['register'], event['value'])})
        elif kind == 'psg-request':
            result.append({'order': len(result), 'kind': 'psg', 'value': event['value']})
        elif kind == 'dac-descriptor':
            row = bytes.fromhex(event['row_hex'])
            result.append({'order': len(result), 'kind': 'dac', 'descriptor_id': view.descriptors[row[16]],
                           'pan': {'left': bool(row[39] & 128), 'right': bool(row[39] & 64)}})
        elif kind == 'replace-first-enqueue':
            sound = bytes.fromhex(event['read_hex'])[0]
            result.append({'order': len(result), 'kind': 'queue', 'action': 'replace-first', 'sound_id': sound})
        elif kind == 'voice-routing':
            row_index = (event['row'] - 0xf840) // 48
            if event['pc'] in (0x39176, 0x3888a):
                channel = bytes.fromhex(event['row_hex'])[1]
                row_index = {128: 7, 160: 8, 192: 9, 224: 9}[channel]
            from score_claim_capture import SLOTS
            routing.append({'action': event['action'], 'slot': SLOTS[row_index][0]})
    return result, routing


def write_isolated(index, out, maps, rom):
    source_index = load(index)
    results = []
    for case in source_index['cases']:
        events_path = ROOT / case['files']['events']['path']
        frames_path = ROOT / case['files']['frames']['path']
        assert sha(events_path.read_bytes()) == case['files']['events']['sha256']
        assert sha(frames_path.read_bytes()) == case['files']['frames']['sha256']
        events = load(events_path)['events']
        first = next(event for event in events if event['kind'] == 'tick-before')
        view = SemanticView(rom)
        identity = 'isolated-' + format(case['code'], '02x')
        path = out / (identity + '.jsonl.gz')
        groups = iter(itertools.groupby(events, key=lambda event: event['step']))
        current_step, group = next(groups)
        current_events = list(group)
        count = 0
        with gzip.open(path, 'wt', compresslevel=5) as output, gzip.open(frames_path, 'rt') as frames:
            json_line(output, {'kind': 'initial', 'id': identity,
                               'commands': [{'step': 0, 'code': case['code']}],
                               'state': canonical_state(view.state(bytes.fromhex(first['driver_hex'])), maps)})
            for step, line in enumerate(frames):
                frame = json.loads(line)
                assert frame['step'] == current_step == step
                for event in current_events:
                    if event['kind'] == 'voice-init':
                        view.owners[event['row']] = {'score_id': case['code'], 'voice_id': event['voice_ordinal']}
                intents, routing = native_intents(current_events, view)
                dispatch, clear = dispatch_and_clear(current_events)
                json_line(output, {'kind': 'frame', 'step': step,
                                   'dispatch_sound_id': dispatch, 'clear_primary_priority': clear,
                                   'state': canonical_state(view.state(bytes.fromhex(frame['driver_hex'])), maps),
                                   'intents': intents, 'external_intents': [], 'dac_feedback': [],
                                   'routing': routing, 'pcm_sha256': frame['pcm_sha256'],
                                   'stereo_frames': frame['stereo_frames']})
                count += len(intents)
                try:
                    current_step, group = next(groups)
                    current_events = list(group)
                except StopIteration:
                    assert step + 1 == case['steps']
        results.append({'id': identity, 'steps': case['steps'], 'intents': count,
                        'file': binding(path), 'input_index': binding(index)})
        print(json.dumps({'normalized': identity, 'steps': case['steps'], 'intents': count}), flush=True)
    return results


def run(out, isolated, mixed, cache, catalog):
    rom_path = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    library = load(catalog)
    maps = build_cursor_maps(rom, library)
    saved(out / 'cursor-map-provenance.json', maps)
    results = write_mixed(mixed, out, maps) + write_mixed(cache, out, maps)
    results += write_isolated(isolated, out, maps, rom)
    report = {'schema': 1, 'format': 'canonical-score-observables-v1', 'pass': True,
              'independent_driver_verified': False, 'cursor_maps_verified_against_typed_tables': True,
              'exclusions': EXCLUSIONS, 'cases': results,
              'counts': {'frames': sum(case['steps'] for case in results),
                         'intents': sum(case['intents'] for case in results)},
              'source_bindings': [binding(Path(__file__).resolve()), binding(ROOT / 'scripts/yuyuhakusho/score_claim_capture.py'),
                                  binding(ROOT / 'scripts/yuyuhakusho/score_claim_state_check.py'), binding(catalog), binding(rom_path)],
              'inputs': [binding(isolated), binding(mixed), binding(cache)],
              'limits': ['Research source normalization only. The Go driver must independently produce every compared state and intent.',
                         'Isolated DAC acceptance has the existing positive-note/accepted-descriptor scope; actual guard feedback is only captured in mixed cases.',
                         'Group2 initialization/layering and master-cycle scheduling remain separate.',
                         'Source-byte envelope cursor mapping is retained only in cursor-map-provenance.json.']}
    saved(out / 'index.json', report)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--isolated', type=Path, required=True)
    parser.add_argument('--mixed', type=Path, required=True)
    parser.add_argument('--cache', type=Path, required=True)
    parser.add_argument('--catalog', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    assert output.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-claim')
    output.mkdir(parents=True, exist_ok=False)
    run(output, args.isolated.resolve(), args.mixed.resolve(), args.cache.resolve(), args.catalog.resolve())
