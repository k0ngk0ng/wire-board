import test from "node:test";
import assert from "node:assert/strict";
import { twoReturnValid, twoChoiceName } from "../src/catan-two-state.ts";
import { treasonRemoveSites, progressChoiceAction } from "../src/catan-progress-choice-state.ts";
import { progressOpponents } from "../src/catan-progress-state.ts";
import { catanRuleContext } from "../src/catan-rule-context.ts";
import { catanResultDescription } from "../src/catan-results.ts";

test("two-player city trade cannot return commodities in resource-only mode", () => {
  const hand = [1, 1, 0, 0, 0, 2, 0, 0];
  assert.equal(twoReturnValid(hand, [1, 1, 0, 0, 0, 0, 0, 0], false), true);
  assert.equal(twoReturnValid(hand, [0, 0, 0, 0, 0, 2, 0, 0], false), false);
  assert.equal(twoReturnValid(hand, [0, 0, 0, 0, 0, 2, 0, 0], true), true);
  assert.equal(twoReturnValid(hand, [1, 1, 0, 0, 0], false), false);
});

test("neutral Treason offers occupied colors and only their weakest knights", () => {
  const g = {
    players: [{}, {}], two: {knights: "catan-for-two-knights-2025"},
    citiesKnights: {
      knights: [{owner: -2, vertex: 1, strength: 2}, {owner: -2, vertex: 2, strength: 1}, {owner: -2, vertex: 3, strength: 1}, {owner: 0, vertex: 4, strength: 1}],
      pending: {kind: "treason_remove", source: "two_neutral", color: -2, players: [0]},
    },
  };
  assert.deepEqual(progressOpponents(g, 0, 22), [-2]);
  assert.deepEqual(treasonRemoveSites(g, 0), [2, 3]);
  assert.deepEqual(treasonRemoveSites(g, 1), []);
  const room = {you: 0, status: "playing", game: {phase: "catan_treason_remove", catan: g}};
  const selection = {map: {type: "treason_remove", id: 1}};
  assert.equal(progressChoiceAction(room, selection), null);
  selection.map.id = 2;
  assert.deepEqual(progressChoiceAction(room, selection), {type: "catan_treason_remove", vertex: 2});
  room.spectating = true;
  assert.equal(progressChoiceAction(room, selection), null);
  delete g.two;
  assert.deepEqual(progressOpponents(g, 0, 22), []);
});

test("city scenario keeps thirteen-point rules while saved base games stay base", () => {
  const room = {kind: "catan", capacity: 2, catanTwoRules: "catan-for-two-2025", catanTwoScenario: "cities-knights"};
  assert.equal(catanRuleContext(room).target, 13);
  assert.equal(catanRuleContext(room).citiesKnights, true);
  room.game = {catan: {players: [{}, {}], two: {}, citiesKnights: {}, vertices: []}};
  assert.match(catanResultDescription(room.game.catan), /防御者/);
  assert.doesNotMatch(catanResultDescription(room.game.catan), /军队奖励/);
  delete room.game.catan.citiesKnights;
  assert.equal(catanRuleContext(room).target, 10);
  assert.equal(catanRuleContext(room).citiesKnights, false);
  const g = {two: {pending: {kind: "knight"}}};
  assert.equal(twoChoiceName(g, {vertex: 2, edge: -1}), "一级骑士");
  assert.equal(twoChoiceName(g, {vertex: -1, edge: 2}), "道路");
  g.two.pending.kind = "knight_promote";
  assert.equal(twoChoiceName(g, {vertex: 2, edge: -1}), "骑士升级");
});
