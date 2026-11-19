import argparse
import concurrent.futures
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'scripts'))
import cdn_delivery as cdn
from resource_store import configuration

PREFIX = 'bga/pal/'
BUCKET = 'stoneage-assets'
ENDPOINT = 'stoneage-assets.oss-cn-shanghai.aliyuncs.com'
ORIGIN = configuration()['PUBLIC_ORIGIN']


def sha(data):
    return hashlib.sha256(data).hexdigest()


def read_release():
    path = ROOT / 'games/pal/assets/audio-release.json'
    return json.loads(path.read_text()) if path.exists() else {'version': 1, 'catalogs': {}}


def identity(row):
    return row['sha256'], row['bytes'], row['mime'], row['encoding']


def assign_urls(rows, published, asset_id, prefix=PREFIX):
    if prefix != 'bga/pal/':
        raise ValueError('unsafe immutable publication prefix')
    if published.get('version') != 1 or not isinstance(published.get('catalogs'), dict):
        raise ValueError('invalid retained audio publications')
    known = []
    catalogs = list(published['catalogs'].items())
    catalogs.sort(key=lambda pair: pair[0] != asset_id)
    for catalog_id, catalog in catalogs:
        if not catalog.get('cdnVerified'):
            continue
        files = catalog.get('files')
        if files is None and catalog_id == asset_id:
            if catalog.get('objects') != len(rows) or catalog.get('bytes') != sum(row['bytes'] for row in rows):
                raise ValueError('legacy audio publication inventory differs')
            files = {row['path']: {**row, 'url': catalog['assetsBaseURL'] + '/' + row['path']} for row in rows}
        for path, file in (files or {}).items():
            url = file['url']
            if not url.startswith('https://cdn.ichenj.com/bga/') or any(part in url for part in ['?', '#', '\\', '/../']):
                raise ValueError('unsafe retained audio URL')
            if url.startswith(configuration()['CDN_ORIGIN'] + '/bga/pal/'):
                known.append((path, file))
    result = []
    assigned = {}
    for row in rows:
        key = identity(row)
        exact = next((file for path, file in known if path == row['path'] and identity(file) == key), None)
        existing = exact or next((file for path, file in known if identity(file) == key), None)
        if existing:
            url, reused = existing['url'], True
        elif key in assigned:
            url, reused = assigned[key], False
        else:
            suffix = Path(row['path']).suffix
            url, reused = configuration()['CDN_ORIGIN'] + '/' + prefix + row['sha256'] + ('-gzip' if row['encoding'] else '') + suffix, False
        assigned[key] = url
        result.append({**row, 'url': url, 'reused': reused})
    return result


def unique_objects(rows):
    unique = {}
    for row in rows:
        previous = unique.setdefault(row['url'], row)
        if identity(previous) != identity(row):
            raise ValueError('one immutable URL refers to different audio bytes')
    return list(unique.values())


