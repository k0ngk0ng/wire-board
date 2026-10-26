#!/usr/bin/env python3
"""Original emulator E2E acceptance, written before the reusable frontend."""
import argparse
import hashlib
import json
from pathlib import Path

from reference import ReferenceSession, buttons_at


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    root = Path(__file__).resolve().parents[2]
    out = args.out.resolve()
    assert out.is_relative_to(root)
    out.mkdir(parents=True, exist_ok=True)
    events = json.loads((root/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    baseline = json.loads((root/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows = []
    with ReferenceSession(root, out) as ref:
        for frame in range(1, 3601):
            rows.append(ref.step(frame, buttons_at(events, frame)))
            if frame == 2400:
                checkpoint = ref.save()
        cold = [row['reference'] for row in rows]
        assert cold == baseline, 'Reusable frontend changed original coldboot execution'
        observations = {}
        for restore_refresh in (False, True):
            ref.restore(checkpoint, restore_refresh_cycles=restore_refresh)
            replay = [ref.step(f, buttons_at(events, f)) for f in range(2401, 3601)]
            before = rows[2400:]
            differences = {key: [a['reference']['frame'] for a, b in zip(before, replay) if a[key] != b[key]]
                           for key in ('ram', 'video', 'registers')}
            observations[str(restore_refresh)] = {k: {'count': len(v), 'first_frames': v[:20]} for k, v in differences.items()}
        report = {'scope': 'Original ROM coldboot and complete-RAM/video/register replay',
                  'identity': ref.identity, 'frames': 3600, 'replayed_frames': 1200,
                  'coldboot_matches_historical_baseline': True,
                  'restore_refresh_cycles': observations,
                  'checkpoint_refresh_cycles': checkpoint['refresh_cycles'],
                  'pass': all(x['count'] == 0 for x in observations['True'].values())}
        (out/'report.json').write_text(json.dumps(report, indent=2)+'\n')
        (out/'frame-hashes.json').write_text(json.dumps(cold, separators=(',', ':'))+'\n')
    print(json.dumps(report, indent=2))
    assert report['pass'], 'Restore diagnostic still diverges; no RAM ranges may be ignored'


if __name__ == '__main__':
    main()
