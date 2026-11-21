import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / ".local/cs16/verification/community-maps"
OUTPUT.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location("cs16_community_builder", Path(__file__).with_name("build_assets.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
FETCH_SPEC = importlib.util.spec_from_file_location("cs16_community_fetch", Path(__file__).with_name("fetch_sources.py"))
FETCH = importlib.util.module_from_spec(FETCH_SPEC)
FETCH_SPEC.loader.exec_module(FETCH)
SOURCE = ROOT / ".local/cs16/research/original"


def original(map_id):
    return MODULE.Bsp((SOURCE / "cstrike/maps" / (map_id + ".bsp")).read_bytes())


def map_payloads():
    source = (ROOT / "games/cs16/data/maps.go").read_text()
    return json.loads(source.split("`", 2)[1])


class CommunitySourceTest(unittest.TestCase):
    def test_multi_repository_manifest_pins_every_file(self):
        catalog = json.loads((ROOT / "games/cs16/assets/community-sources.json").read_text())
        entries = FETCH.validate_manifest(catalog, "community")
        self.assertGreaterEqual(len(entries), 13)
        self.assertEqual(len(catalog["sources"]), 2)
        for entry in entries:
            self.assertTrue(FETCH.verify_file(ROOT / ".local/cs16/research" / entry["path"], entry))
        for field, value in (("url", "https://example.invalid/map.bsp"), ("source_path", "../escape"), ("source", "unknown")):
            broken = copy.deepcopy(catalog)
            broken["files"][0][field] = value
            with self.assertRaises(ValueError):
                FETCH.validate_manifest(broken, "community")
        broken = copy.deepcopy(catalog)
        source = next(iter(broken["sources"].values()))
        source["commit"] = "main"
        with self.assertRaises(ValueError):
            FETCH.validate_manifest(broken, "community")


class OriginalCommunityMapE2ETest(unittest.TestCase):
    def test_original_entities_determine_modes_and_world_weapons(self):
        ice = MODULE.rules_for_map(original("fy_iceworld"), "fy_iceworld", SOURCE)
        blood = MODULE.rules_for_map(original("cs_bloodstrike"), "cs_bloodstrike", SOURCE)
        for value in (ice, blood):
            self.assertEqual(value["mode"], "elimination")
            self.assertGreaterEqual(len(value["spawnsT"]), 5)
            self.assertGreaterEqual(len(value["spawnsCT"]), 5)
            self.assertFalse(value["bombsites"])
            self.assertFalse(value["hostages"])
        self.assertEqual((len(ice["spawnsT"]), len(ice["spawnsCT"])), (12, 12))
        self.assertEqual(len(ice["worldWeapons"]), 24)
        self.assertEqual({value["weapon"] for value in ice["worldWeapons"]}, {"mp5navy", "tmp", "p90", "mac10", "ak47", "sg552", "m4a1", "aug", "awp", "m3", "xm1014", "m249"})
        self.assertTrue(all(value["count"] == 1 for value in ice["worldWeapons"]))
        self.assertEqual(len(ice["buyZones"]), 1)
        self.assertEqual(ice["buyZones"][0]["min"], {"x": 224 * 256, "y": -800 * 256, "z": -256 * 256})
        self.assertEqual(ice["buyZones"][0]["max"], {"x": 544 * 256, "y": -480 * 256, "z": -64 * 256})
        self.assertEqual((len(blood["spawnsT"]), len(blood["spawnsCT"])), (10, 10))
        self.assertFalse(blood.get("worldWeapons"))
        self.assertFalse(blood["buyZones"])

    def test_nav5_is_bound_to_bsp_and_rejects_corruption(self):
        for map_id, count in (("fy_iceworld", 94), ("cs_bloodstrike", 81)):
            path = ROOT / ".local/cs16/research/navigation" / (map_id + ".nav")
            raw = path.read_bytes()
            bsp = original(map_id)
            parsed = MODULE.parse_nav5(raw, len(bsp.raw))
            self.assertEqual(len(parsed), count)
            with self.assertRaises(ValueError):
                MODULE.parse_nav5(raw, len(bsp.raw) + 1)
            for invalid in (raw[:12], raw[:-1], raw + b"trailing", b"wrong" + raw[5:]):
                with self.assertRaises(ValueError):
                    MODULE.parse_nav5(invalid, len(bsp.raw))
            changed = bytearray(raw)
            struct.pack_into("<I", changed, 4, 6)
            with self.assertRaises(ValueError):
                MODULE.parse_nav5(bytes(changed), len(bsp.raw))

    def test_append_maps_preserves_all_old_assets_and_is_repeatable(self):
        output = ROOT / ".local/cs16/assets"
        baseline = json.loads((OUTPUT / "baseline.json").read_text())
        before = json.loads((output / "manifest.json").read_text())
        frozen = {key: copy.deepcopy(before[key]) for key in ("models", "players", "weapons", "sounds", "source")}
        value = MODULE.extend_maps(SOURCE, output, ("fy_iceworld", "cs_bloodstrike"))
        self.assertEqual(set(value["maps"]), set(baseline["maps"]) | {"fy_iceworld", "cs_bloodstrike"})
        for map_id, entry in baseline["maps"].items():
            self.assertEqual(value["maps"][map_id], entry)
        for key in frozen:
            self.assertEqual(value[key], frozen[key])
        payloads = {item["id"]: item for item in map_payloads()}
        for map_id, prior_sha in baseline["rules"].items():
            self.assertEqual(hashlib.sha256(MODULE.json_bytes(payloads[map_id])).hexdigest(), prior_sha)
        details = []
        for map_id in ("fy_iceworld", "cs_bloodstrike"):
            entry = value["maps"][map_id]
            render = json.loads(gzip.decompress((output / entry["render"]["file"]).read_bytes()))
            rules = json.loads(gzip.decompress((output / entry["rules"]["file"]).read_bytes()))
            self.assertEqual(rules, payloads[map_id])
            self.assertEqual(entry["mode"], "elimination")
            self.assertEqual(entry["source"]["sha256"], hashlib.sha256(original(map_id).raw).hexdigest())
            self.assertGreater(sum(len(item["positions"]) for item in render["meshes"]), 1000)
            self.assertEqual(len(entry["sky"]), 6)
            self.assertGreater(len(rules["nav"]), 20)
            self.assertEqual(len(rules["nav"]), len(rules["navEdges"]))
            details.append({"map": map_id, "sourceSHA256": entry["source"]["sha256"], "rulesSHA256": entry["rules"]["sha256"], "renderSHA256": entry["render"]["sha256"], "nav": len(rules["nav"]), "worldWeapons": len(rules.get("worldWeapons", [])), "passed": True})
        first = (output / "manifest.json").read_bytes()
        MODULE.extend_maps(SOURCE, output, ("fy_iceworld", "cs_bloodstrike"))
        self.assertEqual(first, (output / "manifest.json").read_bytes())
        (OUTPUT / "conversion.json").write_text(json.dumps({"passed": True, "maps": details, "manifestSHA256": hashlib.sha256(first).hexdigest(), "oldMapsUnchanged": list(baseline["maps"]), "modelAndSoundUnchanged": True}, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    unittest.main()
