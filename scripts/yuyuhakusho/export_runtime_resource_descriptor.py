#!/usr/bin/env python3
"""Export identity-only paths/sizes/hashes; never include client resource bodies."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT/'games/yuyuhakusho/data'
RESOURCES = [
    ('rules', 'chu-world-rules'), ('sprites', 'world-sprites'), ('hud', 'hud-assets'),
    ('render_meta', 'renderstate-metadata'), ('arena-default', 'arena-assets-default'),
    *[(f'arena-{arena}', f'arena-assets-{arena}') for arena in ('2', '4', '6', '8', 'a', 'c', 'e')],
    ('arena-a-aux', 'arena-a-aux-assets'),
]


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='verify deterministic bytes without writing')
    args = parser.parse_args()
    records = []
    for identity, name in RESOURCES:
        packed = (DATA/(name+'.json.gz')).read_bytes()
        raw = gzip.decompress(packed)
        json.loads(raw)
        records.append({'id': identity, 'logical_path': f'/yuyuhakusho/data/{name}.json.gz',
                        'raw_json_sha256': sha(raw), 'raw_json_bytes': len(raw),
                        'gzip_sha256': sha(packed), 'gzip_bytes': len(packed)})
    assert len(records) == len({row['id'] for row in records}) == len({row['logical_path'] for row in records}) == 13
    raw = (json.dumps({'schema': 1, 'resources': records}, separators=(',', ':'))+'\n').encode()
    target = DATA/'runtime-resource-descriptor.json'
    if args.check:
        if target.read_bytes() != raw:
            raise ValueError('Resource descriptor differs from current exact input bytes')
    else:
        target.write_bytes(raw)
    report = {'schema': 1, 'pass': True, 'scope': 'Identity-only resource paths, exact raw-JSON/gzip SHA-256 and sizes; no body or client media',
              'resources': len(records), 'bytes': len(raw), 'sha256': sha(raw),
              'logical_path_prefix': '/yuyuhakusho/data/', 'exporter_sha256': sha(Path(__file__).read_bytes())}
    if not args.check:
        (DATA/'runtime-resource-descriptor-manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
