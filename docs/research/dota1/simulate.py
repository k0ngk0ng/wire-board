#!/usr/bin/env python3
"""Disposable-rule laboratory, NOT a production game implementation.

No network, dependencies, realtime movement or hidden combat dice. The only
randomness is drafting and bot decisions. Every round can be resolved on paper.
"""
from collections import Counter
from dataclasses import asdict, dataclass, field
import argparse
import copy
import json
import math
from pathlib import Path
import random
import statistics

ROOT = Path(__file__).resolve().parent
CARDS = ("推进", "突袭", "固守", "打钱", "整备", "技能一", "技能二", "大招")
HEROES = ("axe", "crystal_maiden", "earthshaker", "sven", "juggernaut", "drow", "lina", "sniper")
HP = dict(zip(HEROES, (8, 7, 8, 8, 7, 7, 7, 7)))
ITEMS = {
    # Price, preferred archetype. Effects are defined explicitly in resolve().
    "blink": (6, "spell"), "boots": (3, "push"), "bracer": (3, "tank"),
    "wraith_band": (4, "attack"), "null_talisman": (3, "spell"),
    "wand": (4, "tank"), "blade_mail": (6, "tank"),
    "bkb": (6, "attack"), "mekansm": (7, "support"),
    "vanguard": (6, "tank"), "crystalys": (6, "attack"),
    "aghanim": (8, "spell"), "force_staff": (6, "push"),
    "phase_boots": (6, "push"), "arcane_boots": (6, "support"),
    "quelling_blade": (3, "push"), "blades_attack": (3, "attack"),
}


@dataclass
class Hero:
    kind: str
    team: int
    lane: int
    hp: int
    gold: int = 2
    charge: int = 0
    spent: int = 0
    gear: list = field(default_factory=list)
    wounded: bool = False

    def maximum(self):
        return HP[self.kind] + 2 * ("bracer" in self.gear) + 2 * ("vanguard" in self.gear) + int("wraith_band" in self.gear)


@dataclass(frozen=True)
class Order:
    card: int
    lane: int = -1
    target: int = -1
    buy: str = ""


