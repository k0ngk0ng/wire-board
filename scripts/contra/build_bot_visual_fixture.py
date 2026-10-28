#!/usr/bin/env python3
"""Freeze already normalized, SHA-verified browser graphics for portable bot E2E."""
import base64, gzip, hashlib, json
from pathlib import Path
root = Path(__file__).resolve().parents[2]
public = root / '.local/contra-browser/build/public'
raw = (public / 'manifest.json').read_bytes(); manifest = json.loads(raw)
assert manifest['romSHA256'] == '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
def read(entry, expected):
    path = public / entry['url'].lstrip('/')
    assert path.resolve().is_relative_to(public.resolve())
    data = path.read_bytes()
    assert entry['bytes'] == len(data) == expected
    assert hashlib.sha256(data).hexdigest() == entry['sha256']
    if expected == 4096: assert max(data) <= 3
    return base64.b64encode(data).decode()
pages = {int(key): entry for key, entry in manifest['pages'].items()}
assert pages and all(0 <= key < 128 for key in pages)
fixture = {'schema': 1, 'romSHA256': manifest['romSHA256'], 'sourceManifestSHA256': hashlib.sha256(raw).hexdigest(), 'catalog': {'rgb': {k: manifest['rgb'][k] for k in ['bytes', 'sha256']}, 'pages': {key: {k: entry[k] for k in ['bytes', 'sha256']} for key, entry in pages.items()}}, 'rgb': read(manifest['rgb'], 192), 'pages': {key: read(entry, 4096) for key, entry in pages.items()}}
out = root / 'games/contra/engine/testdata/bot-visual-catalog.json.gz'
out.write_bytes(gzip.compress(json.dumps(fixture, sort_keys=True, separators=(',', ':')).encode(), mtime=0))
print(json.dumps({'pages': len(pages), 'path': str(out.relative_to(root)), 'bytes': out.stat().st_size, 'sha256': hashlib.sha256(out.read_bytes()).hexdigest()}))
