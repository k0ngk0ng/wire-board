import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
from email.utils import formatdate
import fnmatch
import hashlib
import gzip
import hmac
import json
import mimetypes
import os
from pathlib import Path
import re
import shlex
import subprocess
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
RESOURCE_SUFFIXES = {'.png', '.webp', '.jpg', '.jpeg', '.gif', '.svg', '.bmp', '.ico', '.tif', '.tiff', '.avif', '.wav', '.mp3', '.ogg', '.flac', '.aac', '.m4a', '.mp4', '.webm', '.mov', '.woff', '.woff2', '.ttf', '.otf', '.wasm', '.gz', '.xz', '.zip', '.7z', '.tar', '.rar', '.glb', '.gltf', '.rgba', '.rgb', '.pvzr', '.fflate', '.bin', '.rom', '.nes', '.sfc', '.smc', '.mdx', '.exe', '.dll', '.so', '.dylib', '.pdf'}
CONFIG_NAMES = ['PUBLIC_ORIGIN', 'CDN_ORIGIN', 'OSS_BUCKET', 'OSS_ENDPOINT', 'ASSETS_BASE_URL']
RESOURCE_KEY = re.compile(r'bga/([a-z0-9_-]+)/([a-f0-9]{64})(-gzip)?(\.[a-z0-9]+)')
OWNERS = {'_common'} | {path.name for path in (ROOT / 'games').iterdir() if path.is_dir()}
MIME = {'.webp': 'image/webp', '.svg': 'image/svg+xml', '.wasm': 'application/wasm', '.wav': 'audio/wav', '.flac': 'audio/flac', '.glb': 'model/gltf-binary', '.pvzr': 'application/octet-stream', '.fflate': 'application/octet-stream', '.rgba': 'application/octet-stream', '.rgb': 'application/octet-stream', '.gz': 'application/gzip', '.xz': 'application/x-xz'}


def configuration(root=ROOT, required=('PUBLIC_ORIGIN', 'CDN_ORIGIN'), environment=True):
    path = Path(root) / '.local/business.json'
    result = json.loads(path.read_text()) if os.environ.get('GITHUB_ACTIONS') != 'true' and path.is_file() else {}
    if environment:
        result.update({key: os.environ[key] for key in CONFIG_NAMES if os.environ.get(key)})
    if any(not result.get(key) for key in required):
        raise ValueError('Missing business configuration: ' + ', '.join(key for key in required if not result.get(key)))
    for key in ['PUBLIC_ORIGIN', 'CDN_ORIGIN']:
        if key in result:
            url = urllib.parse.urlsplit(result[key])
            if url.scheme != 'https' or not url.netloc or url.username or url.password or url.path not in ['', '/'] or url.query or url.fragment:
                raise ValueError('Expected HTTPS origin for ' + key)
            result[key] = result[key].rstrip('/')
    return result


def export_environment(root=ROOT):
    config = configuration(root, environment=False)
    path = Path(root) / '.local/business.env'
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(''.join(key + '=' + shlex.quote(str(config[key])) + '\n' for key in CONFIG_NAMES if key in config))
    return path


def is_resource(path, source=None):
    payload = Path(path).suffix.lower() in RESOURCE_SUFFIXES or '/assets/public/' in path or path.startswith('web/public/') or bool(re.search(r'/(?:fontglyph[^/]*|ascii)\.h$', path)) or path.endswith('.gz.before-private-io')
    return payload or source is not None and Path(path).suffix.lower() in ['.json', '.jsonl', '.log', '.html', '.txt'] and embedded_resources(source)


def forbidden_origins(source, origins):
    return [urllib.parse.urlsplit(origin).hostname for origin in origins if urllib.parse.urlsplit(origin).hostname and urllib.parse.urlsplit(origin).hostname.casefold() in source.casefold()]


def embedded_resources(source):
    return bool(re.search(r'(?:iVBORw0KGg|/9j/|R0lGOD|T2dnUw|ZkxhQw|UklGR|AGFzbQ|H4sI)[A-Za-z0-9+/=]{64,}|data:(?:image|audio|video|font)/[A-Za-z0-9.+-]+;base64,[A-Za-z0-9+/=]{64,}', source))


