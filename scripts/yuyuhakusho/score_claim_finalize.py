"""Append source-observed DAC pan feedback cases to the frozen40 v3 cases."""
from pathlib import Path
import argparse
import gzip
import itertools
import json

from reference import ROOT
from score_claim_capture import binding, saved
from score_claim_normalize import build_cursor_maps, dispatch_and_clear, load, score_intents
from score_claim_project_v3 import corrected_state


def run(previous_index, pan_index, catalog, out):
    previous, pan_report = load(previous_index), load(pan_index)
    assert len(previous['cases']) == 40 and len(pan_report['cases']) == 2
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    maps = build_cursor_maps(rom, load(catalog))
    cases = list(previous['cases'])
    for case in pan_report['cases']:
        portable = load(ROOT / case['files']['portable']['path'])
        events = load(ROOT / case['files']['provenance']['path'])['events']
        by_step = {step: list(group) for step, group in itertools.groupby(events, key=lambda event: event['step'])}
        path = out / (case['id'] + '.jsonl.gz')
        count = 0
        with gzip.open(path, 'wt', compresslevel=5) as stream:
            initial = {'kind': 'initial', 'id': case['id'], 'commands': portable['commands'],
                       'state': corrected_state(portable['initial'], maps)}
            stream.write(json.dumps(initial, separators=(',', ':')) + '\n')
            for frame in portable['frames']:
                dispatch, clear = dispatch_and_clear(by_step.get(frame['step'], []))
                intents, external = score_intents(frame['intents'])
                result = {'kind': 'frame', **frame, 'dispatch_sound_id': dispatch,
                          'clear_primary_priority': clear, 'state': corrected_state(frame['state'], maps),
                          'intents': intents, 'external_intents': external}
                stream.write(json.dumps(result, separators=(',', ':')) + '\n')
                count += len(intents)
        cases.append({'id': case['id'], 'steps': case['steps'], 'intents': count,
                      'file': binding(path), 'input_index': binding(pan_index)})
    report = {**previous, 'cases': cases, 'previous_index': binding(previous_index),
              'pan_feedback_index': binding(pan_index),
              'counts': {'frames': sum(case['steps'] for case in cases),
                         'intents': sum(case['intents'] for case in cases)},
              'source_bindings': previous['source_bindings'] + [binding(Path(__file__).resolve()),
                  binding(ROOT / 'scripts/yuyuhakusho/score_claim_final_check.py'),
                  binding(ROOT / 'scripts/yuyuhakusho/score_claim_pan_feedback_source_check.py')],
              'limits': previous['limits'] + ['PanRedirect is external byte-nonzero feedback, independent from high-bit DAC blocking; it is not score-owned state.']}
    saved(out / 'index.json', report)
    print(json.dumps({'pass': True, 'cases': len(cases), 'counts': report['counts'],
                      'index': binding(out / 'index.json')}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--previous', type=Path, required=True)
    parser.add_argument('--pan', type=Path, required=True)
    parser.add_argument('--catalog', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    assert output.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-claim')
    output.mkdir(parents=True, exist_ok=False)
    run(args.previous.resolve(), args.pan.resolve(), args.catalog.resolve(), output)
