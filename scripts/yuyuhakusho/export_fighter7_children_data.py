#!/usr/bin/env python3
"""Export only immutable semantic AN/shape data used by the proven source7 children."""
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    fixture = ROOT/'games/yuyuhakusho/engine/testdata/fighter7-children-boundaries.json.gz'
    capture = ROOT/'scripts/yuyuhakusho/capture_fighter7_children_private_frozen.py'
    capture_manifest = json.loads((fixture.parent/'fighter7-children-boundaries-manifest.json').read_text())
    assert sha(capture.read_bytes()) == capture_manifest['capture_sha256']
    all_frames = json.loads(gzip.decompress(fixture.read_bytes()))['library']
    selected = [{'key': row['key'], 'data': row['data']} for row in all_frames
                if row['key']['animation_set'] == 0xd92ca and 0x3c <= row['key']['pose'] <= 0x45]
    selected.sort(key=lambda row: (row['key']['pose'], row['key']['index']))
    assert selected and len({json.dumps(row['key'], sort_keys=True) for row in selected}) == len(selected)
    catalog = ROOT/'games/yuyuhakusho/data/animation-catalog.json.gz'
    existing = {json.dumps(row['key'], sort_keys=True): row['data']
                for row in json.loads(gzip.decompress(catalog.read_bytes()))['frames']}
    duplicates = 0
    for row in selected:
        old = existing.get(json.dumps(row['key'], sort_keys=True))
        if old is not None:
            assert old == row['data'], row['key']
            duplicates += 1
    raw = (json.dumps({'frames': selected}, separators=(',', ':'))+'\n').encode()
    output = ROOT/'games/yuyuhakusho/data/fighter7-children-rules.json'
    output.write_bytes(raw)
    report = {'pass': True, 'scope': 'Independent immutable semantic AN/shape records; no CPU/RAM/ROM payload',
              'source_rom_sha256': 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06',
              'capture_sha256': sha(capture.read_bytes()), 'capture_frozen_source': str(capture.relative_to(ROOT)),
              'export_sha256': sha(Path(__file__).read_bytes()), 'fixture_gzip_sha256': sha(fixture.read_bytes()),
              'frames': len(selected), 'matching_existing_frames': duplicates, 'new_frames': len(selected)-duplicates,
              'output_sha256': sha(raw), 'bytes': len(raw),
              'static_sha256': {name: sha((ROOT/'docs/research/md-yuyuhakusho/static'/name).read_bytes())
                                for name in ['fighter7-children-focused.txt', 'fighter7-children-chain.txt', 'fighter7-children-helper-full.txt']}}
    (ROOT/'games/yuyuhakusho/data/fighter7-children-rules-manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
