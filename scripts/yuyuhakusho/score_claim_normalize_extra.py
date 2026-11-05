"""Apply the frozen v1 normalizer to an additional native mixed case index."""
from pathlib import Path
import argparse
from reference import ROOT
from score_claim_capture import binding, saved
from score_claim_normalize import EXCLUSIONS, build_cursor_maps, load, write_mixed


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--catalog', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    assert output.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-claim')
    output.mkdir(parents=True, exist_ok=False)
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    maps = build_cursor_maps(rom, load(args.catalog.resolve()))
    cases = write_mixed(args.input.resolve(), output, maps)
    saved(output / 'index.json', {'schema': 1, 'format': 'canonical-score-observables-v1',
          'pass': True, 'independent_driver_verified': False, 'cursor_maps_verified_against_typed_tables': True,
          'exclusions': EXCLUSIONS, 'cases': cases,
          'counts': {'frames': sum(case['steps'] for case in cases), 'intents': sum(case['intents'] for case in cases)},
          'inputs': [binding(args.input.resolve()), binding(args.catalog.resolve())],
          'sources': [binding(Path(__file__).resolve()), binding(ROOT / 'scripts/yuyuhakusho/score_claim_normalize.py')],
          'limits': ['Frozen v1 projection; use project_v2 before final driver acceptance.']})
