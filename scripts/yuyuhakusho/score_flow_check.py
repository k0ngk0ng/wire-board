"""First contracts for address-free parser work and ordered parameter reads."""
from pathlib import Path
import argparse
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]
CATALOG_JSON_SHA = '735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830'


def validate_order(event, work):
    roles = [step['role'] for step in work if step['kind'] == 'parameter-read']
    if event['kind'] == 'modulation':
        assert roles == ['modulation-delay', 'modulation-rate', 'modulation-delta', 'modulation-steps'], roles
    elif event['kind'] == 'pan-automation':
        expected = ['pan-mode']
        if event['definition']['mode'] != 'disabled':
            expected += ['pan-pattern', 'pan-start-step', 'pan-limit', 'pan-period', 'pan-elapsed-seed']
        assert roles == expected, roles


def no_source(value):
    if isinstance(value, dict):
        banned = {'pc', 'address', 'offset', 'source', 'raw', 'bytes', 'opcode', 'target',
                  'stack', 'stack_pointer', 'counter_bytes', 'cycles', 'step', 'frame'}
        assert not banned & value.keys(), value.keys()
        for child in value.values():
            no_source(child)
    elif isinstance(value, list):
        for child in value:
            no_source(child)


def check(path):
    data = json.loads(path.read_text())
    assert data['catalog_json_sha256'] == CATALOG_JSON_SHA
    assert data['production_resource'] is False
    no_source(data['records'])
    catalog_path = ROOT / 'games/yuyuhakusho/data/audio-score.json.gz'
    raw = gzip.decompress(catalog_path.read_bytes())
    assert hashlib.sha256(raw).hexdigest() == CATALOG_JSON_SHA
    catalog = json.loads(raw)
    expected = {(score['id'], voice['id'], section, index): event
                for score in catalog['scores'] for voice in score['voices']
                for section in ('prefix', 'cycle')
                for index, event in enumerate(voice['timeline'][section])}
    actual = {}
    for record in data['records']:
        key = record['key']
        identity = (key['score_id'], key['voice_id'], key['section'], key['event_index'])
        assert identity not in actual
        actual[identity] = record
        assert identity in expected
        for variant in record['variants']:
            validate_order(expected[identity], variant['work'])
    assert actual.keys() == expected.keys()
    return {'pass': True, 'logical_events': len(expected),
            'catalog_json_sha256': CATALOG_JSON_SHA,
            'scope': 'Research work variants and parameter order only; no production sidecar or priced clock claim.'}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sidecar', type=Path)
    parser.add_argument('--wrong-order', choices=('modulation', 'pan'))
    args = parser.parse_args()
    if args.wrong_order == 'modulation':
        validate_order({'kind': 'modulation'}, [
            {'kind': 'parameter-read', 'role': role}
            for role in ('modulation-delay', 'modulation-delta', 'modulation-rate', 'modulation-steps')])
    elif args.wrong_order == 'pan':
        validate_order({'kind': 'pan-automation', 'definition': {'mode': 'advance-on-note'}}, [
            {'kind': 'parameter-read', 'role': role}
            for role in ('pan-mode', 'pan-pattern', 'pan-start-step', 'pan-limit', 'pan-elapsed-seed', 'pan-period')])
    else:
        print(json.dumps(check(args.sidecar.resolve()), indent=2))
