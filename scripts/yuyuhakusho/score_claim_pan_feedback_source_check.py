"""Test-first contract for external DAC pan redirection feedback."""
from pathlib import Path
import argparse
import json
from score_claim_check import ROOT, clean, load, sha


def cases():
    return [
        {'id': 'pcm-then-music88', 'steps': 160,
         'commands': [{'step': 0, 'code': 125}, {'step': 10, 'code': 136}], 'headers': [136]},
        {'id': 'music88-pcm-before-pan', 'steps': 160,
         'commands': [{'step': 0, 'code': 136}, {'step': 1, 'code': 125}], 'headers': [136]},
    ]


def check(index, out):
    report = load(index)
    assert report['pass'] and report['cases_requested'] == cases()
    assert {0x37b66, 0x37b70}.issubset(report['observation_points'])
    assert len(report['observation_points']) <= 64
    outcomes = []
    for case in report['cases']:
        assert case['original_equal'] and case['pcm_samples_equal']
        data = load(ROOT / case['files']['portable']['path'])
        clean(data)
        raw = load(ROOT / case['files']['provenance']['path'])['events']
        probes = [event for event in raw if event['pc'] == 0x37b66]
        assert len(probes) == 1
        probe = probes[0]
        assert probe['pan_redirect'] == (probe['pending_byte'] != 0)
        frame = data['frames'][probe['step']]
        assert frame['pan_feedback'] == [{'pan_redirect': probe['pan_redirect']}]
        directed = [intent for intent in frame['intents'] if intent['kind'] == 'dac-pan']
        ym = [intent for intent in frame['intents']
              if intent['kind'] == 'ym' and intent['bank'] == 1 and intent['register'] == 0xb6]
        if probe['pan_redirect']:
            assert len(directed) == 1 and directed[0]['pan'] == {'left': True, 'right': True}
            assert not ym
            assert any(event['pc'] == 0x37b70 for event in raw)
        else:
            assert not directed and len(ym) == 1 and ym[0]['value'] == 192
        outcomes.append({'case': case['id'], 'step': probe['step'],
                         'pending_byte': probe['pending_byte'], 'pan_redirect': probe['pan_redirect']})
    assert [row['pan_redirect'] for row in outcomes] == [True, True]
    result = {'pass': True, 'steps': 320, 'outcomes': outcomes,
              'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__)),
              'contract': 'External PanRedirect is byte!=0; score DAC blocking is separately high-bit-only.',
              'limits': ['Native guard and routed request only; independent root PCM cadence and Go DSP remain separate.']}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.out.resolve())
