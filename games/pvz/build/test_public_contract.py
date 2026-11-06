import gzip
import importlib.util
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[3]
ASSETS = ROOT / "games/pvz/assets/public/pvz/classic-v2"


class PublicAssetContractTests(unittest.TestCase):
    def test_native_payload_is_explicit_standalone_container(self):
        manifest = json.loads((ASSETS / "manifest.json").read_text())
        for kind, sprite in manifest["sprites"].items():
            native = sprite["native"]
            self.assertTrue(native["src"].endswith(".pvzr"), kind)
            self.assertEqual(native["compression"], "gzip")
            self.assertEqual(native["format"], "pvz-reanimation-json-v1")
            encoded = (ASSETS / native["src"]).read_bytes()
            self.assertEqual(encoded[:2], b"\x1f\x8b")
            self.assertTrue(json.loads(gzip.decompress(encoded))["clips"])

    def test_every_actual_public_path_is_portable(self):
        for path in ASSETS.rglob("*"):
            if path.is_file():
                relative = path.relative_to(ASSETS).as_posix()
                self.assertRegex(relative, r"^[A-Za-z0-9._/-]+$", relative)
                self.assertFalse(relative.endswith(".json.gz"), relative)

    def test_actual_cdn_inventory_accepts_complete_directory(self):
        spec = importlib.util.spec_from_file_location("pvz_cdn_delivery", ROOT / "scripts/cdn_delivery.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        rows = module.inventory(ASSETS, "https://cdn.ichenj.com/bga/web/" + "a" * 40 + "/")
        payloads = [row for row in rows if row["path"].endswith(".pvzr")]
        self.assertEqual(len(payloads), 99)
        for row in payloads:
            self.assertEqual(row["mime"], "application/octet-stream")
            self.assertEqual(row["encoding"], "")
            self.assertEqual(row["bytes"], row["wireBytes"])


if __name__ == "__main__":
    unittest.main()
