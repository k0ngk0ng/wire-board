import { test } from "node:test";
import assert from "node:assert/strict";
import {
  newProgressSelection,
  progressPlayAction,
  progressMapTargets,
  pickProgressTarget,
  progressGain,
  commercialOfferAction,
} from "../src/catan-progress-state.ts";
function room(card) {
  return {
    status: "playing",
    you: 0,
    spectating: false,
    game: {
      turn: 0,
      finished: false,
      phase: card === 0 ? "catan_roll" : "catan_turn",
      catan: {
        players: [
          { resources: [3, 3, 3, 3, 3, 3, 3, 3] },
          { publicScore: 5 },
          { publicScore: 2, eliminated: true },
        ],
        vertices: [
          { id: 0, owner: 0, level: 2 },
          { id: 1, owner: 0, level: 1 },
          { id: 2, owner: 1, level: 2 },
        ],
        tiles: [
          { id: 0, resource: 3, number: 5, vertices: [0, 1] },
          { id: 1, resource: 3, number: 9, vertices: [1, 2] },
          { id: 2, resource: 5, number: 0, vertices: [2] },
        ],
        bank: [19, 19, 19, 3, 19, 12, 12, 12],
        robber: 2,
        legal: { cities: [1], roads: [4] },
        progressPlayable: [card],
        inventionTiles: [0, 1],
        smithingOptions: [[6], [6, 7], [7]],
        merchantTiles: [0, 1],
        guildDuesTargets: [1],
        intrigueTargets: [8],
        diplomacyRoads: [4],
        citiesKnights: {
          players: [{ progress: [card], improvements: [3, 2, 0] }],
          knights: [{ owner: 1, vertex: 8, strength: 1 }],
          walls: [],
          metropolises: [-1, -1, -1],
          invasions: 1,
          tradePowers: { player: 0, fleets: [], harbors: [[1], []] },
        },
      },
    },
  };
}
function selection(card, fields = {}) {
  return { ...newProgressSelection(card), ...fields };
}
test("all 23 active progress types construct their specific actions; public VP never plays", () => {
  for (const card of Array.from({ length: 25 }, (_, i) => i).filter(
    (i) => ![9, 23].includes(i),
  )) {
    const r = room(card),
      fields =
        {
          0: { dice: [2, 5] },
          1: { color: 0 },
          2: { picks: [0] },
          3: { picks: [0, 1] },
          5: { picks: [1] },
          8: { picks: [6, 7] },
          11: { target: 1 },
          12: { picks: [0] },
          13: { color: 7 },
          14: { color: 0 },
          15: { color: 5 },
          16: { picks: [4] },
          18: { target: 1 },
          19: { picks: [8] },
          21: { picks: [0] },
          22: { target: 1 },
        }[card] || {};
    const action = progressPlayAction(r, selection(card, fields));
    assert.ok(action, `card ${card}`);
    assert.equal(action.type, "catan_progress");
    assert.equal(action.card, card);
    if (fields.picks && card === 3)
      assert.deepEqual(action, {
        type: "catan_progress",
        card: 3,
        tile: 0,
        target: 1,
      });
    if (card === 8) assert.deepEqual(action.targets, [6, 7]);
    if (card === 0) assert.deepEqual(action.tokens, [2, 5]);
  }
  for (const card of [9, 23])
    assert.equal(progressPlayAction(room(card), selection(card)), null);
});
test("discounted city upgrades keep metropolis conditions; medicine checks reduced cost and server priority sites", () => {
  const r = room(1);
  assert.ok(progressPlayAction(r, selection(1, { color: 0 })));
  r.game.catan.players[0].resources[5] = 2;
  assert.equal(progressPlayAction(r, selection(1, { color: 0 })), null);
  r.game.catan.citiesKnights.metropolises = [-1, 0, -1];
  r.game.catan.players[0].resources[5] = 3;
  assert.equal(progressPlayAction(r, selection(1, { color: 0 })), null);
  const m = room(5);
  m.game.catan.players[0].resources[3] = 1;
  m.game.catan.players[0].resources[4] = 2;
  assert.deepEqual(progressMapTargets(m.game.catan, 0, selection(5)), [1]);
  m.game.catan.players[0].resources[4] = 1;
  assert.deepEqual(progressMapTargets(m.game.catan, 0, selection(5)), []);
});
test("invention needs two different legal tiles and smithing respects inventory-changing order", () => {
  const r = room(3);
  for (const picks of [[0], [0, 0], [0, 2]])
    assert.equal(progressPlayAction(r, selection(3, { picks })), null);
  const g = room(8).game.catan;
  let s = pickProgressTarget(g, 0, selection(8), 6);
  assert.deepEqual(progressMapTargets(g, 0, s), [6, 7]);
  s = pickProgressTarget(g, 0, s, 7);
  assert.deepEqual(s.picks, [6, 7]);
  s = pickProgressTarget(g, 0, s, 6);
  assert.deepEqual(s.picks, []);
  s = pickProgressTarget(g, 0, s, 7);
  assert.deepEqual(progressMapTargets(g, 0, s), [7]);
  assert.equal(
    progressPlayAction(room(8), selection(8, { picks: [7, 6] })),
    null,
  );
  assert.equal(progressPlayAction(room(8), selection(8, { picks: [] })), null);
});
test("private phase, owner and public target boundaries are enforced; skip still consumes a playable non-alchemy card", () => {
  for (const change of [
    { spectating: true },
    { you: 1 },
    { status: "finished" },
  ])
    assert.equal(
      progressPlayAction({ ...room(4), ...change }, selection(4)),
      null,
    );
  const r = room(4);
  r.game.catan.citiesKnights.pending = { kind: "wedding" };
  assert.equal(progressPlayAction(r, selection(4)), null);
  const a = room(0);
  a.game.phase = "catan_turn";
  assert.equal(progressPlayAction(a, selection(0)), null);
  assert.equal(progressPlayAction(room(0), selection(0, { skip: true })), null);
  assert.deepEqual(progressPlayAction(room(8), selection(8, { skip: true })), {
    type: "catan_progress",
    card: 8,
    choice: "skip",
  });
  for (const card of [11, 18, 22])
    assert.equal(
      progressPlayAction(room(card), selection(card, { target: 2 })),
      null,
    );
  for (const [card, color] of [
    [14, 5],
    [15, 0],
    [13, 8],
  ])
    assert.equal(
      progressPlayAction(room(card), selection(card, { color })),
      null,
    );
  const t = room(21);
  t.game.catan.citiesKnights.invasions = 0;
  t.game.catan.progressPlayable = [];
  assert.equal(progressPlayAction(t, selection(21, { skip: true })), null);
  assert.equal(progressGain(room(4).game.catan, 0, 4), 3); // Two unique hexes, capped by bank, not per building.
});
test("each harbor offer uses its own remaining opponents and an owned ordinary resource", () => {
  const r = room(10);
  assert.deepEqual(commercialOfferAction(r, 0, 1, 0), {
    type: "catan_commercial_offer",
    card: 0,
    target: 1,
    color: 0,
  });
  for (const args of [
    [1, 1, 0],
    [0, 0, 0],
    [0, 2, 0],
    [0, 1, 5],
    [null, 1, 0],
  ])
    assert.equal(commercialOfferAction(r, ...args), null);
  r.game.catan.players[0].resources[0] = 0;
  assert.equal(commercialOfferAction(r, 0, 1, 0), null);
  r.game.catan.players[0].resources[0] = 1;
  r.game.catan.citiesKnights.pending = { kind: "commercial_harbor" };
  assert.equal(commercialOfferAction(r, 0, 1, 0), null);
});
