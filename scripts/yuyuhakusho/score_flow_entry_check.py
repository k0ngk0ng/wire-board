"""Test-first logical-entry selection contract; no hidden cycle owner."""
from pathlib import Path
import argparse
import importlib
import json

ROOT = Path(__file__).resolve().parents[2]
EXPECTED = [(136, 6), (136, 7), (149, 1), (149, 2), (149, 5), (149, 6), (149, 7), (149, 8)]


def must_reject(call):
    try:
        call()
    except ValueError:
        return
    raise AssertionError('ambiguous or unknown entry was silently accepted')


def check(input_path, out):
    module = importlib.import_module('score_flow_entry')
    data = json.loads(input_path.read_text())
    result = module.convert(data)
    assert result['production_resource'] is False and len(result['records']) == 39045
    variants = []
    for record in result['records']:
        key = record['key']
        if key['section'] == 'prefix' or key['event_index'] > 0:
            assert len(record['variants']) == 1 and record['variants'][0]['entries'] == ['continue']
            assert module.select(record, 'continue') == record['variants'][0]['work']
            must_reject(lambda: module.select(record, 'cycle-wrap'))
        else:
            must_reject(lambda: module.select(record, 'continue'))
            first, repeat = module.select(record, 'from-prefix'), module.select(record, 'cycle-wrap')
            if first != repeat:
                assert repeat == [{'kind': 'token-read', 'form': 'control'},
                                  {'kind': 'control-dispatch', 'action': 'jump'},
                                  {'kind': 'jump'}] + first
                variants.append((key['score_id'], key['voice_id']))
        must_reject(lambda: module.select(record, 'unknown-entry'))
    assert variants == EXPECTED
    out.write_text(json.dumps({'pass': True, 'records': 39045, 'occurrence_sensitive_cycle_entry_keys': variants,
                               'no_new_persistent_owner': True, 'unknown_entry_rejected': True}, indent=2) + '\n')
    print(out.read_text())


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.input.resolve(), args.out.resolve())
