import importlib.util
import json
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("hotline_finalize_assets", ROOT / "scripts/hotlinemiami/finalize_assets.py")
ASSETS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ASSETS)


class RuntimeAssetTest(unittest.TestCase):
    def test_selected_original_assets_are_complete(self):
        source = ASSETS.load_source_manifest()
        selections = ASSETS.character_definitions()
        self.assertEqual(len(selections), 5)
        for character in selections:
            self.assertIn(character.get("sourceSprite", character["sprite"]), source["sprites"])
            self.assertIn(character["portraitSprite"], source["sprites"])
        self.assertEqual(len(ASSETS.weapon_definitions()), 7)

    def test_native_sprite_origin_is_retained(self):
        source = ASSETS.load_source_manifest()
        sprite = source["sprites"]["sprPWalkUnarmed"]
        self.assertEqual((sprite["originX"], sprite["originY"]), (30, 30))
        self.assertEqual(sprite["frames"][0]["originX"], 30)

    def test_room_preview_and_background_share_dimensions(self):
        manifest = ASSETS.create_runtime_assets()
        self.assertEqual(len(manifest["maps"]), 3)
        self.assertEqual(len(manifest["characters"]), 5)
        self.assertEqual(len(manifest["weapons"]), 7)
        for arena in manifest["maps"]:
            image = manifest["images"][arena["image"]]
            self.assertEqual((image["width"], image["height"]), (arena["width"], arena["height"]))
            self.assertIn(arena["preview"], manifest["images"])
            self.assertEqual(arena["rules"]["width"], arena["width"] * 256)
        self.assertEqual(set(manifest["sounds"]), {"pistol", "uzi", "m16", "shotgun", "knife", "bat", "katana", "pickup", "throw", "down", "kill", "door"})
        for sound in manifest["sounds"].values():
            self.assertEqual(sound["sourcePCMsha256"], sound["decodedPCMsha256"])
            self.assertGreater(sound["sampleRate"], 0)
        for name in ["bodyguard-walk", "father-walk"]:
            sprite = manifest["sprites"][name]
            self.assertEqual(len(sprite["frames"]), 8)
            self.assertEqual((sprite["frames"][0]["w"], sprite["frames"][0]["h"]), (32, 32))
            self.assertIn("adaptation", sprite)
        self.assertEqual(manifest["music"]["bytes"], 8855631)
        self.assertEqual(manifest["music"]["mime"], "audio/ogg")


if __name__ == "__main__":
    unittest.main()
