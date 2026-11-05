"""Test-first contract for the canonical score state and reused full oracle."""
from pathlib import Path
import argparse
import gzip
import importlib
import json

from score_claim_check import ROOT, clean, load, sha

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


def check(index, out):
    helper = importlib.import_module('score_claim_normalize')
    report = load(index)
    assert report['pass'] and report['format'] == 'canonical-score-observables-v1'
    assert report['exclusions'] == EXCLUSIONS
    assert not report['independent_driver_verified']
    assert len(report['cases']) == 39
    original_steps = mixed_steps = frames = intents = 0
    ids = set()
    for case in report['cases']:
        assert case['id'] not in ids
        ids.add(case['id'])
        path = ROOT / case['file']['path']
        assert sha(path) == case['file']['sha256']
        with gzip.open(path, 'rt') as stream:
            first = json.loads(next(stream))
            assert first['kind'] == 'initial' and first['id'] == case['id']
            clean(first)
            for step, line in enumerate(stream):
                packet = json.loads(line)
                clean(packet)
                assert packet['kind'] == 'frame' and packet['step'] == step
                state = packet['state']
                assert not ({'priority', 'queue', 'selected_sound', 'phase'} & state.keys())
                assert len(state['voices']) == 18
                for voice in state['voices']:
                    assert not ({'duration_multiplier', 'last_duration', 'loaded_feedback',
                                 'volume_envelope_position', 'pitch_envelope_position'} & voice.keys())
                    for name in ('volume_envelope_cursor', 'pitch_envelope_cursor'):
                        cursor = voice[name]
                        assert set(cursor) == {'phase', 'index'}
                        assert cursor['phase'] in ('prefix', 'cycle', 'ended', 'disabled')
                        assert isinstance(cursor['index'], int) and cursor['index'] >= 0
                assert packet['dispatch_sound_id'] in [0, 130, 131, 132, 133, 134, 135, 136, 138, 141, 149, 143] + list(range(160, 172))
                assert isinstance(packet['clear_primary_priority'], bool)
                assert all(event['kind'] != 'pcm-command' for event in packet['intents'])
                assert [event['order'] for event in packet['intents']] == list(range(len(packet['intents'])))
                frames += 1
                intents += len(packet['intents'])
            assert step + 1 == case['steps']
        if case['id'].startswith('isolated-'):
            original_steps += case['steps']
        else:
            mixed_steps += case['steps']
    assert original_steps == 122810 and mixed_steps == 2851 and frames == 125661
    assert report['counts']['frames'] == frames and report['counts']['intents'] == intents
    assert report['cursor_maps_verified_against_typed_tables']
    assert helper.EXCLUSIONS == EXCLUSIONS
    result = {'pass': True, 'canonical_cases': 39, 'frames': frames, 'intents': intents,
              'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__)),
              'scope': 'Source-normalized oracle completeness, schema and exclusions; independent driver equality remains separate.'}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.out.resolve())