def owner_for(path):
    parts = Path(path).parts
    if len(parts) > 1 and parts[0] == 'games' and parts[1] in OWNERS:
        return parts[1]
    for part in parts:
        if part in OWNERS:
            return part
        for owner in sorted(OWNERS, key=len, reverse=True):
            if owner != '_common' and (part.lower().startswith(owner + '-') or part.lower().endswith('-' + owner)):
                return owner
    return '_common'


def object_key(owner, body, suffix, encoding=''):
    if owner not in OWNERS or not re.fullmatch(r'\.[a-z0-9]+', suffix):
        raise ValueError('Invalid resource owner or file extension')
    if encoding not in ['', 'gzip']:
        raise ValueError('Invalid resource encoding')
    return 'bga/' + owner + '/' + hashlib.sha256(body).hexdigest() + ('-gzip' if encoding else '') + suffix


def validate_record(record):
    match = RESOURCE_KEY.fullmatch(record.get('key', ''))
    if not match or match[1] not in OWNERS or match[2] != record.get('sha256') or type(record.get('bytes')) is not int or record['bytes'] <= 0 or not isinstance(record.get('mime'), str):
        raise ValueError('Invalid resource record')
    if bool(match[3]) != (record.get('encoding', '') == 'gzip') or record.get('encoding', '') not in ['', 'gzip']:
        raise ValueError('Resource encoding differs from its immutable key')
    return record


def published_url(entry, owner, origin):
    target = urllib.parse.urlsplit(origin)
    if target.scheme != 'https' or not target.netloc or target.username or target.password or target.path not in ['', '/'] or target.query or target.fragment:
        raise ValueError('Invalid configured CDN origin')
    if 'url' in entry:
        source = urllib.parse.urlsplit(entry['url'])
        if source.scheme != 'https' or not source.netloc or source.username or source.password or source.query or source.fragment:
            raise ValueError('Invalid published resource URL')
        key = source.path.lstrip('/')
    else:
        suffix = Path(entry.get('file') or entry.get('output', '')).suffix.lower()
        key = entry.get('key') or 'bga/' + owner + '/' + entry['sha256'] + ('-gzip' if entry.get('encoding') == 'gzip' else '') + suffix
    match = RESOURCE_KEY.fullmatch(key)
    if not match or owner not in OWNERS or match[1] != owner or match[2] != entry['sha256'] or bool(match[3]) != (entry.get('encoding', '') == 'gzip') or entry.get('encoding', '') not in ['', 'gzip']:
        raise ValueError('Published resource belongs to another game')
    return origin.rstrip('/') + '/' + key


def validate_body(record, body, mime):
    if record.get('encoding') == 'gzip':
        body = gzip.decompress(body)
    if len(body) != record['bytes'] or hashlib.sha256(body).hexdigest() != record['sha256'] or mime != record['mime']:
        raise ValueError('Resource SHA-256, size or MIME mismatch: ' + record['key'])
    return body


