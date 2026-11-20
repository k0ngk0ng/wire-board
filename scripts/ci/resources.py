import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
from email.utils import formatdate
import fnmatch
import hashlib
import hmac
from http.client import IncompleteRead
import json
import os
from pathlib import Path
import re
import shlex
import sys
import threading
from urllib.parse import urlsplit
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'scripts'))
from resource_store import configuration, destination, owner_for, validate_body, validate_record

POLICY = json.loads((Path(__file__).parent / 'traffic-policy.json').read_text())


def embedded_paths(root, files):
    selected = set()
    sources = [*root.glob('*.go'), *(root / 'games').rglob('*.go'), *(root / 'internal').rglob('*.go')]
    for source in sources:
        if source.name.endswith('_test.go'):
            continue
        prefix = source.parent.relative_to(root).as_posix()
        prefix = '' if prefix == '.' else prefix + '/'
        for directive in re.findall(r'^\s*//go:embed[ \t]+(.+)$', source.read_text(), re.M):
            for pattern in shlex.split(directive):
                pattern = prefix + pattern.removeprefix('all:')
                selected.update(name for name in files if fnmatch.fnmatchcase(name, pattern) or name.startswith(pattern.rstrip('/') + '/'))
    return selected


def imported_paths(root, files):
    selected = set()
    directories = [root / 'web/src', *(root / 'games').glob('*/web')]
    for directory in directories:
        for source in directory.rglob('*'):
            if not source.is_file() or source.suffix not in ['.ts', '.tsx', '.js', '.mjs', '.css']:
                continue
            text = source.read_text()
            references = re.findall(r'''["'](\.[^"'\n]+)["']''', text)
            references += re.findall(r'''url\(\s*([^\s)'"\n]+)''', text)
            for reference in references:
                target = (source.parent / reference.split('?', 1)[0].split('#', 1)[0]).resolve()
                if target.is_relative_to(root):
                    pattern = target.relative_to(root).as_posix()
                    if any(character in pattern for character in '*?['):
                        selected.update(name for name in files if fnmatch.fnmatchcase(name, pattern))
                    elif pattern in files:
                        selected.add(pattern)
    return selected


