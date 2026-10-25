#!/usr/bin/env python3
"""Real local HTTP checks for the build downloader.

Failure inventory, before implementation: a trickling response never reaches
the socket idle timeout; HTTP errors are accepted as media; transient errors
cannot recover; redirects or binary bytes are corrupted; retry count is
unbounded. Existing manifest length/SHA checks must remain in place.
"""
import importlib.util
import threading
import time
import unittest
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


if __name__ == "__main__":
    unittest.main()