class ObjectStore:
    def __init__(self, endpoint, bucket, key, secret, local=False):
        url = urllib.parse.urlsplit(endpoint)
        if url.scheme != 'https' and not (local and url.scheme == 'http' and url.hostname in ['127.0.0.1', 'localhost']):
            raise ValueError('Expected HTTPS OSS endpoint')
        self.endpoint = endpoint.rstrip('/')
        self.bucket = bucket
        self.key = key
        self.secret = secret
        self.urlopen = urllib.request.urlopen

    def request(self, method, key, body=None, mime='', extra_headers=None, encoding=''):
        if not re.fullmatch(r'bga/[A-Za-z0-9._/-]+', key) or any(part.startswith('.') for part in key.split('/')):
            raise ValueError('Unsafe OSS resource key')
        date = formatdate(usegmt=True)
        digest = base64.b64encode(hashlib.md5(body).digest()).decode() if body is not None else ''
        oss_headers = {'x-oss-forbid-overwrite': 'true'} if method == 'PUT' else {}
        oss_headers.update(extra_headers or {})
        extra = ''.join(name + ':' + value + '\n' for name, value in sorted(oss_headers.items()))
        canonical = method + '\n' + digest + '\n' + mime + '\n' + date + '\n' + extra + '/' + self.bucket + '/' + key
        signature = base64.b64encode(hmac.new(self.secret.encode(), canonical.encode(), hashlib.sha1).digest()).decode()
        headers = {'Date': date, 'Authorization': 'OSS ' + self.key + ':' + signature}
        headers.update(oss_headers)
        if method == 'PUT' and body is not None:
            headers.update({'Content-MD5': digest, 'Content-Type': mime, 'x-oss-forbid-overwrite': 'true', 'Cache-Control': 'public, max-age=31536000, immutable'})
        if encoding:
            headers['Content-Encoding'] = encoding
        request = urllib.request.Request(self.endpoint + '/' + key, data=body, method=method, headers=headers)
        with self.urlopen(request, timeout=60) as response:
            return response.read(), response.headers.get_content_type()

    def get(self, key):
        try:
            return self.request('GET', key)
        except urllib.error.HTTPError as error:
            error.close()
            if error.code == 404:
                return None
            raise

    def put(self, key, body, mime, encoding=''):
        return self.request('PUT', key, body, mime, encoding=encoding)

    def copy(self, key, source):
        if not re.fullmatch(r'(?:bga|stoneage)/[A-Za-z0-9._/-]+', source) or any(part.startswith('.') for part in source.split('/')):
            raise ValueError('Unsafe OSS copy source')
        return self.request('PUT', key, extra_headers={'x-oss-copy-source': '/' + self.bucket + '/' + source})


