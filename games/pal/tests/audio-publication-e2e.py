import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile

ROOT = Path(__file__).resolve().parents[3]
PUBLISHER = ROOT / 'games/pal/assets/publish-audio.py'
MEDIA = ROOT / '.local/pal/audio-media-e2e'
REPORT = ROOT / '.local/pal/verification/audio-publication-plan.json'


def main():
    assert PUBLISHER.is_file(), 'immutable PAL audio publisher has not been implemented'
    spec = importlib.util.spec_from_file_location('pal_audio_publication', PUBLISHER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    raw = (MEDIA / 'index.json').read_bytes()
    verification = json.loads((ROOT / '.local/pal/verification/audio-media-export.json').read_text())
    pin, rows = module.plan(raw, MEDIA, verification)
    assert pin['assetID'] == hashlib.sha256(raw).hexdigest()
    assert len(rows) == 546 and sum(row['requiresRange'] for row in rows) == 458
    assert len({row['url'] for row in rows}) == 546
    assert all(row['url'] == pin['assetsBaseURL'] + '/' + row['path'] for row in rows)
    assert rows[0]['url'].startswith('https://cdn.ichenj.com/bga/v20261002-1/pal/audio-v1/' + pin['assetID'] + '/')
    assert next(row for row in rows if row['path'] == 'index.json')['encoding'] == 'gzip'
    assert all(not row['encoding'] for row in rows if row['requiresRange'])
    assert sum(row['bytes'] for row in rows) == 475155573
    failures = []
    for label, mutate in [
        ('unverified', lambda index, proof: proof.update(passed=False)),
        ('changed-index', lambda index, proof: index.update(version=2)),
        ('unsafe-path', lambda index, proof: index['files'].update({'../outside.flac': {'size': 1, 'sha256': '0' * 64, 'mime': 'audio/flac'}})),
        ('wrong-size', lambda index, proof: index['files']['sounds/1.flac'].update(size=1)),
        ('wrong-mime', lambda index, proof: index['files']['sounds/1.flac'].update(mime='text/plain')),
    ]:
        index, proof = json.loads(raw), copy.deepcopy(verification)
        mutate(index, proof)
        altered = (json.dumps(index, ensure_ascii=False, indent=2) + '\n').encode()
        if label in ('unsafe-path', 'wrong-size', 'wrong-mime'):
            proof['indexSHA256'] = hashlib.sha256(altered).hexdigest()
        try:
            module.plan(altered, MEDIA, proof)
        except (ValueError, OSError):
            failures.append(label)
        else:
            raise AssertionError('invalid publication plan accepted: ' + label)
    with tempfile.TemporaryDirectory(prefix='audio-publication-negative-', dir=ROOT / '.local/pal') as temporary:
        folder = Path(temporary)
        (folder / 'index.json').write_bytes(raw)
        (folder / 'sounds').symlink_to(MEDIA / 'sounds', target_is_directory=True)
        try:
            module.plan(raw, folder, verification)
        except (ValueError, OSError):
            failures.append('source-symlink')
        else:
            raise AssertionError('source symlink accepted')
    REPORT.write_text(json.dumps({'passed': True, 'scope': 'full verified local publication inventory, hash-addressed immutable URLs, audio Range/encoding declarations, source digest and path protection; offline only, no upload or CDN acceptance', 'pin': pin, 'objects': len(rows), 'audioObjects': 458, 'failuresRejected': failures, 'writesPerformed': 0}, indent=2) + '\n')
    print(json.dumps({'passed': True, 'objects': len(rows), 'failuresRejected': len(failures), 'remoteWrites': 0}))


if __name__ == '__main__':
    main()
