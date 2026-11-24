import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
from urllib.parse import urlsplit
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
import cdn_delivery as cdn
from resource_store import configuration, OWNERS

URL = re.compile(re.escape(configuration()['CDN_ORIGIN']) + r'/bga/(?:web/[a-f0-9]{40}/|[a-z_][a-z0-9_-]*/)[A-Za-z0-9._/-]+\Z')
ENCODED = {'.js', '.css', '.svg', '.json', '.txt', '.md', '.glb'}


def valid_url(value):
    if not isinstance(value, str) or not URL.fullmatch(value):
        raise ValueError('Invalid published asset URL')
    path = urlsplit(value).path
    if PurePosixPath(path).as_posix() != path:
        raise ValueError('Noncanonical published asset URL')
    if any(part.startswith('.') for part in path.split('/') if part):
        raise ValueError('Private or aliased published asset URL')
    return value


def normalized(record):
    value = {key: record[key] for key in ['url', 'bytes', 'sha256', 'mime']}
    valid_url(value['url'])
    if type(value['bytes']) is not int or value['bytes'] <= 0 or not re.fullmatch('[a-f0-9]{64}', value['sha256']) or not isinstance(value['mime'], str) or not value['mime']:
        raise ValueError('Invalid published asset identity')
    value['encoding'] = record.get('encoding', 'gzip' if 'key' not in record and PurePosixPath(urlsplit(value['url']).path).suffix in ENCODED else '')
    if value['encoding'] not in ['', 'gzip']:
        raise ValueError('Invalid published encoding')
    return value


def identity(row):
    return tuple(row[key] for key in ['sha256', 'bytes', 'mime', 'encoding'])


def objects(baseline):
    result = {}
    for raw in list(baseline.get('objects', {}).values()) + list(baseline.get('files', {}).values()) + list(baseline.get('external', {}).values()):
        row = normalized(raw)
        previous = result.get(row['url'])
        if previous is not None and identity(previous) != identity(row):
            raise ValueError('Published URL has conflicting identities')
        result[row['url']] = row
    return result


def plan(rows, baseline, owners=None):
    if owners is not None:
        canonical = []
        for original in rows:
            row = dict(original)
            path = row['path']
            owner = owners.get(path, path.split('/')[0] if path.split('/')[0] in OWNERS else '_common')
            if Path(path).name == 'wasm_exec.js':
                owner = '_common'
            if owner not in OWNERS:
                raise ValueError('Unknown resource owner')
            base = row['url'].split('/bga/')[0] + '/bga/'
            filename = Path(path).name
            if path.startswith('assets/') and re.search(r'-[A-Za-z0-9_-]{8,}\.(?:js|css)$', filename):
                key = filename
            else:
                key = row['sha256'] + ('-gzip' if row.get('encoding') == 'gzip' else '') + Path(path).suffix
            row['url'] = base + owner + '/' + key
            canonical.append(row)
        return canonical
    known = objects(baseline)
    by_identity = {}
    for url, row in sorted(known.items()):
        by_identity.setdefault(identity(row), url)
    by_path = {path.lstrip('/'): normalized(row) for path, row in baseline.get('files', {}).items()}
    result = []
    for raw in sorted(rows, key=lambda row: row['path']):
        row = dict(raw)
        previous = by_path.get(row['path'])
        key = identity(row)
        if previous is not None and identity(previous) == key:
            row['url'] = previous['url']
        elif key in by_identity:
            row['url'] = by_identity[key]
        else:
            by_identity[key] = row['url']
        result.append(row)
    return result


def relative_asset(value, parent, paths):
    if value in paths:
        return value
    parts = []
    for part in (PurePosixPath(parent).parent / value).parts:
        if part == '..':
            if not parts:
                return None
            parts.pop()
        elif part not in ['', '.']:
            parts.append(part)
    path = '/'.join(parts)
    return path if path in paths else None


def rewritten(source, parent, rows, base):
    text = source.decode()
    urls = {row['path']: row['url'] for row in rows}
    for path, target in urls.items():
        if target != base + path:
            text = text.replace(base + path, target)
    if parent.endswith('.js'):
        text = re.sub(r'\breturn\s*' + re.escape(json.dumps(base)) + r'\+([A-Za-z_$][\w$]*)(?=[;}])', lambda match: 'return new URL(' + match[1] + ',' + json.dumps(base) + ').href', text)
        def quoted(match):
            quote, value = match[1], match[2]
            path = relative_asset(value, parent, urls)
            if path is not None and path.startswith('assets/'):
                return quote + urls[path] + quote
            return match[0]
        text = re.sub(r'([\"\'])([./A-Za-z0-9_-]+\.(?:js|css|wasm|png|webp|svg|jpg|woff2?))\1', quoted, text)
    elif parent.endswith('.css'):
        def css_url(match):
            value = match[1].strip().strip('\"\'')
            path = relative_asset(value, parent, urls)
            return 'url(' + json.dumps(urls[path]) + ')' if path is not None else match[0]
        text = re.sub(r'url\(([^)]+)\)', css_url, text)
    return text.encode()


