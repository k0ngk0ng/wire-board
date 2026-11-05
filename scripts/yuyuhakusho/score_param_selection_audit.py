"""Read-only F5-to-next-note boundary audit; no production cursor state."""
from pathlib import Path
import collections
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    index_path = ROOT / '.local/md-yuyuhakusho/audio-score-source/full/index.json'
    index = json.loads(index_path.read_bytes())
    counts = collections.Counter()
    examples = []
    for case in index['cases']:
        path = ROOT / case['files']['events']['path']
        assert digest(path) == case['files']['events']['sha256']
        events = json.loads(gzip.decompress(path.read_bytes()))['events']
        pending = {}
        for event in events:
            kind, row = event['kind'], event.get('row')
            if kind == 'control-before' and event['command'] == 0xf5:
                state = bytes.fromhex(event['row_hex'])
                pending[row] = (state[11], bytes.fromhex(event['read_hex'])[0])
                counts['volume_selection_events'] += 1
            elif kind == 'note-outcome' and row in pending:
                old, new = pending.pop(row)
                state = bytes.fromhex(event['row_hex'])
                counts['next_note_outcomes'] += 1
                if state[0] & 16:
                    counts['selection_then_tied_note'] += 1
                    if old != new:
                        counts['tied_note_changes_definition'] += 1
                        if state[12]:
                            counts['changed_definition_retains_nonzero_position'] += 1
                            if len(examples) < 12:
                                examples.append({'code': case['code'], 'step': event['step'],
                                                 'row': row, 'old_envelope': old,
                                                 'new_envelope': new,
                                                 'source_position': state[12]})
            elif kind == 'track-stop':
                pending.pop(row, None)
        assert not pending, ('F5 has no observed next note', case['code'], pending)
    report = {'scope': 'Read-only original F5 selection and next-note tie audit',
              'counts': dict(counts), 'examples': examples,
              'index_sha256': digest(index_path),
              'checker_sha256': digest(Path(__file__).resolve()),
              'limits': ['Research source positions are not added to production Go state.',
                         'Unreached selection/tie combinations are not established by this audit.']}
    target = ROOT / 'games/yuyuhakusho/verification/audio-score-param/selection-audit.json'
    target.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
