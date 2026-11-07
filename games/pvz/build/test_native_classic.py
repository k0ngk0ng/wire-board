import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("native_classic", Path(__file__).with_name("native_classic.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class NativeClassicTests(unittest.TestCase):
    def test_boss_rolling_effect_uses_runtime_rate_two(self):
        self.assertEqual(module.runtime_fps("bossfireball", "idle", 12), 2)
        self.assertEqual(module.runtime_fps("bossiceball", "idle", 12), 2)
        self.assertEqual(module.runtime_fps("bossiceball", "form", 12), 12)

    def test_imitater_morph_uses_native_runtime_rate(self):
        self.assertEqual(module.runtime_fps("imitater", "attack", 48), 26)
        self.assertEqual(module.semantic_aliases("imitater")["imitater-explode"], "attack")

    def test_original_one_shot_actions_hold_their_last_pose(self):
        for marker in ("anim_drop", "anim_raise", "anim_grab", "anim_shooting", "anim_smash", "anim_placeladder", "anim_spawn_1"):
            self.assertFalse(module.looping_marker(marker), marker)
        for marker in ("anim_walk", "anim_idle", "anim_chew", "anim_head_idle"):
            self.assertTrue(module.looping_marker(marker), marker)

    def test_canvas_matrix_keeps_skew_axes(self):
        self.assertEqual(module.canvas_matrix((1, 2, 3, 4, 5, 6)), [1, 3, 2, 4, 5, 6])

    def test_bounds_include_rotated_image_corners(self):
        self.assertEqual(module.layer_bounds([0, 1, -1, 0, 10, 20], 4, 2), (8, 20, 10, 24))

    def test_semantic_clips_keep_special_movement(self):
        self.assertEqual(module.semantic_aliases("snorkel")["diving"], "swim")
        self.assertEqual(module.semantic_aliases("digger")["digging"], "drill")
        self.assertEqual(module.semantic_aliases("bungee")["grabbing"], "grab")
        self.assertEqual(module.semantic_aliases("zomboss")["boss-stomp"], "stomp_1")


if __name__ == "__main__":
    unittest.main()
