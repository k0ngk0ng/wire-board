"""Retain the board selector across every captured original action phase."""
import hashlib
import json
from pathlib import Path
from audit_world_boundaries import load

ROOT = Path(__file__).resolve().parents[2]


def main():
    rows, raw = load('world-actions-cpu-long', 'world-actions')
    records = []
    for i, row in enumerate(rows):
        before = raw[i * 4096:i * 4096 + 2048]
        after = raw[i * 4096 + 2048:(i + 1) * 4096]
        records.append(dict(frame=int(row['frame']), before=before[0x6f6], after=after[0x6f6]))
    target = ROOT / 'games/basketball/engine/testdata/action-board-selection.json'
    payload = (json.dumps(records, separators=(',', ':')) + '\n').encode()
    target.write_bytes(payload)
    target.with_name('action-board-selection-source.json').write_text(json.dumps(dict(
        records=len(records), source_is_original_execution=True,
        trace_sha256=hashlib.sha256(raw).hexdigest(), sha256=hashlib.sha256(payload).hexdigest(),
        address='fixed DEA9 writes 06F6 during DE7D falling-hoop entry',
        evidence_id='ev_aa459179a50b5b5cdf7e9449f36c0569739e78a72a02a0cea52805ec5402cce5',
    ), indent=2) + '\n')


if __name__ == '__main__':
    main()
