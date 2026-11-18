import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile

ROOT = Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('pal_server_distribution', ROOT / 'games/pal/build/server-distribution.py')
distribution = importlib.util.module_from_spec(spec)
spec.loader.exec_module(distribution)
FOLDER = ROOT / '.local/pal/server-packages/a9234deb9d91a84e0d273f0d0ef3e90e81a7fbdf052c41ed4fcb0c91a35566ac'
manifest = json.loads((FOLDER / 'manifest.json').read_text())
proof = {'passed': True, 'runtimeSHA256': manifest['current'], 'sources': distribution.sources()}
pin, rows = distribution.plan(FOLDER, proof, {'version': 1, 'catalogs': {}})
assert len(rows) == 3 and pin['assetID'] == hashlib.sha256((FOLDER / 'manifest.json').read_bytes()).hexdigest()
release = {'version': 1, 'current': pin['assetID'], 'catalogs': {pin['assetID']: {**pin, 'cdnVerified': True}}}
repeat, _ = distribution.plan(FOLDER, proof, release)
assert repeat['files'] == pin['files'] and repeat['newObjects'] == repeat['uploadBytes'] == 0
rejected = []


def reject(label, operation):
    try:
        operation()
    except (ValueError, KeyError, FileNotFoundError):
        rejected.append(label)
        return
    raise AssertionError('invalid server distribution accepted: ' + label)


reject('failed-source-proof', lambda: distribution.plan(FOLDER, {**proof, 'passed': False}, release))
reject('wrong-source-proof-core', lambda: distribution.plan(FOLDER, {**proof, 'runtimeSHA256': '0' * 64}, release))
reject('changed-source-identity', lambda: distribution.plan(FOLDER, {**proof, 'sources': {}}, release))
bad = copy.deepcopy(pin)
bad['sources'] = {}
bad['cdnVerified'] = True
reject('stale-native-source-publication', lambda: distribution.validate(bad))
with tempfile.TemporaryDirectory(prefix='distribution-e2e-', dir=ROOT / '.local/pal') as temporary:
    parent = Path(temporary)
    published = {**pin, 'cdnVerified': True}
    requested = []

    def download(row):
        requested.append(row['url'])
        name = next(name for name, entry in published['files'].items() if entry['url'] == row['url'])
        return (FOLDER / name).read_bytes()

    target = parent / 'candidate' / 'pal'
    result = distribution.prepare(published, target, download)
    assert result['passed'] and len(requested) == 3
    before = {name: (target / name).read_bytes() for name in pin['files']}
    requested.clear()
    assert distribution.prepare(published, target, download)['reused'] and not requested
    assert all((target / name).read_bytes() == data for name, data in before.items())
    (target / 'unexpected.png').write_bytes(b'not a server rule')
    reject('existing-client-payload', lambda: distribution.prepare(published, target, download))
    (target / 'unexpected.png').unlink()
    reject('foreign-download-url', lambda: distribution.prepare({**published, 'files': {**published['files'], 'manifest.json': {**published['files']['manifest.json'], 'url': 'https://example.com/rules.json'}}}, parent / 'foreign' / 'pal', download))
    corrupt = parent / 'corrupt' / 'pal'
    reject('corrupt-download', lambda: distribution.prepare(published, corrupt, lambda row: b'corrupt'))
    assert not corrupt.exists()
    partial = parent / 'partial' / 'pal'

    def interrupted(row):
        if row['mime'] == 'application/wasm':
            raise ValueError('network interrupted')
        return download(row)

    reject('partial-download-transaction', lambda: distribution.prepare(published, partial, interrupted))
    assert not partial.exists() and not list(partial.parent.glob('staging-*'))
    linked = parent / 'linked'
    linked.symlink_to(FOLDER, target_is_directory=True)
    reject('symlink-target-parent', lambda: distribution.prepare(published, linked / 'pal', download))
    renamed = copy.deepcopy(manifest)
    renamed['retained'] = ['f' * 64]
    variant = parent / 'variant'
    variant.mkdir()
    for name in pin['files']:
        path = variant / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes((FOLDER / name).read_bytes())
    (variant / 'manifest.json').write_text(json.dumps(renamed) + '\n')
    new_core = variant / 'cores' / ('f' * 64 + '.wasm')
    new_core.write_bytes(b'fixture core')
    reject('wrong-retained-core-content', lambda: distribution.plan(variant, proof, release))
    linked = parent / 'linked-object'
    linked.mkdir()
    for name in pin['files']:
        path = linked / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes((FOLDER / name).read_bytes())
    path = linked / 'authority.zip'
    path.unlink()
    path.symlink_to(FOLDER / 'authority.zip')
    reject('symlink-source-object', lambda: distribution.plan(linked, proof, release))
row = pin['files']['manifest.json']
raw = (FOLDER / 'manifest.json').read_bytes()
assert distribution.decode(row, gzip.compress(raw, mtime=0), {'Content-Type': 'application/json', 'Content-Encoding': 'gzip'}) == raw
reject('oversized-gzip', lambda: distribution.decode(row, gzip.compress(b'x' * (len(raw) + 1)), {'Content-Type': 'application/json', 'Content-Encoding': 'gzip'}))
reject('wrong-mime', lambda: distribution.decode(row, raw, {'Content-Type': 'image/png'}))
reject('wrong-encoding', lambda: distribution.decode(row, raw, {'Content-Type': 'application/json', 'Content-Encoding': 'br'}))
report = {'passed': True, 'scope': 'offline exact source-bound server-only distribution, immutable repeat zero upload, transactional prepare/reuse and download corruption/path/size rejection; no variant upload', 'files': len(rows), 'repeatUploadBytes': repeat['uploadBytes'], 'rejected': rejected, 'remoteWrites': 0}
(ROOT / '.local/pal/verification/server-distribution-e2e.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