def plan(raw, media, verification, published=None):
    media = Path(media)
    if media.is_symlink() or not media.resolve().is_relative_to(ROOT):
        raise ValueError('audio source must stay inside the workspace without symlinks')
    identity = sha(raw)
    if not verification.get('passed') or verification.get('indexSHA256') != identity:
        raise ValueError('audio export has not passed its exact source read-back')
    index = json.loads(raw)
    if index['version'] != 1 or len(index['sounds']) != 363 or len(index['music']) != 87 or len(index['cd']) != 8 or len(index['files']) != 545:
        raise ValueError('incomplete original audio publication')
    for name, field in [('source-manifest.json', 'sourceManifestSHA256'), ('media-source-manifest.json', 'mediaManifestSHA256')]:
        if index[field] != sha((ROOT / 'games/pal/assets' / name).read_bytes()):
            raise ValueError('original audio source manifest differs')
    metadata = {}
    for kind, group in [(2, 'sounds'), (1, 'music'), (3, 'cd')]:
        for item in index[group].values():
            metadata[item['path']] = {**item, 'kind': kind}
            if group == 'music':
                metadata[item['midiPath']] = {'originalSHA256': item['sourceSHA256']}
    rows = []
    base = configuration()['CDN_ORIGIN'] + '/' + PREFIX.rstrip('/')
    for name, file in sorted(index['files'].items()):
        if not re.fullmatch(r'(?:sounds/\d+\.(?:flac|wav)|music/\d+\.flac|cd/0[2-9]\.ogg|midi/\d+\.mid\.gz)', name):
            raise ValueError('unsafe audio object path')
        path = media / name
        if path.is_symlink() or not path.resolve().is_relative_to(media.resolve()) or any(parent.is_symlink() for parent in path.parents if parent != media):
            raise ValueError('audio object escapes through symlink')
        data = path.read_bytes()
        mime = 'application/gzip' if name.endswith('.gz') else 'audio/' + path.suffix[1:]
        if file['mime'] != mime or len(data) != file['size'] or sha(data) != file['sha256'] or name not in metadata:
            raise ValueError('audio object differs from verified export: ' + name)
        rows.append({'path': name, 'url': base + '/' + name, 'bytes': len(data), 'sha256': sha(data), 'mime': mime, 'encoding': '', 'requiresRange': mime.startswith('audio/'), 'audio': metadata[name]})
    if len(metadata) != 545:
        raise ValueError('unlisted original audio objects')
    if (media / 'index.json').read_bytes() != raw:
        raise ValueError('audio index changed during publication planning')
    rows.append({'path': 'index.json', 'url': base + '/index.json', 'bytes': len(raw), 'sha256': identity, 'mime': 'application/json', 'encoding': 'gzip', 'requiresRange': False})
    rows = assign_urls(rows, read_release() if published is None else published, identity)
    objects = unique_objects(rows)
    fresh = [row for row in objects if not row['reused']]
    pin = {'version': 1, 'assetID': identity, 'assetsBaseURL': base, 'objects': len(objects), 'logicalFiles': len(rows), 'audioObjects': sum(row['requiresRange'] for row in objects), 'bytes': sum(row['bytes'] for row in objects), 'reusedObjects': len(objects) - len(fresh), 'newObjects': len(fresh), 'uploadBytes': sum(len(gzip.compress(raw, mtime=0)) if row['path'] == 'index.json' else row['bytes'] for row in fresh), 'files': {row['path']: {key: row[key] for key in ['url', 'bytes', 'sha256', 'mime', 'encoding']} for row in rows}}
    return pin, sorted(rows, key=lambda row: row['path'])


