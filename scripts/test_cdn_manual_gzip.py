import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import struct
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest.mock import patch

import cdn_delivery as cdn
import asset_reuse as reuse

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/cs16/verification/cdn'
TEMP = WORK / 'tmp'
TEMP.mkdir(parents=True, exist_ok=True)


def glb():
    document = json.dumps({'asset': {'version': '2.0'}}).encode()
    document += b' ' * (-len(document) % 4)
    return b'glTF' + struct.pack('<II', 2, len(document) + 20) + struct.pack('<I4s', len(document), b'JSON') + document


class ManualGzipDelivery(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(dir=TEMP, prefix='manual-')
        self.addCleanup(self.temporary.cleanup)
        self.dist = Path(self.temporary.name) / 'dist'
        self.game = self.dist / 'cs16'
        self.game.mkdir(parents=True)
        (self.dist / 'index.html').write_text('entry')
        (self.dist / 'game-load-manifest.json').write_text('{}')
        self.base = cdn.configuration()['CDN_ORIGIN'] + '/bga/'
        self.records = []
        self.manifest = {'schema': 1, 'maps': {}, 'models': {}, 'resources': self.records}

    def resource(self, file='models/player.glb.gz', body=None, mime='model/gltf-binary'):
        body = glb() if body is None else body
        encoded = gzip.compress(body, mtime=0)
        target = self.game / file
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(encoded)
        record = {'file': file, 'bytes': len(encoded), 'sha256': cdn.sha(encoded), 'mime': mime, 'encoding': 'gzip', 'decodedBytes': len(body), 'decodedSHA256': cdn.sha(body)}
        self.records.append(record)
        self.save()
        return target, record, encoded, body

    def save(self):
        (self.game / 'manifest.json').write_text(json.dumps(self.manifest))

    def inventory(self):
        with patch.object(cdn, 'urlopen', side_effect=AssertionError('inventory must not access the network')):
            return cdn.inventory(self.dist, self.base)

    def test_recursive_records_preserve_compressed_identity_and_trim_all_payloads(self):
        model, record, encoded, _ = self.resource()
        rules, rule, compressed, _ = self.resource('maps/de_dust2/rules.json.gz', b'{"hulls":[],"spawns":[]}', 'application/json')
        self.manifest['models']['player'] = dict(record)
        self.manifest['maps']['de_dust2'] = {'rules': dict(rule)}
        self.save()
        rows = self.inventory()
        selected = {row['path']: row for row in rows}
        for target, data in [(model, encoded), (rules, compressed)]:
            row = selected[target.relative_to(self.dist).as_posix()]
            self.assertEqual(row['mime'], 'application/gzip')
            self.assertEqual(row['encoding'], '')
            self.assertEqual(row['bytes'], len(data))
            self.assertEqual(row['wireBytes'], len(data))
            self.assertEqual(row['sha256'], cdn.sha(data))
        cdn.trim(self.dist, {'passed': True, 'files': rows})
        self.assertEqual({p.name for p in self.dist.iterdir()}, {'index.html', 'game-load-manifest.json', 'cdn-assets-manifest.json'})
        mapping = json.loads((self.dist / 'cdn-assets-manifest.json').read_text())['files']
        self.assertEqual(mapping['/cs16/models/player.glb.gz']['sha256'], cdn.sha(encoded))

    def test_undeclared_or_incomplete_gzip_never_becomes_publishable(self):
        target, record, _, _ = self.resource()
        for case in ['no-manifest', 'undeclared', 'encoding', 'decoded-size', 'decoded-sha', 'mime']:
            with self.subTest(case=case):
                self.manifest = {'schema': 1, 'resources': [dict(record)]}
                if case == 'undeclared':
                    self.manifest['resources'] = []
                if case not in ['no-manifest', 'undeclared']:
                    del self.manifest['resources'][0][{'encoding': 'encoding', 'decoded-size': 'decodedBytes', 'decoded-sha': 'decodedSHA256', 'mime': 'mime'}[case]]
                self.save()
                if case == 'no-manifest':
                    (self.game / 'manifest.json').unlink()
                with self.assertRaises(ValueError):
                    self.inventory()
                self.assertTrue(target.exists())

    def test_identity_conflicts_bad_paths_and_wrong_types_fail_closed(self):
        _, record, _, _ = self.resource()
        for field, value in [('bytes', 1), ('sha256', '0' * 64), ('decodedBytes', 1), ('decodedSHA256', '0' * 64), ('mime', 'application/json'), ('bytes', True), ('decodedBytes', True), ('file', '../other/player.glb.gz'), ('file', 'models//player.glb.gz'), ('file', '/models/player.glb.gz')]:
            with self.subTest(field=field, value=value):
                altered = {**record, field: value}
                self.manifest = {'schema': 1, 'resources': [altered]}
                self.save()
                with self.assertRaises(ValueError):
                    self.inventory()
        self.manifest = {'schema': 1, 'resources': [dict(record), {**record, 'sha256': '0' * 64}]}
        self.save()
        with self.assertRaises(ValueError):
            self.inventory()

    def test_bad_gzip_streams_and_expansion_limits_are_rejected(self):
        target, record, encoded, _ = self.resource()
        damaged = bytearray(encoded)
        damaged[-5] ^= 1
        for case, body in [('plain', b'glTF'), ('truncated', encoded[:-1]), ('crc', bytes(damaged)), ('trailing', encoded + b'x'), ('members', encoded + encoded)]:
            with self.subTest(case=case):
                target.write_bytes(body)
                self.manifest = {'schema': 1, 'resources': [{**record, 'bytes': len(body), 'sha256': cdn.sha(body)}]}
                self.save()
                with self.assertRaises(ValueError):
                    self.inventory()
        target.write_bytes(encoded)
        self.manifest = {'schema': 1, 'resources': [record]}
        self.save()
        for limit in ['MANUAL_GZIP_MAX_COMPRESSED_BYTES', 'MANUAL_GZIP_MAX_DECODED_BYTES']:
            with self.subTest(limit=limit), patch.object(cdn, limit, 1, create=True):
                with self.assertRaises(ValueError):
                    self.inventory()

    def test_compressed_size_is_checked_before_reading_payload_body(self):
        target, record, encoded, _ = self.resource()
        self.manifest = {'schema': 1, 'resources': [{**record, 'bytes': len(encoded) - 1}]}
        self.save()
        read_bytes = Path.read_bytes
        def guarded(path):
            if path == target:
                raise AssertionError('Oversized body must not be read')
            return read_bytes(path)
        with patch.object(Path, 'read_bytes', guarded), self.assertRaises(ValueError):
            self.inventory()

    def test_decoded_mime_requires_valid_json_or_glb_structure(self):
        for file, body, mime in [('rules.json.gz', b'not json', 'application/json'), ('player.glb.gz', b'{"asset":{"version":"2.0"}}', 'model/gltf-binary'), ('player.glb.gz', b'glTF' + b'\0' * 20, 'model/gltf-binary'), ('model.bin.gz', b'opaque', 'application/octet-stream')]:
            with self.subTest(file=file, body=body):
                shutil.rmtree(self.game)
                self.game.mkdir()
                self.records = []
                self.manifest = {'schema': 1, 'resources': self.records}
                self.resource(file, body, mime)
                with self.assertRaises(ValueError):
                    self.inventory()

    def test_missing_and_symlinked_resources_or_manifest_are_rejected(self):
        target, _, encoded, _ = self.resource()
        target.unlink()
        with self.assertRaises(ValueError):
            self.inventory()
        outside = Path(self.temporary.name) / 'outside.glb.gz'
        outside.write_bytes(encoded)
        target.symlink_to(outside)
        with self.assertRaises(ValueError):
            self.inventory()
        target.unlink()
        target.write_bytes(encoded)
        manifest = self.game / 'manifest.json'
        copied = Path(self.temporary.name) / 'outside-manifest.json'
        shutil.copyfile(manifest, copied)
        manifest.unlink()
        manifest.symlink_to(copied)
        with self.assertRaises(ValueError):
            self.inventory()

    def test_explicit_manual_gzip_can_coexist_with_its_uncompressed_resource(self):
        target, _, _, body = self.resource()
        target.with_suffix('').write_bytes(body)
        rows = self.inventory()
        self.assertIn('cs16/models/player.glb.gz', [row['path'] for row in rows])
        self.assertIn('cs16/models/player.glb', [row['path'] for row in rows])
        cdn.trim(self.dist, {'passed': True, 'files': rows})
        self.assertFalse(target.exists())

    def test_inventory_tamper_is_rejected_before_server_packaging(self):
        target, _, encoded, _ = self.resource()
        rows = self.inventory()
        target.write_bytes(encoded + b'x')
        with self.assertRaises(ValueError):
            cdn.trim(self.dist, {'passed': True, 'files': rows})
        self.assertTrue(target.exists())

    def test_loopback_readback_requests_identity_and_receives_exact_gzip_bytes(self):
        target, _, encoded, _ = self.resource()
        row = next(row for row in self.inventory() if row['path'].endswith('.glb.gz'))
        requests = []
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                requests.append(self.headers.get('Accept-Encoding'))
                self.send_response(200)
                self.send_header('Content-Type', 'application/gzip')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Content-Length', str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)
            def log_message(self, *args):
                pass
        server = HTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            row['url'] = 'http://127.0.0.1:' + str(server.server_port) + '/' + target.name
            cdn.verify_once(row)
            self.assertEqual(requests, ['identity'])
        finally:
            server.shutdown()
            server.server_close()
            thread.join()
        headers = {'Content-Type': 'application/gzip', 'Access-Control-Allow-Origin': '*'}
        cdn.validate_response(row, encoded, headers)
        with self.assertRaises(ValueError):
            cdn.validate_response(row, gzip.compress(encoded, mtime=0), {**headers, 'Content-Encoding': 'gzip'})

    def test_application_gzip_reuse_and_packaging_e2e(self):
        target, _, encoded, _ = self.resource()
        directory = self.dist / 'assets'
        directory.mkdir()
        body = b'export const ready=true;'
        (directory / 'app.js').write_bytes(body)
        entry = {'url': self.base + 'assets/app.js', 'bytes': len(body)}
        load = {'assetBase': self.base, 'entry': entry['url'], 'games': {'cs16': [entry]}, 'home': {'files': [entry]}, 'owners': {}, 'media': {}}
        (self.dist / 'game-load-manifest.json').write_text(json.dumps(load))
        with patch.object(reuse, 'urlopen', side_effect=AssertionError('reuse must not access the network')):
            reuse.prepare(self.dist, {})
            before = {p.relative_to(self.dist).as_posix(): p.read_bytes() for p in self.dist.rglob('*') if p.is_file()}
            reuse.prepare(self.dist, {})
        self.assertEqual(before, {p.relative_to(self.dist).as_posix(): p.read_bytes() for p in self.dist.rglob('*') if p.is_file()})
        rows = self.inventory()
        row = next(row for row in rows if row['path'] == 'cs16/models/player.glb.gz')
        self.assertEqual(row['url'], self.base + 'cs16/' + cdn.sha(encoded) + '.gz')
        self.assertEqual(row['encoding'], '')
        cdn.trim(self.dist, {'passed': True, 'files': rows})
        self.assertFalse(target.exists())

    def test_publication_ci_includes_manual_gzip_without_game_suites(self):
        spec = importlib.util.spec_from_file_location('scope', ROOT / 'scripts/ci/scope.py')
        scope = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(scope)
        plan = scope.plan(['scripts/test_cdn_manual_gzip.py'], scope.game_names(ROOT))
        self.assertEqual(plan['games'], [])
        self.assertEqual(plan['quick_components'], ['ci'])
        self.assertIn(['python3', '-B', 'scripts/test_cdn_manual_gzip.py'], scope.quick_commands(ROOT, ['ci']))


if __name__ == '__main__':
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(ManualGzipDelivery)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    report = {'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors), 'network': 'loopback-only', 'uploadRequests': 0, 'sourceSHA256': hashlib.sha256(Path(cdn.__file__).read_bytes()).hexdigest()}
    (WORK / 'manual-gzip.json').write_text(json.dumps(report, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
