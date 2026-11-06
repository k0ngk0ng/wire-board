"""Copy PCM intents from the frozen 42 original score traces; no CPU execution."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
INDEX = ROOT / 'games/yuyuhakusho/audio/score/testdata/original-index.json'


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and not out.exists()
    index = json.loads(INDEX.read_text())
    assert len(index['cases']) == 42
    sources, rows = [], []
    counts = {'dac': 0, 'dac-pan': 0}
    for case in index['cases']:
        path = (INDEX.parent / case['path']).resolve()
        assert path.is_relative_to(INDEX.parent)
        packed = path.read_bytes()
        assert sha(packed) == case['sha256'], ('original source changed', path)
        sources.append({'path': str(path.relative_to(ROOT)), 'sha256': case['sha256']})
        for line in gzip.decompress(packed).splitlines():
            frame = json.loads(line)
            for intent in frame.get('intents', []):
                if intent['kind'] in counts:
                    counts[intent['kind']] += 1
                    rows.append({'case': case['id'], 'step': frame['step'], 'intent': intent})
    assert counts == {'dac': 9230, 'dac-pan': 2}
    document = {'version': 1, 'scope': 'Literal PCM-only intents from original score traces, no new timing or feedback oracle',
                'index': {'path': str(INDEX.relative_to(ROOT)), 'sha256': sha(INDEX.read_bytes())},
                'sources': sources, 'counts': counts, 'rows': rows}
    raw = (json.dumps(document, sort_keys=True, separators=(',', ':')) + '\n').encode()
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(gzip.compress(raw, compresslevel=9, mtime=0))
    print(json.dumps({'cases': len(sources), 'counts': counts, 'raw_bytes': len(raw), 'gzip_bytes': out.stat().st_size,
                      'raw_sha256': sha(raw), 'gzip_sha256': sha(out.read_bytes())}))


if __name__ == '__main__':
    main()
