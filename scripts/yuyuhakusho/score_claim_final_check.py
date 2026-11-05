"""Frozen acceptance of the final42 canonical cases including pan feedback."""
from pathlib import Path
import argparse
import gzip
import json
from score_claim_check import ROOT, clean, load, sha


def check(index, out):
    report = load(index)
    assert report['pass'] and report['projection_version'] == 3 and len(report['cases']) == 42
    base = load(ROOT / report['previous_index']['path'])
    assert len(base['cases']) == 40 and report['cases'][:40] == base['cases']
    assert report['exclusions'] == base['exclusions']
    extra_frames = extra_intents = 0
    feedback_cases = []
    for case in report['cases'][40:]:
        path = ROOT / case['file']['path']
        assert sha(path) == case['file']['sha256']
        with gzip.open(path, 'rt') as stream:
            first = json.loads(next(stream))
            clean(first)
            for line in stream:
                frame = json.loads(line)
                clean(frame)
                assert frame['step'] == extra_frames % 160
                if frame['pan_feedback']:
                    assert frame['pan_feedback'] == [{'pan_redirect': True}]
                    pan = [event for event in frame['intents'] if event['kind'] == 'dac-pan']
                    assert len(pan) == 1 and pan[0]['pan'] == {'left': True, 'right': True}
                    assert not any(event['kind'] == 'ym' and event['bank'] == 1 and event['register'] == 0xb6 for event in frame['intents'])
                    feedback_cases.append((case['id'], frame['step']))
                assert all(event['kind'] != 'pcm-command' for event in frame['intents'])
                assert [event['order'] for event in frame['intents']] == list(range(len(frame['intents'])))
                extra_frames += 1
                extra_intents += len(frame['intents'])
    assert feedback_cases == [('pcm-then-music88', 11), ('music88-pcm-before-pan', 1)]
    assert extra_frames == 320
    assert report['counts'] == {'frames': 126141, 'intents': base['counts']['intents'] + extra_intents}
    result = {'pass': True, 'cases': 42, **report['counts'],
              'base40_unchanged': True, 'pan_feedback_cases': feedback_cases,
              'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__)),
              'limits': ['Canonical source oracle only; independent Go driver/DSP equality is separate.']}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.out.resolve())
