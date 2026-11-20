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
        self.assertEqual(len(selections), 8)
        for character in selections:
            self.assertIn(character.get("sourceSprite", character["sprite"]), source["sprites"])
            self.assertIn(character["portraitSprite"], source["sprites"])
        self.assertEqual(len(ASSETS.weapon_definitions()), 12)
        self.assertEqual(next(weapon for weapon in ASSETS.weapon_definitions() if weapon["id"] == "knife")["sourceFrame"], 5)

    def test_native_sprite_origin_is_retained(self):
        source = ASSETS.load_source_manifest()
        sprite = source["sprites"]["sprPWalkUnarmed"]
        self.assertEqual((sprite["originX"], sprite["originY"]), (30, 30))
        self.assertEqual(sprite["frames"][0]["originX"], 30)

    def test_story_explosion_is_not_baked_into_tension(self):
        self.assertFalse(ASSETS.static_environment("Enviroment/Furniture/Levels/05_Tension/explosion/sprRoomExplosion"))
        self.assertTrue(ASSETS.static_environment("Enviroment/Furniture/Levels/05_Tension/sprBombCables"))

    def test_room_preview_and_background_share_dimensions(self):
        manifest = ASSETS.create_runtime_assets()
        self.assertEqual(len(manifest["maps"]), 6)
        self.assertEqual(len(manifest["characters"]), 8)
        self.assertEqual(len(manifest["weapons"]), 12)
        for arena in manifest["maps"]:
            image = manifest["images"][arena["image"]]
            self.assertEqual((image["width"], image["height"]), (arena["width"], arena["height"]))
            self.assertIn(arena["preview"], manifest["images"])
            self.assertEqual(arena["rules"]["width"], arena["width"] * 256)
        self.assertEqual(set(manifest["sounds"]), {"pistol", "uzi", "m16", "shotgun", "knife", "bat", "katana", "magnum", "mp5", "double-barrel", "axe", "machete", "pickup", "throw", "down", "kill", "door"})
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
        baseline = json.loads((ROOT / ".local/hotlinemiami/verification/assets/expansion-baseline.json").read_text())
        for identity, resource in baseline["images"].items():
            if identity == "weapon-knife":
                self.assertNotEqual(manifest["images"][identity]["sha256"], resource["sha256"])
            else:
                self.assertEqual(manifest["images"][identity], resource, identity)
        for identity, resource in baseline["sounds"].items():
            self.assertEqual(manifest["sounds"][identity], resource, identity)
        self.assertEqual(manifest["music"], baseline["music"])


if __name__ == "__main__":
    unittest.main()
