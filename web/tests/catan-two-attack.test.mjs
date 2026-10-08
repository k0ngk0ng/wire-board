import test from "node:test";
import assert from "node:assert/strict";
import { twoRetreatAction, twoRetreatTargets } from "../src/catan-two-state.ts";
import { catanRuleContext } from "../src/catan-rule-context.ts";
import {
  attackSelectedAction,
  emptyAttackSelection,
} from "../src/catan-attack-state.ts";
test("two attack draft and saved rules do not leak into base games", () => {
  const room = {
    kind: "catan",
    capacity: 2,
    catanTwoRules: "catan-for-two-2025",
    catanTwoScenario: "barbarian-attack",
  };
  assert.equal(catanRuleContext(room).twoAttack, true);
  assert.equal(catanRuleContext(room).target, 12);
  room.game = { catan: { players: [{}, {}] } };
  assert.equal(catanRuleContext(room).twoAttack, false);
  assert.equal(catanRuleContext(room).attack, false);
  assert.equal(catanRuleContext(room).target, 10);
});
test("barbarian transfer requires a legal source/destination and active token window", () => {
  const room = {
    status: "playing",
    you: 0,
    seats: [{}, {}],
    game: {
      turn: 0,
      catan: {
        players: [{}, {}],
        attack: {},
        two: {
          tokenWindow: true,
          tokens: [5, 5],
          cost: 1,
          attackMoves: { 3: [4, 5] },
        },
      },
    },
  };
  assert.deepEqual(twoRetreatTargets(room), []);
  assert.deepEqual(twoRetreatAction(room, { piece: 3, tile: 4 }), {
    type: "catan_two_robber",
    card: 3,
    tile: 4,
  });
  for (const pick of [
    null,
    { tile: 4 },
    { piece: 3, tile: 3 },
    { piece: 8, tile: 4 },
  ])
    assert.equal(twoRetreatAction(room, pick), null);
  for (const mutate of [
    (r) => (r.spectating = true),
    (r) => (r.seats[0].autoPlay = true),
    (r) => (r.game.catan.two.spent = true),
    (r) => (r.game.catan.two.tokenWindow = false),
    (r) => (r.game.turn = 1),
    (r) => (r.game.catan.two.tokens[0] = 0),
  ]) {
    const r = structuredClone(room);
    mutate(r);
    assert.equal(twoRetreatAction(r, { piece: 3, tile: 4 }), null);
  }
});
test("neutral knight uses current server recruitment choices with shared prompt", () => {
  const room = {
    status: "playing",
    you: 0,
    seats: [{}, {}],
    game: {
      phase: "catan_attack_card",
      turn: 0,
      catan: {
        players: [{}, {}],
        attack: {
          canAct: true,
          pending: { id: 1, player: 0, card: "knighthood", neutral: true },
          edges: [8, 9],
        },
      },
    },
  };
  const selection = { ...emptyAttackSelection(), target: 8 };
  assert.deepEqual(attackSelectedAction(room, selection), {
    type: "catan_attack_card",
    prompt: 1,
    choice: "knighthood",
    edge: 8,
  });
  room.game.catan.attack.edges = [9];
  assert.equal(attackSelectedAction(room, selection), null);
});
