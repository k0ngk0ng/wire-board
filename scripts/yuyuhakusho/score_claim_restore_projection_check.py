"""Freeze the exact five missing PSG writes before building canonical v3."""
from pathlib import Path
import argparse
import gzip
import json
from score_claim_check import ROOT, load, sha

EXPECTED = [('music82-a1', 94, 7), ('music82-a7', 80, 39), ('music82-ab', 46, 14),
            ('higher-priority-replaces', 98, 7), ('lower-priority-rejected', 94, 7)]


def check(index, out):
    report = load(index)
    assert report['projection_version'] == 3 and len(report['cases']) == 40
    found = [(row['case'], row['step'], row['order']) for row in report['restored_psg_requests']]
    assert found == EXPECTED
    for case in report['cases']:
        path, previous = ROOT / case['file']['path'], ROOT / case['previous_file']['path']
        assert sha(path) == case['file']['sha256'] and sha(previous) == case['previous_file']['sha256']
        if case['file']['sha256'] == case['previous_file']['sha256']:
            continue
        with gzip.open(path, 'rt') as after, gzip.open(previous, 'rt') as before:
            assert next(after) == next(before)
            for step, (new_line, old_line) in enumerate(zip(after, before)):
                new, old = json.loads(new_line), json.loads(old_line)
                orders = [order for identity, at, order in EXPECTED if identity == case['id'] and at == step]
                removed = [event for event in new['intents'] if event['order'] in orders]
                assert removed == [{'order': order, 'kind': 'psg', 'value': 231} for order in orders]
                remaining = [event for event in new['intents'] if event['order'] not in orders]
                new['intents'] = [{**event, 'order': order} for order, event in enumerate(remaining)]
                assert new == old, (case['id'], step, 'Unrelated field or request changed')
    assert report['counts'] == {'frames': 125821, 'intents': 1499361}
    result = {'pass': True, 'cases': 40, 'frames': 125821, 'intents': 1499361,
              'only_added_direct_psg_restore_writes': 5, 'all_other_fields_and_requests_unchanged': True,
              'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__))}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--legacy', action='store_true')
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path)
    args = parser.parse_args()
    if args.legacy:
        report = load(args.index.resolve())
        case = next(row for row in report['cases'] if row['id'] == 'music82-a1')
        with gzip.open(ROOT / case['file']['path'], 'rt') as stream:
            next(stream)
            frame = next(json.loads(line) for line in stream if json.loads(line)['step'] == 94)
        assert any(event['kind'] == 'psg' and event['value'] == 231 for event in frame['intents']), 'Missing source39186 noise restore at music82-a1 step94'
    else:
        check(args.index.resolve(), args.out.resolve())
