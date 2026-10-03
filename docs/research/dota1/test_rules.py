import copy
import json
import random
import unittest

from simulate import Game, HEROES, HP, Order, choose, from_public, legal, play


def mirrored(game):
    view, n = game.public(), game.n
    view["heroes"] = view["heroes"][n:] + view["heroes"][:n]
    for h in view["heroes"]:
        h["team"] = 1 - h["team"]
    for key in ("towers", "core", "kills"):
        view[key].reverse()
    view["track"] = [-x for x in view["track"]]
    view["first"] = 1 - view["first"]
    return from_public(view)


def mirror_orders(orders, n):
    return [Order(a.card, a.lane, (a.target+n) % (2*n) if a.target >= 0 else -1, a.buy)
            for a in orders[n:] + orders[:n]]


class PaperRules(unittest.TestCase):
    def duel(self):
        return Game(1, ["drow", "lina"])

    def test_three_basic_cards_counter_each_other(self):
        for left, right in ((0, 2), (2, 1), (1, 0)):
            g = self.duel()
            a = Order(left, 0 if left == 0 else -1, 1 if left == 1 else -1)
            b = Order(right, 0 if right == 0 else -1, 0 if right == 1 else -1)
            g.resolve([a, b])
            self.assertGreater(g.track[0], 0, (left, right))

    def test_guard_blocks_basic_strike_without_losing_lane_pressure(self):
        g = self.duel()
        g.resolve([Order(2), Order(1, target=0)])
        self.assertEqual(g.heroes[0].hp, HP[g.heroes[0].kind])
        self.assertEqual(g.track, [1])

    def test_damage_cancels_push(self):
        g = self.duel()
        g.resolve([Order(0, 0), Order(1, target=0)])
        self.assertEqual(g.heroes[0].hp, HP[g.heroes[0].kind]-3)
        self.assertEqual(g.track, [-1])

    def test_retreat_does_not_dodge_lethal_damage_or_buy_after_death(self):
        g = self.duel()
        g.heroes[0].hp = 3
        g.heroes[0].gold = 6
        g.resolve([Order(4, buy="bracer"), Order(1, target=0)])
        self.assertEqual(g.kills, [0, 1])
        self.assertEqual(g.heroes[0].gear, [])
        self.assertEqual(g.heroes[0].lane, -1)
        self.assertIn(Order(0, 0), g.legal(0))

    def test_used_card_requires_recovery(self):
        g = self.duel()
        g.resolve([Order(2), Order(2)])
        self.assertNotIn(Order(2), g.legal(0))
        g.resolve([Order(4), Order(4)])
        g.resolve([Order(0, 0), Order(0, 0)])
        self.assertIn(Order(2), g.legal(0))

    def test_invalid_batch_is_atomic(self):
        g = self.duel()
        before = g.public()
        with self.assertRaises(AssertionError):
            g.resolve([Order(0, 0), Order(7, target=0)])
        self.assertEqual(g.public(), before)

    def test_axe_protects_ally_from_strike(self):
        g = Game(2, ["axe", "lina", "sven", "crystal_maiden"])
        for h in g.heroes:
            h.lane = 0
        g.resolve([Order(5), Order(3), Order(1, target=1), Order(0, 1)])
        self.assertEqual(g.heroes[1].hp, HP[g.heroes[1].kind])
        self.assertEqual(g.heroes[0].hp, 8)

    def test_sven_team_shield_preserves_spellcasters_push(self):
        for protected in (False, True):
            g = Game(2, ["sven", "lina", "crystal_maiden", "drow"])
            for h in g.heroes:
                h.lane = 0
            g.resolve([Order(6 if protected else 3), Order(5), Order(5), Order(2)])
            self.assertEqual(g.heroes[1].hp, HP[g.heroes[1].kind] if protected else HP[g.heroes[1].kind]-2)

    def test_bkb_protects_active_attacker(self):
        g = self.duel()
        g.heroes[0].gear = ["bkb"]
        g.resolve([Order(1, target=1), Order(5)])
        self.assertEqual(g.heroes[0].hp, HP[g.heroes[0].kind])
        self.assertEqual(g.heroes[1].hp, HP[g.heroes[1].kind]-3)

    def test_silence_also_cancels_defensive_hero_skills(self):
        g = Game(2, ["sven", "lina", "drow", "crystal_maiden"])
        for h in g.heroes:
            h.lane = 0
        g.resolve([Order(6), Order(3), Order(6), Order(5)])
        self.assertEqual(g.heroes[1].hp, HP[g.heroes[1].kind]-2)

    def test_upgrading_boots_uses_price_difference_and_same_slot(self):
        g = self.duel()
        g.heroes[0].gear = ["boots", "bracer"]
        g.heroes[0].gold = 3
        g.resolve([Order(4, buy="phase_boots"), Order(4)])
        self.assertEqual(g.heroes[0].gold, 0)
        self.assertEqual(g.heroes[0].gear, ["bracer", "phase_boots"])

    def test_force_staff_adds_real_movement_action(self):
        g = Game(2, ["lina", "drow", "sven", "axe"])
        g.heroes[0].gear = ["force_staff"]
        force = Order(1, lane=1, target=3)
        self.assertIn(force, g.legal(0))
        g.resolve([force, Order(4), Order(4), Order(3)])
        self.assertEqual(g.heroes[0].lane, 1)
        self.assertLess(g.heroes[3].hp, g.heroes[3].maximum())

    def test_earlier_lethal_attack_prevents_return_attack(self):
        g = self.duel()
        for h in g.heroes:
            h.hp = 3
        g.resolve([Order(1, target=1), Order(1, target=0)])
        self.assertEqual(g.kills, [1, 0])
        self.assertEqual(g.heroes[0].hp, 3)

    def test_reflection_can_still_cause_mutual_death(self):
        g = self.duel()
        g.heroes[0].hp = 1
        g.heroes[1].hp = 3
        g.heroes[1].gear = ["blade_mail"]
        g.resolve([Order(1, target=1), Order(3)])
        self.assertEqual(g.kills, [1, 1])
        self.assertEqual([h.gold for h in g.heroes], [1, 1])

    def test_restore_roundtrip_including_kill_tiebreak(self):
        for n in (1, 2, 3):
            rng = random.Random(n)
            g = Game(n, rng.sample(HEROES, 2*n))
            g.kills = [2, 1]
            g.first = 1
            for _ in range(8):
                restored = from_public(json.loads(json.dumps(g.public())))
                orders = [rng.choice(g.legal(p)) for p in range(2*n)]
                g.resolve(orders)
                restored.resolve(orders)
                self.assertEqual(g.public(), restored.public())
                self.assertEqual(g.winner, restored.winner)
                if g.finished:
                    break

    def test_no_side_or_iteration_order_advantage(self):
        for n in (1, 2, 3):
            for seed in range(30):
                rng = random.Random(seed+100)
                g = Game(n, rng.sample(HEROES, 2*n))
                while not g.finished:
                    other = mirrored(g)
                    orders = [rng.choice(g.legal(p)) for p in range(2*n)]
                    g.resolve(orders)
                    other.resolve(mirror_orders(orders, n))
                    self.assertEqual(mirrored(g).public(), other.public(), (n, seed, g.round))
                    if g.finished:
                        self.assertEqual(g.winner, 1-other.winner)

    def test_pending_enemy_orders_cannot_change_bot_choice(self):
        g = Game(3)
        g.pending = {3: Order(0, 1)}
        before = choose(g.public(), 0, "balanced", random.Random(9))
        g.pending = {3: Order(1, target=0), 4: Order(4)}
        self.assertEqual(before, choose(g.public(), 0, "balanced", random.Random(9)))
        self.assertNotIn("pending", g.public())
        with self.assertRaises(AssertionError):
            choose(g.public(), 0, "balanced", random.Random(9), {3: Order(4)})

    def test_all_three_modes_finish_without_missing_actions(self):
        for n in (1, 2, 3):
            rng = random.Random(n + 30)
            g = Game(n, rng.sample(HEROES, 2*n))
            while not g.finished:
                g.resolve([choose(g.public(), p, "balanced" if p == 0 else "random", rng)
                           for p in range(n*2)])
            self.assertIn(g.winner, (0, 1))
            self.assertLessEqual(g.round-1, 60)

    def test_all_modes_do_not_end_at_two_kills_or_one_tower(self):
        for n in (1, 2, 3):
            g = Game(n)
            g.kills[0] = 2
            g.towers[1][0] = 0
            g.resolve([Order(4) for _ in g.heroes])
            self.assertFalse(g.finished)

    def test_all_modes_finish_only_with_destroyed_ancient(self):
        for n in (1, 2, 3):
            g = Game(n)
            g.core[1] = 1
            g.towers[1][0] = 0
            orders = [Order(4) for _ in g.heroes]
            orders[0] = Order(0, 0)
            g.resolve(orders)
            self.assertEqual((g.winner, g.end_reason), (0, "ancient"))

    def test_fountain_cannot_be_used_for_endless_recovery(self):
        g = self.duel()
        g.resolve([Order(4), Order(4)])
        self.assertEqual(g.legal(0), [Order(0, 0)])

    def test_regression_repeated_mutual_death_cannot_stall_these_duels(self):
        for seed, roster in ((10017, ["juggernaut", "earthshaker"]), (10034, ["juggernaut", "drow"])):
            g, _, _ = play(1, seed, roster=roster)
            self.assertTrue(g.finished)
            self.assertEqual(g.end_reason, "ancient")

    def test_kills_advance_reinforcements_without_skipping_tower(self):
        g = self.duel()
        g.heroes[1].hp = 3
        g.resolve([Order(1, target=1), Order(3)])
        self.assertEqual(g.towers[1][0], 0)
        self.assertEqual(g.core[1], 2)

    def test_blink_enables_spell_flank_but_previous_damage_disables_it(self):
        g = Game(3, ["earthshaker", "drow", "lina", "axe", "sven", "crystal_maiden"])
        g.heroes[0].gear = ["blink"]
        flank = Order(5, lane=2, target=5)
        self.assertIn(flank, g.legal(0))
        g.heroes[0].wounded = True
        self.assertNotIn(flank, g.legal(0))
        g.heroes[0].wounded = False
        orders = [Order(3) for _ in g.heroes]
        orders[0] = flank
        g.resolve(orders)
        self.assertEqual(g.heroes[0].lane, 2)
        self.assertEqual(g.heroes[5].hp, 4)

    def test_failed_retreat_does_not_trigger_arcane_boots(self):
        g = Game(2, ["lina", "drow", "sven", "crystal_maiden"])
        for h in g.heroes:
            h.lane = 0
        g.heroes[0].hp = 3
        g.heroes[0].gear = ["arcane_boots"]
        g.resolve([Order(4), Order(3), Order(1, target=0), Order(3)])
        self.assertEqual(g.heroes[1].charge, 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)