def prepare(dist, baseline):
    if dist.is_symlink():
        raise ValueError('Symlink build output')
    dist = dist.resolve()
    if dist == ROOT or not dist.is_relative_to(ROOT) or dist.is_symlink():
        raise ValueError('Build output must remain inside the repository')
    file = dist / 'game-load-manifest.json'
    manifest = json.loads(file.read_bytes())
    base = manifest['assetBase']
    if not cdn.BASE.fullmatch(base):
        raise ValueError('Expected exact build base')
    delivery_path = dist / 'cdn-assets-manifest.json'
    previous_delivery = json.loads(delivery_path.read_bytes()) if delivery_path.exists() else {}
    original_mapping = {path.lstrip('/'): row for path, row in previous_delivery.get('files', {}).items()} or manifest.get('assetMap', {})
    if delivery_path.exists():
        delivery_path.unlink()
    reverse = {row['url']: path for path, row in original_mapping.items()}
    def logical(record):
        if 'path' in record:
            return record['path']
        url = record['url']
        if url.startswith(base):
            return url[len(base):]
        if url in reverse:
            return reverse[url]
        raise ValueError('Unknown program reference')
    groups = [[(record, logical(record)) for record in group] for group in manifest['games'].values()]
    home = manifest.get('home', {})
    home_group = [(record, logical(record)) for record in home.get('files', [])]
    entry = manifest.get('entryPath') or logical({'url': manifest['entry']})
    worker = manifest.get('workerPath') or (logical({'url': manifest['worker']}) if manifest.get('worker') else None)
    manifest.pop('assetMap', None)
    manifest.pop('assetObjects', None)
    file.write_text(json.dumps(manifest, separators=(',', ':')) + '\n')
    owners = manifest.get('owners', {}) if base.endswith('/bga/') else None
    def assign(rows):
        return plan(rows, baseline, owners=owners)
    originals = {str(p.relative_to(dist)): p.read_bytes() for p in (dist / 'assets').rglob('*') if p.is_file() and p.suffix in ['.js', '.css']}
    html = (dist / 'index.html').read_bytes()
    def write_programs(rows):
        for path, source in originals.items():
            data = rewritten(source, path, rows, base)
            (dist / path).write_bytes(data)
            (dist / (path + '.gz')).write_bytes(gzip.compress(data, mtime=0))
        (dist / 'index.html').write_bytes(rewritten(html, 'index.html', rows, base))
    candidates = assign(cdn.inventory(dist, base))
    published = {path.lstrip('/'): normalized(record) for path, record in baseline.get('files', {}).items()}
    seeded = False
    for row in candidates:
        previous = published.get(row['path'])
        if row['path'] in originals and previous is not None and row['mime'] == previous['mime'] and row['encoding'] == previous['encoding'] and row['url'] != previous['url'] and owners is None:
            row['url'] = previous['url']
            seeded = True
    if seeded:
        write_programs(candidates)
    previous_urls = None
    for iteration in range(16):
        rows = assign(cdn.inventory(dist, base))
        urls = {row['path']: row['url'] for row in rows}
        if urls == previous_urls:
            break
        previous_urls = urls
        write_programs(rows)
    else:
        raise ValueError('Published program references did not converge')
    rows = assign(cdn.inventory(dist, base))
    by_path = {row['path']: row for row in rows}
    for group in groups + [home_group]:
        for record, path in group:
            record.update(path=path, url=by_path[path]['url'], bytes=by_path[path]['bytes'])
    if home_group:
        home['bytes'] = sum(record['bytes'] for record, _ in home_group)
    known = {url: dict(record, verified=True) for url, record in objects(baseline).items()}
    old_urls = set(known)
    for row in rows:
        value = normalized(row)
        old = known.get(value['url'])
        if old is not None and identity(old) != identity(value):
            raise ValueError('Attempt to change a published object')
        value['verified'] = value['url'] in old_urls
        known[value['url']] = value
    old_urls = set(objects(baseline))
    unique = {row['url']: row for row in rows}
    summary = {'logicalFiles': len(rows), 'reusedObjects': sum(url in old_urls for url in unique), 'reusedBytes': sum(row['bytes'] for url, row in unique.items() if url in old_urls), 'newObjects': sum(url not in old_urls for url in unique), 'uploadBytes': sum(row['wireBytes'] for url, row in unique.items() if url not in old_urls)}
    manifest.update(mediaBase='/bga' if owners is not None else '/_media', entry=by_path[entry]['url'], entryPath=entry, assetPlan=summary, build=cdn.sha((dist / 'index.html').read_bytes()))
    if worker:
        manifest.update(worker=by_path[worker]['url'], workerPath=worker)
    delivery = {'version': 1, 'assetBase': base, 'files': {'/' + row['path']: normalized(row) for row in rows}, 'objects': known}
    external = dict(manifest.get('external', {}))
    if manifest.get('externalRegistrySHA256'):
        external_raw = (ROOT / 'assets/external-resources.json').read_bytes()
        if cdn.sha(external_raw) != manifest['externalRegistrySHA256']:
            raise ValueError('External resource registry changed after the build')
        origin = base.split('/bga/')[0]
        for path, record in json.loads(external_raw)['files'].items():
            external[path] = {**record, 'url': origin + '/' + record['key']}
    if external:
        delivery['external'] = {path: normalized(record) for path, record in external.items() if path.lstrip('/') not in by_path}
    if manifest.get('sourceSHA'):
        delivery['sourceSHA'] = manifest['sourceSHA']
    delivery_path.write_text(json.dumps(delivery, separators=(',', ':'), sort_keys=True) + '\n')
    file.write_text(json.dumps(manifest, separators=(',', ':'), sort_keys=True) + '\n')
    return summary


