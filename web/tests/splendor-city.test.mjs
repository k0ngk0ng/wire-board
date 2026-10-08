import { test } from "node:test";
import assert from "node:assert/strict";
import {
  gemCardCounts,
  gemNobleEligible,
  gemCityProgress,
  splendorResultDescription,
  splendorCitySource,
} from "../src/splendor-city-state.ts";

test("city source distinguishes new rooms from legacy saved games and non-city play", () => {
  assert.match(splendorCitySource({ cities: true }, undefined, true), /本站城市分组/);
  assert.match(splendorCitySource({ cities: true }, "wire-board-cities-v1"), /本站城市分组/);
  for (const catalogue of [undefined, "2025-secondary-v1", "2025-cities-bga-v1"]) {
    assert.match(splendorCitySource({ cities: true }, catalogue), /沿用开局时保存/);
    assert.doesNotMatch(splendorCitySource({ cities: true }, catalogue), /本站城市分组/);
  }
  assert.equal(splendorCitySource({}, "wire-board-cities-v1"), "");
});

test("noble highlights match Orient physical counts while preserving base-save discounts", () => {
  const noble = { cost: [3, 0, 0, 0, 0] };
  const p = { cards: [], bonus: [3, 0, 0, 0, 0] };
  assert.equal(gemNobleEligible(p, noble), true);
  p.cards = [{ color: 0, orient: "double", bonusCount: 2 }, { color: 0 }];
  assert.equal(gemNobleEligible(p, noble), false);
  p.cards.push({ color: 0, orient: "copy", bonusCount: 1 });
  assert.equal(gemNobleEligible(p, noble), true);
});

const card = (color, orient, bonusCount) => ({ color, orient, bonusCount });
const player = (cards = [], score = 14) => ({
  cards,
  score,
  bonus: [0, 0, 0, 0, 0],
});
const city = (cost, any = 0, points = 14) => ({ cost, any, points });

test("cities count physical acquired cards and paired copies, not double discounts or colorless cards", () => {
  const p = player([
    card(0),
    card(0, "double", 2),
    card(1, "copy", 2),
    card(4, "copy_cascade", 1),
    card(-1, "gold"),
    card(-1, "copy"),
  ]);
  p.bonus = [3, 2, 0, 0, 1];
  p.reserved = [card(0), card(0)];
  assert.deepEqual(gemCardCounts(p), [2, 1, 0, 0, 1]);
  assert.equal(gemCityProgress(p, city([3, 0, 0, 0, 0])).eligible, false);
  assert.equal(gemCityProgress(p, city([2, 1, 0, 0, 1])).eligible, true);
});

test("another color cannot reuse printed required colors or add several smaller colors together", () => {
  const p = player([
    ...Array(5).fill(card(1)),
    ...Array(3).fill(card(0)),
    ...Array(3).fill(card(2)),
  ]);
  const target = city([0, 4, 0, 0, 0], 4);
  assert.equal(gemCityProgress(p, target).other, 3);
  assert.equal(gemCityProgress(p, target).eligible, false);
  p.cards.push(card(2, "copy", 2));
  assert.equal(gemCityProgress(p, target).eligible, true);
  p.score = 13;
  assert.equal(gemCityProgress(p, target).eligible, false);
});

test("any one color, points-only goals, elimination and progress reversal stay accurate", () => {
  const p = player(Array(5).fill(card(3)), 17);
  assert.equal(gemCityProgress(p, city([0, 0, 0, 0, 0], 5, 15)).eligible, true);
  p.cards.pop();
  assert.equal(
    gemCityProgress(p, city([0, 0, 0, 0, 0], 5, 15)).eligible,
    false,
  );
  assert.equal(gemCityProgress(p, city([0, 0, 0, 0, 0], 0, 17)).eligible, true);
  p.eliminated = true;
  assert.equal(
    gemCityProgress(p, city([0, 0, 0, 0, 0], 0, 17)).eligible,
    false,
  );
});

test("results explain city qualification instead of a fifteen-point ending, retaining ties and timeout wins", () => {
  const s = { players: [player(), player()], options: { cities: true } };
  assert.match(splendorResultDescription(s), /仅满足城市条件/);
  assert.match(splendorResultDescription(s), /发展卡更少.*共同获胜/);
  assert.doesNotMatch(splendorResultDescription(s), /15/);
  s.options = {};
  assert.match(splendorResultDescription(s), /15 分/);
  s.options = { cities: true };
  s.players[0].eliminated = true;
  assert.match(splendorResultDescription(s), /最后留在牌桌/);
});
