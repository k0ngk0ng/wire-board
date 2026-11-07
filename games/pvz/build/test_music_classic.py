import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("music_classic", Path(__file__).with_name("music_classic.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ClassicMusicTests(unittest.TestCase):
    def test_original_night_waiting_order_is_the_only_allowed_silent_stem(self):
        self.assertTrue(module.intentional_silence("night", "drums", 92, bytes(16)))
        self.assertFalse(module.intentional_silence("night", "main", 92, bytes(16)))
        self.assertFalse(module.intentional_silence("night", "drums", 76, bytes(16)))
        self.assertFalse(module.intentional_silence("day", "drums", 92, bytes(16)))
        self.assertFalse(module.intentional_silence("night", "drums", 92, b"\x01\x00"))

    def test_native_orders_and_night_drums_are_preserved(self):
        config = module.music_definitions()
        self.assertEqual({key: config[key]["startOrder"] for key in ("day", "night", "pool", "fog", "roof", "menu", "boss")},
                         {"day": 0, "night": 48, "pool": 94, "fog": 125, "roof": 184, "menu": 152, "boss": 158})
        self.assertEqual(config["night"]["drumsOrder"], 92)
        self.assertEqual(config["night"]["drumsInitiallyMuted"], False)

    def test_actual_binary_channel_ranges_cover_all_thirty_channels(self):
        config = module.music_definitions()
        self.assertEqual(config["pool"]["channels"]["drums"], list(range(18, 29)))
        self.assertEqual(config["pool"]["channels"]["hihats"], list(range(18, 25)) + [29])
        self.assertEqual(config["menu"]["channels"]["main"], list(range(30)))

    def test_loop_metadata_rejects_empty_or_outside_loop(self):
        self.assertEqual(module.loop_seconds(44100, 88200, 44100), (1, 2))
        for start, end, rate in ((0, 0, 44100), (100, 99, 44100), (-1, 99, 44100), (1, 99, 0)):
            with self.assertRaises(ValueError):
                module.loop_seconds(start, end, rate)


if __name__ == "__main__":
    unittest.main()
