import argparse
import concurrent.futures
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
PREFIX = 'bga/v20261002-1/pal/ending-v1/'
RELEASE = ROOT / 'games/pal/assets/ending-release.json'
spec = importlib.util.spec_from_file_location('pal_audio_publication', ROOT / 'games/pal/assets/publish-audio.py')
audio = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audio)
cdn = audio.cdn


def sha(data):
    return hashlib.sha256(data).hexdigest()


def guard(path, base=ROOT):
    path = path.absolute()
    if path.is_symlink() or not path.resolve().is_relative_to(base.resolve()) or any(parent.is_symlink() for parent in path.parents if parent.is_relative_to(ROOT)):
        raise ValueError('ending publication path escapes its workspace directory')


def retained_release():
    guard(RELEASE)
    return json.loads(RELEASE.read_text()) if RELEASE.exists() else {'version': 1, 'catalogs': {}}


def plan(raw, media, verification, published=None):
    media = Path(media)
    guard(media)
    index = json.loads(raw)
    source = json.loads((ROOT / 'games/pal/assets/media-source-manifest.json').read_text())
    if not verification.get('passed') or verification.get('files') != 320 or len(verification.get('clips', [])) != 3:
        raise ValueError('original ending export has not passed complete read-back')
    if index.get('version') != 1 or index.get('sourceCommit') != source['commit'] or not isinstance(index.get('files'), dict):
        raise ValueError('unexpected original ending source version')
    for name in index['files']:
        if not re.fullmatch(r'ending/[4-6](?:/\d+\.rgb\.gz|\.json|\.flac)', name):
            raise ValueError('unsafe ending object path')
    if len(index['files']) != 320:
        raise ValueError('incomplete original ending export')
    asset_id = sha(raw)
    base = 'https://cdn.ichenj.com/' + PREFIX + asset_id
    rows = {}
    for name, file in sorted(index['files'].items()):
        path = media / name
        guard(path, media)
        data = path.read_bytes()
        mime = 'application/gzip' if name.endswith('.gz') else 'application/json' if name.endswith('.json') else 'audio/flac'
        if file.get('size') != len(data) or file.get('sha256') != sha(data) or file.get('mime') != mime:
            raise ValueError('ending file differs from its verified export: ' + name)
        rows[name] = {'path': name, 'bytes': len(data), 'sha256': sha(data), 'mime': mime, 'encoding': 'gzip' if name.endswith('.json') else '', 'requiresRange': mime == 'audio/flac'}
    required = set()
    for clip, frames in [(4, 200), (5, 2294), (6, 2196)]:
        name = f'ending/{clip}.json'
        original = next(item for item in source['files'] if item['path'] == f'{clip}.avi')
        verified = next(item for item in verification['clips'] if item['clip'] == clip)
        metadata = json.loads((media / name).read_bytes())
        if metadata['version'] != 1 or metadata['width'] != 288 or metadata['height'] != 180 or metadata['rate'] != [15, 1] or metadata['frames'] != frames or metadata['sourceSHA256'] != original['sha256'] or verified['sourceSHA256'] != original['sha256'] or verified['frames'] != frames:
            raise ValueError('original ending metadata differs from the complete source proof')
        required.add(name)
        digest = hashlib.sha256()
        first = 0
        for chunk in metadata['chunks']:
            path = f'ending/{clip}/{first}.rgb.gz'
            if chunk['first'] != first or type(chunk['frames']) is not int or not 0 < chunk['frames'] <= 15 or chunk['path'] != path or path not in rows:
                raise ValueError('original ending RGB blocks changed')
            pixels = gzip.decompress((media / path).read_bytes())
            if len(pixels) != chunk['frames'] * 288 * 180 * 3:
                raise ValueError('original ending RGB block is truncated')
            digest.update(pixels)
            rows[path].update(rgbSHA256=sha(pixels), rgbBytes=len(pixels))
            required.add(path)
            first += chunk['frames']
        if first != frames or len(metadata['chunks']) != verified['chunks'] or digest.hexdigest() != metadata['rgbSHA256'] or digest.hexdigest() != verified['rgbSHA256']:
            raise ValueError('original ending complete RGB read-back differs')
        track = metadata['audio']
        path = f'ending/{clip}.flac'
        if path not in rows or track['path'] != path or track['sampleRate'] != 22050 or track.get('bits') != 8 or verified.get('audioBits') != 8 or track['channels'] not in [1, 2] or track['pcmSHA256'] != verified['audioPCM'] or type(track['samples']) is not int or track['samples'] <= 0:
            raise ValueError('original ending PCM metadata differs')
        required.add(path)
        rows[path]['audio'] = {**track, 'kind': 2}
    if required != set(rows) or sum(row['bytes'] for row in rows.values()) != verification['bytes']:
        raise ValueError('original ending publication inventory changed')
    guard(media / 'index.json', media)
    if (media / 'index.json').read_bytes() != raw:
        raise ValueError('original ending index changed during planning')
    rows['index.json'] = {'path': 'index.json', 'bytes': len(raw), 'sha256': asset_id, 'mime': 'application/json', 'encoding': 'gzip', 'requiresRange': False}
    previous = retained_release() if published is None else published
    if previous.get('version') != 1 or not isinstance(previous.get('catalogs'), dict):
        raise ValueError('invalid retained ending publication metadata')
    known = audio.read_release()
    known = {'version': 1, 'catalogs': {**known['catalogs'], **previous['catalogs']}}
    mapped = audio.assign_urls(list(rows.values()), known, asset_id, prefix=PREFIX)
    objects = audio.unique_objects(mapped)
    fresh = [row for row in objects if not row['reused']]
    pin = {'version': 1, 'assetID': asset_id, 'assetsBaseURL': base, 'objects': len(objects), 'logicalFiles': len(mapped), 'audioObjects': sum(row['requiresRange'] for row in objects), 'bytes': sum(row['bytes'] for row in objects), 'reusedObjects': len(objects) - len(fresh), 'newObjects': len(fresh), 'uploadBytes': sum(len(gzip.compress((media / row['path']).read_bytes(), mtime=0)) if row['encoding'] == 'gzip' else row['bytes'] for row in fresh), 'files': {row['path']: {key: row[key] for key in ['url', 'bytes', 'sha256', 'mime', 'encoding']} for row in mapped}}
    return pin, sorted(mapped, key=lambda row: row['path'])


