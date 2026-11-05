"""Test-first native proof for hidden FM patch selection versus loaded cache."""
from pathlib import Path
import argparse
import json
from score_claim_check import ROOT, clean, load, sha


def check(index, out):
    report = load(index)
    assert report['pass'] and len(report['cases']) == 1
    case = report['cases'][0]
    assert case['id'] == 'music82-hidden-patch' and case['steps'] == 900
    assert case['original_equal'] and case['pcm_samples_equal']
    path = ROOT / case['files']['portable']['path']
    assert sha(path) == case['files']['portable']['sha256']
    data = load(path)
    clean(data)
    assert data['commands'] == [{'step': 0, 'code': 130}, {'step': 776, 'code': 164}]
    assert data['headers'] == [130, 164]
    fm = [next(v for v in frame['state']['voices'] if v['slot'] == 'music.fm5') for frame in data['frames']]
    assert not fm[775]['claimed'] and fm[776]['claimed'] and fm[781]['claimed']
    assert fm[780]['tone'] != fm[781]['tone']
    assert fm[780]['loaded_algorithm'] == fm[781]['loaded_algorithm'] == 2
    assert fm[780]['loaded_feedback'] == fm[781]['loaded_feedback'] == 7
    assert not fm[854]['claimed'] and fm[854]['note_off']
    assert fm[854]['tone'] == fm[781]['tone'] and fm[854]['loaded_algorithm'] == 4
    assert fm[854]['loaded_feedback'] == 0
    provenance = load(ROOT / case['files']['provenance']['path'])['events']
    patch_reads = [(e['step'], e['row']) for e in provenance if e['pc'] == 0x38f78]
    assert not any(776 <= step < 854 and row == 0xf930 for step, row in patch_reads)
    assert (854, 0xf930) in patch_reads
    result = {'pass': True, 'scope': 'hidden selection/cache separation and current-patch forced-rest restore',
              'steps': 900, 'selection_step': 781, 'restore_step': 854,
              'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__)),
              'limits': ['Native oracle only; independent score driver equality remains separate.']}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.out.resolve())
