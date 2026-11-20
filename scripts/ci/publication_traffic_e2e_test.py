import base64
import gzip
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import ssl
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'scripts'))
import cdn_delivery as cdn

WORK = ROOT / '.local/ci-traffic'
WORK.mkdir(parents=True, exist_ok=True)


class PublicationTrafficE2E(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(dir=WORK)
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.objects = {}
        self.calls = []
        self.signed = True
        self.race = False
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def authorize(self):
                fixture.calls.append((self.command, self.path))
                extra = ''.join(name.lower() + ':' + value + '\n' for name, value in sorted((name.lower(), value) for name, value in self.headers.items() if name.lower().startswith('x-oss-')))
                canonical = self.command + '\n' + self.headers.get('Content-MD5', '') + '\n' + self.headers.get('Content-Type', '') + '\n' + self.headers['Date'] + '\n' + extra + '/fixture' + self.path
                signature = base64.b64encode(hmac.new(b'secret', canonical.encode(), hashlib.sha1).digest()).decode()
                fixture.signed = fixture.signed and self.headers.get('Authorization') == 'OSS key:' + signature
                return fixture.signed

            def do_GET(self):
                self.authorize()
                self.send_error(500, 'GET forbidden')

            def do_HEAD(self):
                if not self.authorize():
                    self.send_error(403)
                    return
                if self.path not in fixture.objects:
                    self.send_error(404)
                    return
                body, headers = fixture.objects[self.path]
                self.send_response(200)
                for name, value in {**headers, 'Content-Length': str(len(body)), 'ETag': '"' + hashlib.md5(body).hexdigest() + '"'}.items():
                    self.send_header(name, value)
                self.end_headers()

            def do_PUT(self):
                if not self.authorize():
                    self.send_error(403)
                    return
                body = self.rfile.read(int(self.headers['Content-Length']))
                if self.headers.get('x-oss-forbid-overwrite') != 'true' or base64.b64encode(hashlib.md5(body).digest()).decode() != self.headers.get('Content-MD5'):
                    self.send_error(400)
                    return
                headers = {name: self.headers[name] for name in ['Content-Type', 'Content-Encoding', 'x-oss-meta-sha256', 'x-oss-meta-decoded-bytes'] if name in self.headers}
                fixture.objects[self.path] = (body, headers)
                self.send_response(409 if fixture.race else 200)
                self.send_header('Content-Length', '0')
                self.end_headers()

        cert, key = self.root / 'cert.pem', self.root / 'key.pem'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=localhost', '-addext', 'subjectAltName=IP:127.0.0.1', '-keyout', str(key), '-out', str(cert)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert, key)
        self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.addCleanup(cdn.close_connections)
        self.trusted = ssl.create_default_context(cafile=str(cert))
        self.endpoint = '127.0.0.1:' + str(self.server.server_port)

    def row(self, name, body, encoding=''):
        (self.root / name).write_bytes(body)
        return {'path': name, 'url': 'https://example.invalid/bga/_common/' + cdn.sha(body) + '.bin', 'sha256': cdn.sha(body), 'bytes': len(body), 'mime': 'application/octet-stream', 'encoding': encoding}

    def publish(self, rows):
        with patch.object(cdn, 'credentials', return_value=('key', 'secret')), patch.object(cdn, '_https_proxy', None), patch('ssl._create_default_https_context', return_value=self.trusted):
            return cdn.publish(self.root, rows, 'fixture', self.endpoint)

    def test_new_repeat_and_changed_publication_never_get_bodies(self):
        first = self.row('first.bin', b'first payload')
        fresh = self.publish([first, first])
        self.assertEqual(fresh['uploadedObjects'], 1)
        self.assertEqual(fresh['downloadedBytes'], 0)
        reused = self.publish([first])
        self.assertEqual(reused['uploadedBytes'], 0)
        second = self.row('second.bin', b'changed payload')
        changed = self.publish([first, second])
        self.assertEqual(changed['uploadedObjects'], 1)
        self.assertTrue(self.signed)
        self.assertFalse(any(method == 'GET' for method, path in self.calls))
        headers = next(iter(self.objects.values()))[1]
        self.assertEqual(headers['x-oss-meta-sha256'], first['sha256'])
        self.assertEqual(headers['x-oss-meta-decoded-bytes'], str(first['bytes']))
        (WORK / 'publication-bytes.json').write_text(json.dumps({'passed': True, 'first': fresh, 'repeat': reused, 'changed': changed, 'methods': [method for method, path in self.calls]}, indent=2) + '\n')

    def test_verified_external_receipts_need_no_oss_requests(self):
        row = self.row('known.bin', b'known bytes')
        (self.root / 'known.bin').unlink()
        row.update(previouslyPublished=True, receiptVerified=True)
        stats = self.publish([row] * 1000)
        self.assertEqual(self.calls, [])
        self.assertEqual(stats['uploadedBytes'], 0)
        self.assertEqual(stats['downloadedBytes'], 0)

    def test_head_rejects_modified_bytes_mime_encoding_and_size(self):
        row = self.row('expected.bin', b'expected bytes')
        key = '/' + row['url'].split('/bga/', 1)[1]
        key = '/bga/' + key.lstrip('/')
        for body, headers in [(b'wrong contents', {'Content-Type': row['mime']}), (b'expected bytes', {'Content-Type': 'text/html'}), (b'expected bytes', {'Content-Type': row['mime'], 'Content-Encoding': 'gzip'}), (b'wrong length', {'Content-Type': row['mime']})]:
            self.objects[key] = (body, headers)
            with self.assertRaises(ValueError) as raised:
                self.publish([row])
            self.assertEqual(raised.exception.publication['downloadedBytes'], 0)
            self.assertGreater(raised.exception.publication['headRequests'], 0)
        self.assertFalse(any(method in ['GET', 'PUT'] for method, path in self.calls))

    def test_unknown_external_object_without_digest_metadata_is_rejected(self):
        row = self.row('unknown.bin', b'unknown bytes')
        (self.root / 'unknown.bin').unlink()
        self.objects['/bga/' + row['url'].split('/bga/', 1)[1]] = (b'unknown bytes', {'Content-Type': row['mime']})
        with self.assertRaises(ValueError):
            self.publish([row])
        self.assertFalse(any(method in ['GET', 'PUT'] for method, path in self.calls))

    def test_put_conflict_is_resolved_using_head_and_not_get(self):
        self.race = True
        row = self.row('raced.bin', b'raced bytes', encoding='gzip')
        self.publish([row])
        self.assertEqual([method for method, path in self.calls], ['HEAD', 'PUT', 'HEAD'])
        body = next(iter(self.objects.values()))[0]
        self.assertEqual(gzip.decompress(body), b'raced bytes')

    def test_failed_batch_keeps_completed_parallel_upload_counters(self):
        bad = self.row('bad.bin', b'expected bad bytes')
        good = self.row('good.bin', b'new good bytes')
        key = '/bga/' + bad['url'].split('/bga/', 1)[1]
        self.objects[key] = (b'corrupt existing bytes', {'Content-Type': bad['mime']})
        with self.assertRaises(ValueError) as raised:
            self.publish([bad, good])
        self.assertEqual(sum(method == 'PUT' for method, path in self.calls), 1)
        self.assertEqual(raised.exception.publication['uploadedObjects'], 1)
        self.assertEqual(raised.exception.publication['uploadedBytes'], good['bytes'])
        self.assertEqual(raised.exception.publication['downloadedBytes'], 0)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(PublicationTrafficE2E))
    (WORK / 'publication-e2e.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
