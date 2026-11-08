#!/usr/bin/env python3
"""Publish generated HD files to their own immutable StoneAge CDN version."""
import argparse
from email.utils import formatdate
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import sys
import time
from urllib.request import Request
from urllib.error import HTTPError, URLError

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'scripts'))
import cdn_delivery as cdn

spec = importlib.util.spec_from_file_location('classic_publish', ROOT / 'scripts/publish-classic-game-assets.py')
common = importlib.util.module_from_spec(spec)
spec.loader.exec_module(common)
VERSION = 'arena-2x-gpt-image-2.5-v1'
common.PREFIX = 'stoneage/assets/hd/' + VERSION
common.CDN = 'https://cdn.ichenj.com/' + common.PREFIX


class Client(common.OSSClient):
    def _request(self, method, path, body=None, mime='', cdn=False):
        for attempt in range(5):
            try:
                url = path if cdn else common.ENDPOINT + '/' + path
                headers = ({'Origin': 'https://bga.ichenj.com', 'Accept-Encoding': 'identity'}
                           if cdn else common.oss_headers(method, path, body, mime, self.key, self.secret, formatdate(usegmt=True)))
                with globals()['cdn'].urlopen(Request(url, data=body, method=method, headers=headers), timeout=45) as r:
                    data = r.read()
                    if cdn:
                        assert r.headers.get('Access-Control-Allow-Origin') in ('*', 'https://bga.ichenj.com'), 'HD CDN CORS mismatch'
                    return data, r.headers.get_content_type()
            except HTTPError as error:
                error.close()
                if error.code not in ({404, 429, 500, 502, 503, 504} if cdn else {429, 500, 502, 503, 504}) or attempt == 4:
                    raise
            except (URLError, OSError):
                if attempt == 4:
                    raise
            time.sleep(min(2 ** attempt, 8))


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--directory', type=Path, required=True)
    p.add_argument('--publish', action='store_true')
    p.add_argument('--workers', type=int, default=8)
    args = p.parse_args()
    folder = args.directory.resolve()
    assert folder.name == VERSION and not folder.is_symlink()
    manifest = json.loads((folder / 'manifest.json').read_text())
    assert manifest['status'] == 'complete' and manifest['exportedFiles'] == 9477
    assert manifest['sourceArtManifestSHA256'] == cdn.sha((ROOT / 'games/stoneage/web/art.json').read_bytes())
    hd = json.loads((ROOT / 'games/stoneage/web/hd.json').read_text())
    assert set(manifest['frames']) == set(hd['resources'])
    entries = []
    for file, row in manifest['frames'].items():
        relative = PurePosixPath(file)
        assert not relative.is_absolute() and '..' not in relative.parts and relative.suffix == '.png'
        path = folder / file
        assert not path.is_symlink() and path.resolve().is_relative_to(folder)
        data = path.read_bytes()
        assert len(data) == row['bytes'] and cdn.sha(data) == row['sha256']
        assert hd['resources'][file]['sha256'] == row['sha256']
        assert common.media_dimensions(data, 'image/png') == tuple(row['physicalSize'])
        entries.append(dict(urlPath='/' + file, bytes=len(data), sha256=row['sha256'], data=data, mime='image/png'))
    for file in ('manifest.json', 'arena-animation-index.json'):
        data = (folder / file).read_bytes()
        entries.append(dict(urlPath='/' + file, bytes=len(data), sha256=cdn.sha(data), data=data, mime='application/json'))
    common.ensure_unique(entries)
    target = ROOT / '.local/stoneage/release/hd-publish.json'
    report = dict(passed=False, mode='publish' if args.publish else 'preflight',
                  assetBase=common.CDN, expectedObjects=len(entries),
                  expectedBytes=sum(e['bytes'] for e in entries), localVerified=True, objects=[], errors=[])
    common.write_report(target, report)
    print(f"Preflight verified {len(entries)} immutable HD objects", flush=True)
    if args.publish:
        client = Client((ROOT / 'config/secrets/oss-access-key-id').read_text().strip(),
                        (ROOT / 'config/secrets/oss-access-key-secret').read_text().strip())
        art = json.loads((ROOT / 'games/stoneage/web/art.json').read_text())
        report['anchor'] = common.verify_anchor(client, 'stoneage/assets/battle/battle_00.png', art['resources']['battle/battle_00.png']['sha256'])
        def progress(results, errors):
            if len(results) % 100 == 0 or errors:
                report.update(objects=results, errors=errors)
                common.write_report(target, report)
                print(f"Verified {len(results)}/{len(entries)} HD objects; errors={len(errors)}", flush=True)
        results, errors = common.publish_entries(client, entries, args.workers, progress)
        report.update(objects=results, errors=errors, corsVerified=True)
        report['passed'] = not errors and len(results) == len(entries)
    else:
        report['passed'] = True
    common.write_report(target, report)
    if not report['passed']:
        raise SystemExit('HD publication/readback failed; see task report')
    print(json.dumps(dict(passed=True, objects=len(entries), published=args.publish)), flush=True)


if __name__ == '__main__':
    main()
