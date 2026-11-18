import copy
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]
MEDIA = ROOT / '.local/pal/audio-media-e2e'
REPORT = ROOT / '.local/pal/verification/audio-publication-dedup.json'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    spec = importlib.util.spec_from_file_location('pal_audio_publish_dedup', ROOT / 'games/pal/assets/publish-audio.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    raw = (MEDIA / 'index.json').read_bytes()
    index = json.loads(raw)
    proof = json.loads((ROOT / '.local/pal/verification/audio-media-export.json').read_text())
    release = json.loads((ROOT / 'games/pal/assets/audio-release.json').read_text())
    first, rows = module.plan(raw, MEDIA, proof, release)
    published_first = {**first, 'cdnVerified': True}
    assert first['newObjects'] == 0 and first['uploadBytes'] == 0 and first['reusedObjects'] == 546
    assert first['files']['music/31.flac']['url'] == release['catalogs'][first['assetID']]['assetsBaseURL'] + '/music/31.flac'
    before = {name: file['sha256'] for name, file in index['files'].items()}
    with tempfile.TemporaryDirectory(prefix='audio-dedup-', dir=ROOT / '.local/pal') as temporary:
        folder = Path(temporary)
        for name in index['files']:
            target = folder / name
            target.parent.mkdir(parents=True, exist_ok=True)
            os.link(MEDIA / name, target)
        variant = copy.deepcopy(index)
        variant['publicationMetadataFixture'] = 'application-build-metadata-changed-with-identical-assets'
        changed = (json.dumps(variant, ensure_ascii=False, indent=2) + '\n').encode()
        (folder / 'index.json').write_bytes(changed)
        variant_proof = {**proof, 'indexSHA256': sha(changed)}
        metadata_pin, metadata_rows = module.plan(changed, folder, variant_proof, {'version': 1, 'catalogs': {first['assetID']: published_first}})
        assert metadata_pin['assetID'] != first['assetID']
        assert metadata_pin['newObjects'] == 1 and metadata_pin['reusedObjects'] == 545
        assert all(metadata_pin['files'][name]['url'] == first['files'][name]['url'] for name in index['files'])
        assert metadata_pin['uploadBytes'] == len(gzip.compress(changed, mtime=0))
        replacement = folder / 'music/31.flac'
        replacement.unlink()
        subprocess.run(['ffmpeg', '-v', 'error', '-y', '-i', str(MEDIA / 'music/31.flac'), '-c:a', 'flac', '-compression_level', '0', '-map_metadata', '-1', str(replacement)], check=True)
        altered = replacement.read_bytes()
        assert sha(altered) != index['files']['music/31.flac']['sha256']
        variant['files']['music/31.flac']['sha256'] = sha(altered)
        variant['files']['music/31.flac']['size'] = len(altered)
        changed = (json.dumps(variant, ensure_ascii=False, indent=2) + '\n').encode()
        (folder / 'index.json').write_bytes(changed)
        variant_proof['indexSHA256'] = sha(changed)
        single, single_rows = module.plan(changed, folder, variant_proof, {'version': 1, 'catalogs': {first['assetID']: published_first}})
        assert single['newObjects'] == 2 and single['reusedObjects'] == 544
        assert single['uploadBytes'] == len(gzip.compress(changed, mtime=0)) + len(altered)
        assert single['files']['music/31.flac']['url'] != first['files']['music/31.flac']['url']
        assert all(single['files'][name]['url'] == first['files'][name]['url'] for name in index['files'] if name != 'music/31.flac')
    sample = {key: value for key, value in first['files']['music/31.flac'].items()}
    renamed = module.assign_urls([{'path': 'music/renamed.flac', **{key: sample[key] for key in ['sha256', 'bytes', 'mime', 'encoding']}}], {'version': 1, 'catalogs': {first['assetID']: published_first}}, first['assetID'])
    assert renamed[0]['url'] == sample['url'] and renamed[0]['reused']
    fresh = {'sha256': 'f' * 64, 'bytes': 10, 'mime': 'audio/flac', 'encoding': ''}
    twins = module.assign_urls([{'path': 'sounds/1.flac', **fresh}, {'path': 'sounds/2.flac', **fresh}], {'version': 1, 'catalogs': {}}, first['assetID'])
    assert twins[0]['url'] == twins[1]['url']
    assert len(module.unique_objects(twins)) == 1
    for name, expected in before.items():
        assert sha((MEDIA / name).read_bytes()) == expected
    REPORT.write_text(json.dumps({'passed': True, 'scope': 'offline complete original inventory, existing publication URL retention, metadata-only new catalog, one real losslessly recompressed clip, renamed-object binding and within-batch dedup; no variant upload or new sound-content acceptance', 'repeat': {key: first[key] for key in ['reusedObjects', 'newObjects', 'uploadBytes']}, 'metadataOnly': {key: metadata_pin[key] for key in ['reusedObjects', 'newObjects', 'uploadBytes']}, 'singleChangedFile': {key: single[key] for key in ['reusedObjects', 'newObjects', 'uploadBytes']}, 'renamedReused': True, 'batchDuplicateUploadedObjects': 1, 'remoteWrites': 0}, indent=2) + '\n')
    print(json.dumps({'passed': True, 'repeatNewObjects': first['newObjects'], 'repeatUploadBytes': first['uploadBytes'], 'remoteWrites': 0}))


if __name__ == '__main__':
    main()
