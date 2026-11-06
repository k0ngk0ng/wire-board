import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("render_classic", Path(__file__).with_name("render_classic.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class RenderClassicTests(unittest.TestCase):
    def test_affine_keeps_translation_and_rotation(self):
        value = dict(x=10, y=20, kx=0, ky=0, sx=2, sy=3)
        self.assertEqual(module.matrix(value), (2, 0, 0, 3, 10, 20))
        value.update(kx=90, ky=90)
        a, b, c, d, _, _ = module.matrix(value)
        self.assertAlmostEqual(a, 0)
        self.assertAlmostEqual(b, -3)
        self.assertAlmostEqual(c, 2)
        self.assertAlmostEqual(d, 0)

    def test_only_correct_armor_layers_visible(self):
        self.assertFalse(module.visible("anim_bucket", "normal"))
        self.assertFalse(module.visible("anim_cone", "buckethead"))
        self.assertTrue(module.visible("anim_bucket", "buckethead"))
        self.assertTrue(module.visible("anim_cone", "conehead"))
        self.assertFalse(module.visible("anim_hair", "conehead"))
        self.assertFalse(module.visible("Zombie_innerarm_screendoor", "normal"))

    def test_clip_reads_marker_frames(self):
        definition = {"tracks": [{"name": "anim_walk", "frames": [{"f": -1}, {"f": 0}, {"f": 0}, {"f": -1}]}]}
        self.assertEqual(module.clip_range(definition, "anim_walk"), (1, 2))


if __name__ == "__main__":
    unittest.main()
