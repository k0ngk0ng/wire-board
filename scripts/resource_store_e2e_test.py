import gzip
import base64
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/resource-cleanup/tests'
WORK.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location('resource_store', ROOT / 'scripts/resource_store.py')
store = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(store)


class ResourceStoreE2E(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(dir=WORK)
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.objects = {}
        self.puts = 0
        self.private_reads = False
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                if fixture.private_reads:
                    canonical='GET\n\n\n'+self.headers.get('Date','')+'\n/fixture'+self.path
                    signature=base64.b64encode(hmac.new(b'test-secret',canonical.encode(),hashlib.sha1).digest()).decode()
                    if self.headers.get('Authorization')!='OSS test-key:'+signature:
                        self.send_error(403)
                        return
                row = fixture.objects.get(self.path)
                if row is None:
                    self.send_error(404)
                    return
                body, mime = row
                self.send_response(200)
                self.send_header('Content-Type', mime)
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def do_PUT(self):
                if self.path in fixture.objects:
                    self.send_error(409)
                    return
                if self.headers.get('x-oss-forbid-overwrite') != 'true':
                    self.send_error(400)
                    return
                source = self.headers.get('x-oss-copy-source')
                if source:
                    fixture.objects[self.path] = fixture.objects[source.removeprefix('/fixture')]
                else:
                    fixture.objects[self.path] = (self.rfile.read(int(self.headers['Content-Length'])), self.headers['Content-Type'])
                fixture.puts += 1
                self.send_response(200)
                self.end_headers()

            def log_message(self, format, *args):
                return

        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.origin = 'http://127.0.0.1:' + str(self.server.server_port)
        self.client = store.ObjectStore(self.origin, 'fixture', 'test-key', 'test-secret', local=True)

    def test_game_and_shared_paths_use_content_identity_without_release_directories(self):
        self.assertEqual(store.owner_for('docs/research/md-yuyuhakusho/evidence/frame.png'), 'yuyuhakusho')
        self.assertEqual(store.owner_for('docs/research/racing-nfs/car.png'), 'racing')
        self.assertEqual(store.owner_for('assets/poker/source/backs/pine.svg'), '_common')
        for owner in ['splendor', 'stoneage', '_common']:
            key = store.object_key(owner, b'generated resource', '.webp')
            self.assertEqual(key, 'bga/' + owner + '/' + hashlib.sha256(b'generated resource').hexdigest() + '.webp')
        for owner in ['web', 'v20261002-1', '../splendor', 'unknown-game']:
            with self.assertRaises(ValueError):
                store.object_key(owner, b'generated resource', '.webp')

    def test_real_http_publication_reuses_renames_and_never_overwrites_changed_bytes(self):
        first = store.publish(self.client, 'splendor', 'cards.webp', b'generated resource')
        second = store.publish(self.client, 'splendor', 'renamed.webp', b'generated resource')
        third = store.publish(self.client, 'splendor', 'cards.webp', b'changed resource')
        self.assertEqual(first['key'], second['key'])
        self.assertNotEqual(first['key'], third['key'])
        self.assertEqual(self.puts, 2)
        self.assertEqual(self.objects['/' + first['key']][0], b'generated resource')

    def test_real_http_copy_migrates_known_bytes_without_overwrite(self):
        body = b'generated pixels'
        self.objects['/stoneage/assets/bitmap.png'] = (body, 'image/png')
        record = {'key': store.object_key('stoneage', body, '.png'), 'sha256': hashlib.sha256(body).hexdigest(), 'bytes': len(body), 'mime': 'image/png'}
        store.copy_resource(self.client, record, 'stoneage/assets/bitmap.png')
        store.copy_resource(self.client, record, 'stoneage/assets/bitmap.png')
        self.assertEqual(self.puts, 1)
        self.assertEqual(self.objects['/' + record['key']], (body, 'image/png'))

    def test_compressed_and_identity_representations_keep_distinct_immutable_keys(self):
        body = b'generated metadata'
        plain = store.publish(self.client, 'splendor', 'index.json', body)
        compressed = store.publish(self.client, 'splendor', 'index.json', body, encoding='gzip')
        self.assertNotEqual(plain['key'], compressed['key'])
        self.assertEqual(gzip.decompress(self.objects['/' + compressed['key']][0]), body)
        store.hydrate({'version': 1, 'files': {'index.json': compressed}}, self.root, self.origin, local=True)
        self.assertEqual((self.root / 'index.json').read_bytes(), body)

    def test_manifest_publisher_keeps_logical_paths_but_publishes_only_game_content_keys(self):
        source = self.root / 'games/splendor/assets/public/splendor/cards.webp'
        source.parent.mkdir(parents=True)
        source.write_bytes(b'generated pixels')
        entry = {'urlPath': '/splendor/cards.webp', 'output': 'games/splendor/assets/public/splendor/cards.webp', 'sha256': hashlib.sha256(b'generated pixels').hexdigest(), 'bytes': len(b'generated pixels'), 'mime': 'image/webp'}
        manifest = {'entries': [entry, {**entry, 'urlPath': '/splendor/alias.webp'}]}
        plan = store.publish_manifest(self.client, manifest, self.root, 'splendor', self.origin)
        self.assertEqual(self.puts, 0)
        self.assertEqual(plan['uniqueObjects'], 1)
        result = store.publish_manifest(self.client, manifest, self.root, 'splendor', self.origin, execute=True)
        self.assertEqual(self.puts, 1)
        self.assertTrue(all(value['url'].startswith(self.origin + '/bga/splendor/') for value in result['files'].values()))
        source.write_bytes(b'changed')
        with self.assertRaises(ValueError):
            store.publish_manifest(self.client, manifest, self.root, 'splendor', self.origin, execute=True)
        self.assertEqual(self.puts, 1)

    def test_corrupt_preexisting_object_fails_without_put(self):
        key = store.object_key('splendor', b'expected', '.webp')
        self.objects['/' + key] = (b'corrupt', 'image/webp')
        with self.assertRaises(ValueError):
            store.publish(self.client, 'splendor', 'cards.webp', b'expected')
        self.assertEqual(self.puts, 0)

    def test_manifest_hydrates_clean_checkout_and_rejects_modified_download(self):
        record = store.publish(self.client, 'splendor', 'cards.webp', b'generated resource')
        registry = {'version': 1, 'files': {'games/splendor/assets/public/splendor/cards.webp': record}}
        store.hydrate(registry, self.root, self.origin, local=True)
        target = self.root / 'games/splendor/assets/public/splendor/cards.webp'
        self.assertEqual(target.read_bytes(), b'generated resource')
        target.unlink()
        self.objects['/' + record['key']] = (b'wrong bytes', 'image/webp')
        with self.assertRaises(ValueError):
            store.hydrate(registry, self.root, self.origin, local=True)
        self.assertFalse(target.exists())

    def test_signed_oss_hydration_restores_private_sources_without_cdn_access(self):
        self.private_reads=True
        record=store.publish(self.client,'splendor','cards.webp',b'generated private resource')
        registry={'version':1,'files':{'games/splendor/cards.webp':record}}
        store.hydrate(registry,self.root,'https://unreachable.example.invalid',client=self.client)
        target=self.root/'games/splendor/cards.webp'
        self.assertEqual(target.read_bytes(),b'generated private resource')
        target.unlink()
        self.objects['/'+record['key']]=(b'corrupted','image/webp')
        with self.assertRaises(ValueError):
            store.hydrate(registry,self.root,'https://unreachable.example.invalid',client=self.client)
        self.assertFalse(target.exists())
        self.objects.clear()
        with self.assertRaises(ValueError):
            store.hydrate(registry,self.root,'https://unreachable.example.invalid',client=self.client)
        self.assertFalse(target.exists())
        self.assertEqual(self.puts,1)

    def test_manifest_rejects_paths_outside_checkout_and_unknown_owners(self):
        record = store.publish(self.client, 'splendor', 'cards.webp', b'generated resource')
        for path in ['../escape.webp', '/absolute.webp', 'games//splendor/card.webp', '.git/config']:
            with self.assertRaises(ValueError):
                store.hydrate({'version': 1, 'files': {path: record}}, self.root, self.origin, local=True)
        wrong = dict(record, key=record['key'].replace('/splendor/', '/unknown/'))
        with self.assertRaises(ValueError):
            store.hydrate({'version': 1, 'files': {'games/splendor/card.webp': wrong}}, self.root, self.origin, local=True)

    def test_symlink_target_is_rejected(self):
        record = store.publish(self.client, 'splendor', 'cards.webp', b'generated resource')
        (self.root / 'elsewhere').mkdir()
        (self.root / 'games').symlink_to(self.root / 'elsewhere', target_is_directory=True)
        with self.assertRaises(ValueError):
            store.hydrate({'version': 1, 'files': {'games/splendor/card.webp': record}}, self.root, self.origin, local=True)

    def test_business_configuration_reads_local_file_and_environment_overrides_it(self):
        (self.root / '.local').mkdir()
        (self.root / '.local/business.json').write_text(json.dumps({'PUBLIC_ORIGIN': 'https://local.example', 'CDN_ORIGIN': 'https://cdn.example'}))
        with patch.dict(os.environ, {}, clear=True):
            self.assertEqual(store.configuration(self.root)['PUBLIC_ORIGIN'], 'https://local.example')
            exported = store.export_environment(self.root)
            self.assertEqual(exported, self.root / '.local/business.env')
            self.assertIn('PUBLIC_ORIGIN=https://local.example', exported.read_text())
        with patch.dict(os.environ, {'PUBLIC_ORIGIN': 'https://override.example'}, clear=True):
            self.assertEqual(store.configuration(self.root)['PUBLIC_ORIGIN'], 'https://override.example')
        with patch.dict(os.environ, {'GITHUB_ACTIONS': 'true'}, clear=True):
            with self.assertRaises(ValueError):
                store.configuration(self.root)

    def test_resource_and_configuration_lint_rejects_payload_and_private_origin(self):
        self.assertTrue(store.is_resource('assets/cards.webp'))
        self.assertTrue(store.is_resource('assets/audio.tar.xz'))
        self.assertTrue(store.is_resource('assets/font.ttf'))
        self.assertTrue(store.is_resource('games/pal/native/upstream/fontglyph.h'))
        self.assertTrue(store.is_resource('games/pal/native/upstream/ascii.h'))
        self.assertTrue(store.is_resource('evidence/state.json.gz.before-private-io'))
        self.assertTrue(store.is_resource('games/pokepath/assets/public/pokepath/routes/0.json'))
        self.assertTrue(store.is_resource('web/public/runtime.js'))
        self.assertFalse(store.is_resource('games/splendor/server/rules.json'))
        self.assertFalse(store.is_resource('games/splendor/web/board.tsx'))
        self.assertEqual(store.forbidden_origins('https://fixture.example/api', ['https://fixture.example']), ['fixture.example'])
        self.assertEqual(store.forbidden_origins('https://other.example/api', ['https://fixture.example']), [])
        self.assertEqual(store.forbidden_origins('https://FIXTURE.EXAMPLE/api', ['https://fixture.example']), ['fixture.example'])
        encoded = base64.b64encode(bytes([137, 80, 78, 71, 13, 10, 26, 10]) + b'generated' * 20).decode()
        self.assertTrue(store.embedded_resources("const image = '" + encoded + "';"))
        self.assertTrue(store.is_resource('proof.json', json.dumps({'src': 'data:image/png;base64,' + encoded})))
        self.assertFalse(store.embedded_resources('export function draw() { return 1; }'))


if __name__ == '__main__':
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(ResourceStoreE2E)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    (WORK / 'result.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'errors': len(result.errors), 'failures': len(result.failures)}) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
