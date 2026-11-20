import base64
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import ssl
import subprocess
import sys
import tempfile
import threading
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / '.local/ci-traffic'
WORK.mkdir(parents=True, exist_ok=True)


class ResourceTrafficE2E(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(dir=WORK)
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.cache = self.root / 'cache'
        self.objects = {}
        self.requests = []
        self.wire_bytes = 0
        self.files = {}
        self.add_resource('games/splendor/assets/public/splendor/card.webp', b'public card')
        self.add_resource('games/splendor/engine/testdata/replay.bin', b'private replay')
        self.add_resource('games/yuyuhakusho/engine/testdata/replay.bin', b'unrelated replay')
        self.add_resource('docs/research/md-yuyuhakusho/capture.bin', b'research')
        self.add_resource('internal/server/testdata/input.bin', b'shared fixture')
        self.add_resource('web/src/controllers/assets/pad.svg', b'controller source image')
        self.add_resource('games/splendor/web/assets/card.svg', b'game source image')
        self.add_resource('games/splendor/assets/preview.png', b'imported preview')
        picker = self.root / 'games/splendor/web/picker.ts'
        picker.parent.mkdir(parents=True, exist_ok=True)
        picker.write_text('import preview from "../assets/preview.png";\nexport default preview;\n')
        self.add_resource('games/contra/server/data/rules.bin', b'embedded rules')
        source = self.root / 'games/contra/server/embed.go'
        source.parent.mkdir(parents=True, exist_ok=True)
        source.write_text('package server\nimport _ "embed"\n//go:embed data/rules.bin\nvar rules []byte\n')
        registry = self.root / 'assets/resources.json'
        registry.parent.mkdir(parents=True, exist_ok=True)
        registry.write_text(json.dumps({'version': 1, 'files': self.files}))
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                fixture.requests.append(self.path)
                canonical = 'GET\n\n\n' + self.headers.get('Date', '') + '\n/fixture' + self.path
                signature = base64.b64encode(hmac.new(b'fixture-secret', canonical.encode(), hashlib.sha1).digest()).decode()
                if self.headers.get('Authorization') != 'OSS fixture-key:' + signature:
                    self.send_error(403)
                    return
                body, mime = fixture.objects[self.path]
                self.send_response(200)
                self.send_header('Content-Type', mime)
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                fixture.wire_bytes += len(body)

        cert, key = self.root / 'cert.pem', self.root / 'key.pem'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=localhost', '-addext', 'subjectAltName=IP:127.0.0.1', '-keyout', str(key), '-out', str(cert)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert, key)
        self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.environment = {**os.environ, 'PUBLIC_ORIGIN': 'https://public.invalid', 'CDN_ORIGIN': 'https://cdn.invalid', 'OSS_ENDPOINT': '127.0.0.1:' + str(self.server.server_port), 'OSS_BUCKET': 'fixture', 'OSS_ACCESS_KEY_ID': 'fixture-key', 'OSS_ACCESS_KEY_SECRET': 'fixture-secret', 'SSL_CERT_FILE': str(cert), 'GITHUB_ACTIONS': 'true', 'GITHUB_EVENT_NAME': 'workflow_dispatch'}
        self.environment['GITHUB_WORKFLOW_REF'] = 'fixture/repo/.github/workflows/resource-cache.yml@refs/heads/main'

    def add_resource(self, name, body):
        if name.startswith('games/'):
            (self.root / '/'.join(name.split('/')[:2])).mkdir(parents=True, exist_ok=True)
        owner = name.split('/')[1] if name.startswith('games/') else '_common'
        digest = hashlib.sha256(body).hexdigest()
        record = {'key': 'bga/' + owner + '/' + digest + Path(name).suffix, 'sha256': digest, 'bytes': len(body), 'mime': 'application/octet-stream'}
        self.files[name] = record
        self.objects['/' + record['key']] = (body, record['mime'])
        return record

    def invoke(self, action, scope='game', game='splendor', budget=None, github_output=None):
        report = self.root / 'reports' / (action + '.json')
        command = [sys.executable, '-B', str(ROOT / 'scripts/ci/resources.py'), action, '--root', str(self.root), '--scope', scope, '--cache', str(self.cache), '--report', str(report)]
        if scope == 'game':
            command += ['--game', game]
        if budget is not None:
            command += ['--max-download-bytes', str(budget)]
        if github_output is not None:
            command += ['--github-output', str(github_output)]
        result = subprocess.run(command, cwd=ROOT, env=self.environment, capture_output=True, text=True)
        return result, json.loads(report.read_text()) if report.exists() else {}

    def test_cold_normal_ci_fails_before_any_paid_request(self):
        result, report = self.invoke('restore')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.requests, [])
        self.assertFalse(report['passed'])
        self.assertEqual(report['downloadedBytes'], 0)
        self.assertGreater(report['missingObjects'], 0)

    def test_explicit_bounded_prime_then_repeat_restore_has_zero_oss_downloads(self):
        result, prime = self.invoke('prime', budget=1024)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertGreater(prime['downloadedBytes'], 0)
        self.assertEqual(prime['downloadedBytes'], self.wire_bytes)
        self.assertNotIn('/' + self.files['games/yuyuhakusho/engine/testdata/replay.bin']['key'], self.requests)
        before = len(self.requests)
        for unused in range(2):
            shutil.rmtree(self.root / 'games/splendor', ignore_errors=True)
            (self.root / 'games/splendor').mkdir(parents=True, exist_ok=True)
            result, report = self.invoke('restore')
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(report['downloadedBytes'], 0)
        self.assertEqual(len(self.requests), before)
        self.assertEqual((self.root / 'games/splendor/engine/testdata/replay.bin').read_bytes(), b'private replay')
        self.assertFalse((self.root / 'games/yuyuhakusho/engine/testdata/replay.bin').exists())
        (WORK / 'cache-reuse.json').write_text(json.dumps({'passed': True, 'primeBytes': prime['downloadedBytes'], 'normalRestoreBytes': report['downloadedBytes'], 'primeRequests': before, 'repeatRequests': len(self.requests) - before}, indent=2) + '\n')

    def test_insufficient_or_implicit_maintenance_budget_does_not_start_network(self):
        for budget in [None, 0, 1, -1, 'invalid', 2147483649]:
            result, report = self.invoke('prime', budget=budget)
            self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.requests, [])

    def test_normal_restore_cannot_enable_paid_fallback_with_a_budget(self):
        result, report = self.invoke('restore', budget=1024)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.requests, [])

    def test_corrupt_cache_fails_without_oss_repair(self):
        result, report = self.invoke('prime', budget=1024)
        self.assertEqual(result.returncode, 0, result.stderr)
        files = [p for p in self.cache.rglob('*') if p.is_file() and p.suffix != '.json']
        self.assertTrue(files)
        files[0].write_bytes(b'corrupt')
        shutil.rmtree(self.root / 'games/splendor', ignore_errors=True)
        (self.root / 'games/splendor').mkdir(parents=True, exist_ok=True)
        before = len(self.requests)
        result, report = self.invoke('restore')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(self.requests), before)
        self.assertEqual(report['downloadedBytes'], 0)

    def test_frontend_server_and_shared_plans_do_not_include_unrelated_test_inputs(self):
        for scope in ['frontend', 'server', 'shared']:
            result, plan = self.invoke('plan', scope=scope)
            self.assertEqual(result.returncode, 0, result.stderr)
            names = set(plan['files'])
            self.assertNotIn('games/yuyuhakusho/engine/testdata/replay.bin', names)
            self.assertNotIn('docs/research/md-yuyuhakusho/capture.bin', names)
            self.assertIn('games/contra/server/data/rules.bin', names)
            if scope == 'frontend':
                self.assertIn('games/splendor/assets/public/splendor/card.webp', names)
                self.assertIn('web/src/controllers/assets/pad.svg', names)
                self.assertIn('games/splendor/web/assets/card.svg', names)
                self.assertIn('games/splendor/assets/preview.png', names)
            else:
                self.assertNotIn('games/splendor/assets/public/splendor/card.webp', names)
            if scope == 'shared':
                self.assertIn('internal/server/testdata/input.bin', names)

    def test_cache_key_is_stable_for_unrelated_game_and_source_commit_changes(self):
        result, first = self.invoke('plan')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.add_resource('games/yuyuhakusho/engine/testdata/second.bin', b'more unrelated replay')
        (self.root / 'assets/resources.json').write_text(json.dumps({'version': 1, 'files': self.files}))
        self.environment['GITHUB_SHA'] = 'b' * 40
        result, second = self.invoke('plan')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(first['cacheKey'], second['cacheKey'])

    def test_default_game_cache_and_report_stay_under_the_game_directory(self):
        output = self.root / 'github-output'
        result = subprocess.run([sys.executable, '-B', str(ROOT / 'scripts/ci/resources.py'), 'plan', '--root', str(self.root), '--scope', 'game', '--game', 'splendor', '--github-output', str(output)], cwd=ROOT, env=self.environment, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        values = dict(line.split('=', 1) for line in output.read_text().splitlines())
        self.assertEqual(values['path'], '.local/splendor/cache/ci-resources')
        self.assertEqual(values['report'], '.local/splendor/verification/ci-traffic/resources.json')

    def test_symlink_and_traversal_inputs_are_rejected_without_network(self):
        bad = self.files['games/splendor/engine/testdata/replay.bin']
        for name in ['../escape.bin', 'games/splendor/../escape.bin']:
            self.files[name] = bad
            (self.root / 'assets/resources.json').write_text(json.dumps({'version': 1, 'files': self.files}))
            result, report = self.invoke('prime', budget=1024)
            self.assertNotEqual(result.returncode, 0)
            self.files.pop(name)
        self.assertEqual(self.requests, [])

    def test_legacy_hydration_is_disabled_in_actions_before_accessing_secrets(self):
        result = subprocess.run([sys.executable, '-B', str(ROOT / 'scripts/resource_store.py'), 'hydrate', '--oss'], cwd=ROOT, env=self.environment, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('verified GitHub resource caches', result.stderr)
        self.assertEqual(self.requests, [])

    def test_oversized_source_response_is_rejected_without_exceeding_the_budget(self):
        record = self.files['games/splendor/engine/testdata/replay.bin']
        self.objects['/' + record['key']] = (b'oversized' * 1000, record['mime'])
        result, report = self.invoke('prime', budget=1024)
        self.assertNotEqual(result.returncode, 0)
        self.assertLessEqual(report['downloadedBytes'], report['budgetBytes'])
        self.assertFalse((self.root / 'games/splendor/engine/testdata/replay.bin').exists())

    def test_another_manual_workflow_cannot_enable_paid_resource_reads(self):
        self.environment['GITHUB_WORKFLOW_REF'] = 'fixture/repo/.github/workflows/build.yml@refs/heads/main'
        result, report = self.invoke('prime', budget=1024)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.requests, [])

    def test_manual_repair_records_the_immutable_cache_replacement(self):
        result, report = self.invoke('prime', budget=1024)
        self.assertEqual(result.returncode, 0, result.stderr)
        logical = 'games/splendor/engine/testdata/replay.bin'
        record = self.files[logical]
        (self.cache / 'objects' / record['key']).write_bytes(b'corrupt')
        (self.root / logical).unlink()
        before = len(self.requests)
        output = self.root / 'repair-output'
        result, report = self.invoke('prime', budget=1024, github_output=output)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(report['repairedObjects'], 1)
        self.assertEqual(len(self.requests) - before, 1)
        self.assertIn('repaired=true', output.read_text())


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(ResourceTrafficE2E))
    (WORK / 'resources-e2e.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
