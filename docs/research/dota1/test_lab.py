import random
import unittest
from lab_server import ROOMS, command


class SetupFlow(unittest.TestCase):
    def setUp(self):
        ROOMS.clear()

    def begin(self, n):
        room = command({"action": "new", "n": n})
        self.assertEqual(room["phase"], "waiting")
        self.assertIsNone(room["teams"])
        self.assertNotIn("game", room)
        self.assertEqual(len(room["seats"]), n*2)
        room = command({"action": "start", "id": room["id"]})
        self.assertEqual(room["phase"], "teams")
        self.assertEqual(room["teams"].count(0), n)
        self.assertEqual(room["teams"].count(1), n)
        room = command({"action": "teams", "id": room["id"], "teams": room["teams"]})
        self.assertEqual(room["phase"], "heroes")
        room = command({"action": "hero", "id": room["id"], "hero": "lina"})
        self.assertEqual(len(set(h["kind"] for h in room["game"]["heroes"])), n*2)
        return room

    def test_all_modes_team_only_after_start_then_draft_then_play(self):
        for n in (1, 2, 3):
            room = self.begin(n)
            self.assertEqual(room["phase"], "playing")
            self.assertEqual(room["game"]["round"], 1)

    def test_imbalanced_teams_rejected(self):
        room = command({"action": "new", "n": 3})
        command({"action": "start", "id": room["id"]})
        with self.assertRaises(ValueError):
            command({"action": "teams", "id": room["id"], "teams": [0]*6})
        self.assertEqual(ROOMS[room["id"]]["phase"], "teams")

    def test_teams_locked_after_confirmation(self):
        room = self.begin(2)
        with self.assertRaises(ValueError):
            command({"action": "teams", "id": room["id"], "teams": [0,0,1,1]})

    def test_human_and_bots_complete_all_modes_without_hidden_plan_response(self):
        for n in (1, 2, 3):
            room = self.begin(n)
            while not room["finished"]:
                index = random.Random(room["game"]["round"]).randrange(len(room["legal"]))
                old_round = room["game"]["round"]
                req = {"action": "act", "id": room["id"], "round": old_round, "choice": index}
                room = command(req)
                self.assertNotIn("pending", room["game"])
                with self.assertRaises(ValueError):
                    command(req)
            self.assertIn(room["winner"], (0, 1))

    def test_one_round_autoplay_then_return_to_manual(self):
        room = self.begin(3)
        room = command({"action": "auto", "id": room["id"], "round": 1})
        self.assertEqual(room["game"]["round"], 2)
        self.assertTrue(room["legal"])
        room = command({"action": "act", "id": room["id"], "round": 2, "choice": 0})
        self.assertEqual(room["game"]["round"], 3)


if __name__ == "__main__":
    unittest.main(verbosity=2)
