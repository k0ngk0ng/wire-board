"""First portable score-control window acceptance; no Go-generated golden."""
from pathlib import Path
import argparse
import collections
import copy
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]
CATALOG = '735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830'


def clean(value):
    if isinstance(value, dict):
        assert not {'pc', 'address', 'source_address', 'driver_hex', 'row_hex', 'before_driver_hex',
                    'after_driver_hex', 'cycle', 'event_index', 'raw', 'stack', 'at', 'refresh'} & value.keys()
        for child in value.values():
            clean(child)
    elif isinstance(value, list):
        for child in value:
            clean(child)


def check(path):
    data = json.loads(gzip.decompress(path.read_bytes()))
    assert data['schema'] == 1 and data['catalog_json_sha256'] == CATALOG
    clean(data)
    counts = collections.Counter()
    fade_io = collections.Counter()
    for case in data['cases']:
        assert case['setup']['factory'] == 'score.New + accepted whole driver calls'
        for window in case['windows']:
            counts[window['action']] += 1
            before, after = window['before'], window['after']
            assert set(before) == set(after) == {'tempo_remaining', 'tempo_period', 'voices'}
            assert len(before['voices']) == len(after['voices']) == 18
            assert window['legal_before_whole_tick_boundary_proved']
            if window['action'] == 'stop-dac-flags':
                wanted = copy.deepcopy(before)
                for name in ('active', 'note_off', 'claimed', 'tie'):
                    wanted['voices'][0][name] = False
                assert after == wanted and window['score_intents'] == []
                assert window['after_authority'] == 'observed-DAC-row plus single-flags-store source projection'
            elif window['action'] == 'fade-step':
                assert before['tempo_remaining'] == after['tempo_remaining'] and before['tempo_period'] == after['tempo_period']
                for i, (old, new) in enumerate(zip(before['voices'], after['voices'])):
                    old, new = dict(old), dict(new)
                    if 1 <= i <= 9:
                        old.pop('base_level'); new.pop('base_level')
                        old.pop('active'); new.pop('active')
                    assert old == new
                assert window['root_hardware_intents'] == []
                fade_io.update(event['kind'] for event in window['score_intents'])
            elif window['action'] == 'reset-score':
                assert after['tempo_remaining'] == after['tempo_period'] == 0
                assert all(voice['owner'] is None and not voice['active'] and voice['note_wait'] == 0 for voice in after['voices'])
                assert window['score_intents'] == []
                assert collections.Counter(event['kind'] for event in window['root_hardware_intents']) == {'ym': 31, 'psg': 4}
            else:
                raise AssertionError('unknown action')
            assert [event['order'] for event in window['score_intents']] == list(range(len(window['score_intents'])))
            assert [event['order'] for event in window['root_hardware_intents']] == list(range(len(window['root_hardware_intents'])))
    assert counts == {'stop-dac-flags': 2, 'fade-step': 88, 'reset-score': 4}
    assert fade_io == {'ym': 968, 'psg': 42}
    assert data['coverage']['fm_before_level_range'] == [14, 84]
    assert data['coverage']['native_fm_deactivation_windows'] == 0
    assert data['coverage']['native_psg_deactivations'] == 6
    return {'pass': True, 'windows': 94, 'actions': dict(counts), 'fade_intents': dict(fade_io),
            'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            'scope': 'Original owner projections and ordered IO; score Go action implementation remains independent.'}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixture', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(check(args.fixture.resolve()), indent=2))
