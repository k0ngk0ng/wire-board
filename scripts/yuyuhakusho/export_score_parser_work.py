"""Promote source-verified, unpriced grammar work to a separate typed resource."""
from pathlib import Path
import argparse
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]
CATALOG_SHA = '735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830'
RESEARCH_SHA = '2eaaa1e20f7e6467656d4e0ae19a119718b55701fc802e8edaaeb06a383da1d5'


def main(out):
    assert out.is_relative_to(ROOT)
    path = ROOT / '.local/md-yuyuhakusho/audio-score-flow/research-sidecar-logical-entry.json'
    assert hashlib.sha256(path.read_bytes()).hexdigest() == RESEARCH_SHA
    source = json.loads(path.read_text())
    catalog_raw = gzip.decompress((ROOT / 'games/yuyuhakusho/data/audio-score.json.gz').read_bytes())
    assert hashlib.sha256(catalog_raw).hexdigest() == CATALOG_SHA == source['catalog_json_sha256']
    assert source['production_resource'] is False and source['unpriced_work'] is True
    native = json.loads((ROOT / '.local/md-yuyuhakusho/audio-score-flow/native-replay.json').read_text())
    assert native['pass']
    library = {'schema': 1, 'format': 'typed-score-parser-work-v1',
               'catalog_json_sha256': CATALOG_SHA, 'coverage': 'parser-structure-only',
               'unpriced': True, 'records': source['records']}
    assert len(library['records']) == 39045
    forbidden = {'pc', 'address', 'opcode', 'target', 'source_bytes', 'cycles', 'timestamp', 'stack', 'raw'}
    def check(value):
        if isinstance(value, dict):
            assert not forbidden & value.keys()
            for child in value.values(): check(child)
        elif isinstance(value, list):
            for child in value: check(child)
    check(library)
    raw = (json.dumps(library, separators=(',', ':')) + '\n').encode()
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(gzip.compress(raw, mtime=0))
    print(json.dumps({'pass': True, 'records': 39045, 'catalog_json_sha256': CATALOG_SHA,
                      'resource_sha256': hashlib.sha256(out.read_bytes()).hexdigest(),
                      'resource_bytes': out.stat().st_size, 'unpriced': True}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    main(args.out.resolve())
