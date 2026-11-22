import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / ".local/cs16/verification/assets"
OUTPUT.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location("cs16_build_assets", Path(__file__).with_name("build_assets.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def miptex(name="stone", pixels=None):
    width = 8
    height = 8
    content = bytes(range(64)) if pixels is None else pixels
    header = struct.pack("<16s6I", name.encode("ascii"), width, height, 40, 104, 120, 124)
    palette = b"".join(bytes((index, 255 - index, index // 2)) for index in range(256))
    return header + content + bytes(16 + 4 + 1) + struct.pack("<H", 256) + palette


class AssetParsingTest(unittest.TestCase):
    def test_rejects_invalid_bsp_and_truncation(self):
        for value in (b"", struct.pack("<i", 29) + bytes(120), struct.pack("<i", 30) + bytes(120)):
            with self.assertRaises(ValueError):
                MODULE.Bsp(value)

    def test_texture_palette_and_nul_name(self):
        raw = bytearray(miptex())
        raw[6:16] = b"\0dirtyxxxx"
        value = MODULE.parse_miptex(bytes(raw))
        self.assertEqual(value["name"], "stone")
        self.assertEqual(value["rgba"][:4], bytes((0, 255, 0, 255)))
        with self.assertRaises(ValueError):
            MODULE.parse_miptex(bytes(raw[:-2]))

    def test_transparent_palette_index(self):
        value = MODULE.parse_miptex(miptex("{glass", bytes([255]) * 64))
        self.assertEqual(value["rgba"][3], 0)
        value = MODULE.parse_miptex(miptex("stone", bytes([255]) * 64))
        self.assertEqual(value["rgba"][3], 255)

    def test_texcoord_rounds_like_source_float32(self):
        coordinates = MODULE.texture_coordinates((0, 48, 0), (0, 1.3333333730697632, 0, 0, 0, 0, 1, 0))
        self.assertEqual(coordinates[0], 64)

    def test_source_material_name_normalization(self):
        self.assertEqual(MODULE.texture_material("+0{WOODEN_PANEL", {"wooden_panel": "W"}), "W")
        self.assertEqual(MODULE.texture_material("!water", {}), "S")
        self.assertEqual(MODULE.texture_material("unknown", {}), "C")

    def test_png_is_stable(self):
        first = MODULE.png_bytes(2, 1, bytes((255, 0, 0, 255, 0, 255, 0, 128)))
        self.assertEqual(first[:8], b"\x89PNG\r\n\x1a\n")
        self.assertEqual(first, MODULE.png_bytes(2, 1, bytes((255, 0, 0, 255, 0, 255, 0, 128))))

    def test_output_must_stay_in_owned_directory(self):
        for path in (ROOT, ROOT / "games/cs16/assets", ROOT.parent / "outside"):
            with self.assertRaises(ValueError):
                MODULE.owned_output(path)
        self.assertEqual(MODULE.owned_output(ROOT / ".local/cs16/assets"), ROOT / ".local/cs16/assets")

    def test_tga_channels_and_origin(self):
        header = bytearray(18)
        header[2] = 2
        struct.pack_into("<HH", header, 12, 1, 2)
        header[16] = 24
        width, height, rgba = MODULE.decode_tga(bytes(header) + bytes((255, 0, 0, 0, 0, 255)))
        self.assertEqual((width, height), (1, 2))
        self.assertEqual(rgba, bytes((255, 0, 0, 255, 0, 0, 255, 255)))


class OriginalMapE2ETest(unittest.TestCase):
    def test_original_breakable_materials_match_bsp_entities(self):
        source = ROOT / ".local/cs16/research/original"
        observations = []
        for map_id in MODULE.MAPS:
            bsp = MODULE.Bsp((source / "cstrike/maps" / (map_id + ".bsp")).read_bytes())
            rule = MODULE.rules_for_map(bsp, map_id, source)
            brushes = {brush["id"]: brush for brush in rule["brushes"]}
            for index, entity in enumerate(bsp.entities):
                if entity.get("classname") == "func_breakable":
                    expected = int(entity.get("material", "0"))
                    self.assertEqual(brushes[index].get("material"), expected, (map_id, index))
                    observations.append({"map": map_id, "id": index, "material": expected})
        (OUTPUT / "breakable-materials.json").write_text(json.dumps({"passed": True, "entities": observations}, indent=2) + "\n")

    def test_material_rule_migration_rejects_geometry_or_source_changes(self):
        old = {"id": "fixture", "sourceSHA256": "a" * 64, "brushes": [{"id": 1, "kind": "func_breakable", "health": 20, "bounds": {"min": {"x": 0}, "max": {"x": 10}}}]}
        updated = json.loads(json.dumps(old))
        updated["brushes"][0]["material"] = 7
        rows = MODULE.material_rule_compatibility([old], [updated], [])
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["mapId"], "fixture")
        self.assertEqual(rows[0]["from"], hashlib.sha256(MODULE.json_bytes(old)).hexdigest())
        self.assertEqual(rows[0]["to"], hashlib.sha256(MODULE.json_bytes(updated)).hexdigest())
        self.assertEqual(MODULE.material_rule_compatibility([updated], [updated], rows), rows)
        for field in ["sourceSHA256", "id", "bounds"]:
            changed = json.loads(json.dumps(updated))
            if field == "sourceSHA256":
                changed[field] = "b" * 64
            elif field == "id":
                changed["brushes"][0][field] = 2
            else:
                changed["brushes"][0][field]["max"]["x"] = 11
            self.assertEqual(MODULE.material_rule_compatibility([old], [changed], rows), [])

    def test_original_assault_legacy_rescue_is_marked_spherical(self):
        source = ROOT / ".local/cs16/research/original"
        bsp = MODULE.Bsp((source / "cstrike/maps/cs_assault.bsp").read_bytes())
        rule = MODULE.rules_for_map(bsp, "cs_assault", source)
        self.assertTrue(rule["rescueLegacy"])
        self.assertEqual(len(rule["rescueZones"]), len(rule["spawnsCT"]))
        self.assertFalse(any(entity.get("classname") in ("func_hostage_rescue", "info_hostage_rescue") for entity in bsp.entities))

    def test_rule_refresh_freezes_models_and_media(self):
        source = ROOT / ".local/cs16/research/original"
        output = ROOT / ".local/cs16/assets"
        prior = json.loads((output / "manifest.json").read_text())
        model_rules = ROOT / "games/cs16/data/model_rules.go"
        before = hashlib.sha256(model_rules.read_bytes()).hexdigest()
        updated = MODULE.refresh_rules(source, output)
        for key in ("models", "players", "weapons", "sounds", "resources"):
            self.assertEqual(updated[key], prior[key])
        for map_id in prior["maps"]:
            for key in ("render", "preview", "sky"):
                self.assertEqual(updated["maps"][map_id][key], prior["maps"][map_id][key])
        self.assertEqual(before, hashlib.sha256(model_rules.read_bytes()).hexdigest())
        first = (output / "manifest.json").read_bytes()
        MODULE.refresh_rules(source, output)
        self.assertEqual(first, (output / "manifest.json").read_bytes())
        (OUTPUT / "rule-refresh.json").write_text(json.dumps({"passed": True, "manifestSHA256": hashlib.sha256(first).hexdigest(), "mapsSHA256": hashlib.sha256((ROOT / "games/cs16/data/maps.go").read_bytes()).hexdigest(), "frozenModelRulesSHA256": before}, indent=2) + "\n")

    def test_model_refresh_freezes_maps_and_go_rules(self):
        output = ROOT / ".local/cs16/assets"
        source = ROOT / ".local/cs16/research/original"
        prior = json.loads((output / "manifest.json").read_text())
        go_files = [ROOT / "games/cs16/data/maps.go", ROOT / "games/cs16/data/model_rules.go"]
        before = {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in go_files}
        updated = MODULE.refresh_models(source, output)
        self.assertEqual(updated["maps"], prior["maps"])
        self.assertEqual(updated["sounds"], prior["sounds"])
        self.assertEqual(updated["resources"], prior["resources"])
        self.assertEqual(len(updated["players"]), 8)
        self.assertEqual(len(updated["weapons"]), 29)
        self.assertEqual(before, {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in go_files})
        first = (output / "manifest.json").read_bytes()
        MODULE.refresh_models(source, output)
        self.assertEqual(first, (output / "manifest.json").read_bytes())
        (OUTPUT / "model-refresh.json").write_text(json.dumps({"passed": True, "manifestSHA256": hashlib.sha256(first).hexdigest(), "frozenGoSHA256": before, "models": len(updated["models"])}, indent=2) + "\n")

    def test_four_original_maps_export_complete_rule_and_render_data(self):
        source = ROOT / ".local/cs16/research/original"
        output = OUTPUT / "public"
        output.mkdir(parents=True, exist_ok=True)
        report = []
        for map_id in ("de_dust2", "de_inferno", "de_aztec", "cs_assault"):
            rule, metadata = MODULE.build_map(source, output, map_id)
            self.assertEqual(rule["id"], map_id)
            self.assertGreaterEqual(len(rule["spawnsT"]), 5)
            self.assertGreaterEqual(len(rule["spawnsCT"]), 5)
            self.assertGreater(len(rule["planes"]), 100)
            self.assertGreater(len(rule["nodes"]), 100)
            self.assertGreater(len(rule["surfaces"]), 1000)
            for surface in rule["surfaces"]:
                self.assertIn(surface["material"], "CMDVGT SWPYNX".replace(" ", ""))
                self.assertGreaterEqual(len(surface["vertices"]), 3)
                self.assertGreaterEqual(surface["plane"], 0)
                self.assertLess(surface["plane"], len(rule["planes"]))
            self.assertEqual(len(rule["hulls"]), 2)
            for brush in rule["brushes"]:
                if brush["kind"] == "func_door_rotating":
                    self.assertEqual(brush["move"], {"x": 0, "y": 0, "z": 0})
            self.assertGreater(len(rule["nav"]), 100)
            self.assertEqual(len(rule["nav"]), len(rule["navEdges"]))
            if map_id.startswith("de_"):
                self.assertEqual(rule["mode"], "bomb")
                self.assertGreaterEqual(len(rule["bombsites"]), 2)
            else:
                self.assertEqual(rule["mode"], "hostage")
                self.assertEqual(len(rule["hostages"]), 4)
                self.assertTrue(rule["rescueZones"])
            render_path = output / metadata["render"]["file"]
            render = json.loads(render_path.read_text())
            self.assertGreater(sum(len(mesh["positions"]) for mesh in render["meshes"]), 10000)
            self.assertGreater(len(render["textures"]), 30)
            if map_id == "de_aztec":
                self.assertGreaterEqual(len(render["decorations"]), 20)
                self.assertEqual({item["model"] for item in render["decorations"]}, {"prop-pred-plant", "prop-abone-template1", "prop-skeleton"})
            for mesh in render["meshes"]:
                self.assertEqual(len(mesh["positions"]), len(mesh["normals"]))
                self.assertEqual(len(mesh["positions"]) // 3, len(mesh["uv"]) // 2)
                self.assertEqual(len(mesh["uv"]), len(mesh["uv2"]))
                for index in range(0, len(mesh["positions"]), 9):
                    triangle = mesh["positions"][index:index + 9]
                    first = [triangle[axis + 3] - triangle[axis] for axis in range(3)]
                    second = [triangle[axis + 6] - triangle[axis] for axis in range(3)]
                    cross = (first[1] * second[2] - first[2] * second[1], first[2] * second[0] - first[0] * second[2], first[0] * second[1] - first[1] * second[0])
                    self.assertGreater(sum(cross[axis] * mesh["normals"][index + axis] for axis in range(3)), 0)
            resources = list(render["textures"].values()) + [render["lightmap"], metadata["preview"], metadata["rules"], metadata["render"]]
            for resource in resources:
                raw = (output / resource["file"]).read_bytes()
                self.assertEqual(len(raw), resource["bytes"])
                self.assertEqual(hashlib.sha256(raw).hexdigest(), resource["sha256"])
            report.append({"map": map_id, "sourceSHA256": rule["sourceSHA256"], "planes": len(rule["planes"]), "nodes": len(rule["nodes"]), "nav": len(rule["nav"]), "meshes": len(render["meshes"]), "triangles": sum(len(mesh["positions"]) // 9 for mesh in render["meshes"]), "resources": len(resources), "passed": True})
        (OUTPUT / "original-maps.json").write_text(json.dumps({"passed": True, "maps": report}, ensure_ascii=False, indent=2) + "\n")

    def test_full_manifest_covers_original_models_and_audio(self):
        source = ROOT / ".local/cs16/research/original"
        output = ROOT / ".local/cs16/assets"
        manifest = MODULE.build_assets(source, output)
        source_sounds = set()
        for library in ("valve", "cstrike"):
            source_sounds.update(path.relative_to(source / library / "sound").as_posix() for path in (source / library / "sound").rglob("*.wav"))
        self.assertEqual(set(manifest["sounds"]), source_sounds)
        self.assertEqual(len(manifest["players"]), 8)
        self.assertEqual(len(manifest["weapons"]), 29)
        for item in manifest["weapons"].values():
            self.assertEqual(set(item), {"view", "player", "world"})
        self.assertTrue((ROOT / "games/cs16/data/maps.go").exists())
        self.assertTrue((ROOT / "games/cs16/data/model_rules.go").exists())
        records = MODULE.resource_records(manifest, output)
        self.assertIn("games/cs16/assets/public/cs16/manifest.json", records["files"])
        self.assertGreater(len(records["files"]), 500)
        self.assertFalse(any(key.endswith(".glb") for key in records["files"]))
        self.assertTrue(all(value["key"].startswith("bga/cs16/") for value in records["files"].values()))
        store_spec = importlib.util.spec_from_file_location("cs16_resource_store", ROOT / "scripts/resource_store.py")
        store = importlib.util.module_from_spec(store_spec)
        store_spec.loader.exec_module(store)
        for record in records["files"].values():
            store.validate_record(record)
        (OUTPUT / "manifest-summary.json").write_text(json.dumps({"passed": True, "maps": len(manifest["maps"]), "players": len(manifest["players"]), "weapons": len(manifest["weapons"]), "models": len(manifest["models"]), "sounds": len(manifest["sounds"]), "manifestSHA256": hashlib.sha256((output / "manifest.json").read_bytes()).hexdigest()}, indent=2) + "\n")


if __name__ == "__main__":
    unittest.main()