def successful_readback(sha, required=True):
    if not re.fullmatch('[a-f0-9]{40}', sha):
        raise ValueError('Invalid source identity')
    rows = json.loads(subprocess.check_output(['gh', 'api', 'repos/k0ngk0ng/wire-board/commits/' + sha + '/statuses?per_page=100'], text=True))
    status = next((row for row in rows if row['context'] == 'cdn-local-readback'), None)
    if status is None or status['state'] != 'success' or not re.fullmatch(r'run [0-9]+; sha256=[a-f0-9]{64}', status.get('description', '')):
        if required:
            raise ValueError('Baseline has no successful local CDN readback')
        return False
    return int(status['description'].split()[1].rstrip(';'))


def published_baseline():
    with urlopen(configuration()['PUBLIC_ORIGIN'] + '/cdn-assets-manifest.json', timeout=30) as response:
        baseline = json.load(response)
    base = baseline['assetBase']
    if not cdn.BASE.fullmatch(base):
        raise ValueError('Invalid deployed asset base')
    source_sha = baseline.get('sourceSHA') or base.rstrip('/').split('/')[-1]
    deployed_verified = successful_readback(source_sha, required=False)
    sha = os.environ.get('VERIFIED_ANCESTOR', '')
    source_run = successful_readback(sha, required=False) if re.fullmatch('[a-f0-9]{40}', sha) else None
    if not deployed_verified:
        if not source_run:
            raise ValueError('Baseline has no successful local CDN readback')
        baseline = {'version': 1, 'assetBase': base, 'files': {}}
    objects(baseline)
    if source_run:
        runs = json.loads(subprocess.check_output(['gh', 'api', 'repos/k0ngk0ng/wire-board/actions/workflows/build.yml/runs?head_sha=' + sha + '&status=success&per_page=100'], text=True))['workflow_runs']
        run = next((row for row in runs if row['id'] == source_run and row['head_sha'] == sha and row['head_branch'] == 'main' and row['event'] == 'push'), None)
        if run is None:
            raise ValueError('Verified ancestor build is missing')
        folder = ROOT / '.local/asset-reuse' / ('baseline-' + sha)
        receipt_path = folder.with_suffix('.receipt.json')
        cached = folder.exists()
        if not cached:
            folder.mkdir(parents=True)
            subprocess.run(['gh', 'run', 'download', str(run['id']), '--name', 'web-dist', '--dir', str(folder)], check=True)
        elif not receipt_path.is_file():
            raise ValueError('Incomplete baseline cache')
        previous = json.loads((folder / 'game-load-manifest.json').read_bytes())
        if previous.get('sourceSHA') != sha and previous['assetBase'] != configuration()['CDN_ORIGIN'] + '/bga/web/' + sha + '/':
            raise ValueError('Ancestor artifact is not bound to its source')
        rows = cdn.inventory(folder, previous['assetBase'])
        receipt = {'sourceSHA': sha, 'sourceRun': source_run, 'files': rows}
        if cached and json.loads(receipt_path.read_bytes()) != receipt:
            raise ValueError('Verified baseline cache changed')
        if not cached:
            receipt_path.write_text(json.dumps(receipt, separators=(',', ':')) + '\n')
        history = objects(baseline)
        delivery = folder / 'cdn-assets-manifest.json'
        if delivery.exists():
            ancestor = json.loads(delivery.read_bytes())
            history.update(objects(ancestor))
            if not deployed_verified:
                baseline = ancestor
        for row in rows:
            history.setdefault(row['url'], normalized(row))
        baseline['objects'] = history
    return baseline


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--dist', type=Path, default=ROOT / 'web/dist')
    parser.add_argument('--baseline', type=Path)
    args = parser.parse_args()
    if args.baseline is not None:
        if not args.baseline.resolve().is_relative_to(ROOT):
            raise ValueError('Baseline must remain in the repository')
        baseline = json.loads(args.baseline.read_bytes())
    else:
        baseline = published_baseline()
    summary = prepare(args.dist, baseline)
    folder = ROOT / '.local/asset-reuse'
    folder.mkdir(parents=True, exist_ok=True)
    (folder / 'plan.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary))


if __name__ == '__main__':
    main()
