"""Correct only pan period/elapsed names in the frozen canonical observations."""
from pathlib import Path
import argparse
import copy
import gzip
import json

from reference import ROOT
from score_claim_capture import binding, saved
from score_claim_normalize import EXCLUSIONS, load


def run(indices, out):
    cases = []
    for index in indices:
        report = load(index)
        assert report['pass'] and report['exclusions'] == EXCLUSIONS
        for case in report['cases']:
            source = ROOT / case['file']['path']
            destination = out / source.name
            count = 0
            with gzip.open(source, 'rt') as origin, gzip.open(destination, 'wt', compresslevel=5) as target:
                for line in origin:
                    packet = json.loads(line)
                    for voice in packet['state']['voices']:
                        pan = voice['pan_automation']
                        pan['period'], pan['elapsed'] = pan['elapsed'], pan['period']
                    target.write(json.dumps(packet, separators=(',', ':')) + '\n')
                    count += packet['kind'] == 'frame'
            assert count == case['steps']
            cases.append({**case, 'file': binding(destination), 'previous_file': case['file']})
            print(json.dumps({'projected': case['id'], 'steps': count}), flush=True)
    assert len(cases) == 40
    result = {'schema': 1, 'format': 'canonical-score-observables-v1', 'projection_version': 2,
              'projection_name': 'source-bound-pan-period-elapsed-correction', 'pass': True,
              'independent_driver_verified': False, 'exclusions_frozen': True,
              'cursor_maps_verified_against_typed_tables': True, 'exclusions': EXCLUSIONS,
              'cases': cases, 'counts': {'frames': sum(case['steps'] for case in cases),
                                       'intents': sum(case['intents'] for case in cases)},
              'previous_indices': [binding(path) for path in indices],
              'source_bindings': [binding(Path(__file__).resolve()),
                                  binding(ROOT / 'scripts/yuyuhakusho/score_claim_pan_projection_check.py'),
                                  binding(ROOT / '.local/md-yuyuhakusho/audio-score-parameters/parameter-instructions.txt')],
              'limits': ['Only oracle projection corrected. No original program was rerun or existing v1 evidence overwritten.',
                         'All unrelated fields, intents and fixed exclusions must remain identical to v1.',
                         'Independent Go driver and DSP equality remain separate.']}
    saved(out / 'index.json', result)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, action='append', required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    assert output.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-claim')
    output.mkdir(parents=True, exist_ok=False)
    run([path.resolve() for path in args.index], output)
