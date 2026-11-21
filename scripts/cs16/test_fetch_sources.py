import hashlib
import gc
import http.server
import importlib.util
import json
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import threading
import unittest
import warnings
import zipfile


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / ".local/cs16/verification/source-fetch"
TEMP = ROOT / ".local/cs16/tmp"
OUTPUT.mkdir(parents=True, exist_ok=True)
TEMP.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location("cs16_fetch_sources", Path(__file__).with_name("fetch_sources.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
PAYLOAD = b"cs16-fixed-source-fixture"


def record(path="maps/sample.bsp", url="https://example.invalid/source", payload=PAYLOAD):
    return {"path": path, "url": url, "bytes": len(payload), "sha256": hashlib.sha256(payload).hexdigest(), "git_blob_sha1": hashlib.sha1(b"blob " + str(len(payload)).encode() + b"\0" + payload).hexdigest()}


class SourceHandler(http.server.BaseHTTPRequestHandler):
    requests = []

    def log_message(self, *_values):
        pass

    def do_GET(self):
        type(self).requests.append(self.path)
        if self.path == "/redirect":
            self.send_response(302)
            self.send_header("Location", "/good")
            self.end_headers()
            return
        if self.path == "/missing":
            self.send_error(404)
            return
        data = PAYLOAD
        if self.path == "/bad":
            data = bytes(len(PAYLOAD))
        if self.path == "/short":
            data = PAYLOAD[:-1]
        if self.path == "/long":
            data = PAYLOAD + b"extra"
        self.send_response(200)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


class SourceFetchTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), SourceHandler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.origin = "http://127.0.0.1:" + str(cls.server.server_port)

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join()

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(dir=TEMP)
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.destination = self.root / "research/original"

    def test_http_fetch_validates_and_reuses_without_second_request(self):
        entry = record(url=self.origin + "/good")
        first = MODULE.fetch_entry(entry, self.destination)
        self.assertEqual(first["status"], "downloaded")
        self.assertEqual((self.destination / entry["path"]).read_bytes(), PAYLOAD)
        count = len(SourceHandler.requests)
        second = MODULE.fetch_entry(entry, self.destination)
        self.assertEqual(second["status"], "reused")
        self.assertEqual(len(SourceHandler.requests), count)
        self.assertFalse(list(TEMP.glob("cs16-source-*")))

    def test_http_failures_preserve_existing_target_and_clean_temporary_files(self):
        target = self.destination / "maps/sample.bsp"
        target.parent.mkdir(parents=True)
        target.write_bytes(b"previous")
        for suffix in ("missing", "redirect", "bad", "short", "long"):
            with self.subTest(suffix=suffix):
                entry = record(url=self.origin + "/" + suffix)
                with self.assertRaises(ValueError):
                    MODULE.fetch_entry(entry, self.destination)
                self.assertEqual(target.read_bytes(), b"previous")
                self.assertFalse(list(TEMP.glob("cs16-source-*")))
        self.assertNotIn("/good", SourceHandler.requests[-5:])

    def test_http_error_response_is_closed_without_resource_warnings(self):
        with warnings.catch_warnings(record=True) as observed:
            warnings.simplefilter("always", ResourceWarning)
            with self.assertRaises(ValueError):
                MODULE.fetch_entry(record(url=self.origin + "/missing"), self.destination)
            gc.collect()
        self.assertFalse([item for item in observed if issubclass(item.category, ResourceWarning)])

    def test_offline_check_never_downloads_missing_or_corrupt_sources(self):
        entry = record(url=self.origin + "/good")
        count = len(SourceHandler.requests)
        with self.assertRaises(ValueError):
            MODULE.fetch_entry(entry, self.destination, check=True)
        target = self.destination / entry["path"]
        target.parent.mkdir(parents=True)
        target.write_bytes(bytes(len(PAYLOAD)))
        with self.assertRaises(ValueError):
            MODULE.fetch_entry(entry, self.destination, check=True)
        self.assertEqual(len(SourceHandler.requests), count)

    def test_path_escape_and_symlink_components_are_rejected(self):
        for value in ("../escape", "/absolute", "maps//sample", "maps/./sample", "maps\\sample", "C:/escape", ""):
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    MODULE.fetch_entry(record(path=value, url=self.origin + "/good"), self.destination)
        other = self.root / "other"
        other.mkdir()
        self.destination.mkdir(parents=True)
        (self.destination / "maps").symlink_to(other, target_is_directory=True)
        with self.assertRaises(ValueError):
            MODULE.fetch_entry(record(url=self.origin + "/good"), self.destination)
        self.assertEqual(list(other.iterdir()), [])

    def test_manifests_reject_unpinned_urls_duplicate_paths_and_bad_digests(self):
        source = json.loads((ROOT / "games/cs16/assets/original-sources.json").read_text())
        self.assertEqual(MODULE.validate_manifest(source, "original"), source["files"])
        for mutate in (lambda item: item.update(url="https://example.invalid/file"), lambda item: item.update(sha256="bad"), lambda item: item.update(bytes=-1)):
            broken = json.loads(json.dumps(source))
            mutate(broken["files"][0])
            with self.assertRaises(ValueError):
                MODULE.validate_manifest(broken, "original")
        broken = json.loads(json.dumps(source))
        broken["files"].append({**broken["files"][0], "path": broken["files"][0]["path"].upper()})
        with self.assertRaises(ValueError):
            MODULE.validate_manifest(broken, "original")

    def test_reference_archive_rejects_escape_and_symbolic_links_before_extraction(self):
        for name, symbolic in (("source/../escape", False), ("source/link", True)):
            archive = self.root / "bad.zip"
            with zipfile.ZipFile(archive, "w") as bundle:
                bundle.writestr("source/valid.txt", b"valid")
                member = zipfile.ZipInfo(name)
                if symbolic:
                    member.create_system = 3
                    member.external_attr = (stat.S_IFLNK | 0o777) << 16
                bundle.writestr(member, b"bad")
            with self.assertRaises(ValueError):
                MODULE.extract_reference(archive, self.root / "reference")
            self.assertFalse((self.root / "reference/valid.txt").exists())

    def test_reference_archive_extracts_and_reuses_verified_members(self):
        archive = self.root / "reference.zip"
        with zipfile.ZipFile(archive, "w") as bundle:
            bundle.writestr("source/src/sample.cpp", PAYLOAD)
        target = self.root / "reference"
        first = MODULE.extract_reference(archive, target)
        second = MODULE.extract_reference(archive, target)
        self.assertEqual(first, {"written": 1, "reused": 0})
        self.assertEqual(second, {"written": 0, "reused": 1})
        self.assertEqual((target / "src/sample.cpp").read_bytes(), PAYLOAD)

    def test_fixed_source_check_cli_is_reproducible_and_completely_offline(self):
        result = subprocess.run([sys.executable, "-B", str(ROOT / "scripts/cs16/fetch_sources.py"), "--check"], cwd=ROOT, capture_output=True, text=True, timeout=60)
        self.assertEqual(result.returncode, 0, result.stderr)
        report = json.loads(result.stdout)
        self.assertEqual(report["downloaded"], 0)
        catalogs = [("original-sources.json", "original"), ("navigation-sources.json", "navigation")]
        if (ROOT / "games/cs16/assets/community-sources.json").is_file():
            catalogs.append(("community-sources.json", "community"))
        expected = sum(len(MODULE.validate_manifest(json.loads((ROOT / "games/cs16/assets" / name).read_text()), kind)) for name, kind in catalogs)
        self.assertEqual(report["reused"], expected)
        self.assertTrue(report["check"])
        (OUTPUT / "offline-community-check.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    unittest.main()
