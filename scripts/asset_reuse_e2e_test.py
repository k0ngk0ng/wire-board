import contextlib
import gzip
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch
from urllib.error import HTTPError
from io import BytesIO

import cdn_delivery as cdn
import asset_reuse as reuse

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/asset-reuse'
WORK.mkdir(parents=True, exist_ok=True)
OLD = 'https://cdn.ichenj.com/bga/web/' + 'a' * 40 + '/'
NEW = 'https://cdn.ichenj.com/bga/web/' + 'b' * 40 + '/'


class AssetReuseE2E(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(dir=WORK)
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.dist = self.root / 'dist'
        (self.dist / 'assets').mkdir(parents=True)
        (self.dist / 'assets/app.js').write_bytes(b'export const ready=true;')
        (self.dist / 'index.html').write_text('<script type="module" src="' + NEW + 'assets/app.js"></script>')
        (self.dist / 'game-load-manifest.json').write_text(json.dumps({'assetBase': NEW, 'entry': NEW + 'assets/app.js', 'games': {'fixture': [{'url': NEW + 'assets/app.js', 'bytes': 24}]}, 'home': {'bytes': 24, 'files': [{'url': NEW + 'assets/app.js', 'bytes': 24}]}, 'media': {}}))
        self.body = b'original image pixels'
        self.baseline = {'version': 1, 'assetBase': OLD, 'files': {'/images/original.webp': self.record(OLD + 'images/original.webp', self.body, 'image/webp')}}

    def record(self, url, body, mime, encoding=''):
        return {'url': url, 'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest(), 'mime': mime, 'encoding': encoding}

    def put(self, name, body):
        file = self.dist / name
        file.parent.mkdir(parents=True, exist_ok=True)
        file.write_bytes(body)

    def prepare(self):
        baseline = self.root / 'baseline.json'
        baseline.write_text(json.dumps(self.baseline))
        subprocess.run([sys.executable, '-B', str(ROOT / 'scripts/asset_reuse.py'), '--dist', str(self.dist), '--baseline', str(baseline)], cwd=ROOT, check=True, capture_output=True)
        return cdn.inventory(self.dist, NEW)

    def test_cli_preserves_original_url_and_deduplicates_renames_and_batch_aliases(self):
        self.put('images/original.webp', self.body)
        self.put('images/renamed.webp', self.body)
        self.put('images/alias.webp', self.body)
        rows = {row['path']: row for row in self.prepare()}
        for name in ['original', 'renamed', 'alias']:
            self.assertEqual(rows['images/' + name + '.webp']['url'], OLD + 'images/original.webp')
        manifest = json.loads((self.dist / 'game-load-manifest.json').read_text())
        self.assertEqual(manifest['mediaBase'], '/_media')
        self.assertNotIn('assetMap', manifest)
        self.assertNotIn('assetObjects', manifest)
        self.assertIn(OLD + 'images/original.webp', json.loads((self.dist / 'cdn-assets-manifest.json').read_text())['objects'])
        before = {str(p.relative_to(self.dist)): p.read_bytes() for p in self.dist.rglob('*') if p.is_file()}
        self.prepare()
        self.assertEqual(before, {str(p.relative_to(self.dist)): p.read_bytes() for p in self.dist.rglob('*') if p.is_file()})

    def test_one_changed_file_preserves_other_urls_and_old_references(self):
        self.put('images/original.webp', self.body + b'changed')
        self.put('images/other.webp', self.body)
        rows = {row['path']: row for row in self.prepare()}
        self.assertEqual(rows['images/other.webp']['url'], OLD + 'images/original.webp')
        self.assertEqual(rows['images/original.webp']['url'], NEW + 'images/original.webp')
        self.assertIn(OLD + 'images/original.webp', json.loads((self.dist / 'cdn-assets-manifest.json').read_text())['objects'])

    def test_documentation_only_baseline_is_optional_but_production_proof_is_required(self):
        with patch.object(reuse.subprocess, 'check_output', return_value='[]'):
            self.assertFalse(reuse.successful_readback('a' * 40, required=False))
            with self.assertRaises(ValueError):
                reuse.successful_readback('a' * 40)

    def test_verified_candidate_cache_replays_exact_run_and_rejects_changed_bytes(self):
        sha = 'd' * 40
        base = 'https://cdn.ichenj.com/bga/web/' + sha + '/'
        @contextlib.contextmanager
        def origin(url, timeout=30):
            yield BytesIO(json.dumps(self.baseline).encode())
        def download(command, check=True):
            folder = Path(command[-1])
            (folder / 'assets').mkdir()
            (folder / 'assets/previous.js').write_bytes(b'export const old=true;')
            (folder / 'game-load-manifest.json').write_text(json.dumps({'assetBase': base}))
        runs = json.dumps({'workflow_runs': [{'id': 8123, 'head_sha': sha, 'head_branch': 'main', 'event': 'push'}]})
        with patch.object(reuse, 'ROOT', self.root), patch.object(reuse, 'urlopen', origin), patch.object(reuse, 'successful_readback', return_value=8123), patch.object(reuse.subprocess, 'check_output', return_value=runs), patch.object(reuse.subprocess, 'run', side_effect=download) as downloads, patch.dict(os.environ, {'VERIFIED_ANCESTOR': sha}):
            first = reuse.published_baseline()
            second = reuse.published_baseline()
            self.assertEqual(first, second)
            self.assertEqual(downloads.call_count, 1)
            (self.root / '.local/asset-reuse' / ('baseline-' + sha) / 'assets/previous.js').write_bytes(b'corrupted candidate cache')
            with self.assertRaises(ValueError):
                reuse.published_baseline()

    def test_response_format_is_part_of_identity(self):
        for field, value in [('bytes', 999), ('mime', 'image/png'), ('encoding', 'gzip')]:
            with self.subTest(field=field):
                baseline = json.loads(json.dumps(self.baseline))
                baseline['files']['/images/original.webp'][field] = value
                rows = reuse.plan([{'path': 'images/renamed.webp', **self.record(NEW + 'images/renamed.webp', self.body, 'image/webp'), 'wireBytes': len(self.body)}], baseline)
                self.assertEqual(rows[0]['url'], NEW + 'images/renamed.webp')

    def test_invalid_published_addresses_fail_closed(self):
        for url in ['https://evil.invalid/image.webp', OLD + '../private', OLD + '.private/token', OLD + 'image.webp?token=x', OLD + 'images//alias.webp', OLD + 'images/']:
            with self.subTest(url=url):
                baseline = json.loads(json.dumps(self.baseline))
                baseline['files']['/images/original.webp']['url'] = url
                with self.assertRaises(ValueError):
                    reuse.plan([], baseline)

    def test_html_css_and_program_references_follow_reused_images(self):
        self.put('images/original.webp', self.body)
        self.put('assets/app.js', ('export const image="' + NEW + 'images/original.webp";').encode())
        self.put('assets/app.css', ('body{background:url("' + NEW + 'images/original.webp")}').encode())
        self.prepare()
        for name in ['assets/app.js', 'assets/app.css']:
            self.assertIn((OLD + 'images/original.webp').encode(), (self.dist / name).read_bytes())
            self.assertNotIn((NEW + 'images/original.webp').encode(), (self.dist / name).read_bytes())
        manifest = json.loads((self.dist / 'game-load-manifest.json').read_text())
        self.assertEqual(manifest['games']['fixture'][0]['bytes'], (self.dist / 'assets/app.js').stat().st_size)

    def test_reused_program_never_resolves_a_changed_dependency_from_its_old_directory(self):
        parent = b'import "./child.js";'
        self.baseline['files']['/assets/parent.js'] = self.record(OLD + 'assets/parent.js', parent, 'text/javascript', 'gzip')
        self.put('assets/parent.js', parent)
        self.put('assets/child.js', b'export const version=2;')
        rows = {row['path']: row for row in self.prepare()}
        self.assertEqual(rows['assets/parent.js']['url'], NEW + 'assets/parent.js')
        self.assertIn((NEW + 'assets/child.js').encode(), (self.dist / 'assets/parent.js').read_bytes())

    def test_missing_known_published_object_is_not_silently_recreated(self):
        row = {'path': 'images/original.webp', **self.record(OLD + 'images/original.webp', self.body, 'image/webp'), 'wireBytes': len(self.body), 'previouslyPublished': True}
        with patch.object(cdn, 'credentials', return_value=('fixture', 'fixture')), patch.object(cdn, 'urlopen', side_effect=HTTPError('url', 404, 'missing', {}, BytesIO())) as request:
            with self.assertRaises(ValueError):
                cdn.publish(self.dist, [row], 'fixture', 'fixture.oss-cn-shanghai.aliyuncs.com')
            self.assertEqual(request.call_count, 1)

    def test_repeat_upload_has_zero_puts_and_corrupt_existing_object_is_never_overwritten(self):
        self.put('images/original.webp', self.body)
        self.put('images/new.webp', b'new pixels')
        self.put('images/new-alias.webp', b'new pixels')
        rows = self.prepare()
        store = {}
        puts = []
        lock = threading.Lock()
        for row in rows:
            if row['url'].startswith(OLD):
                store[row['url'].removeprefix('https://cdn.ichenj.com/')] = (self.body, {'Content-Type': row['mime']})

        @contextlib.contextmanager
        def transport(request, timeout=60):
            key = request.full_url.split('.aliyuncs.com/', 1)[1]
            if request.get_method() == 'PUT':
                self.assertEqual(request.get_header('X-oss-forbid-overwrite'), 'true')
                with lock:
                    self.assertNotIn(key, store)
                    store[key] = (request.data, {'Content-Type': request.get_header('Content-type'), 'Content-Encoding': request.get_header('Content-encoding', '')})
                    puts.append(key)
                response = BytesIO(b'')
                response.status = 200
                response.headers = {}
                yield response
            elif key in store:
                body, headers = store[key]
                response = BytesIO(body)
                response.status = 200
                response.headers = headers
                yield response
            else:
                raise HTTPError(request.full_url, 404, 'missing', {}, BytesIO())

        with patch.object(cdn, 'credentials', return_value=('fixture', 'fixture')), patch.object(cdn, 'urlopen', transport):
            first = cdn.publish(self.dist, rows, 'fixture', 'fixture.oss-cn-shanghai.aliyuncs.com')
            self.assertEqual(first['uploadedObjects'], 2)
            self.assertEqual(len(puts), 2)
            puts.clear()
            second = cdn.publish(self.dist, rows, 'fixture', 'fixture.oss-cn-shanghai.aliyuncs.com')
            self.assertEqual(second['uploadedObjects'], 0)
            self.assertEqual(second['uploadedBytes'], 0)
            self.assertFalse(puts)
            key = 'bga/web/' + 'a' * 40 + '/images/original.webp'
            store[key] = (b'corrupted published pixels', {'Content-Type': 'image/webp'})
            with self.assertRaises(ValueError):
                cdn.publish(self.dist, rows, 'fixture', 'fixture.oss-cn-shanghai.aliyuncs.com')
            self.assertFalse(puts)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(AssetReuseE2E))
    (WORK / 'result.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
