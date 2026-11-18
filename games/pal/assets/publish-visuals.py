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
PREFIX = 'bga/v20261002-1/pal/visual-v1/'
RELEASE = ROOT / 'games/pal/assets/visual-release.json'
spec = importlib.util.spec_from_file_location('pal_ending_publication', ROOT / 'games/pal/assets/publish-ending.py')
ending = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ending)
audio, cdn = ending.audio, ending.cdn
classic_spec = importlib.util.spec_from_file_location('classic_asset_publication', ROOT / 'scripts/publish-classic-game-assets.py')
classic = importlib.util.module_from_spec(classic_spec)
classic_spec.loader.exec_module(classic)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def retained_release():
    ending.guard(RELEASE)
    return json.loads(RELEASE.read_text()) if RELEASE.exists() else {'version': 1, 'catalogs': {}}


def plan(raw, media, verification, published=None):
    media = Path(media)
    ending.guard(media)
    asset_id = sha(raw)
    if not verification.get('passed') or verification.get('indexSHA256') != asset_id:
        raise ValueError('current visual export has not passed its exact isolation check')
    index = json.loads(raw)
    if index.get('version') != 1 or not isinstance(index.get('files'), dict) or len(index['files']) != 3484:
        raise ValueError('incomplete original visual/font inventory')
    originals = {item['path']: item['sha256'] for item in json.loads((ROOT / 'games/pal/assets/source-manifest.json').read_text())['files']}
    upstream = json.loads((ROOT / 'games/pal/native/upstream-manifest.json').read_text())
    expected_sources = {name: originals[name] for name in ['abc.mkf', 'ball.mkf', 'data.mkf', 'f.mkf', 'fbp.mkf', 'fire.mkf', 'gop.mkf', 'map.mkf', 'mgo.mkf', 'pat.mkf', 'rgm.mkf', 'rng.mkf']}
    for name in ['fontglyph.h', 'fontglyph_cn.h', 'ascii.h']:
        item = next(row for row in upstream['files'] if row['path'] == name)
        expected_sources['font/' + name] = item['sha256']
    if index.get('sources') != expected_sources:
        raise ValueError('original visual/font source identities changed')
    for name in ['fonts.json', 'fonts/glyphs.bin.gz', 'fonts/widths.bin.gz', 'fonts/small.bin.gz', 'fonts/license.txt', 'palettes.json']:
        if name not in index['files']:
            raise ValueError('original font or palette registration is missing')
    path_pattern = re.compile(r'(?:atlases/(?:abc|ball|data|f|fbp|fire|gop|mgo|rgm)/\d+\.(?:json|png)|(?:maps|movies)/\d+\.(?:json|bin\.gz)|palettes\.json|fonts\.json|fonts/(?:glyphs|widths|small)\.bin\.gz|fonts/license\.txt)')
    rows = []
    for name, file in sorted(index['files'].items()):
        if not path_pattern.fullmatch(name):
            raise ValueError('unsafe original visual object path')
        path = media / name
        ending.guard(path, media)
        data = path.read_bytes()
        mime = 'application/json' if name.endswith('.json') else 'image/png' if name.endswith('.png') else 'text/plain' if name.endswith('.txt') else 'application/octet-stream' if name.startswith('fonts/') else 'application/gzip'
        if len(data) != file.get('size') or sha(data) != file.get('sha256') or file.get('mime') != mime:
            raise ValueError('visual object differs from its verified export: ' + name)
        row = {'path': name, 'bytes': len(data), 'sha256': sha(data), 'mime': mime, 'encoding': 'gzip' if mime in ['application/json', 'text/plain'] else '', 'requiresRange': False}
        if mime == 'image/png':
            row['dimensions'] = list(classic.media_dimensions(data, mime))
        elif name.endswith('.gz'):
            decoded = gzip.decompress(data)
            if not decoded or len(decoded) > 64*1024*1024:
                raise ValueError('visual decoded payload exceeds its limit')
            row.update(decodedBytes=len(decoded), decodedSHA256=sha(decoded))
        elif mime == 'application/json':
            json.loads(data)
        rows.append(row)
    if (media / 'index.json').read_bytes() != raw:
        raise ValueError('visual index changed during planning')
    rows.append({'path': 'index.json', 'bytes': len(raw), 'sha256': asset_id, 'mime': 'application/json', 'encoding': 'gzip', 'requiresRange': False})
    previous = retained_release() if published is None else published
    if previous.get('version') != 1 or not isinstance(previous.get('catalogs'), dict):
        raise ValueError('invalid retained visual publication metadata')
    known = {'version': 1, 'catalogs': {**audio.read_release()['catalogs'], **ending.retained_release()['catalogs'], **previous['catalogs']}}
    rows = audio.assign_urls(rows, known, asset_id, prefix=PREFIX)
    objects = audio.unique_objects(rows)
    fresh = [row for row in objects if not row['reused']]
    pin = {'version': 1, 'assetID': asset_id, 'assetsBaseURL': 'https://cdn.ichenj.com/' + PREFIX + asset_id, 'objects': len(objects), 'logicalFiles': len(rows), 'bytes': sum(row['bytes'] for row in objects), 'reusedObjects': len(objects) - len(fresh), 'newObjects': len(fresh), 'uploadBytes': sum(len(gzip.compress((media / row['path']).read_bytes(), mtime=0)) if row['encoding'] else row['bytes'] for row in fresh), 'files': {row['path']: {key: row[key] for key in ['url', 'bytes', 'sha256', 'mime', 'encoding']} for row in rows}}
    return pin, sorted(rows, key=lambda row: row['path'])


