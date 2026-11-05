"""Test-first acceptance for mixed native score claim/restore observations."""
from pathlib import Path
import argparse
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]
ISOLATED_STEPS = [23, 75, 25, 31, 94, 72, 26, 56, 46, 107, 46, 102]


def cases():
    result = []
    for code, steps in zip(range(160, 172), ISOLATED_STEPS):
        result.append({'id': 'music82-' + format(code, '02x'),
                       'steps': 40 + steps + 24,
                       'commands': [{'step': 0, 'code': 130}, {'step': 40, 'code': code}],
                       'headers': [130, code]})
    result += [
        {'id': 'higher-priority-replaces', 'steps': 160,
         'commands': [{'step': 0, 'code': 130}, {'step': 40, 'code': 164}, {'step': 44, 'code': 161}],
         'headers': [130, 164, 161]},
        {'id': 'lower-priority-rejected', 'steps': 160,
         'commands': [{'step': 0, 'code': 130}, {'step': 40, 'code': 161}, {'step': 44, 'code': 160}],
         'headers': [130, 161]},
        {'id': 'music-replaces-primary', 'steps': 160,
         'commands': [{'step': 0, 'code': 130}, {'step': 40, 'code': 164}, {'step': 48, 'code': 131}],
         'headers': [130, 164, 131]},
    ]
    return result


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def clean(value):
    if isinstance(value, dict):
        forbidden = {'pc', 'address', 'row', 'row_hex', 'driver_hex', 'raw',
                     'bank_address', 'source', 'opcode', 'loop_stack', 'stack', 'pointer'}
        assert not (forbidden & value.keys()), value.keys()
        for child in value.values():
            clean(child)
    elif isinstance(value, list):
        for child in value:
            clean(child)


def check(index, out):
    report = load(index)
    assert report['schema'] == 1 and report['format'] == 'native-score-claim-oracle-v1'
    assert report['pass'] and not report['go_driver_verified']
    assert not report['dsp_verified'] and report['cases_requested'] == cases()
    assert report['rom_sha256'] == 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
    assert len(report['cases']) == 15
    totals = {'steps': 0, 'ym': 0, 'psg': 0, 'dac': 0, 'feedback': 0, 'routing': 0}
    evidence = {}
    for expected, case in zip(cases(), report['cases']):
        assert case['id'] == expected['id'] and case['steps'] == expected['steps']
        assert case['original_equal'] and case['pcm_samples_equal']
        for binding in case['files'].values():
            path = (ROOT / binding['path']).resolve()
            assert path.is_relative_to(ROOT) and sha(path) == binding['sha256']
        portable = load(ROOT / case['files']['portable']['path'])
        clean(portable)
        assert portable['commands'] == expected['commands']
        assert portable['headers'] == expected['headers']
        assert len(portable['frames']) == expected['steps']
        assert [frame['step'] for frame in portable['frames']] == list(range(expected['steps']))
        assert len(portable['initial']['voices']) == 18
        assert all(len(frame['state']['voices']) == 18 for frame in portable['frames'])
        for frame in portable['frames']:
            assert frame['stereo_frames'] in (735, 736)
            assert [event['order'] for event in frame['intents']] == list(range(len(frame['intents'])))
            for intent in frame['intents']:
                assert intent['kind'] in ('ym', 'psg', 'dac', 'queue', 'pcm-command')
                if intent['kind'] == 'ym':
                    assert 0 <= intent['bank'] <= 1 and 0 <= intent['register'] <= 255 and 0 <= intent['value'] <= 255
                    totals['ym'] += 1
                elif intent['kind'] == 'psg':
                    assert 0 <= intent['value'] <= 255
                    totals['psg'] += 1
                elif intent['kind'] == 'dac':
                    assert intent['descriptor_id'].startswith('descriptor-')
                    totals['dac'] += 1
                elif intent['kind'] == 'queue':
                    assert intent == {'kind': 'queue', 'order': intent['order'], 'action': 'replace-first', 'sound_id': 125}
            for feedback in frame['dac_feedback']:
                assert isinstance(feedback['blocks_score_dac'], bool)
                assert feedback['mode_priority'] in (0, 64, 128, 192)
                totals['feedback'] += 1
            totals['routing'] += len(frame['routing'])
        evidence[case['id']] = portable
        totals['steps'] += expected['steps']
    assert totals['steps'] == 1951 and totals['ym'] > 0 and totals['psg'] > 0 and totals['dac'] > 0
    assert totals['feedback'] >= totals['dac'] and totals['routing'] >= 30
    mixed = evidence['music82-a1']['frames']
    assert any(any(v['active'] and v['claimed'] and v['domain'] == 'music' for v in frame['state']['voices']) for frame in mixed[40:90])
    assert all(not v['claimed'] for v in mixed[-1]['state']['voices'] if v['domain'] == 'music')
    ab = evidence['music82-ab']['frames']
    assert sum(intent['kind'] == 'queue' for frame in ab for intent in frame['intents']) == 1
    assert any(intent['kind'] == 'pcm-command' and intent['sound_id'] == 125 for frame in ab for intent in frame['intents'])
    result = {'pass': True, 'scope': 'original/observer equality and mixed typed oracle completeness; not independent driver equality',
              'totals': totals, 'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__)),
              'limits': report['limits']}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.out.resolve())