def verify(row):
    if row['requiresRange']:
        return audio.verify(row)

    def operation():
        request = urllib.request.Request(row['url'], headers={'Origin': audio.ORIGIN, 'Accept-Encoding': 'gzip'})
        with cdn.urlopen(request, timeout=90) as response:
            body = response.read()
            cdn.validate_response(row, body, response.headers)
            data = cdn.decoded(body, response.headers)
            result = {**row, 'cors': response.headers.get('Access-Control-Allow-Origin')}
        if row['path'].endswith('.rgb.gz'):
            pixels = gzip.decompress(data)
            if len(pixels) != row['rgbBytes'] or sha(pixels) != row['rgbSHA256']:
                raise ValueError('CDN ending RGB differs from original AVI')
            result['decodedRGBVerified'] = True
        return result
    return cdn.retry_request(operation, row['url'])


def write(path, value, base=ROOT / '.local/pal'):
    guard(path, base)
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + '.tmp')
    guard(temporary, base)
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')
    temporary.replace(path)


def main():
    parser = argparse.ArgumentParser()
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument('--publish', action='store_true')
    modes.add_argument('--verify-only', action='store_true')
    parser.add_argument('--media', type=Path, default=ROOT / '.local/pal/ending-media-e2e')
    parser.add_argument('--report', type=Path, default=ROOT / '.local/pal/verification/ending-publication.json')
    args = parser.parse_args()
    if os.environ.get('GITHUB_ACTIONS') == 'true' and (args.publish or args.verify_only):
        parser.error('ending CDN read-back must run locally')
    guard(args.media)
    raw = (args.media / 'index.json').read_bytes()
    proof = json.loads((ROOT / '.local/pal/verification/ending-media-export.json').read_text())
    pin, rows = plan(raw, args.media, proof)
    report = {'passed': False, 'stage': 'preflight', 'pin': pin, 'files': rows, 'remoteWritesRequested': args.publish}
    write(args.report, report)
    if not args.publish and not args.verify_only:
        report.update(passed=True, remoteWritesPerformed=0)
        write(args.report, report)
        print(json.dumps({'passed': True, **{key: value for key, value in pin.items() if key != 'files'}}))
        return
    fresh = [row for row in audio.unique_objects(rows) if not row['reused']]
    report.update(reusedObjects=pin['reusedObjects'], newObjects=pin['newObjects'], uploadBytes=pin['uploadBytes'])
    if args.publish and fresh:
        spec = importlib.util.spec_from_file_location('classic_asset_publication', ROOT / 'scripts/publish-classic-game-assets.py')
        classic = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(classic)
        key, secret = cdn.credentials()
        client = classic.OSSClient(key, secret)
        anchor = json.loads((ROOT / 'games/snooker/assets/manifest.json').read_text())['entries'][0]
        report['anchor'] = classic.verify_anchor(client, 'bga/v20261002-1' + anchor['urlPath'], anchor['sha256'])
        write(args.report, report)
        cdn.publish(args.media, fresh, audio.BUCKET, audio.ENDPOINT)
        report['stage'] = 'uploaded'
        write(args.report, report)
    else:
        report['remoteWritesPerformed'] = 0
    verified = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        pending = [pool.submit(verify, row) for row in rows]
        for future in concurrent.futures.as_completed(pending):
            verified.append(future.result())
            if len(verified) % 25 == 0 or len(verified) == len(rows):
                report.update(stage='local-cdn-readback', verifiedFiles=len(verified), verified=sorted(verified, key=lambda row: row['path']))
                write(args.report, report)
                print(json.dumps({'cdnVerified': len(verified), 'total': len(rows)}), flush=True)
    if plan(raw, args.media, proof) != (pin, rows):
        raise ValueError('original ending export changed during publication')
    report.update(passed=True, stage='verified', verificationLocation='local', files=sorted(verified, key=lambda row: row['path']), allDecodedPCMVerified=True, allDecodedRGBVerified=True)
    report.pop('verified', None)
    write(args.report, report)
    release = retained_release()
    published = {key: value for key, value in pin.items() if key not in ['reusedObjects', 'newObjects', 'uploadBytes']}
    published['cdnVerified'] = True
    previous = release['catalogs'].get(pin['assetID'])
    if previous and any(previous.get(key) != published[key] for key in ['assetID', 'files', 'assetsBaseURL', 'objects', 'bytes']):
        raise ValueError('retained original ending publication changed')
    release['catalogs'][pin['assetID']] = published
    write(RELEASE, release, ROOT / 'games/pal/assets')
    print(json.dumps({'passed': True, **{key: value for key, value in pin.items() if key != 'files'}, 'report': str(args.report.resolve().relative_to(ROOT))}))


if __name__ == '__main__':
    main()