def verify(row):
    def operation():
        request = urllib.request.Request(row['url'], headers={'Origin': audio.ORIGIN, 'Accept-Encoding': 'gzip'})
        with cdn.urlopen(request, timeout=90) as response:
            body = response.read()
            cdn.validate_response(row, body, response.headers)
            data = cdn.decoded(body, response.headers)
            result = {**row, 'cors': response.headers.get('Access-Control-Allow-Origin')}
        if row['mime'] == 'image/png':
            if list(classic.media_dimensions(data, 'image/png')) != row['dimensions']:
                raise ValueError('CDN original PNG dimensions changed')
            result['pngVerified'] = True
        elif row['path'].endswith('.gz'):
            decoded = gzip.decompress(data)
            if len(decoded) != row['decodedBytes'] or sha(decoded) != row['decodedSHA256']:
                raise ValueError('CDN original decoded visual payload changed')
            result['decodedVerified'] = True
        elif row['mime'] == 'application/json':
            json.loads(data)
        return result
    return cdn.retry_request(operation, row['url'])


def main():
    parser = argparse.ArgumentParser()
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument('--publish', action='store_true')
    modes.add_argument('--verify-only', action='store_true')
    parser.add_argument('--media', type=Path, default=ROOT / '.local/pal/visual/pal/v1')
    parser.add_argument('--report', type=Path, default=ROOT / '.local/pal/verification/visual-publication.json')
    args = parser.parse_args()
    if os.environ.get('GITHUB_ACTIONS') == 'true' and (args.publish or args.verify_only):
        parser.error('visual CDN read-back must run locally')
    ending.guard(args.media)
    raw = (args.media / 'index.json').read_bytes()
    proof = json.loads((ROOT / '.local/pal/verification/visual-export-isolation.json').read_text())
    pin, rows = plan(raw, args.media, proof)
    report = {'passed': False, 'stage': 'preflight', 'pin': pin, 'files': rows, 'remoteWritesRequested': args.publish}
    ending.write(args.report, report)
    if not args.publish and not args.verify_only:
        report.update(passed=True, remoteWritesPerformed=0)
        ending.write(args.report, report)
        print(json.dumps({'passed': True, **{key: value for key, value in pin.items() if key != 'files'}}))
        return
    fresh = [row for row in audio.unique_objects(rows) if not row['reused']]
    report.update(reusedObjects=pin['reusedObjects'], newObjects=pin['newObjects'], uploadBytes=pin['uploadBytes'])
    if args.publish and fresh:
        key, secret = cdn.credentials()
        client = classic.OSSClient(key, secret)
        anchor = json.loads((ROOT / 'games/snooker/assets/manifest.json').read_text())['entries'][0]
        report['anchor'] = classic.verify_anchor(client, 'bga/v20261002-1' + anchor['urlPath'], anchor['sha256'])
        ending.write(args.report, report)
        cdn.publish(args.media, fresh, audio.BUCKET, audio.ENDPOINT)
        report['stage'] = 'uploaded'
        ending.write(args.report, report)
    else:
        report['remoteWritesPerformed'] = 0
    checked = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        pending = [pool.submit(verify, row) for row in rows]
        for future in concurrent.futures.as_completed(pending):
            checked.append(future.result())
            if len(checked) % 100 == 0 or len(checked) == len(rows):
                report.update(stage='local-cdn-readback', verifiedFiles=len(checked), verified=sorted(checked, key=lambda row: row['path']))
                ending.write(args.report, report)
                print(json.dumps({'cdnVerified': len(checked), 'total': len(rows)}), flush=True)
    if plan(raw, args.media, proof) != (pin, rows):
        raise ValueError('visual release inputs changed during publication')
    report.update(passed=True, stage='verified', verificationLocation='local', files=sorted(checked, key=lambda row: row['path']), pngAndDecodedPayloadsVerified=True)
    report.pop('verified', None)
    ending.write(args.report, report)
    release = retained_release()
    published = {key: value for key, value in pin.items() if key not in ['reusedObjects', 'newObjects', 'uploadBytes']}
    published['cdnVerified'] = True
    previous = release['catalogs'].get(pin['assetID'])
    if previous and any(previous.get(key) != published[key] for key in ['assetID', 'files', 'assetsBaseURL', 'objects', 'bytes']):
        raise ValueError('retained original visual publication changed')
    release['catalogs'][pin['assetID']] = published
    ending.write(RELEASE, release, ROOT / 'games/pal/assets')
    print(json.dumps({'passed': True, **{key: value for key, value in pin.items() if key != 'files'}, 'report': str(args.report.resolve().relative_to(ROOT))}))


if __name__ == '__main__':
    main()
