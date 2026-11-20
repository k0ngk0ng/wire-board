import importlib.util
import struct
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
MODULE_PATH = ROOT / "scripts/hotlinemiami/extract_assets.py"
SPEC = importlib.util.spec_from_file_location("hotline_extract_assets", MODULE_PATH)
EXTRACT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(EXTRACT)


class AssetExtractionTest(unittest.TestCase):
    def test_wad_rejects_truncated_and_escaping_entries(self):
        for source in [b"", struct.pack("<II", 8, 1), struct.pack("<III", 25, 1, 5) + b"../x\0" + struct.pack("<II", 0, 0)]:
            with self.assertRaises(ValueError):
                EXTRACT.read_wad(source)

    def test_original_wad_index_and_raw_texture(self):
        path = ROOT / ".local/hotlinemiami/research/original/gog-linux-2.0.0.4/selected/HotlineMiami_GL.wad"
        data = path.read_bytes()
        entries = EXTRACT.read_wad(data)
        self.assertEqual(len(entries), 633)
        texture = EXTRACT.decode_texture(entries["GL/AssetScripts/b/sprBoss1Walk.ags.phyre"])
        self.assertEqual((texture["width"], texture["height"]), (128, 64))
        self.assertEqual(len(texture["rgba"]), 32768)
        self.assertEqual(len(texture["frames"]), 8)
        self.assertEqual(texture["frames"][0], {"name": "sprBoss1Walk_0", "x": 0, "y": 32, "w": 32, "h": 32})
        self.assertEqual(texture["frames"][1]["y"], 0)
        self.assertEqual(texture["rgba"][:512], entries["GL/AssetScripts/b/sprBoss1Walk.ags.phyre"][-512:])

    def test_png_preserves_all_rgba_bytes(self):
        rgba = bytes([255, 0, 0, 255, 0, 255, 0, 128, 0, 0, 255, 0, 24, 46, 78, 255])
        png = EXTRACT.encode_png(2, 2, rgba)
        self.assertEqual(EXTRACT.decode_generated_png(png), (2, 2, rgba))

    def test_shared_atlas_names_match_native_sprite_dimensions(self):
        path = ROOT / ".local/hotlinemiami/research/original/gog-linux-2.0.0.4/selected/HotlineMiami_GL.wad"
        entries = EXTRACT.read_wad(path.read_bytes())
        texture = EXTRACT.decode_texture(entries["GL/AssetScripts/p/player.ags.phyre"])
        names = {frame["name"]: frame for frame in texture["frames"]}
        self.assertEqual((names["sprPWalkShotgun_0"]["w"], names["sprPWalkShotgun_0"]["h"]), (44, 32))
        self.assertEqual((names["sprRoosterMask_0"]["w"], names["sprRoosterMask_0"]["h"]), (28, 28))

    def test_background_atlas_supports_non_numbered_names(self):
        path = ROOT / ".local/hotlinemiami/research/original/gog-linux-2.0.0.4/selected/HotlineMiami_GL.wad"
        entries = EXTRACT.read_wad(path.read_bytes())
        texture = EXTRACT.decode_texture(entries["GL/AssetScripts/background.ags.phyre"])
        self.assertIn("background0", [frame["name"] for frame in texture["frames"]])

    def test_texture_rejects_non_textures_and_truncation(self):
        for source in [b"", b"RGBA8" * 100, b"RYHP" + b"\0" * 120]:
            with self.assertRaises(ValueError):
                EXTRACT.decode_texture(source)

    def test_output_cannot_escape_game_workspace(self):
        with self.assertRaises(ValueError):
            EXTRACT.validate_output(ROOT / "web/public/hotlinemiami")
        self.assertEqual(EXTRACT.validate_output(ROOT / ".local/hotlinemiami/assets"), ROOT / ".local/hotlinemiami/assets")


if __name__ == "__main__":
    unittest.main()
