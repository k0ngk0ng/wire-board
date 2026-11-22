import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import unittest
import zlib


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / ".local/cs16/verification/combat-effect-assets"
OUTPUT.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location("cs16_combat_effect_assets", Path(__file__).with_name("build_combat_effects.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
EXPECTED = {"blood": (9, 16, 16, [-8, 8], 3), "bloodspray": (10, 64, 64, [-32, 32], 3), "fexplo": (30, 80, 96, [-40, 48], 1), "eexplo": (25, 80, 80, [-40, 40], 1)}


def png_pixels(raw):
    at = 8
    compressed = b""
    while at < len(raw):
        size = struct.unpack_from(">I", raw, at)[0]
        kind = raw[at + 4:at + 8]
        value = raw[at + 8:at + 8 + size]
        at += size + 12
        if kind == b"IHDR":
            width, height, depth, color = struct.unpack_from(">IIBB", value)
            if depth != 8 or color != 6:
                raise ValueError("Expected an RGBA PNG")
        if kind == b"IDAT":
            compressed += value
    pixels = zlib.decompress(compressed)
    rows = []
    for y in range(height):
        start = y * (width * 4 + 1)
        if pixels[start] != 0:
            raise ValueError("Expected deterministic unfiltered PNG rows")
        rows.append(pixels[start + 1:start + 1 + width * 4])
    return width, height, b"".join(rows)


class OriginalCombatEffectsE2ETest(unittest.TestCase):
    def test_source_frame_geometry_alpha_and_pixels_survive_conversion(self):
        source = ROOT / ".local/cs16/research/original"
        output = ROOT / ".local/cs16/assets"
        before = json.loads((output / "manifest.json").read_text())
        frozen = {key: copy.deepcopy(value) for key, value in before.items() if key != "effects"}
        go_files = [ROOT / "games/cs16/data/maps.go", ROOT / "games/cs16/data/model_rules.go"]
        go_hashes = {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in go_files}
        manifest = MODULE.build_effects(source, output)
        self.assertEqual(set(manifest["effects"]), set(EXPECTED))
        for key in frozen:
            self.assertEqual(manifest[key], frozen[key])
        report = []
        for name, (count, width, height, origin, texture_format) in EXPECTED.items():
            entry = manifest["effects"][name]
            raw = (source / "valve/sprites" / (name + ".spr")).read_bytes()
            native = MODULE.SPRITES.parse_sprite(raw)
            self.assertEqual(entry["sourceSHA256"], hashlib.sha256(raw).hexdigest())
            self.assertEqual(entry["textureFormat"], texture_format)
            self.assertEqual(entry["orientation"], 2)
            self.assertEqual(entry["blend"], "additive" if texture_format == 1 else "normal")
            if name in ("blood", "bloodspray"):
                self.assertEqual(entry["paletteIndex"], 247)
                self.assertEqual(entry["tint"], [103, 0, 0])
                palette = (source / "cstrike/gfx/palette.lmp").read_bytes()
                self.assertEqual(entry["paletteSourceSHA256"], hashlib.sha256(palette).hexdigest())
            self.assertEqual(len(entry["frames"]), count)
            self.assertNotIn("fps", entry)
            atlas = (output / entry["atlas"]["file"]).read_bytes()
            atlas_width, atlas_height, pixels = png_pixels(atlas)
            self.assertEqual((atlas_width, atlas_height), (entry["width"], entry["height"]))
            self.assertEqual(hashlib.sha256(atlas).hexdigest(), entry["atlas"]["sha256"])
            visible = 0
            for index, frame in enumerate(entry["frames"]):
                self.assertEqual((frame["width"], frame["height"]), (width, height))
                self.assertEqual(frame["origin"], origin)
                cropped = b"".join(pixels[((frame["y"] + row) * atlas_width + frame["x"]) * 4:((frame["y"] + row) * atlas_width + frame["x"] + width) * 4] for row in range(height))
                self.assertEqual(cropped, native["frames"][index]["rgba"])
                alpha = cropped[3::4]
                self.assertGreater(alpha.count(0), 0)
                visible += sum(value > 0 for value in alpha)
                if texture_format == 3:
                    self.assertTrue(all(value in (0, 255) for value in alpha))
            self.assertGreater(visible, 0)
            report.append({"name": name, "frames": count, "sourceSHA256": entry["sourceSHA256"], "atlasSHA256": entry["atlas"]["sha256"], "atlasBytes": len(atlas), "textureFormat": texture_format, "orientation": 2, "pixelsMatch": True})
        self.assertEqual(go_hashes, {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in go_files})
        first = (output / "manifest.json").read_bytes()
        MODULE.build_effects(source, output)
        self.assertEqual(first, (output / "manifest.json").read_bytes())
        (OUTPUT / "effects-e2e.json").write_text(json.dumps({"passed": True, "effects": report, "manifestSHA256": hashlib.sha256(first).hexdigest(), "frozenGoSHA256": go_hashes, "existingFieldsUnchanged": list(frozen)}, indent=2) + "\n")


if __name__ == "__main__":
    unittest.main()
