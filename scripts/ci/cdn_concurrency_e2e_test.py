import json
import hashlib
import io
from contextlib import redirect_stdout
from pathlib import Path
import ssl
import subprocess
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'scripts'))
import cdn_delivery as cdn

WORK = ROOT / '.local/resource-cleanup/cdn-concurrency'
WORK.mkdir(parents=True, exist_ok=True)


class PublicationConcurrencyE2E(unittest.TestCase):
    def exercise(self, corrupt):
        with tempfile.TemporaryDirectory(dir=WORK) as temporary:
            folder = Path(temporary)
            cert, key = folder / 'cert.pem', folder / 'key.pem'
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=localhost', '-addext', 'subjectAltName=IP:127.0.0.1', '-keyout', str(key), '-out', str(cert)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            payload = b'unchanged immutable resource'
            lock, released = threading.Lock(), threading.Event()
            state = {'active': 0, 'peak': 0, 'paths': [], 'authorized': True, 'gets': 0}

            class Handler(BaseHTTPRequestHandler):
                protocol_version = 'HTTP/1.1'

                def log_message(self, *args):
                    pass

                def do_GET(self):
                    state['gets'] += 1
                    self.send_error(500, 'Publication cannot download object bodies')

                def do_HEAD(self):
                    with lock:
                        state['active'] += 1
                        state['peak'] = max(state['peak'], state['active'])
                        state['paths'].append(self.path)
                        state['authorized'] = state['authorized'] and self.headers.get('Authorization', '').startswith('OSS test:')
                        if state['active'] >= 8:
                            released.set()
                    released.wait(0.15)
                    body = payload + b'corrupt' if corrupt and self.path.endswith('/63.bin') else payload
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/octet-stream')
                    self.send_header('Content-Length', str(len(body)))
                    self.send_header('ETag', '"' + hashlib.md5(body).hexdigest() + '"')
                    self.end_headers()
                    with lock:
                        state['active'] -= 1

            class Server(ThreadingHTTPServer):
                daemon_threads = True
                request_queue_size = 128

            server = Server(('127.0.0.1', 0), Handler)
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(cert, key)
            server.socket = context.wrap_socket(server.socket, server_side=True)
            trusted = ssl.create_default_context(cafile=str(cert))
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            rows = [{'url': 'https://example.invalid/bga/_common/' + str(index) + '.bin', 'path': str(index) + '.bin', 'bytes': len(payload), 'sha256': cdn.sha(payload), 'mime': 'application/octet-stream', 'encoding': ''} for index in range(64)]
            for row in rows:
                (folder / row['path']).write_bytes(payload)
            try:
                with patch.object(cdn, 'credentials', return_value=('test', 'test')), patch.object(cdn, '_https_proxy', None), patch('ssl._create_default_https_context', return_value=trusted):
                    if corrupt:
                        with self.assertRaisesRegex(ValueError, 'Immutable object already differs'):
                            cdn.publish(folder, rows, 'fixture', '127.0.0.1:' + str(server.server_port))
                    else:
                        output = io.StringIO()
                        with redirect_stdout(output):
                            stats = cdn.publish(folder, rows + rows[:8], 'fixture', '127.0.0.1:' + str(server.server_port))
                        self.assertEqual(stats['reusedObjects'], 64)
                        self.assertEqual(stats['uploadedObjects'], 0)
                        self.assertEqual(stats['putRequests'], 0)
                        self.assertEqual(stats['reusedBytes'], 64 * len(payload))
                        self.assertEqual(stats['reusedWireBytes'], 0)
                        self.assertEqual(stats['getRequests'], 0)
                        progress = [json.loads(line) for line in output.getvalue().splitlines() if line.startswith('{')]
                        self.assertTrue(any(row.get('stage') == 'oss-verification' and row.get('verifiedObjects') == 64 and row.get('totalObjects') == 64 for row in progress))
                self.assertTrue(state['authorized'])
                self.assertEqual(state['gets'], 0)
                self.assertEqual(sorted(state['paths']), sorted('/bga/_common/' + str(index) + '.bin' for index in range(64)))
                self.assertGreaterEqual(state['peak'], 8, 'Bulk publication must not serialize a large immutable inventory through six connections')
                self.assertLessEqual(state['peak'], 64)
                return {'corrupt': corrupt, 'requests': len(state['paths']), 'peak': state['peak'], 'getRequests': state['gets'], 'downloadedBytes': 0, 'passed': True}
            finally:
                released.set()
                cdn.close_connections()
                server.shutdown()
                server.server_close()
                thread.join(timeout=2)

    def test_reuses_every_exact_object_in_parallel_without_duplicate_uploads(self):
        result = self.exercise(False)
        (WORK / 'exact-objects.json').write_text(json.dumps(result, indent=2) + '\n')

    def test_late_corruption_fails_the_entire_publication(self):
        result = self.exercise(True)
        (WORK / 'corrupt-object.json').write_text(json.dumps(result, indent=2) + '\n')


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(PublicationConcurrencyE2E))
    (WORK / 'result.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
