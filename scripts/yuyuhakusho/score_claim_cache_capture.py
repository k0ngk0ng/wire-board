"""Repeat the declared native hidden-patch/cache boundary without new inputs."""
from pathlib import Path
import argparse
import score_claim_capture
from reference import ROOT


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    assert output.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-claim')
    output.mkdir(parents=True, exist_ok=False)
    score_claim_capture.cases = lambda: [
        {'id': 'music82-hidden-patch', 'steps': 900,
         'commands': [{'step': 0, 'code': 130}, {'step': 776, 'code': 164}],
         'headers': [130, 164]}]
    score_claim_capture.run(output)
