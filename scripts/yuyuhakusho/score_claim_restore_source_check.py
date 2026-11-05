"""Source-bound missing-hook regression for direct PSG noise restoration."""
from pathlib import Path
import argparse
import collections
import json
from score_claim_check import ROOT, cases, load, sha
from score_claim_capture import OBSERVATION_POINTS

REQUIRED = [0x39186, 0x3889a, 0x3890c]
EXPECTED = [('music82-a1', 94, 231), ('music82-a7', 80, 231), ('music82-ab', 46, 231),
            ('higher-priority-replaces', 98, 231), ('lower-priority-rejected', 94, 231)]


def check(index, out):
    report = load(index)
    assert set(REQUIRED).issubset(report['observation_points'])
    assert len(report['observation_points']) <= 64 and report['pass']
    assert report['cases_requested'] == cases() and len(report['cases']) == 15
    found = []
    for case in report['cases']:
        assert case['original_equal'] and case['pcm_samples_equal']
        events = load(ROOT / case['files']['provenance']['path'])['events']
        before = load(ROOT / case['previous_files']['provenance']['path'])['events']
        assert [event for event in events if event['pc'] not in REQUIRED] == before
        for event in events:
            if event['pc'] in REQUIRED:
                assert event['kind'] == 'psg-request'
                assert event['value'] == bytes.fromhex(event['restored_row_hex'])[37]
                found.append((case['id'], event['step'], event['value']))
    assert found == EXPECTED
    result = {'pass': True, 'source_points': REQUIRED, 'new_direct_psg_requests': found,
              'old_observed_events_unchanged': True, 'index_sha256': sha(index),
              'checker_sha256': sha(Path(__file__)),
              'limits': ['Observed mixed coverage only; isolated non-reachability is independently bounded from original state/path evidence.']}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--legacy', action='store_true')
    parser.add_argument('--index', type=Path)
    parser.add_argument('--out', type=Path)
    args = parser.parse_args()
    if args.legacy:
        assert set(REQUIRED).issubset(OBSERVATION_POINTS), ('Missing direct PSG restore observations', REQUIRED)
    else:
        check(args.index.resolve(), args.out.resolve())
