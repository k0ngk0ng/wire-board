import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import tempfile
import urllib.request
import zlib

ROOT = Path(__file__).resolve().parents[3]
RELEASE = ROOT / 'games/pal/assets/server-release.json'
PREFIX = 'bga/v20261002-1/pal/server-rule-v1/'
spec = importlib.util.spec_from_file_location('pal_audio_publication', ROOT / 'games/pal/assets/publish-audio.py')
audio = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audio)
cdn = audio.cdn


def sha(data):
    return hashlib.sha256(data).hexdigest()


def guard(path):
    path = Path(path).absolute()
    if not path.resolve().is_relative_to(ROOT) or path.is_symlink() or any(parent.is_symlink() for parent in path.parents if parent.is_relative_to(ROOT)):
        raise ValueError('server distribution path escapes workspace or contains a symlink')


def sources():
    folder = ROOT / 'games/pal/native'
    paths = [path for path in folder.rglob('*') if path.is_file() and path.suffix in ['.c', '.h']]
    paths += [folder / 'upstream-manifest.json', folder / 'upstream/LICENSE']
    paths += [ROOT / 'games/pal/build' / name for name in ['native.py', 'runtime_archive.py', 'font_data.py']]
    result = {}
    for path in sorted(paths):
        guard(path)
        result[str(path.relative_to(ROOT))] = sha(path.read_bytes())
    return result


def retained_release():
    guard(RELEASE)
    return json.loads(RELEASE.read_text()) if RELEASE.exists() else {'version': 1, 'catalogs': {}}


def package_files(folder):
    folder = Path(folder)
    guard(folder)
    raw = (folder / 'manifest.json').read_bytes()
    manifest = json.loads(raw)
    valid = lambda value: isinstance(value, str) and re.fullmatch('[a-f0-9]{64}', value)
    authority = json.loads((ROOT / 'games/pal/assets/authority-release.json').read_text())
    if set(manifest) != {'format', 'resourceID', 'authoritySHA256', 'current', 'retained'} or manifest['format'] != 1 or manifest['resourceID'] != authority['resourceID'] or manifest['authoritySHA256'] != authority['archiveSHA256'] or not valid(manifest['current']) or not isinstance(manifest['retained'], list) or len(manifest['retained']) > 64:
        raise ValueError('invalid server distribution manifest')
    cores = [manifest['current'], *manifest['retained']]
    if not all(valid(core) for core in cores) or len(set(cores)) != len(cores):
        raise ValueError('invalid or duplicate retained native core')
    names = {'manifest.json', 'authority.zip', *('cores/' + core + '.wasm' for core in cores)}
    found = set()
    for path in folder.rglob('*'):
        guard(path)
        if path.is_dir() and path == folder / 'cores':
            continue
        name = path.relative_to(folder).as_posix()
        if name not in names or not path.is_file():
            raise ValueError('unexpected server distribution payload')
        found.add(name)
    if found != names:
        raise ValueError('incomplete server distribution payload')
    files = {name: (folder / name).read_bytes() for name in sorted(names)}
    if sha(files['authority.zip']) != manifest['authoritySHA256'] or len(files['authority.zip']) != authority['archiveBytes']:
        raise ValueError('server authority archive differs')
    for core in cores:
        if sha(files['cores/' + core + '.wasm']) != core:
            raise ValueError('server native core differs')
    return manifest, files


def plan(folder, proof, previous=None):
    folder = Path(folder)
    manifest, files = package_files(folder)
    if not proof.get('passed') or proof.get('runtimeSHA256') != manifest['current'] or proof.get('sources') != sources():
        raise ValueError('server native core source reproduction is not current')
    previous = retained_release() if previous is None else previous
    if previous.get('version') != 1 or not isinstance(previous.get('catalogs'), dict):
        raise ValueError('invalid retained server publication')
    if previous.get('current'):
        old = previous['catalogs'][previous['current']]
        if not set([old['currentCore'], *old['retainedCores']]).issubset([manifest['current'], *manifest['retained']]):
            raise ValueError('server publication would drop a retained native core')
    asset_id = sha(files['manifest.json'])
    rows = []
    for name, data in files.items():
        mime = 'application/json' if name == 'manifest.json' else 'application/zip' if name == 'authority.zip' else 'application/wasm'
        rows.append({'path': name, 'bytes': len(data), 'sha256': sha(data), 'mime': mime, 'encoding': '' if mime == 'application/zip' else 'gzip', 'requiresRange': False})
    rows = audio.assign_urls(rows, previous, asset_id, prefix=PREFIX)
    objects = audio.unique_objects(rows)
    fresh = [row for row in objects if not row['reused']]
    pin = {'version': 1, 'assetID': asset_id, 'currentCore': manifest['current'], 'retainedCores': manifest['retained'], 'sources': proof['sources'], 'assetsBaseURL': 'https://cdn.ichenj.com/' + PREFIX + asset_id, 'objects': len(objects), 'bytes': sum(row['bytes'] for row in objects), 'reusedObjects': len(objects) - len(fresh), 'newObjects': len(fresh), 'uploadBytes': sum(len(gzip.compress(files[row['path']], mtime=0)) if row['encoding'] else row['bytes'] for row in fresh), 'files': {row['path']: {key: row[key] for key in ['url', 'bytes', 'sha256', 'mime', 'encoding']} for row in rows}}
    return pin, rows