def publish(client, owner, filename, body, mime=None, encoding=''):
    suffix = Path(filename).suffix.lower()
    mime = mime or MIME.get(suffix) or mimetypes.guess_type(filename)[0] or 'application/octet-stream'
    record = {'key': object_key(owner, body, suffix, encoding), 'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest(), 'mime': mime}
    if encoding:
        record['encoding'] = encoding
    validate_record(record)
    previous = client.get(record['key'])
    if previous is not None:
        validate_body(record, *previous)
        return record
    try:
        client.put(record['key'], gzip.compress(body, mtime=0) if encoding else body, mime, encoding)
    except urllib.error.HTTPError as error:
        error.close()
        if error.code != 409:
            raise
    previous = client.get(record['key'])
    if previous is None:
        raise ValueError('Uploaded resource is missing')
    validate_body(record, *previous)
    return record


def copy_resource(client, record, source):
    validate_record(record)
    previous = client.get(record['key'])
    if previous is not None:
        validate_body(record, *previous)
        return record
    try:
        client.copy(record['key'], source)
    except urllib.error.HTTPError as error:
        error.close()
        if error.code != 409:
            raise
    previous = client.get(record['key'])
    if previous is None:
        raise ValueError('Copied resource is missing')
    validate_body(record, *previous)
    return record


def publish_manifest(client, manifest, root, owner, origin, execute=False, pack=None):
    if owner not in OWNERS or not isinstance(manifest.get('entries'), list) or not manifest['entries']:
        raise ValueError('Invalid game resource manifest')
    prepared = []
    records = {}
    for entry in manifest['entries']:
        logical = entry.get('urlPath') or entry.get('file')
        if not isinstance(logical, str) or logical in records:
            raise ValueError('Missing or duplicate logical resource path')
        paths = []
        if entry.get('output'):
            paths.append(destination(Path(root), entry['output']))
        paths.append(destination(Path(root), 'games/' + owner + '/assets/public/' + logical.lstrip('/')))
        if pack is not None and entry.get('file'):
            paths.append(destination(Path(root), str(Path(pack).relative_to(root) / entry['file'])))
        target = next((path for path in paths if path.is_file()), None)
        if target is None:
            raise ValueError('Missing local resource: ' + logical)
        body = target.read_bytes()
        if len(body) != entry.get('bytes') or hashlib.sha256(body).hexdigest() != entry.get('sha256'):
            raise ValueError('Local resource differs from manifest: ' + logical)
        suffix = target.suffix.lower()
        mime = entry.get('mime') or MIME.get(suffix) or mimetypes.guess_type(target.name)[0] or 'application/octet-stream'
        encoding = entry.get('encoding', '')
        record = {'key': object_key(owner, body, suffix, encoding), 'bytes': len(body), 'sha256': entry['sha256'], 'mime': mime}
        if encoding:
            record['encoding'] = encoding
        validate_record(record)
        records[logical] = {**record, 'url': origin.rstrip('/') + '/' + record['key']}
        prepared.append((target, record))
    verified = {}
    if execute:
        if client is None:
            raise ValueError('Publication requires an OSS client')
        for target, record in prepared:
            if record['key'] in verified:
                continue
            body = target.read_bytes()
            if len(body) != record['bytes'] or hashlib.sha256(body).hexdigest() != record['sha256']:
                raise ValueError('Resource changed during publication')
            verified[record['key']] = publish(client, owner, target.name, body, mime=record['mime'], encoding=record.get('encoding', ''))
    return {'passed': True, 'mode': 'published' if execute else 'preflight', 'game': owner, 'logicalFiles': len(records), 'uniqueObjects': len({record['key'] for record in records.values()}), 'files': records}


def destination(root, path):
    parts = Path(path).parts
    if not path or Path(path).is_absolute() or Path(path).as_posix() != path or any(part.startswith('.') or part == '' for part in parts):
        raise ValueError('Unsafe checkout resource path')
    target = root / path
    current = root
    for part in parts:
        current = current / part
        if current.is_symlink():
            raise ValueError('Symlink in checkout resource path')
    if not target.resolve().is_relative_to(root.resolve()):
        raise ValueError('Resource escapes checkout')
    return target


def hydration_paths(registry, root, game):
    if game is None:
        return None
    if game not in OWNERS:
        raise ValueError('Unknown resource game')
    selected = {name for name in registry['files'] if owner_for(name) in [game, '_common']}
    sources = [*Path(root).glob('*.go'), *(Path(root) / 'games').rglob('*.go'), *(Path(root) / 'internal').rglob('*.go')]
    for source in sources:
        if source.name.endswith('_test.go'):
            continue
        directives = re.findall(r'^\s*//go:embed[ \t]+(.+)$', source.read_text(), re.M)
        if not directives:
            continue
        prefix = source.parent.relative_to(root).as_posix()
        prefix = '' if prefix == '.' else prefix + '/'
        for directive in directives:
            for pattern in shlex.split(directive):
                parts = pattern.removeprefix('all:').split('/')
                for name in registry['files']:
                    if not name.startswith(prefix):
                        continue
                    relative = name[len(prefix):].split('/')
                    if len(relative) >= len(parts) and all(fnmatch.fnmatchcase(value, mask) for value, mask in zip(relative, parts)):
                        selected.add(name)
    return selected


def hydrate(registry, root, origin, local=False, paths=None, client=None):
    url = urllib.parse.urlsplit(origin)
    if url.scheme != 'https' and not (local and url.scheme == 'http' and url.hostname in ['127.0.0.1', 'localhost']):
        raise ValueError('Expected HTTPS CDN origin')
    if registry.get('version') != 1 or not isinstance(registry.get('files'), dict):
        raise ValueError('Invalid source resource registry')
    selected = [(path, validate_record(record), destination(Path(root), path)) for path, record in registry['files'].items() if paths is None or path in paths]
    def download(item):
        path, record, target = item
        if target.is_file() and len(target.read_bytes()) == record['bytes'] and hashlib.sha256(target.read_bytes()).hexdigest() == record['sha256']:
            return path
        if client is None:
            with urllib.request.urlopen(origin.rstrip('/') + '/' + record['key'], timeout=60) as response:
                body = validate_body(record, response.read(), response.headers.get_content_type())
        else:
            response = client.get(record['key'])
            if response is None:
                raise ValueError('Missing source resource: ' + record['key'])
            body = validate_body(record, *response)
        target.parent.mkdir(parents=True, exist_ok=True)
        temporary = target.with_name(target.name + '.resource-tmp')
        try:
            temporary.write_bytes(body)
            temporary.replace(target)
        finally:
            temporary.unlink(missing_ok=True)
        return path
    with ThreadPoolExecutor(max_workers=32) as pool:
        return list(pool.map(download, selected))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['hydrate', 'check', 'publish', 'export-env'])
    parser.add_argument('--registry', type=Path, default=ROOT / 'assets/resources.json')
    parser.add_argument('--game')
    parser.add_argument('--oss', action='store_true')
    parser.add_argument('--manifest', type=Path)
    parser.add_argument('--pack', type=Path)
    parser.add_argument('--execute', action='store_true')
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    if args.action == 'export-env':
        export_environment()
        return
    config = configuration()
    if args.action == 'publish':
        if not args.manifest or not args.game or not args.manifest.resolve().is_relative_to(ROOT):
            raise ValueError('Publication requires a repository manifest and game ID')
        client = None
        if args.execute:
            config = configuration(required=CONFIG_NAMES[:4])
            key = os.environ.get('OSS_ACCESS_KEY_ID') or (ROOT / 'config/secrets/oss-access-key-id').read_text().strip()
            secret = os.environ.get('OSS_ACCESS_KEY_SECRET') or (ROOT / 'config/secrets/oss-access-key-secret').read_text().strip()
            client = ObjectStore('https://' + config['OSS_ENDPOINT'], config['OSS_BUCKET'], key, secret)
        result = publish_manifest(client, json.loads(args.manifest.read_text()), ROOT, args.game, config['CDN_ORIGIN'], args.execute, args.pack.resolve() if args.pack else None)
        report = args.report or ROOT / '.local/resource-publish' / args.game / 'result.json'
        if not report.resolve().is_relative_to(ROOT):
            raise ValueError('Publication report must remain inside the repository')
        report.parent.mkdir(parents=True, exist_ok=True)
        report.write_text(json.dumps(result, indent=2) + '\n')
        print(json.dumps({'passed': True, 'mode': result['mode'], 'objects': result['uniqueObjects'], 'report': str(report)}))
        return
    if args.action == 'hydrate':
        registry = json.loads(args.registry.read_text())
        paths = hydration_paths(registry, ROOT, args.game)
        client = None
        if args.oss:
            config = configuration(required=CONFIG_NAMES[:4])
            key = os.environ.get('OSS_ACCESS_KEY_ID') or (ROOT / 'config/secrets/oss-access-key-id').read_text().strip()
            secret = os.environ.get('OSS_ACCESS_KEY_SECRET') or (ROOT / 'config/secrets/oss-access-key-secret').read_text().strip()
            client = ObjectStore('https://' + config['OSS_ENDPOINT'], config['OSS_BUCKET'], key, secret)
        restored = hydrate(registry, ROOT, config['CDN_ORIGIN'], paths=paths, client=client)
        print(json.dumps({'passed': True, 'restored': len(restored)}))
        return
    paths = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT).decode().split('\0')[:-1]
    violations = []
    for path in paths:
        if is_resource(path):
            violations.append({'path': path, 'rule': 'no-resource-payloads'})
        target = ROOT / path
        if target.is_file():
            source = target.read_text(errors='replace')
            if embedded_resources(source):
                violations.append({'path': path, 'rule': 'no-embedded-resource-payloads'})
            for origin in forbidden_origins(source, [config['PUBLIC_ORIGIN']]):
                violations.append({'path': path, 'rule': 'no-business-origin'})
    if violations:
        raise ValueError(json.dumps(violations))
    print(json.dumps({'passed': True, 'files': len(paths), 'rules': ['no-resource-payloads', 'no-business-origin']}))


if __name__ == '__main__':
    main()
