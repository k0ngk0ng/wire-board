"""Portable score-control owner projections from saved original action windows."""
from pathlib import Path
import argparse
import collections
import copy
import gzip
import hashlib
import json

from reference import ROOT
from score_claim_capture import SemanticView, ym_effective
from score_claim_normalize import build_cursor_maps
from score_claim_project_v3 import corrected_state

JSON_SHA = '735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830'
SLOT = {'dac': 0, 'fm1': 1, 'fm2': 2, 'fm3': 3, 'fm4': 4, 'fm5': 5,
        'fm6': 6, 'psg1': 7, 'psg2': 8, 'psg3': 9}


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def bind(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path), 'bytes': path.stat().st_size}


def load(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def event_key(event):
    return event['step'], event['pc'], event['at'], event['kind']


def score_projection(driver, view, maps):
    full = corrected_state(view.state(driver), maps)
    return {name: full[name] for name in ('tempo_remaining', 'tempo_period', 'voices')}


def chip_intents(events):
    result = []
    for event in events:
        if event['kind'] == 'ym-request':
            result.append({'order': len(result), 'kind': 'ym', 'bank': event['bank'],
                           'register': event['register'], 'value': ym_effective(event['register'], event['value'])})
        elif event['kind'] == 'psg-request':
            result.append({'order': len(result), 'kind': 'psg', 'value': event['value']})
    return result


def export(out):
    actions_index = ROOT / '.local/md-yuyuhakusho/root-control-actions/index.json'
    native_index = ROOT / '.local/md-yuyuhakusho/root-control-canonical-v2/capture.json'
    actions, capture = load(actions_index), load(native_index)
    assert actions['pass'] and capture['pass']
    catalog_path = ROOT / 'games/yuyuhakusho/data/audio-score.json.gz'
    raw_catalog = gzip.decompress(catalog_path.read_bytes())
    assert hashlib.sha256(raw_catalog).hexdigest() == JSON_SHA
    catalog = json.loads(raw_catalog)
    definitions = {score['id']: score for score in catalog['scores']}
    rom_path = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    assert hashlib.sha256(rom).hexdigest() == capture['identity']['original']['rom_sha256']
    maps = build_cursor_maps(rom, catalog)
    cases, provenance = [], []
    counts = collections.Counter()
    fm_levels, psg_deactivations, fm_deactivations = [], 0, 0
    source_bindings = [bind(actions_index), bind(native_index), bind(catalog_path), bind(rom_path)]
    skipped = []
    for source_case in actions['cases']:
        action_path = ROOT / source_case['output_path']
        canonical_path = ROOT / source_case['source_path']
        assert sha(action_path) == source_case['output_sha256'] and sha(canonical_path) == source_case['source_sha256']
        action_data, body = load(action_path), load(canonical_path)
        windows = [segment for segment in action_data['segments'] if segment['kind'] in ('stop-dac-flags', 'fade-step', 'full-clear04')]
        if not windows:
            skipped.append({'id': source_case['case'], 'reason': 'No requested score mutation window; idle pause is not active-fade coverage.'})
            continue
        source_bindings += [bind(action_path), bind(canonical_path)]
        view = SemanticView(rom)
        owners_at = {}
        accepted = []
        for event in body['events']:
            if event['kind'] == 'dispatch' and event['selected'] in definitions:
                definition = definitions[event['selected']]
                for voice in definition['voices']:
                    slot = SLOT[voice['channel']]
                    if definition['kind'] != 'music':
                        slot = {3: 10, 4: 11, 5: 12, 7: 13, 8: 14, 9: 15}[slot]
                    view.owners[0xf840 + slot * 48] = {'score_id': definition['id'], 'voice_id': voice['id']}
                accepted.append({'whole_tick': event['step'], 'sound_id': definition['id']})
            if 'driver_hex' in event:
                owners_at[event_key(event)] = copy.deepcopy(view.owners)
        portable_windows, action_schedule = [], []
        for ordinal, segment in enumerate(windows):
            kind = segment['kind']
            action = {'stop-dac-flags': 'stop-dac-flags', 'fade-step': 'fade-step', 'full-clear04': 'reset-score'}[kind]
            start = segment['source_event'] if kind == 'stop-dac-flags' else segment['events'][0]
            view.owners = copy.deepcopy(owners_at[event_key(start)])
            before = bytes.fromhex(start['driver_hex'] if kind == 'stop-dac-flags' else segment['before_driver_hex'])
            if kind == 'stop-dac-flags':
                observed_row = bytes.fromhex(segment['after_dac_hex'])
                assert bytes.fromhex(segment['before_dac_hex']) == before[64:112]
                assert observed_row[1:] == before[65:112] and observed_row[0] == 0
                after = before[:64] + observed_row + before[112:]
                authority = 'observed-DAC-row plus single-flags-store source projection'
                original_events = []
            else:
                after = bytes.fromhex(segment['after_driver_hex'])
                authority = 'direct original before/after window, score-owner field projection'
                original_events = segment['events']
            step = segment['step']
            assert step > 0
            previous = bytes.fromhex(body['frames'][step - 1]['driver_hex'])
            # These saved action cases all have tempo0 and control/duck idle.
            # Root fields are deliberately outside this equality/projection.
            assert before[1:3] == previous[1:3] == b'\x00\x00'
            assert before[64:928] == previous[64:928], (source_case['case'], step, 'not a whole-tick score boundary')
            before_view, after_view = score_projection(before, view, maps), score_projection(after, view, maps)
            original_io = chip_intents(original_events)
            if kind == 'fade-step':
                assert before[1:3] == after[1:3]
                for slot in range(18):
                    first = before[64 + 48 * slot:112 + 48 * slot]
                    last = after[64 + 48 * slot:112 + 48 * slot]
                    changed = {index for index, (a, b) in enumerate(zip(first, last)) if a != b}
                    assert changed <= ({0, 9} if 1 <= slot <= 9 else set())
                    if 1 <= slot <= 6 and first[0] & 128:
                        fm_levels.append(first[9])
                        fm_deactivations += not bool(last[0] & 128)
                    if 7 <= slot <= 9 and first[0] & 128:
                        psg_deactivations += not bool(last[0] & 128)
            if kind == 'full-clear04':
                assert after[1:3] == b'\x00\x00' and not any(after[64:928])
                assert collections.Counter(event['kind'] for event in original_io) == {'ym': 31, 'psg': 4}
            score_io = original_io if kind == 'fade-step' else []
            root_io = original_io if kind == 'full-clear04' else []
            portable_windows.append({'id': ordinal, 'whole_tick': step, 'action': action,
                                     'stage': 'before-score-advance in the captured tempo0 profile',
                                     'before': before_view, 'after': after_view,
                                     'score_intents': score_io, 'root_hardware_intents': root_io,
                                     'after_authority': authority,
                                     'legal_before_whole_tick_boundary_proved': True,
                                     'preserve_driver_ticks': True, 'preserve_event_cursor': action != 'reset-score'})
            action_schedule.append({'whole_tick': step, 'action': action})
            provenance.append({'case': source_case['case'], 'window': ordinal, 'kind': kind,
                               'source_step': step, 'start_event': start,
                               'before_driver_hex': before.hex(), 'after_driver_hex': after.hex(),
                               'original_events': original_events, 'previous_frame_score_equal': True,
                               'after_authority': authority})
            counts[action] += 1
        cases.append({'id': source_case['case'], 'setup': {
            'factory': 'score.New + accepted whole driver calls',
            'accepted_score_dispatches': accepted, 'prior_score_actions': action_schedule,
            'action_order': 'Apply listed actions before Advance of their whole_tick; each window uses only earlier whole calls.',
            'feedback_for_state_setup': 'Valid DAC feedback may be supplied by caller; setup is a legal-state construction, not an original IO golden.'},
            'windows': portable_windows})
    coverage = {'fm_before_level_range': [min(fm_levels), max(fm_levels)],
                'native_fm_deactivation_windows': fm_deactivations,
                'native_psg_deactivations': psg_deactivations,
                'uncovered': ['FM127-to128 deactivation', 'FM255-to0 wrap', 'carrier TL carry skips',
                              'fade while a primary claim suppresses music writes', 'active-fade pause',
                              'nonzero-tempo placement between tempo and voice scans']}
    fixture = {'schema': 1, 'catalog_json_sha256': JSON_SHA,
               'format': 'native-score-control-owner-projection-v1', 'cases': cases,
               'coverage': coverage, 'skipped_cases': skipped,
               'view_projection': ['tempo_remaining', 'tempo_period', 'voices'],
               'root_fields_excluded': ['priority', 'selected', 'queue', 'fade', 'pause', 'duck', 'choice'],
               'no_go_generated_expected_values': True, 'new_cpu_capture': False}
    out.mkdir(parents=True, exist_ok=False)
    testdata = out / 'testdata'
    testdata.mkdir()
    target = testdata / 'score-control-actions.json.gz'
    raw = (json.dumps(fixture, separators=(',', ':')) + '\n').encode()
    target.write_bytes(gzip.compress(raw, mtime=0))
    private_path = out / 'provenance.json.gz'
    private_path.write_bytes(gzip.compress((json.dumps(provenance, separators=(',', ':')) + '\n').encode(), mtime=0))
    report = {'pass': True, 'source_cases': len(actions['cases']), 'action_cases': len(cases),
              'windows': sum(counts.values()), 'counts': dict(counts), 'coverage': coverage,
              'fixture': bind(target), 'private_provenance': bind(private_path),
              'source_bindings': source_bindings + [bind(Path(__file__).resolve()),
                  bind(ROOT / 'scripts/yuyuhakusho/root-control-export-actions.py'),
                  bind(ROOT / 'scripts/yuyuhakusho/score_claim_capture.py'),
                  bind(ROOT / 'scripts/yuyuhakusho/score_claim_normalize.py'),
                  bind(ROOT / 'scripts/yuyuhakusho/score_claim_project_v3.py')],
              'limits': ['Stop flags after is observed row plus statically single-store projection, not a full immediate after sample.',
                         'ResetScore is state-only owner projection;35 observed full-clear hardware requests belong to Root.',
                         'Virtual Go Ticks/event cursor preservation is an action contract, not a native RAM value.',
                         'No action expected values are generated by Go.']}
    (out / 'index.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'pass': True, 'windows': report['windows'], 'counts': dict(counts),
                      'fixture': report['fixture'], 'coverage': coverage}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    target = args.out.resolve()
    if not target.is_relative_to(ROOT / '.local/md-yuyuhakusho/score-control-oracle'):
        raise ValueError('output outside owned control-oracle scope')
    export(target)
