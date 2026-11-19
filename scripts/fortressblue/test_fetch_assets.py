"""Real local HTTP checks for the build downloader.

Failure inventory, before implementation: a trickling response never reaches
the socket idle timeout; HTTP errors are accepted as media; transient errors
cannot recover; redirects or binary bytes are corrupted; retry count is
unbounded. Existing manifest length/SHA checks must remain in place.
"""
import importlib.util
import json
import hashlib
import threading
import time
import unittest
import tempfile
from unittest.mock import patch
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

spec = importlib.util.spec_from_file_location("fetch_assets", Path(__file__).with_name("fetch-assets.py"))
fetch_assets = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fetch_assets)


class DownloadHTTPTests(unittest.TestCase):
    def setUp(self):
        self.counts = {}
        counts = self.counts

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                counts[self.path] = counts.get(self.path, 0) + 1
                if self.path == "/redirect":
                    self.send_response(302)
                    self.send_header("Location", "/ok")
                    self.end_headers()
                    return
                if self.path == "/error" or (self.path == "/retry" and counts[self.path] == 1):
                    self.send_response(503)
                    self.end_headers()
                    return
                data = b"\x00\xffmedia\x00" if self.path != "/slow" else b"x" * 40
                self.send_response(200)
                self.send_header("Content-Length", str(len(data)))
                self.send_header("Content-Type", "application/octet-stream")
                self.end_headers()
                try:
                    if self.path == "/slow":
                        for byte in data:
                            self.wfile.write(bytes([byte]))
                            self.wfile.flush()
                            time.sleep(0.05)
                    else:
                        self.wfile.write(data)
                except (BrokenPipeError, ConnectionResetError):
                    pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={"poll_interval": 0.01})
        self.thread.start()
        self.base = f"http://127.0.0.1:{self.server.server_port}"
        fetch_assets.TRANSFER_TIMEOUT = 0.2
        fetch_assets.DOWNLOAD_ATTEMPTS = 2
        fetch_assets.RETRY_DELAY = 0

    def tearDown(self):
        self.server.shutdown()
        self.thread.join()
        self.server.server_close()

    def test_signed_oss_restore_uses_canonical_registry_without_cdn_access(self):
        parent=Path(__file__).resolve().parents[2]/'.local/fortressblue/media-tests'
        parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent) as name:
            root=Path(name)
            body=b"\x00\xffmedia\x00"
            entry={'urlPath':'/fortressblue/v2/media.webp','output':'games/fortressblue/assets/public/fortressblue/v2/media.webp','sha256':hashlib.sha256(body).hexdigest(),'bytes':len(body)}
            record={'key':'bga/fortressblue/'+entry['sha256']+'.webp','sha256':entry['sha256'],'bytes':len(body),'mime':'application/octet-stream'}
            path=root/'assets/external-resources.json'
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps({'version':1,'files':{entry['urlPath']:record}}))
            client=fetch_assets.ObjectStore(self.base,'fixture','test-key','test-secret',local=True)
            with patch.object(fetch_assets,'ROOT',root):
                fetch_assets.restore_oss(client,[entry],'https://unreachable.example.invalid')
                self.assertEqual((root/entry['output']).read_bytes(),body)
                self.assertEqual(self.counts['/'+record['key']],1)
                fetch_assets.restore_oss(client,[entry],'https://unreachable.example.invalid')
                self.assertEqual(self.counts['/'+record['key']],1)
                (root/entry['output']).unlink()
                record['sha256']='f'*64
                path.write_text(json.dumps({'version':1,'files':{entry['urlPath']:record}}))
                with self.assertRaises(ValueError):
                    fetch_assets.restore_oss(client,[entry],'https://unreachable.example.invalid')
                self.assertFalse((root/entry['output']).exists())

    def test_binary_redirect_preserves_bytes(self):
        self.assertEqual(fetch_assets.fetch(self.base + "/redirect"), b"\x00\xffmedia\x00")

    def test_transient_error_retries(self):
        self.assertEqual(fetch_assets.fetch(self.base + "/retry"), b"\x00\xffmedia\x00")
        self.assertEqual(self.counts["/retry"], 2)

    def test_http_failure_is_bounded(self):
        with self.assertRaises(RuntimeError):
            fetch_assets.fetch(self.base + "/error")
        self.assertEqual(self.counts["/error"], 2)

    def test_trickling_response_has_total_deadline(self):
        began = time.monotonic()
        with self.assertRaises(RuntimeError):
            fetch_assets.fetch(self.base + "/slow")
        self.assertLess(time.monotonic() - began, 1.5)


class VerifiedMediaCacheTests(unittest.TestCase):
    def setUp(self):
        parent=Path(__file__).resolve().parents[2]/".local/fortressblue/media-tests";parent.mkdir(parents=True,exist_ok=True)
        self.temp=tempfile.TemporaryDirectory(dir=parent);self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);self.source=self.root/"artifact";self.source.mkdir()
        self.body=b"immutable media";self.relative="fortressblue/v2/image.webp"
        self.entry={"urlPath":"/"+self.relative,"output":"games/fortressblue/assets/public/"+self.relative,"bytes":len(self.body),"sha256":fetch_assets.sha(self.body)}
        p=self.source/self.relative;p.parent.mkdir(parents=True);p.write_bytes(self.body)
        (self.source/"index.html").write_text("old program must not be copied")
        self.patch=patch.object(fetch_assets,"ROOT",self.root);self.patch.start();self.addCleanup(self.patch.stop)
    def test_only_exact_manifest_media_is_restored(self):
        self.assertEqual(fetch_assets.restore_media(self.source,[self.entry]),1)
        self.assertEqual((self.root/self.entry['output']).read_bytes(),self.body)
        self.assertFalse((self.root/'web/dist/index.html').exists())
    def test_missing_or_changed_media_is_not_accepted(self):
        (self.source/self.relative).write_bytes(b'changed')
        self.assertEqual(fetch_assets.restore_media(self.source,[self.entry]),0)
        self.assertFalse((self.root/self.entry['output']).exists())
        self.assertEqual(fetch_assets.restore_media(self.root/'missing',[self.entry]),0)
    def test_symlinks_and_invalid_destinations_are_rejected(self):
        source=self.source/self.relative;source.unlink();source.symlink_to(self.source/'index.html')
        with self.assertRaises(ValueError):fetch_assets.restore_media(self.source,[self.entry])
        source.unlink();source.write_bytes(self.body)
        for fields in [{'output':'../escape.webp'},{'urlPath':'/../index.html'}]:
            with self.assertRaises(ValueError):fetch_assets.restore_media(self.source,[{**self.entry,**fields}])
        public=self.root/'games/fortressblue/assets/public';public.parent.mkdir(parents=True)
        escaped=self.root/'outside-public';escaped.mkdir();public.symlink_to(escaped,target_is_directory=True)
        with self.assertRaises(ValueError):fetch_assets.restore_media(self.source,[self.entry])

if __name__ == "__main__":
    unittest.main()
