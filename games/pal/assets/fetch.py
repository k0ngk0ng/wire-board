#!/usr/bin/env python3
"""Fetch pinned PAL 98 resources into this workspace and verify every byte.

Only game data is downloaded. No original executable is downloaded or run.
The manifest records upstream commit, file size, Git blob SHA-1 and SHA-256.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
from pathlib import Path, PurePosixPath
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
MANIFEST = Path(__file__).with_name('source-manifest.json')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--media', action='store_true', help='fetch the separately pinned original AVI and CD tracks')
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    output = (args.output or ROOT / ('.local/pal/original-98-media' if args.media else '.local/pal/original-98')).resolve()
    if not output.is_relative_to(ROOT):
        parser.error('output must remain inside the current workspace')
    manifest = json.loads((MANIFEST.with_name('media-source-manifest.json') if args.media else MANIFEST).read_text())
    output.mkdir(parents=True, exist_ok=True)

    def verify(data, item):
        blob = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest()
        return len(data) == item['size'] and hashlib.sha256(data).hexdigest() == item['sha256'] and blob == item['gitBlobSHA1']

    def obtain(item):
        relative = PurePosixPath(item['path'])
        source = PurePosixPath(item['sourcePath'])
        if relative.is_absolute() or '..' in relative.parts or source.is_absolute() or '..' in source.parts:
            raise ValueError('invalid source manifest path')
        target = output.joinpath(*relative.parts)
        if target.exists() and verify(target.read_bytes(), item):
            return {'path': item['path'], 'verified': True, 'cached': True}
        url = 'https://raw.githubusercontent.com/' + manifest['repository'] + '/' + manifest['commit'] + '/' + str(source)
        request = urllib.request.Request(url, headers={'User-Agent': 'wire-board-pal-resource-verification'})
        with urllib.request.urlopen(request, timeout=60) as response:
            data = response.read(item['size'] + 1)
        if not verify(data, item):
            raise ValueError('resource integrity mismatch: ' + item['path'])
        target.parent.mkdir(parents=True, exist_ok=True)
        temporary = target.with_name(target.name + '.download')
        try:
            temporary.write_bytes(data)
            temporary.replace(target)
        finally:
            temporary.unlink(missing_ok=True)
        return {'path': item['path'], 'verified': True, 'cached': False}

    with ThreadPoolExecutor(max_workers=5) as pool:
        results = list(pool.map(obtain, manifest['files']))
    (output / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    report = {'passed': True, 'sourceCommit': manifest['commit'], 'files': len(results),
              'bytes': sum(item['size'] for item in manifest['files']), 'results': results}
    report_dir = ROOT / '.local/pal/verification'
    report_dir.mkdir(parents=True, exist_ok=True)
    (report_dir / ('media-download.json' if args.media else 'download.json')).write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({k: report[k] for k in ['passed', 'sourceCommit', 'files', 'bytes']}))


if __name__ == '__main__':
    main()
