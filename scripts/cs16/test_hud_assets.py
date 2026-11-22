import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import unittest


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / ".local/cs16/verification/hud"
OUTPUT.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location("cs16_hud_assets", Path(__file__).with_name("build_hud.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def sprite(texture_format=1, pixels=bytes((0, 1, 2, 3))):
    palette = bytearray(768)
    palette[3:6] = bytes((128, 128, 128))
    palette[6:9] = bytes((255, 255, 255))
    palette[9:12] = bytes((255, 0, 0))
    palette[765:768] = bytes((255, 255, 255))
    header = struct.pack("<4siiifiiifi", b"IDSP", 2, 2, texture_format, 2, 2, 2, 1, 0, 1)
    return header + struct.pack("<H", 256) + palette + struct.pack("<5i", 0, -1, 1, 2, 2) + pixels


class SpriteParsingTest(unittest.TestCase):
    def test_additive_palette_becomes_exact_transparent_intensity(self):
        value = MODULE.parse_sprite(sprite())
        self.assertEqual(value["textureFormat"], 1)
        self.assertEqual(value["frames"][0]["rgba"], bytes((0, 0, 0, 0, 255, 255, 255, 128, 255, 255, 255, 255, 255, 0, 0, 255)))

    def test_alpha_test_and_index_alpha_are_distinct(self):
        tested = MODULE.parse_sprite(sprite(3, bytes((255, 0, 1, 2))))
        self.assertEqual(tested["frames"][0]["rgba"][:4], bytes((255, 255, 255, 0)))
        indexed = MODULE.parse_sprite(sprite(2, bytes((0, 1, 128, 255))))
        self.assertEqual(indexed["frames"][0]["rgba"], bytes((255, 255, 255, 0, 255, 255, 255, 1, 255, 255, 255, 128, 255, 255, 255, 255)))

    def test_invalid_sprite_bounds_versions_and_trailing_data_fail(self):
        raw = sprite()
        invalid = [b"", b"nope" + raw[4:], raw[:-1], raw + b"trailing"]
        for offset, value in ((4, 3), (20, 0), (28, 0), (28, 1000000), (810, 1)):
            changed = bytearray(raw)
            struct.pack_into("<i", changed, offset, value)
            invalid.append(bytes(changed))
        for value in invalid:
            with self.assertRaises(ValueError):
                MODULE.parse_sprite(value)

    def test_hud_list_records_count_mismatch_but_rejects_bad_rows(self):
        value = MODULE.parse_hud_list("3\nc4 640 640hud7 0 148 32 32\n//original comment\n")
        self.assertEqual(value["declared"], 3)
        self.assertEqual(len(value["entries"]), 1)
        for text in ("0\nc4 640 bank 0 0 1 1", "1\nc4 640 ../escape 0 0 1 1", "1\nc4 640 bank -1 0 1 1", "2\nc4 640 bank 0 0 1 1\nc4 640 bank 0 0 1 1"):
            with self.assertRaises(ValueError):
                MODULE.parse_hud_list(text)

    def test_crop_rejects_out_of_bounds_and_preserves_pixels(self):
        frame = MODULE.parse_sprite(sprite())["frames"][0]
        self.assertEqual(MODULE.crop_frame(frame, (1, 0, 1, 2)), bytes((255, 255, 255, 128, 255, 0, 0, 255)))
        for bounds in ((-1, 0, 1, 1), (0, 0, 3, 1), (0, 0, 1, 0)):
            with self.assertRaises(ValueError):
                MODULE.crop_frame(frame, bounds)


class OriginalHudE2ETest(unittest.TestCase):
    def test_original_hud_atlas_and_buy_sound_are_complete_and_repeatable(self):
        output = ROOT / ".local/cs16/assets"
        before = json.loads((output / "manifest.json").read_text())
        frozen = {key: copy.deepcopy(before[key]) for key in ("maps", "players", "weapons", "source", "resources")}
        go_files = [ROOT / "games/cs16/data/maps.go", ROOT / "games/cs16/data/model_rules.go"]
        go_hashes = {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in go_files}
        result = MODULE.build_hud(ROOT / ".local/cs16/research/original", output)
        hud = result["hud"]
        for key in frozen:
            self.assertEqual(result[key], frozen[key])
        for key, value in before["models"].items():
            self.assertEqual(result["models"][key], value)
        self.assertEqual(result["models"]["w-defuser"], result["models"]["w-thighpack"])
        source_model = ROOT / ".local/cs16/research/original/cstrike/models/w_thighpack.mdl"
        self.assertEqual(result["models"]["w-defuser"]["sourceSHA256"], hashlib.sha256(source_model.read_bytes()).hexdigest())
        for key, value in before["sounds"].items():
            self.assertEqual(result["sounds"][key], value)
        self.assertEqual(go_hashes, {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in go_files})
        self.assertEqual(hud["schema"], 1)
        self.assertEqual(hud["defaultResolution"], 640)
        self.assertEqual(len(hud["source"]["skippedDefinitions"]), 18)
        self.assertTrue(all(value["resolution"] == 320 for value in hud["source"]["skippedDefinitions"]))
        native = hud["sprites"]["640"]
        for name in ("c4", "buyzone", "bombticking", "bombticking1", "d_headshot", "cross", "suit_full", "suit_empty", "suithelmet_full", "suithelmet_empty", "stopwatch"):
            self.assertIn(name, native)
        for weapon in MODULE.GUNS:
            self.assertIn("d_" + weapon, native)
            self.assertIn(hud["weapons"][weapon]["ammo"], native)
        self.assertEqual(native["c4"]["sourceRect"], [0, 148, 32, 32])
        self.assertEqual(native["d_headshot"]["sourceRect"], [0, 240, 36, 16])
        sound = result["sounds"]["items/9mmclip1.wav"]
        self.assertTrue((output / sound["file"]).read_bytes().startswith(b"fLaC"))
        self.assertIn("pcmSHA256", sound)
        flashlight = result["sounds"]["items/flashlight1.wav"]
        self.assertTrue((output / flashlight["file"]).read_bytes().startswith(b"fLaC"))
        self.assertIn("pcmSHA256", flashlight)
        atlas = (output / hud["atlas"]["file"]).read_bytes()
        self.assertEqual(hashlib.sha256(atlas).hexdigest(), hud["atlas"]["sha256"])
        first = (output / "manifest.json").read_bytes()
        MODULE.build_hud(ROOT / ".local/cs16/research/original", output)
        self.assertEqual(first, (output / "manifest.json").read_bytes())
        (OUTPUT / "hud-e2e.json").write_text(json.dumps({"passed": True, "atlasSHA256": hud["atlas"]["sha256"], "atlasBytes": len(atlas), "manifestSHA256": hashlib.sha256(first).hexdigest(), "sprites": {key: len(value) for key, value in hud["sprites"].items()}, "weapons": len(hud["weapons"]), "frozenGoSHA256": go_hashes, "buySoundSHA256": sound["sha256"]}, indent=2) + "\n")


if __name__ == "__main__":
    unittest.main()
