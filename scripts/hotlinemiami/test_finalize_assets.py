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

    def test_native_people_are_never_static_furniture(self):
        objects = ASSETS.read_object_data()
        sprites = ASSETS.read_sprite_data()
        for identity in [503, 416, 417, 418, 535, 536, 537, 373, 377, 547, 390, 862, 863, 699]:
            self.assertFalse(ASSETS.static_scene_object(identity, objects, sprites), identity)
        for identity in [506, 148, 527, 859, 860, 861, 864]:
            self.assertTrue(ASSETS.static_scene_object(identity, objects, sprites), identity)

    def test_actor_identity_mismatch_is_rejected(self):
        objects = ASSETS.read_object_data()
        sprites = ASSETS.read_sprite_data()
        changed = {**objects, 503: (503, 873, *objects[503][2:])}
        with self.assertRaises(ValueError):
            ASSETS.static_scene_object(503, changed, sprites)

    def test_map_only_refresh_preserves_other_resource_bytes(self):
        baseline = json.loads((ROOT / ".local/hotlinemiami/verification/assets/large-map-baseline.json").read_text())
        manifest = ASSETS.update_map_assets()
        self.assertEqual(len(manifest["maps"]), 9)
        self.assertEqual(manifest["characters"], baseline["characters"])
        self.assertEqual(manifest["weapons"], baseline["weapons"])
        self.assertEqual(manifest["sprites"], baseline["sprites"])
        self.assertEqual(manifest["sounds"], baseline["sounds"])
        self.assertEqual(manifest["music"], baseline["music"])
        native = json.loads((ROOT / "games/hotlinemiami/data/maps.json").read_text())
        self.assertEqual([arena["rules"] for arena in manifest["maps"]], [ASSETS.fixed_rules(arena) for arena in native])
        changed = {"map-push-it", "preview-push-it", "map-overdose", "preview-overdose", "map-tension", "preview-tension"}
        for identity, resource in baseline["images"].items():
            if identity in changed:
                self.assertNotEqual(manifest["images"][identity]["sha256"], resource["sha256"], identity)
            else:
                self.assertEqual(manifest["images"][identity], resource, identity)
        for arena in manifest["maps"]:
            image = manifest["images"][arena["image"]]
            self.assertEqual((image["width"], image["height"]), (arena["width"], arena["height"]))
            self.assertEqual(arena["rules"]["width"], arena["width"] * 256)

    def test_actor_audit_covers_every_native_room(self):
        audit = ASSETS.scene_actor_audit()
        self.assertEqual([arena["removedInstanceCount"] for arena in audit["maps"]], [0, 0, 2, 1, 3, 0, 3, 7, 0])
        for arena in audit["maps"]:
            for actor in arena["removedActors"]:
                self.assertEqual(actor["instanceCount"], len(actor["instances"]))
                self.assertIn("sourceSprite", actor)
                self.assertIn("spriteId", actor)

    def test_room_preview_and_background_share_dimensions(self):
        manifest = ASSETS.update_map_assets()
        self.assertEqual(len(manifest["maps"]), 9)
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


if __name__ == "__main__":
    unittest.main()
