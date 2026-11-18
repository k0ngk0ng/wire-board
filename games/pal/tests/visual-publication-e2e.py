import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile

ROOT = Path(__file__).resolve().parents[3]
MEDIA = ROOT / '.local/pal/visual/pal/v1'
spec = importlib.util.spec_from_file_location('pal_visual_publication', ROOT / 'games/pal/assets/publish-visuals.py')
publication = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publication)
raw = (MEDIA / 'index.json').read_bytes()
index = json.loads(raw)
proof = json.loads((ROOT / '.local/pal/verification/visual-export-isolation.json').read_text())
pin, rows = publication.plan(raw, MEDIA, proof, {'version': 1, 'catalogs': {}})
assert pin['logicalFiles'] == 3485 and len(rows) == 3485
assert all(row['url'].startswith('https://cdn.ichenj.com/bga/') for row in rows)
release = {'version': 1, 'catalogs': {pin['assetID']: {**pin, 'cdnVerified': True}}}
repeat, _ = publication.plan(raw, MEDIA, proof, release)
assert repeat['files'] == pin['files'] and repeat['newObjects'] == repeat['uploadBytes'] == 0
rejected = []


def reject(name, operation):
    try:
        operation()
    except (ValueError, KeyError, FileNotFoundError):
        rejected.append(name)
        return
    raise AssertionError('invalid visual publication accepted: ' + name)


reject('unverified-export', lambda: publication.plan(raw, MEDIA, {**proof, 'passed': False}, release))
reject('changed-index-proof', lambda: publication.plan(raw, MEDIA, {**proof, 'indexSHA256': '0' * 64}, release))
bad = copy.deepcopy(index)
bad['sources']['map.mkf'] = '0' * 64
reject('changed-original-source', lambda: publication.plan(json.dumps(bad).encode(), MEDIA, {**proof, 'indexSHA256': hashlib.sha256(json.dumps(bad).encode()).hexdigest()}, release))
bad = copy.deepcopy(index)
bad['files'].pop('fonts.json')
reject('missing-font-registration', lambda: publication.plan(json.dumps(bad).encode(), MEDIA, {**proof, 'indexSHA256': hashlib.sha256(json.dumps(bad).encode()).hexdigest()}, release))
with tempfile.TemporaryDirectory(prefix='visual-publication-', dir=ROOT / '.local/pal') as temporary:
    folder = Path(temporary)
    for name in index['files']:
        target = folder / name
        target.parent.mkdir(parents=True, exist_ok=True)
        os.link(MEDIA / name, target)
    variant = copy.deepcopy(index)
    variant['publicationFixture'] = 'new-build-with-the-same-client-media'
    changed = (json.dumps(variant, sort_keys=True) + '\n').encode()
    (folder / 'index.json').write_bytes(changed)
    variant_proof = {**proof, 'indexSHA256': hashlib.sha256(changed).hexdigest()}
    metadata, _ = publication.plan(changed, folder, variant_proof, release)
    assert metadata['newObjects'] == 1
    assert all(metadata['files'][name]['url'] == pin['files'][name]['url'] for name in index['files'])
    target = folder / 'fonts/license.txt'
    target.unlink()
    target.write_bytes((MEDIA / 'fonts/license.txt').read_bytes() + b'\n')
    reject('changed-unlisted-bytes', lambda: publication.plan(changed, folder, variant_proof, release))
    variant['files']['fonts/license.txt'].update(size=target.stat().st_size, sha256=hashlib.sha256(target.read_bytes()).hexdigest())
    changed = (json.dumps(variant, sort_keys=True) + '\n').encode()
    (folder / 'index.json').write_bytes(changed)
    variant_proof['indexSHA256'] = hashlib.sha256(changed).hexdigest()
    single, _ = publication.plan(changed, folder, variant_proof, release)
    assert single['newObjects'] == 2
    assert all(single['files'][name]['url'] == pin['files'][name]['url'] for name in index['files'] if name != 'fonts/license.txt')
    target.unlink()
    target.symlink_to(MEDIA / 'fonts/license.txt')
    reject('symlink-object', lambda: publication.plan(changed, folder, variant_proof, release))
assert (MEDIA / 'index.json').read_bytes() == raw
report = {'passed': True, 'scope': 'offline full original visual/font inventory, exact repeat zero-upload, metadata-only and one real text-file change URL retention, source/corrupt/font/path rejection; no variant upload', 'logicalFiles': len(rows), 'uniqueObjects': pin['objects'], 'repeatNewObjects': repeat['newObjects'], 'repeatUploadBytes': repeat['uploadBytes'], 'metadataOnlyNewObjects': metadata['newObjects'], 'singleChangedFileNewObjects': single['newObjects'], 'rejected': rejected, 'remoteWrites': 0}
(ROOT / '.local/pal/verification/visual-publication-e2e.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