class Game:
    def __init__(self, n, roster=None):
        assert n in (1, 2, 3)
        self.n = n
        roster = roster or list(HEROES[:2 * n])
        self.heroes = [Hero(k, i // n, i % n, HP[k]) for i, k in enumerate(roster)]
        self.towers = [[1 if n == 1 else 2] * n for _ in range(2)]
        self.core = [n + 1] * 2
        self.track = [0] * n
        self.round = 1
        self.first = 0
        self.finished = False
        self.winner = -1
        self.end_reason = ""
        self.kills = [0, 0]
        self.log = []

    def public(self):
        # Deliberately does not contain a pending-order field or RNG state.
        return {"n": self.n, "round": self.round, "first": self.first, "towers": copy.deepcopy(self.towers),
                "core": self.core[:], "track": self.track[:],
                "heroes": [asdict(h) for h in self.heroes], "kills": self.kills[:]}

    def legal(self, p):
        return legal(self.public(), p)

    def resolve(self, orders, verify=True):
        assert not self.finished and len(orders) == len(self.heroes)
        # Validate the whole simultaneous batch before mutating anything.
        if verify:
            for p, order in enumerate(orders):
                assert order in self.legal(p), (p, order)
        hs, size = self.heroes, len(self.heroes)
        shield, pressure, disabled = [0] * size, [0] * size, [0] * size
        heal, silence, immune = [0] * size, [False] * size, [False] * size
        taunt, reflect, events = {}, [0] * size, []
        old_lanes = [h.lane for h in hs]
        old_wounds = [h.wounded for h in hs]
        for p, (h, a) in enumerate(zip(hs, orders)):
            h.wounded = False
            if a.card not in (0, 4) and a.lane >= 0:
                assert ("force_staff" in h.gear and abs(h.lane-a.lane) == 1) or ("blink" in h.gear and not old_wounds[p] and a.card >= 5)
                h.lane = a.lane
            if a.card == 4:
                # A planned retreat does not dodge damage already aimed at you.
                pass
            elif a.card == 0:
                h.lane = a.lane
                pressure[p] = 3 if old_lanes[p] == h.lane else 1 if old_lanes[p] < 0 else 2
                pressure[p] += int("quelling_blade" in h.gear)
                pressure[p] += int(old_lanes[p] != h.lane and bool(set(h.gear) & {"boots", "phase_boots", "arcane_boots"}))
                shield[p] += 2 * int("phase_boots" in h.gear)
            elif a.card == 2:
                shield[p], pressure[p] = 3, 2
                heal[p] += 2 * int("wand" in h.gear)
            elif a.card == 1:
                pressure[p] = 1
            if a.card != 4:
                h.spent |= 1 << a.card
            if a.card == 7:
                h.charge -= 3
            immune[p] = ("bkb" in h.gear and a.card in (1, 7)) or (h.kind == "juggernaut" and a.card == 5)
            reflect[p] = 2 if "blade_mail" in h.gear and a.card == 2 else int("blade_mail" in h.gear)

        def enemies(p):
            return [j for j, x in enumerate(hs) if x.team != hs[p].team and x.lane == hs[p].lane and x.lane >= 0]

        def allies(p):
            return [j for j, x in enumerate(hs) if x.team == hs[p].team and x.lane == hs[p].lane and x.lane >= 0]

        def hit(p, target, damage, magical=False, anywhere=False):
            if target < 0 or hs[target].lane < 0:
                return
            if not anywhere and hs[target].lane != hs[p].lane:
                return
            damage += int(magical and "null_talisman" in hs[p].gear)
            damage += 2 * int(orders[p].card == 7 and "aghanim" in hs[p].gear)
            events.append((p, target, damage, magical))

        # Silence resolves simultaneously before other hero skills. Spell
        # immunity was established above, before control effects.
        for p, (h, a) in enumerate(zip(hs, orders)):
            if h.lane >= 0 and h.kind == "drow" and a.card == 6:
                for j in enemies(p):
                    if not immune[j]:
                        silence[j] = True
                pressure[p] += 2
        # Defensive cards are exposed before attack cards.
        for p, (h, a) in enumerate(zip(hs, orders)):
            if h.lane < 0:
                continue
            if h.kind == "axe" and a.card == 5 and not silence[p]:
                taunt[(h.team, h.lane)] = p
                shield[p] += 4
                pressure[p] += 2
            if h.kind == "sven" and a.card == 6 and not silence[p]:
                for j in allies(p):
                    shield[j] += 3
                pressure[p] += 2
            if "mekansm" in h.gear and a.card == 2:
                for j in allies(p):
                    heal[j] += 2

        for p, (h, a) in enumerate(zip(hs, orders)):
            if h.lane < 0:
                continue
            if a.card == 1:
                damage = 3 + len(set(h.gear) & {"wraith_band", "blades_attack"}) + 2 * int("crystalys" in h.gear)
                damage += int(h.kind == "juggernaut" and h.hp == h.maximum())
                hit(p, a.target, damage)
                if h.kind == "sven":
                    for j in enemies(p):
                        if j != a.target:
                            hit(p, j, 1)
            if a.card < 5 or silence[p]:
                continue
            k, spell, target = h.kind, a.card, a.target
            if spell == 7:
                pressure[p] += 1
                if k in ("crystal_maiden", "earthshaker", "juggernaut"):
                    for j in enemies(p):
                        hit(p, j, 4 if k != "earthshaker" else min(5, 2 + len(enemies(p))), True)
                else:
                    damage = 7 if k == "axe" and target >= 0 and hs[target].hp <= 4 else 6
                    hit(p, target, damage, k not in ("sven", "drow"), k == "sniper")
                continue
            if k == "axe":
                if spell == 6:
                    hit(p, target, 3, True)
                    pressure[p] += 1
            elif k == "crystal_maiden":
                targets = enemies(p) if spell == 5 else ([target] if target in enemies(p) else [])
                for j in targets:
                    hit(p, j, 2 if spell == 5 else 4, True)
                    disabled[j] += int(not immune[j])
                pressure[p] += 1
            elif k == "earthshaker":
                hit(p, target, 3 if spell == 5 else 4, spell == 5)
                pressure[p] += 1
                if spell == 5 and target in enemies(p) and not immune[target]:
                    disabled[target] += 2
            elif k == "sven":
                if spell == 5:
                    hit(p, target, 3, True)
                    if target in enemies(p) and not immune[target]:
                        disabled[target] += 2
            elif k == "juggernaut":
                if spell == 5:
                    for j in enemies(p):
                        hit(p, j, 3, True)
                    pressure[p] += 1
                else:
                    for j in allies(p):
                        heal[j] += 3
                    pressure[p] += 1
            elif k == "drow":
                if spell == 5:
                    hit(p, target, 3)
                    if target in enemies(p) and not immune[target]:
                        disabled[target] += 2
            elif k == "lina":
                for j in enemies(p):
                    hit(p, j, 3 if spell == 5 else 2, True)
                    if spell == 6 and not immune[j]:
                        disabled[j] += 1
                pressure[p] += 1
            elif k == "sniper":
                if spell == 5:
                    for j in enemies(p):
                        hit(p, j, 1, True)
                    pressure[p] += 2
                else:
                    hit(p, target, 4)

        # Everyone commits together, but damage resolves in a public alternating
        # order. This makes initiative a decision input and prevents repeated
        # mutually lethal duels that never advance either wave.
        sources = [set() for _ in hs]
        for p, h in enumerate(hs):
            h.hp = min(h.maximum(), h.hp + heal[p])
        first = (self.first + self.round - 1) % 2
        attack_order = [team*self.n + (offset+self.round-1) % self.n
                        for offset in range(self.n) for team in (first, 1-first)]
        for actor in attack_order:
            if hs[actor].hp <= 0:
                continue
            damage, reflected = [0] * size, [0] * size
            for p, j, amount, magic in events:
                if p != actor or hs[j].hp <= 0:
                    continue
                if orders[p].card == 1:
                    protector = taunt.get((hs[j].team, hs[j].lane), j)
                    if hs[protector].hp > 0:
                        j = protector
                if magic and immune[j]:
                    continue
                amount -= int(not magic and "vanguard" in hs[j].gear)
                damage[j] += max(0, amount)
                sources[j].add(p)
                retaliation = reflect[j] + int(hs[j].kind == "axe" and orders[p].card == 1 and hs[p].lane == hs[j].lane)
                reflected[p] += retaliation
                if retaliation:
                    sources[p].add(j)
            # All targets and reflection of this one card settle together.
            for p, h in enumerate(hs):
                absorbed = min(shield[p], damage[p])
                shield[p] -= absorbed
                wounds = damage[p] - absorbed + reflected[p]
                h.hp -= wounds
                if wounds:
                    pressure[p] = 0
                    h.wounded = True
        dead = set()
        gold_changes = [0] * size
        reinforcements = [[0] * self.n for _ in range(2)]
        for p, h in enumerate(hs):
            if h.lane < 0:
                continue
            if h.hp <= 0:
                dead.add(p)
                self.kills[1 - h.team] += 1
                reinforcements[1-h.team][h.lane] += 3
                gold_changes[p] -= 2
                for attacker in sources[p]:
                    gold_changes[attacker] += 1
                h.lane, h.hp, h.spent = -1, h.maximum(), 0
                pressure[p] = 0
        for p, h in enumerate(hs):
            h.gold = min(12, max(0, h.gold + gold_changes[p]))
        for p, (h, a) in enumerate(zip(hs, orders)):
            if a.card == 4 and p not in dead:
                if a.buy:
                    price = purchase_price(h.gear, a.buy)
                    h.gold -= price
                    if a.buy in ("phase_boots", "arcane_boots") and "boots" in h.gear:
                        h.gear.remove("boots")
                    elif len(h.gear) >= 2:
                        h.gear.pop(0)
                    h.gear.append(a.buy)
                h.lane, h.spent, h.hp = -1, 0, h.maximum()
        siege = reinforcements
        for p, h in enumerate(hs):
            if h.lane >= 0:
                siege[h.team][h.lane] += max(0, pressure[p] - disabled[p])
        for lane in range(self.n):
            difference = siege[0][lane] - siege[1][lane]
            if not difference:
                continue
            direction = 1 if difference > 0 else -1
            self.track[lane] += direction * (2 if abs(difference) >= 3 else 1)
            if abs(self.track[lane]) >= 2:
                defending = 1 if self.track[lane] > 0 else 0
                amount = 3 if self.round >= 21 else 2 if self.round >= 13 else 1
                if self.towers[defending][lane] > 0:
                    self.towers[defending][lane] = max(0, self.towers[defending][lane] - amount)
                else:
                    self.core[defending] = max(0, self.core[defending] - amount)
                self.track[lane] = 0
        for p, (h, a) in enumerate(zip(hs, orders)):
            h.gold = min(12, h.gold + int(self.round % 2 == 0) + 3 * int(a.card == 3 and h.lane >= 0))
            h.charge = min(3, h.charge + 1)
            if a.card == 4 and p not in dead and "arcane_boots" in h.gear:
                for friend in hs:
                    if friend.team == h.team:
                        friend.charge = min(3, friend.charge + 1)
        if min(self.core) == 0:
            self.finished = True
            self.winner = (self.first + self.round - 1) % 2 if self.core[0] == self.core[1] else int(self.core[1] > self.core[0])
            self.end_reason = "ancient"
        self.log.append({"round": self.round, "cards": [a.card for a in orders],
                         "core": self.core[:], "towers": copy.deepcopy(self.towers), "kills": self.kills[:], "lanes": [h.lane for h in hs]})
        self.round += 1


def purchase_price(gear, item):
    return ITEMS[item][0] - (3 if item in ("phase_boots", "arcane_boots") and "boots" in gear else 0)


def legal(view, p, allow_force=True):
    h = view["heroes"][p]
    if h["lane"] < 0:
        return [Order(0, lane=lane) for lane in range(view["n"])]
    result = [Order(4)]
    result += [Order(4, buy=k) for k in ITEMS if k not in h["gear"] and purchase_price(h["gear"], k) <= h["gold"]
               and not (view["n"] == 1 and k in ("force_staff", "mekansm", "blink"))]
    if not h["spent"] & 1:
        result += [Order(0, lane=lane) for lane in range(view["n"])]
    targets = [j for j, other in enumerate(view["heroes"]) if other["team"] != h["team"] and other["lane"] >= 0 and other["lane"] == h["lane"]]
    for card in (1, 2, 3, 5, 6, 7):
        if h["spent"] & (1 << card) or (card == 7 and h["charge"] < 3):
            continue
        untargeted = card in (2, 3) or (h["kind"], card) in {
            ("axe", 5), ("crystal_maiden", 5), ("crystal_maiden", 7),
            ("earthshaker", 7), ("sven", 6), ("juggernaut", 5),
            ("juggernaut", 6), ("juggernaut", 7), ("drow", 6),
            ("lina", 5), ("lina", 6), ("sniper", 5)}
        card_targets = [j for j, other in enumerate(view["heroes"]) if other["team"] != h["team"] and other["lane"] >= 0] if h["kind"] == "sniper" and card == 7 else targets
        result.extend(Order(card, target=t) for t in ([-1] if untargeted else card_targets or [-1]))
    if allow_force and ("force_staff" in h["gear"] or ("blink" in h["gear"] and not h["wounded"])):
        for lane in range(view["n"]):
            if lane == h["lane"]:
                continue
            alternate = copy.deepcopy(view)
            alternate["heroes"][p]["lane"] = lane
            for a in legal(alternate, p, allow_force=False):
                can_force = "force_staff" in h["gear"] and abs(lane-h["lane"]) == 1
                can_blink = "blink" in h["gear"] and not h["wounded"] and a.card >= 5
                if a.card not in (0, 4) and (can_force or can_blink):
                    result.append(Order(a.card, lane, a.target))
    return list(dict.fromkeys(result))


def item_value(h, item, policy):
    kind = ITEMS[item][1]
    pref = "spell" if h["kind"] in ("lina", "crystal_maiden", "earthshaker") else "attack"
    if h["kind"] == "axe":
        pref = "tank"
    if policy == "rush":
        pref = "push"
    if policy == "hunt":
        pref = "attack"
    return 1.1 + 1.0 * (kind == pref) + 0.35 * (ITEMS[item][0] >= 6) - 1.5 * (len(h["gear"]) >= 2)


def choose(view, p, policy, rng, team_orders=None):
    """No pending actions are accepted by this API; bots only see public state."""
    options = legal(view, p)
    if policy == "balanced":
        return search(view, p, rng, team_orders or {})
    if policy == "random":
        return rng.choice(options)
    hs, h = view["heroes"], view["heroes"][p]
    friends = [x for x in hs if x["team"] == h["team"] and x["lane"] == h["lane"]]
    foes = [x for x in hs if x["team"] != h["team"] and x["lane"] == h["lane"] and x["lane"] >= 0]
    spent = h["spent"].bit_count()
    maximum = HP[h["kind"]] + 2 * ("bracer" in h["gear"]) + 2 * ("vanguard" in h["gear"]) + int("wraith_band" in h["gear"])
    hurt = maximum - h["hp"]
    attack_weight = 1.55 if policy == "hunt" else 0.35 if policy == "rush" else 1
    push_weight = 1.6 if policy == "rush" else 0.35 if policy == "hunt" else 1
    scored = []
    for a in options:
        score = 0.0
        if a.card == 4:
            score = -1.6 + 0.8 * spent + 0.40 * hurt
            if h["hp"] <= 3 and foes:
                score += 3
            if h["lane"] < 0:
                score = -5
            if a.buy:
                score += item_value(h, a.buy, policy)
            if not h["spent"] and not hurt:
                score -= 3
        elif a.card == 0:
            lane = a.lane
            enemies_here = [x for x in hs if x["team"] != h["team"] and x["lane"] == lane]
            friends_here = [x for i, x in enumerate(hs) if i != p and x["team"] == h["team"] and x["lane"] == lane]
            score = push_weight * (3.5 if h["lane"] == lane else 2.7)
            score += 1.3 * (not enemies_here) - 0.8 * len(friends_here)
            score += 1.1 * (view["towers"][1-h["team"]][lane] == 0)
            score += 0.8 * (view["towers"][h["team"]][lane] == 0 and bool(enemies_here))
            if h["lane"] < 0:
                score += 6
            if h["hp"] <= 3 and enemies_here:
                score -= 2
        elif a.card == 3:
            score = (3.6 if policy == "economy" else 1.9) * (0.3 if h["gold"] >= 10 else 1)
            score -= 0.4 * len(foes)
        elif a.card == 2:
            score = push_weight * 1.8 + 0.8 * len(foes) + 0.25 * hurt
            if "blade_mail" in h["gear"]:
                score += 1.3 * bool(foes)
        else:
            target = hs[a.target] if a.target >= 0 else None
            damage = 3 if a.card == 1 else 6 if a.card == 7 else 3
            if h["kind"] == "axe" and a.card == 5:
                score = 2.0 * push_weight + 1.1 * len(foes) + 0.4 * hurt
            elif h["kind"] == "juggernaut" and a.card == 6:
                score = push_weight + sum(min(3, HP[x["kind"]] - x["hp"]) for x in friends) * 0.8
            elif h["kind"] == "sven" and a.card == 6:
                score = push_weight * 2 + 0.6 * len(foes) * len(friends)
            elif h["kind"] == "drow" and a.card == 6:
                score = push_weight * 2 + 0.8 * len(foes)
            else:
                score = (1 if a.card in (1, 5, 7) else 0.6) * push_weight
                if target:
                    score += attack_weight * (damage * 0.6 + max(0, damage + 1 - target["hp"]) * 0.65)
                    # Going home is predictable from low HP and a depleted hand.
                    if target["hp"] <= 3 and target["spent"].bit_count() >= 3:
                        score -= 1.2
                if h["kind"] in ("lina", "crystal_maiden", "juggernaut", "earthshaker") and a.card >= 5:
                    score += 0.8 * max(0, len(foes) - 1)
        scored.append((score + rng.uniform(-0.8, 0.8), a))
    return max(scored, key=lambda x: x[0])[1]


def from_public(view):
    game = Game(view["n"])
    game.heroes = [Hero(**dict(h, gear=h["gear"][:])) for h in view["heroes"]]
    game.round = view["round"]
    game.first = view["first"]
    game.kills = view["kills"][:]
    game.towers = [x[:] for x in view["towers"]]
    game.core = view["core"][:]
    game.track = view["track"][:]
    return game


def value(game, team):
    if game.finished:
        return 0 if game.winner < 0 else 1000 if game.winner == team else -1000
    total = 0.0
    for side in (0, 1):
        score = game.core[side] * 14 + sum(game.towers[side]) * 6
        for h in game.heroes:
            if h.team == side:
                score += h.hp / h.maximum() * HP[h.kind] * 0.6 + h.gold * 0.6 + h.charge * 0.3
                score += sum(ITEMS[x][0] * 0.85 for x in h.gear)
                score -= h.spent.bit_count() * 0.25
                score += 0.8 * (h.lane >= 0)
        score += sum(game.track) * (1 if side == 0 else -1)
        total += score * (1 if side == team else -1)
    return total


def search(view, p, rng, team_orders):
    team = view["heroes"][p]["team"]
    assert all(view["heroes"][j]["team"] == team for j in team_orders)
    guesses = []
    for policy in ("rush", "hunt", "economy", "heuristic"):
        guesses.append([team_orders.get(j) or choose(view, j, policy if h["team"] != team else "heuristic", rng)
                        for j, h in enumerate(view["heroes"])])
    best, chosen = -math.inf, None
    for action in legal(view, p):
        outcomes = []
        for guess in guesses:
            orders = guess[:]
            orders[p] = action
            trial = from_public(view)
            trial.resolve(orders, verify=False)
            outcomes.append(value(trial, team))
        score = statistics.mean(outcomes) * 0.8 + min(outcomes) * 0.2 + rng.uniform(-0.5, 0.5)
        if score > best:
            best, chosen = score, action
    assert chosen is not None
    return chosen


def play(n, seed, policies=("balanced", "balanced"), roster=None):
    rng = random.Random(seed)
    roster = roster or rng.sample(HEROES, 2 * n)
    game = Game(n, roster)
    game.first = rng.randrange(2)
    histogram, purchased = Counter(), Counter()
    while not game.finished:
        assert game.round <= 60, ("Unresolved game exceeds validation horizon", n, seed)
        snapshot = game.public()
        actions = [None] * (2 * n)
        for team in (0, 1):
            proposals = {}
            for offset in range(n):
                p = team * n + (offset + game.round) % n
                actions[p] = choose(snapshot, p, policies[team], rng, proposals)
                proposals[p] = actions[p]
        for a in actions:
            histogram[CARDS[a.card]] += 1
            if a.buy:
                purchased[a.buy] += 1
        game.resolve(actions)
        assert all(0 <= h.gold <= 12 and 0 <= h.charge <= 3 and 0 < h.hp <= h.maximum() for h in game.heroes)
        assert all(-2 < track < 2 for track in game.track)
    return game, histogram, purchased


def run(count):
    result = {"rules": "v0.9-full-ancient", "games_per_mode_matchup": count, "modes": {}}
    for n in (1, 2, 3):
        matchups = {}
        for policy in ("balanced", "random", "rush", "hunt", "economy"):
            wins, reasons, actions, items = Counter(), Counter(), Counter(), Counter()
            lengths, kills = [], []
            hero_appearances, hero_wins = Counter(), Counter()
            initial_side_wins = 0
            for seed in range(count):
                # Swap sides in paired runs rather than trusting seating order.
                roster = random.Random(seed // 2 + 8000).sample(HEROES, n * 2)
                sides = ("balanced", policy)
                if seed % 2:
                    roster = roster[n:] + roster[:n]
                    sides = sides[::-1]
                g, hist, gear = play(n, seed + 10000, sides, roster)
                winner = g.winner if seed % 2 == 0 or g.winner < 0 else 1 - g.winner
                wins[winner] += 1
                reasons[g.end_reason] += 1
                actions.update(hist)
                items.update(gear)
                lengths.append(g.round - 1)
                kills.append(sum(g.kills))
                initial_side_wins += g.winner == g.first
                if policy == "balanced":
                    for h in g.heroes:
                        hero_appearances[h.kind] += 1
                        hero_wins[h.kind] += h.team == g.winner
            matchups[policy] = {"balanced_win": wins[0] / count, "opponent_win": wins[1] / count,
                               "draw": wins[-1] / count, "mean_rounds": round(statistics.mean(lengths), 2),
                               "mean_kills": round(statistics.mean(kills), 2), "endings": dict(reasons),
                               "actions": dict(actions), "items": dict(items),
                               "max_rounds": max(lengths), "first_marker_win": initial_side_wins/count,
                               "heroes": {h: {"appearances": hero_appearances[h], "wins": hero_wins[h]} for h in hero_appearances}}
        result["modes"][f"{n}v{n}"] = matchups
    (ROOT / "simulation-results.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    for mode, matchups in result["modes"].items():
        for policy, data in matchups.items():
            print(mode, policy, {k: v for k, v in data.items() if k not in ("actions", "items", "heroes")})


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--count", type=int, default=100)
    args = parser.parse_args()
    run(args.count)
