"""Add only source-observed direct PSG restore requests to canonical v2."""
from pathlib import Path
import argparse
import gzip
import json

from reference import ROOT
from score_claim_capture import binding, saved
from score_claim_normalize import canonical_state, build_cursor_maps, load, score_intents
from score_claim_restore_source_check import REQUIRED


def corrected_state(state, maps):
    result = canonical_state(state, maps)
    for voice in result['voices']:
        pan = voice['pan_automation']
        pan['period'], pan['elapsed'] = pan['elapsed'], pan['period']
    return result


def run(previous_index, restore_index, catalog, out):
    previous, observed = load(previous_index), load(restore_index)
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    maps = build_cursor_maps(rom, load(catalog))
    replacements = {case['id']: case for case in observed['cases']}
    cases, updates = [], []
    for case in previous['cases']:
        replacement = replacements.get(case['id'])
        entry = {**case, 'previous_file': case['file']}
        if replacement:
            portable = load(ROOT / replacement['files']['portable']['path'])
            events = load(ROOT / replacement['files']['provenance']['path'])['events']
            direct = [event for event in events if event['pc'] in REQUIRED]
            destination = out / (case['id'] + '.jsonl.gz') if direct else None
            target = gzip.open(destination, 'wt', compresslevel=5) if destination else None
            with gzip.open(ROOT / case['file']['path'], 'rt') as origin:
                first_line = next(origin)
                assert json.loads(first_line)['state'] == corrected_state(portable['initial'], maps)
                if target:
                    target.write(first_line)
                for step, line in enumerate(origin):
                    old, raw = json.loads(line), portable['frames'][step]
                    assert old['state'] == corrected_state(raw['state'], maps)
                    intents, external = score_intents(raw['intents'])
                    assert external == old['external_intents']
                    positions = []
                    for event in direct:
                        if event['step'] == step:
                            order = sum(intent['kind'] != 'pcm-command' for intent in raw['intents'][:event['intent_order']])
                            assert intents[order] == {'order': order, 'kind': 'psg', 'value': event['value']}
                            positions.append(order)
                            updates.append({'case': case['id'], 'step': step, 'order': order, 'value': event['value']})
                    remaining = [intent for intent in intents if intent['order'] not in positions]
                    assert [{**intent, 'order': index} for index, intent in enumerate(remaining)] == old['intents']
                    if target:
                        old['intents'] = intents
                        target.write(json.dumps(old, separators=(',', ':')) + '\n')
                assert step + 1 == case['steps']
            if target:
                target.close()
                entry.update(file=binding(destination), intents=case['intents'] + len(direct))
        cases.append(entry)
    assert len(updates) == 5 and len(cases) == 40
    report = {**previous, 'projection_version': 3,
              'projection_name': 'source-bound-pan-and-direct-PSG-restore-complete',
              'cases': cases, 'restored_psg_requests': updates,
              'previous_index': binding(previous_index), 'observed_restore_index': binding(restore_index),
              'counts': {'frames': sum(case['steps'] for case in cases), 'intents': sum(case['intents'] for case in cases)},
              'source_bindings': previous['source_bindings'] + [binding(Path(__file__).resolve()),
                  binding(ROOT / 'scripts/yuyuhakusho/score_claim_restore_projection_check.py'),
                  binding(ROOT / 'scripts/yuyuhakusho/score_claim_restore_source_check.py')],
              'limits': previous['limits'] + ['Five directly observed PSG restoration writes added; all other state/requests/exclusions unchanged.']}
    saved(out / 'index.json', report)
    print(json.dumps({'pass': True, 'counts': report['counts'], 'restored': updates,
                      'index': binding(out / 'index.json')}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--previous', type=Path, required=True)
    parser.add_argument('--restore', type=Path, required=True)
    parser.add_argument('--catalog', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    assert output.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-claim')
    output.mkdir(parents=True, exist_ok=False)
    run(args.previous.resolve(), args.restore.resolve(), args.catalog.resolve(), output)