def verify(row):
    def operation():
        request = urllib.request.Request(row['url'], headers={'Origin': ORIGIN, 'Accept-Encoding': 'identity' if row['requiresRange'] else 'gzip'})
        with cdn.urlopen(request, timeout=90) as response:
            body = response.read()
            cdn.validate_response(row, body, response.headers)
            data = cdn.decoded(body, response.headers)
            cors = response.headers.get('Access-Control-Allow-Origin')
        result = {key: value for key, value in row.items() if key != 'audio'}
        result['cors'] = cors
        if row['requiresRange']:
            expected = data[:64]
            with cdn.urlopen(urllib.request.Request(row['url'], headers={'Origin': ORIGIN, 'Accept-Encoding': 'identity', 'Range': f'bytes=0-{len(expected)-1}'}), timeout=45) as response:
                cdn.validate_range_response(row, expected, response.read(), response.headers, response.status)
            audio = row['audio']
            bits = audio['bits'] if audio['kind'] != 3 else 16
            encoding = 'u8' if bits == 8 else 's16le'
            pcm = subprocess.check_output(['ffmpeg', '-v', 'error', '-i', 'pipe:0', '-map', '0:a:0', '-c:a', 'pcm_' + encoding, '-f', encoding, 'pipe:1'], input=data, cwd=ROOT)
            if sha(pcm) != audio['pcmSHA256'] or len(pcm) != audio['samples'] * audio['channels'] * (bits // 8):
                raise ValueError('CDN decoded PCM differs: ' + row['url'])
            result.update(rangeVerified=True, decodedPCMVerified=True, pcmSHA256=sha(pcm), sampleRate=audio['sampleRate'], channels=audio['channels'], bits=bits, samples=audio['samples'])
        elif row['path'].endswith('.gz'):
            if sha(gzip.decompress(data)) != row['audio']['originalSHA256']:
                raise ValueError('CDN original MIDI bytes differ')
            result['originalMIDIReadBackVerified'] = True
        return result
    return cdn.retry_request(operation, row['url'])


def write(path, value):
    if path.is_symlink() or not path.resolve().is_relative_to(ROOT / '.local/pal'):
        raise ValueError('publication report must stay inside .local/pal')
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix('.json.tmp')
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')
    temporary.replace(path)


def main():
    parser = argparse.ArgumentParser()
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument('--publish', action='store_true')
    modes.add_argument('--verify-only', action='store_true')
    parser.add_argument('--media', type=Path, default=ROOT / '.local/pal/audio-media-e2e')
    parser.add_argument('--report', type=Path, default=ROOT / '.local/pal/verification/audio-publication.json')
    args = parser.parse_args()
    if os.environ.get('GITHUB_ACTIONS') == 'true' and (args.publish or args.verify_only):
        parser.error('audio CDN read-back must run locally')
    raw = (args.media / 'index.json').read_bytes()
    proof = json.loads((ROOT / '.local/pal/verification/audio-media-export.json').read_text())
    pin, rows = plan(raw, args.media, proof)
    report = {'passed': False, 'stage': 'preflight', 'pin': pin, 'files': rows, 'remoteWritesRequested': args.publish}
    write(args.report, report)
    if not args.publish and not args.verify_only:
        report.update(passed=True, stage='preflight', remoteWritesPerformed=0)
        write(args.report, report)
        print(json.dumps({'preflight': True, **{key: value for key, value in pin.items() if key != 'files'}}))
        return
    fresh = [row for row in unique_objects(rows) if not row['reused']]
    report.update(reusedObjects=pin['reusedObjects'], newObjects=pin['newObjects'], uploadBytes=pin['uploadBytes'])
    if args.publish and fresh:
        spec = importlib.util.spec_from_file_location('classic_asset_publication', ROOT / 'scripts/publish-classic-game-assets.py')
        classic = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(classic)
        key, secret = cdn.credentials()
        client = classic.OSSClient(key, secret)
        known = json.loads((ROOT / 'games/snooker/assets/manifest.json').read_text())['entries'][0]
        report['anchor'] = classic.verify_anchor(client, classic.publication_path(known), known['sha256'])
        write(args.report, report)
        cdn.publish(args.media, fresh, BUCKET, ENDPOINT)
        report['stage'] = 'uploaded'
        write(args.report, report)
    if not fresh:
        report['remoteWritesPerformed'] = 0
    verified = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        pending = {pool.submit(verify, row): row for row in rows}
        for future in concurrent.futures.as_completed(pending):
            verified.append(future.result())
            if len(verified) % 25 == 0 or len(verified) == len(rows):
                report.update(stage='local-cdn-readback', verifiedFiles=len(verified), verified=sorted(verified, key=lambda row: row['path']))
                write(args.report, report)
                print(json.dumps({'cdnVerified': len(verified), 'total': len(rows)}), flush=True)
    if plan(raw, args.media, proof) != (pin, rows):
        raise ValueError('local audio changed during publication')
    report.update(passed=True, stage='verified', verificationLocation='local', allDecodedPCMVerified=True, verifiedFiles=len(verified), files=sorted(verified, key=lambda row: row['path']))
    report.pop('verified', None)
    write(args.report, report)
    release = ROOT / 'games/pal/assets/audio-release.json'
    if release.is_symlink():
        raise ValueError('audio release metadata cannot be a symlink')
    existing = json.loads(release.read_text()) if release.exists() else {'version': 1, 'catalogs': {}}
    if existing.get('version') != 1 or not isinstance(existing.get('catalogs'), dict):
        raise ValueError('invalid retained audio publication metadata')
    published = {key: value for key, value in pin.items() if key not in ['reusedObjects', 'newObjects', 'uploadBytes']}
    published['cdnVerified'] = True
    if pin['assetID'] in existing['catalogs']:
        previous = existing['catalogs'][pin['assetID']]
        for key in ['assetID', 'assetsBaseURL', 'objects', 'audioObjects', 'bytes']:
            if previous[key] != published[key]:
                raise ValueError('retained audio publication identity changed')
        for path, file in previous.get('files', {}).items():
            if published['files'].get(path) != file:
                raise ValueError('published audio URL changed for unchanged bytes')
    existing['catalogs'][pin['assetID']] = published
    release.write_text(json.dumps(existing, indent=2) + '\n')
    print(json.dumps({'passed': True, **{key: value for key, value in published.items() if key != 'files'}, 'reusedObjects': pin['reusedObjects'], 'newObjects': pin['newObjects'], 'uploadBytes': pin['uploadBytes'], 'report': str(args.report.resolve().relative_to(ROOT))}))


if __name__ == '__main__':
    main()