def validate(pin):
    valid = lambda value: isinstance(value, str) and re.fullmatch('[a-f0-9]{64}', value)
    if pin.get('version') != 1 or not pin.get('cdnVerified') or not valid(pin.get('assetID')) or pin.get('sources') != sources() or not valid(pin.get('currentCore')) or not isinstance(pin.get('retainedCores'), list) or len(pin['retainedCores']) > 64:
        raise ValueError('server publication is unverified or belongs to different native sources')
    cores = [pin['currentCore'], *pin['retainedCores']]
    if len(set(cores)) != len(cores) or not all(valid(core) for core in cores):
        raise ValueError('invalid server publication core inventory')
    names = {'manifest.json', 'authority.zip', *('cores/' + core + '.wasm' for core in cores)}
    if set(pin.get('files', {})) != names:
        raise ValueError('server publication file inventory differs')
    for name, row in pin['files'].items():
        mime = 'application/json' if name == 'manifest.json' else 'application/zip' if name == 'authority.zip' else 'application/wasm'
        if not row.get('url', '').startswith('https://cdn.ichenj.com/bga/') or re.search(r'[?#\\]|/\.\./', row['url']) or type(row.get('bytes')) is not int or not 0 < row['bytes'] <= 16777216 or not valid(row.get('sha256')) or row.get('mime') != mime or row.get('encoding') != ('' if mime == 'application/zip' else 'gzip'):
            raise ValueError('invalid server publication object')
    if pin['files']['manifest.json']['sha256'] != pin['assetID']:
        raise ValueError('server publication manifest identity differs')


def decode(row, body, headers):
    if headers.get('Content-Type', '').split(';')[0] != row['mime'] or headers.get('Content-Encoding', '') != row['encoding'] or len(body) > row['bytes'] + 65536:
        raise ValueError('server CDN response format or size differs')
    if row['encoding']:
        decoder = zlib.decompressobj(16 + zlib.MAX_WBITS)
        try:
            data = decoder.decompress(body, row['bytes'] + 1)
        except zlib.error as error:
            raise ValueError('invalid server CDN gzip') from error
        if len(data) > row['bytes'] or not decoder.eof or decoder.unused_data or decoder.unconsumed_tail:
            raise ValueError('server CDN decoded payload exceeds its limit')
    else:
        data = body
    if len(data) != row['bytes'] or sha(data) != row['sha256']:
        raise ValueError('server CDN object digest differs')
    return data


def download(row):
    def operation():
        request = urllib.request.Request(row['url'], headers={'Origin': audio.ORIGIN, 'Accept-Encoding': 'gzip'})
        with cdn.urlopen(request, timeout=90) as response:
            data = decode(row, response.read(row['bytes'] + 65537), response.headers)
            if response.headers.get('Access-Control-Allow-Origin') not in ['*', audio.ORIGIN]:
                raise ValueError('server CDN CORS differs')
            return data
    return cdn.retry_request(operation, row['url'])


def matches(folder, pin):
    manifest, files = package_files(folder)
    if sha(files['manifest.json']) != pin['assetID'] or manifest['current'] != pin['currentCore'] or manifest['retained'] != pin['retainedCores']:
        raise ValueError('prepared server package differs from publication')
    for name, row in pin['files'].items():
        if len(files[name]) != row['bytes'] or sha(files[name]) != row['sha256']:
            raise ValueError('prepared server object differs from publication')


def prepare(pin, target, fetch=None):
    validate(pin)
    target = Path(target).absolute()
    guard(target)
    relative = target.relative_to(ROOT)
    if target.name != 'pal' or not (relative.parts[:2] == ('.local', 'pal') or relative.as_posix() == 'dist/package/pal'):
        raise ValueError('server rules must use a dedicated workspace package/pal directory')
    if target.exists():
        matches(target, pin)
        return {'passed': True, 'reused': True, 'assetID': pin['assetID'], 'package': str(relative)}
    target.parent.mkdir(parents=True, exist_ok=True)
    fetch = download if fetch is None else fetch
    with tempfile.TemporaryDirectory(prefix='staging-', dir=target.parent) as temporary:
        folder = Path(temporary)
        for name, row in sorted(pin['files'].items()):
            data = fetch(row)
            if len(data) != row['bytes'] or sha(data) != row['sha256']:
                raise ValueError('downloaded server object differs')
            path = folder / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
        matches(folder, pin)
        folder.rename(target)
    return {'passed': True, 'reused': False, 'assetID': pin['assetID'], 'package': str(relative)}


def verify(folder):
    result = subprocess.run(['go', 'run', './games/pal/build/cmd/verify-production', '--package', str(folder)], cwd=ROOT, check=True, capture_output=True, text=True)
    report = json.loads(result.stdout)
    if not report.get('passed'):
        raise ValueError('server production bundle failed initialization')
    return report
