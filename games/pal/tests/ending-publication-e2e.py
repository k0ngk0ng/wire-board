import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile

ROOT = Path(__file__).resolve().parents[3]
MEDIA = ROOT / '.local/pal/ending-media-e2e'
spec = importlib.util.spec_from_file_location('pal_ending_publication', ROOT / 'games/pal/assets/publish-ending.py')
publication = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publication)
raw = (MEDIA / 'index.json').read_bytes()
index = json.loads(raw)
proof = json.loads((ROOT / '.local/pal/verification/ending-media-export.json').read_text())
retained = {'version': 1, 'catalogs': {}}
pin, rows = publication.plan(raw, MEDIA, proof, retained)
assert pin['logicalFiles'] == 321 and pin['audioObjects'] == 3
assert len([row for row in rows if row['path'].endswith('.rgb.gz')]) == 314
assert all(row['url'].startswith('https://cdn.ichenj.com/bga/') for row in rows)
release = {'version': 1, 'catalogs': {pin['assetID']: {**pin, 'cdnVerified': True}}}
repeat, _ = publication.plan(raw, MEDIA, proof, release)
assert repeat['newObjects'] == repeat['uploadBytes'] == 0
assert repeat['files'] == pin['files']
rejected = []


def reject(name, check):
    try:
        check()
    except (ValueError, KeyError, FileNotFoundError):
        rejected.append(name)
        return
    raise AssertionError('invalid ending publication accepted: ' + name)


reject('unverified-export', lambda: publication.plan(raw, MEDIA, {**proof, 'passed': False}, retained))
bad_proof = copy.deepcopy(proof)
bad_proof['clips'][0]['sourceSHA256'] = '0' * 64
reject('changed-source-proof', lambda: publication.plan(raw, MEDIA, bad_proof, retained))
bad = copy.deepcopy(index)
bad['files'].pop('ending/4.flac')
reject('missing-audio', lambda: publication.plan(json.dumps(bad).encode(), MEDIA, proof, retained))
bad = copy.deepcopy(index)
bad['files']['../../escape'] = bad['files']['ending/4.flac']
reject('escaping-object-path', lambda: publication.plan(json.dumps(bad).encode(), MEDIA, proof, retained))
before = {name: hashlib.sha256((MEDIA / name).read_bytes()).hexdigest() for name in index['files']}
with tempfile.TemporaryDirectory(prefix='ending-publication-', dir=ROOT / '.local/pal') as temporary:
    folder = Path(temporary)
    for name in index['files']:
        path = folder / name
        path.parent.mkdir(parents=True, exist_ok=True)
        os.link(MEDIA / name, path)
    variant = copy.deepcopy(index)
    variant['publicationFixture'] = 'new-application-metadata-with-unchanged-media'
    changed = (json.dumps(variant, indent=2) + '\n').encode()
    (folder / 'index.json').write_bytes(changed)
    metadata_only, _ = publication.plan(changed, folder, proof, release)
    assert metadata_only['newObjects'] == 1
    assert all(metadata_only['files'][name] == pin['files'][name] for name in index['files'])
    target = folder / 'ending/4.flac'
    target.unlink()
    target.write_bytes(b'corrupt original audio')
    reject('changed-media-bytes', lambda: publication.plan(changed, folder, proof, release))
    target.unlink()
    target.symlink_to(MEDIA / 'ending/4.flac')
    reject('symlink-media', lambda: publication.plan(changed, folder, proof, release))
    alias = folder / 'alias'
    alias.symlink_to(MEDIA, target_is_directory=True)
    reject('relative-symlink-parent', lambda: publication.guard(alias.relative_to(ROOT) / 'index.json'))
for name, digest in before.items():
    assert hashlib.sha256((MEDIA / name).read_bytes()).hexdigest() == digest
report = {'passed': True, 'scope': 'offline all three verified original ending RGB/PCM exports, immutable per-file URLs, exact repeat zero-upload, metadata-only update and corrupt/source/path rejection; no variant upload', 'files': len(rows), 'uniqueObjects': pin['objects'], 'repeatNewObjects': repeat['newObjects'], 'repeatUploadBytes': repeat['uploadBytes'], 'metadataOnlyNewObjects': metadata_only['newObjects'], 'rejected': rejected, 'remoteWrites': 0}
(ROOT / '.local/pal/verification/ending-publication-preflight-e2e.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