def resource_plan(root, scope, game):
    registry = json.loads((root / 'assets/resources.json').read_text())
    if registry.get('version') != 1 or not isinstance(registry.get('files'), dict):
        raise ValueError('Invalid resource registry')
    files = registry['files']
    for name, record in files.items():
        destination(root, name)
        validate_record(record)
    selected = embedded_paths(root, files)
    if scope == 'game':
        if not game or not re.fullmatch('[a-z][a-z0-9_-]*', game) or not (root / 'games' / game).is_dir():
            raise ValueError('Unknown resource game')
        selected.update(name for name in files if owner_for(name) == game or owner_for(name) == '_common' and not name.startswith('docs/'))
    elif game:
        raise ValueError('Only game resources accept a game ID')
    if scope == 'frontend':
        selected.update(name for name in files if any(fnmatch.fnmatchcase(name, pattern) for pattern in POLICY['frontendInputs']))
        selected.update(imported_paths(root, files))
        descriptor = root / 'games/yuyuhakusho/data/runtime-resource-descriptor.json'
        if descriptor.is_file():
            for entry in json.loads(descriptor.read_text())['resources']:
                name = 'games/yuyuhakusho/data/' + Path(entry['logical_path']).name
                if name not in files:
                    raise ValueError('Missing declared frontend runtime resource: ' + name)
                selected.add(name)
    elif scope == 'server':
        selected.update(name for name in files if any(fnmatch.fnmatchcase(name, pattern) for pattern in POLICY['serverInputs']))
    elif scope == 'shared':
        selected.update(name for name in files if not name.startswith(('games/', 'docs/', 'web/')))
    result = {name: files[name] for name in sorted(selected)}
    if scope == 'frontend':
        manifest = root / 'games/fortressblue/assets/manifest.json'
        external = root / 'assets/external-resources.json'
        if manifest.is_file() and external.is_file():
            published = json.loads(external.read_text())['files']
            for entry in json.loads(manifest.read_text())['entries']:
                record = validate_record(published[entry['urlPath']])
                if record['sha256'] != entry['sha256'] or record['bytes'] != entry['bytes']:
                    raise ValueError('Fortress publication differs from build input')
                name = entry['output']
                destination(root, name)
                if name in result and result[name] != record:
                    raise ValueError('Conflicting frontend resource identity')
                result[name] = record
    if scope == 'server':
        release = root / 'games/pal/assets/server-release.json'
        if release.is_file():
            document = json.loads(release.read_text())
            pin = document['catalogs'][document['current']]
            for name, entry in pin['files'].items():
                record = {key: entry[key] for key in ['bytes', 'sha256', 'mime', 'encoding']}
                record['key'] = urlsplit(entry['url']).path.lstrip('/')
                validate_record(record)
                target = 'dist/package/pal/' + name
                destination(root, target)
                result[target] = record
    identities = {}
    for record in result.values():
        previous = identities.setdefault(record['key'], record)
        if previous != record:
            raise ValueError('Conflicting immutable resource identity')
    result = dict(sorted(result.items()))
    digest = hashlib.sha256(json.dumps(result, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    prefix = 'ci-resources-v1-' + scope + '-' + (game or 'platform') + '-'
    return {'version': 1, 'scope': scope, 'game': game, 'files': result, 'cachePrefix': prefix, 'cacheKey': prefix + digest, 'objects': len(identities), 'logicalBytes': sum(record['bytes'] for record in result.values()), 'uniqueBytes': sum(record['bytes'] for record in identities.values())}


def matches(path, record):
    if path.is_symlink():
        raise ValueError('Symlink resource input')
    if not path.is_file():
        return False
    if path.stat().st_size != record['bytes']:
        return False
    body = path.read_bytes()
    return len(body) == record['bytes'] and hashlib.sha256(body).hexdigest() == record['sha256']


def workspace_path(root, path):
    path = path.absolute()
    if not path.resolve().is_relative_to(root):
        raise ValueError('Resource output must remain inside the workspace')
    relative = path.relative_to(root)
    if any(part in ['.', '..'] for part in relative.parts):
        raise ValueError('Noncanonical resource output path')
    current = root
    for part in relative.parts:
        current = current / part
        if current.is_symlink():
            raise ValueError('Symlink resource output')
    return path


def restore(plan, root, cache, action, budget, report):
    cache.mkdir(parents=True, exist_ok=True)
    records = {record['key']: record for record in plan['files'].values()}
    origins = {}
    for name, record in plan['files'].items():
        origins.setdefault(record['key'], destination(root, name))
    available, missing, seeded = {}, {}, 0
    for key, record in records.items():
        path = destination(cache, 'objects/' + key)
        if matches(path, record):
            available[key] = path
        elif action == 'prime' and matches(origins[key], record):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(origins[key].read_bytes())
            available[key] = path
            seeded += 1
        else:
            missing[key] = record
    report.update(cacheHits=len(available)-seeded, seededObjects=seeded, missingObjects=len(missing), repairedObjects=seeded+len(missing) if action == 'prime' else 0, cacheBytes=sum(records[key]['bytes'] for key in available))
    limits = {key: record['bytes'] + (record['bytes'] // 1000 + 128 if record.get('encoding') else 0) for key, record in missing.items()}
    planned = sum(limit + 1 for limit in limits.values())
    report['plannedDownloadBytes'] = planned
    if missing and (action != 'prime' or planned > budget):
        raise ValueError('Verified GitHub resource cache is missing or corrupt; prepare this scope with the manual resource-cache workflow and an explicit byte budget')
    if missing:
        config = configuration(root, required=['OSS_BUCKET', 'OSS_ENDPOINT'])
        key_id = os.environ.get('OSS_ACCESS_KEY_ID') or (ROOT / 'config/secrets/oss-access-key-id').read_text().strip()
        secret = os.environ.get('OSS_ACCESS_KEY_SECRET') or (ROOT / 'config/secrets/oss-access-key-secret').read_text().strip()
        lock = threading.Lock()

        def download(item):
            key, record = item
            date = formatdate(usegmt=True)
            canonical = 'GET\n\n\n' + date + '\n/' + config['OSS_BUCKET'] + '/' + key
            signature = base64.b64encode(hmac.new(secret.encode(), canonical.encode(), hashlib.sha1).digest()).decode()
            request = Request('https://' + config['OSS_ENDPOINT'] + '/' + key, headers={'Date': date, 'Authorization': 'OSS ' + key_id + ':' + signature})
            with lock:
                report['getRequests'] += 1
            with urlopen(request, timeout=60) as response:
                length = response.headers.get('Content-Length')
                if length is not None and int(length) > limits[key]:
                    raise ValueError('Resource exceeds its reserved download budget')
                try:
                    body = response.read(limits[key] + 1)
                except IncompleteRead as error:
                    with lock:
                        report['downloadedBytes'] += len(error.partial)
                    raise
                with lock:
                    report['downloadedBytes'] += len(body)
                if len(body) > limits[key]:
                    raise ValueError('Resource exceeds its reserved download budget')
                body = validate_body(record, body, response.headers.get_content_type())
            path = destination(cache, 'objects/' + key)
            path.parent.mkdir(parents=True, exist_ok=True)
            temporary = path.with_name(path.name + '.resource-tmp')
            try:
                temporary.write_bytes(body)
                temporary.replace(path)
            finally:
                temporary.unlink(missing_ok=True)
            return key, path

        with ThreadPoolExecutor(max_workers=8) as pool:
            available.update(pool.map(download, missing.items()))
    for name, record in plan['files'].items():
        source = available[record['key']]
        if not matches(source, record):
            raise ValueError('Verified cache changed during restore')
        target = destination(root, name)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(source.read_bytes())
    report.update(passed=True, restoredFiles=len(plan['files']), missingObjects=0)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['plan', 'restore', 'prime'])
    parser.add_argument('--root', type=Path, default=ROOT)
    parser.add_argument('--scope', choices=['frontend', 'server', 'shared', 'game'], required=True)
    parser.add_argument('--game', default='')
    parser.add_argument('--cache', type=Path)
    parser.add_argument('--report', type=Path)
    parser.add_argument('--max-download-bytes', type=int, default=0)
    parser.add_argument('--github-output', type=Path)
    args = parser.parse_args()
    root = args.root.resolve()
    if not root.is_relative_to(ROOT):
        raise ValueError('Resource workspace must remain inside the repository')
    cache = args.cache or (root / '.local' / args.game / 'cache/ci-resources' if args.scope == 'game' else root / '.local/ci-resource-cache' / (args.scope + '-platform'))
    cache = workspace_path(root, cache)
    report_path = args.report or (root / '.local' / args.game / 'verification/ci-traffic/resources.json' if args.scope == 'game' else root / '.local/ci-traffic' / (args.scope + '-platform.json'))
    report_path = workspace_path(root, report_path)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report = {'passed': False, 'action': args.action, 'scope': args.scope, 'game': args.game, 'downloadedBytes': 0, 'getRequests': 0, 'budgetBytes': args.max_download_bytes, 'missingObjects': 0}
    try:
        if not 0 <= args.max_download_bytes <= min(POLICY['maintenanceMaxDownloadBytes'], 2147483648) or args.action != 'prime' and args.max_download_bytes != POLICY['normalDownloadBytes']:
            raise ValueError('Invalid paid download budget; normal CI requires zero bytes')
        manual_cache = os.environ.get('GITHUB_EVENT_NAME') == 'workflow_dispatch' and '/.github/workflows/resource-cache.yml@' in os.environ.get('GITHUB_WORKFLOW_REF', '')
        if args.action == 'prime' and os.environ.get('GITHUB_ACTIONS') == 'true' and not manual_cache:
            raise ValueError('Paid cache preparation requires a manual workflow dispatch')
        plan = resource_plan(root, args.scope, args.game)
        if args.action == 'plan':
            report.update(plan, passed=True)
            if args.github_output:
                with args.github_output.open('a') as output:
                    output.write('key=' + plan['cacheKey'] + '\nprefix=' + plan['cachePrefix'] + '\npath=' + str(cache.relative_to(root)) + '\nreport=' + report_path.relative_to(root).as_posix() + '\n')
        else:
            report.update(cacheKey=plan['cacheKey'], objects=plan['objects'], logicalBytes=plan['logicalBytes'])
            restore(plan, root, cache, args.action, args.max_download_bytes, report)
            if args.github_output and args.action == 'prime':
                with args.github_output.open('a') as output:
                    output.write('repaired=' + ('true' if report['repairedObjects'] else 'false') + '\n')
    except Exception as error:
        report['error'] = str(error)
        raise
    finally:
        report_path.write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({key: value for key, value in report.items() if key != 'files'}))


if __name__ == '__main__':
    main()
