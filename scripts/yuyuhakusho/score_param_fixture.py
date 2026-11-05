"""Export independent original requests/table interpretations for Go parameters."""
from pathlib import Path
import gzip
import hashlib
import json

from score_parameter_tables_check import raw_pitch, raw_volume
from check_audio_score_parameters import raw_pan_calls

ROOT = Path(__file__).resolve().parents[2]
MASKS = {0x30: 0x7f, 0x40: 0x7f, 0x50: 0xdf, 0x60: 0x9f,
         0x70: 0x1f, 0x80: 0xff, 0xb0: 0x3f}
MODES = ('disabled', 'advance-on-note', 'restart-on-note-once',
         'restart-on-note-loop', 'restart-every-driver-tick')
PATTERNS = ('right-left-right-center', 'right-center-left-center',
            'center-left-center-right')


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load(path):
    return json.loads(gzip.decompress(path.read_bytes()) if path.suffix == '.gz'
                      else path.read_bytes())


def main():
    base = ROOT / '.local/md-yuyuhakusho/audio-score-source'
    catalog_path = ROOT / 'games/yuyuhakusho/data/audio-score-candidate.json.gz'
    index_path = base / 'full/index.json'
    origin_path = base / 'typed-boundary-corrected/provenance.json'
    parameter_path = ROOT / '.local/md-yuyuhakusho/audio-score-parameters/promoted-proof/native-read-provenance.json'
    rom_path = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom, library, index = rom_path.read_bytes(), load(catalog_path), load(index_path)
    assert hashlib.sha256(rom).hexdigest() == index['rom_sha256']
    origin = load(origin_path)
    patches = {row['id']: row for row in library['fm_patches']}
    patch_sources = {(row['bank'], row['index']): row['id'] for row in origin['patches']}
    patch_raw = {}
    for (bank, selection), identity in patch_sources.items():
        patch_raw.setdefault(identity, rom[bank + selection * 25:bank + selection * 25 + 25])
    assert set(patch_raw) == set(patches)
    loads, traces = [], []
    for case in index['cases']:
        path = ROOT / case['files']['events']['path']
        assert digest(path) == case['files']['events']['sha256']
        traces.append({'path': str(path.relative_to(ROOT)), 'sha256': digest(path)})
        events = load(path)['events']
        for i, event in enumerate(events):
            if event['kind'] != 'fm-patch-read':
                continue
            state = bytes.fromhex(event['row_hex'])
            channel, level = state[1], state[9]
            identity = patch_sources[(event['bank_address'], event['instrument_index'])]
            requests = []
            for following in events[i + 1:i + 120]:
                if following['tick'] != event['tick']:
                    break
                if following.get('row') != event['row']:
                    continue
                if following['kind'] == 'ym-request':
                    assert following['port'] == (1 if channel & 4 else 0)
                    register = following['register'] - (channel & 3)
                    family = register if register == 0xb0 else register & 0xf0
                    requests.append({'register': register, 'value': following['value'] & MASKS[family]})
                    if len(requests) == 25:
                        break
                elif following['kind'] in ('control-after', 'fm-patch-read'):
                    break
            assert len(requests) == 25
            loads.append({'patch': identity, 'level': level, 'writes': requests})
    assert len(loads) == 2525
    levels = []
    for identity, data in sorted(patch_raw.items()):
        mask = (8, 8, 8, 8, 10, 14, 14, 15)[data[0] & 7]
        for level in range(256):
            initial, ordinary = [], []
            for source_slot, slot in enumerate((0, 2, 1, 3)):
                total = data[21 + source_slot]
                carrier = bool(mask & 1 << source_slot)
                initial.append({'slot': slot, 'total_level': (total + (level if carrier else 0)) & 127})
                if level < 128 and carrier and total + level <= 255:
                    ordinary.append({'slot': slot, 'total_level': (total + level) & 127})
            levels.append({'patch': identity, 'level': level, 'initial': initial, 'ordinary': ordinary})
    pitch, volume = [], []
    for definitions, pointer, end, model, output in (
            (library['pitch_envelopes'], 0x3a256, 0x3a31a, raw_pitch, pitch),
            (library['psg_volume_envelopes'], 0x3a31a, 0x3a492, raw_volume, volume)):
        for definition in definitions:
            start = int.from_bytes(rom[pointer + (definition['id'] - 1) * 4:pointer + definition['id'] * 4], 'big')
            output.append({'definition': definition, 'expected': model(rom[start:end], 2048)})
    calls = [(tick % 7 == 0, tick % 13 == 12) for tick in range(600)]
    pan_values = [(mode, pattern, 0, length - 1, period)
                  for mode in range(5) for pattern, length in enumerate((2, 3, 4), 1)
                  for period in (0, 1, 3, 255)]
    pan_values.extend(tuple(values) for values in load(parameter_path)['actual_pan_arguments'])
    assert len(pan_values) == 87
    pan = []
    for values in pan_values:
        mode, pattern, initial, limit, period = values
        definition = {'mode': MODES[mode]}
        if mode:
            definition.update(pattern=PATTERNS[pattern - 1], initial_step=initial,
                              step_limit=limit, period=period)
        else:
            # The static disabled-dispatch cases start with arbitrary elapsed
            # values. They must remain unchanged rather than being reset.
            definition.update(initial_step=initial, period=period)
        expected = [{'state': {'step': step, 'elapsed': elapsed}, 'pan': stereo}
                    for step, elapsed, stereo in raw_pan_calls(rom, values, calls)]
        pan.append({'definition': definition,
                    'calls': [{'note': note, 'off': off} for note, off in calls],
                    'expected': expected})
    fixture = {'patches': library['fm_patches'], 'patch_loads': loads, 'levels': levels,
               'pitch': pitch, 'volume': volume, 'patterns': library['pan_patterns'], 'pan': pan}
    target = ROOT / 'games/yuyuhakusho/audio/score/param/testdata/source.json.gz'
    raw = (json.dumps(fixture, separators=(',', ':')) + '\n').encode()
    target.write_bytes(gzip.compress(raw, mtime=0))
    sources = [Path(__file__).resolve(), catalog_path, index_path, origin_path,
               parameter_path, ROOT / 'scripts/yuyuhakusho/score_parameter_tables_check.py',
               ROOT / 'scripts/yuyuhakusho/check_audio_score_parameters.py']
    report = {'schema': 1, 'scope': 'Go parameter source fixture; no driver or DSP claim',
              'rom_sha256': hashlib.sha256(rom).hexdigest(),
              'fixture': {'path': str(target.relative_to(ROOT)), 'sha256': digest(target), 'bytes': target.stat().st_size},
              'counts': {'patch_loads': len(loads), 'actual_patch_requests': len(loads) * 25,
                         'level_cases': len(levels), 'pitch_ticks': sum(len(row['expected']) for row in pitch),
                         'volume_ticks': sum(len(row['expected']) for row in volume),
                         'pan_calls': sum(len(row['expected']) for row in pan)},
              'chip_effective_register_masks': {hex(key): hex(value) for key, value in MASKS.items()},
              'sources': [{'path': str(path.relative_to(ROOT)), 'sha256': digest(path)} for path in sources],
              'native_traces': traces,
              'limits': ['Raw request count/order is preserved; request values project only declared hardware ignored bits.',
                         'Pitch envelope source interpretation has no actual read coverage in these definitions.',
                         'Parameter fixture does not establish continuous driver, mixed claim/restore, or DSP equivalence.']}
    result = ROOT / 'games/yuyuhakusho/verification/audio-score-param/fixture.json'
    result.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({key: report[key] for key in ('fixture', 'counts')}, indent=2))


if __name__ == '__main__':
    main()
