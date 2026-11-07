import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("render_classic", Path(__file__).with_name("render_classic.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class RenderClassicTests(unittest.TestCase):
    def test_shooter_attack_keeps_stalk_leaves_head_and_mouth(self):
        def value(image, frame):
            return dict(x=0, y=0, kx=0, ky=0, sx=1, sy=1, f=frame, a=1, image=image)
        def marker(start, end):
            return [value("", 0 if start <= i <= end else -1) for i in range(6)]
        definition = {"tracks": [
            {"name": "anim_idle", "frames": marker(0, 1)},
            {"name": "anim_shooting", "frames": marker(2, 3)},
            {"name": "frontleaf", "frames": [value("LEAF", 0 if i < 2 else -1) for i in range(6)]},
            {"name": "stalk_bottom", "frames": [value("STALK", 0 if i < 2 else -1) for i in range(6)]},
            {"name": "anim_face", "frames": [value("HEAD", 0 if 2 <= i <= 3 else -1) for i in range(6)]},
            {"name": "idle_mouth", "frames": [value("MOUTH", 0 if 2 <= i <= 3 else -1) for i in range(6)]},
        ]}
        for kind in ("peashooter", "repeater", "snowpea"):
            for offset in (0, 1):
                layers = module.frame_layers(definition, kind, "attack", 2 + offset)
                names = {name for name, frame in layers if frame["f"] >= 0 and frame["image"]}
                self.assertTrue({"frontleaf", "stalk_bottom", "anim_face", "idle_mouth"} <= names, (kind, offset, names))

    def test_attached_head_follows_stem_relative_to_base_pose(self):
        def frame(x, image="", f=0):
            return dict(x=x, y=0, kx=0, ky=0, sx=1, sy=1, f=f, a=1, image=image)
        definition = {"tracks": [
            {"name": "anim_idle", "frames": [frame(0), frame(0), frame(0, f=-1), frame(0, f=-1)]},
            {"name": "anim_shooting", "frames": [frame(0, f=-1), frame(0, f=-1), frame(0), frame(0)]},
            {"name": "anim_stem", "frames": [frame(10), frame(12), frame(10), frame(12)]},
            {"name": "stalk_bottom", "frames": [frame(0, "STALK"), frame(0, "STALK"), frame(0, "STALK", -1), frame(0, "STALK", -1)]},
            {"name": "anim_face", "frames": [frame(5, "HEAD", -1), frame(5, "HEAD", -1), frame(5, "HEAD"), frame(5, "HEAD")]},
        ]}
        layers = dict(module.frame_layers(definition, "peashooter", "attack", 3))
        self.assertEqual(module.matrix(layers["anim_face"])[4], 7)

    def test_gatling_idle_keeps_body_and_gun(self):
        def value(image, f=0):
            return dict(x=0, y=0, kx=0, ky=0, sx=1, sy=1, f=f, a=1, image=image)
        definition = {"tracks": [
            {"name": "anim_idle", "frames": [value(""), value("", -1)]},
            {"name": "anim_head_idle", "frames": [value("", -1), value("")]},
            {"name": "frontleaf", "frames": [value("LEAF"), value("LEAF", -1)]},
            {"name": "stalk_bottom", "frames": [value("STALK"), value("STALK", -1)]},
            {"name": "GatlingPea_barrel1", "frames": [value("GUN", -1), value("GUN")]},
        ]}
        layers = module.frame_layers(definition, "gatlingpea", "idle", 0)
        names = {name for name, frame in layers if frame["f"] >= 0 and frame["image"]}
        self.assertTrue({"frontleaf", "stalk_bottom", "GatlingPea_barrel1"} <= names)

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
