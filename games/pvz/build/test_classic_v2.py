import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("classic_v2", Path(__file__).with_name("classic_v2.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ClassicV2Tests(unittest.TestCase):
    def test_scenes_keep_native_row_geometry_and_water(self):
        scenes = module.scene_definitions()
        self.assertEqual(set(scenes), {"day", "night", "pool", "fog", "roof", "roofnight"})
        self.assertEqual(scenes["day"]["lawn"], dict(x=40, y=80, width=720, height=500))
        self.assertEqual(scenes["pool"]["rows"], 6)
        self.assertEqual(scenes["pool"]["poolRows"], [2, 3])
        self.assertEqual(scenes["pool"]["water"], dict(src="images/pool.jpg", x=34, y=278))
        self.assertEqual(scenes["roof"]["roofSlope"], [100, 80, 60, 40, 20, 0, 0, 0, 0])
        self.assertEqual(scenes["roof"]["lawn"]["height"], 425)

    def test_armor_and_headed_zombies_keep_distinct_layers(self):
        self.assertFalse(module.layer_visible("anim_bucket", "screendoor"))
        self.assertTrue(module.layer_visible("anim_screendoor", "screendoor"))
        self.assertFalse(module.layer_visible("Zombie_outerarm_hand", "screendoor"))
        self.assertTrue(module.layer_visible("Zombie_outerarm_screendoor", "screendoor"))
        self.assertTrue(module.layer_visible("Zombie_flaghand", "flag"))
        self.assertTrue(module.layer_visible("Zombie_duckytube", "duckytube"))
        self.assertFalse(module.layer_visible("anim_head1", "peahead"))

    def test_damage_profiles_use_original_images(self):
        self.assertEqual(module.image_overrides("wallnut", "damage2")["anim_face"], "IMAGE_REANIM_WALLNUT_CRACKED2")
        self.assertEqual(module.image_overrides("tallnut", "damage1")["anim_idle"], "IMAGE_REANIM_TALLNUT_CRACKED1")
        self.assertEqual(module.image_overrides("garlic", "damage2")["anim_face"], "IMAGE_REANIM_GARLIC_BODY3")
        self.assertEqual(module.image_overrides("pumpkin", "damage2")["Pumpkin_front"], "IMAGE_REANIM_PUMPKIN_DAMAGE3")


if __name__ == "__main__":
    unittest.main()
