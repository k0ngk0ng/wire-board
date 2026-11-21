import importlib.util
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("hotline_build_maps", ROOT / "scripts/hotlinemiami/build_maps.py")
MAPS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MAPS)


class MapRecoveryTest(unittest.TestCase):
    def test_room_records_consume_complete_native_source(self):
        root = MAPS.METADATA
        rooms = MAPS.read_rooms((root / "hotline_rooms.bin").read_bytes())
        self.assertEqual(len(rooms), 156)
        self.assertEqual((rooms[17]["width"], rooms[17]["height"]), (640, 480))
        self.assertEqual(len(rooms[17]["tiles"]), 899)
        self.assertEqual(len(rooms[33]["objects"]), 231)

    def test_native_map_has_source_collisions_and_safe_spawns(self):
        maps = MAPS.recover_maps()
        self.assertEqual([m["id"] for m in maps], ["no-talk", "full-house", "push-it", "overdose", "tension", "neighbors", "highball", "exposed", "hot-heavy"])
        self.assertEqual([m["sourceRoomId"] for m in maps], [17, 31, 33, 20, 49, 35, 65, 148, 59])
        self.assertEqual([len(m["doors"]) for m in maps[:6]], [7, 7, 10, 9, 6, 2])
        for arena in maps:
            self.assertGreater(len(arena["walls"]), 60)
            self.assertEqual(len(arena["spawns"]), 8)
            for spawn in arena["spawns"]:
                self.assertTrue(MAPS.walkable(spawn["x"], spawn["y"], arena["walls"], 10))
            for wall in arena["walls"]:
                self.assertIn("sourceInstance", wall)

    def test_actor_furniture_is_excluded_without_keyword_false_positives(self):
        maps = MAPS.recover_maps()
        objects = {wall["sourceObject"] for arena in maps for wall in arena["walls"]}
        self.assertTrue(objects.isdisjoint({503, 416, 863}))
        self.assertIn(506, objects)
        self.assertIn(697, objects)

    def test_large_maps_keep_native_room_dimensions(self):
        maps = {arena["id"]: arena for arena in MAPS.recover_maps()}
        for identity, dimensions in [("highball", (808, 800)), ("exposed", (1500, 640)), ("hot-heavy", (800, 640))]:
            self.assertEqual((maps[identity]["width"], maps[identity]["height"]), dimensions)

    def test_truncated_metadata_is_rejected(self):
        with self.assertRaises(ValueError):
            MAPS.read_rooms(b"\0" * 16)


if __name__ == "__main__":
    unittest.main()
