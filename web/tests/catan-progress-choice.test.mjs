import { test } from "node:test";
import assert from "node:assert/strict";
import {
  progressChoiceAction,
  progressChoiceDue,
  progressChoiceHand,
  treasonStrengths,
} from "../src/catan-progress-choice-state.ts";
function fixture(kind, actor = 1) {
  return {
    status: "playing",
    you: actor,
    spectating: false,
    game: {
      phase: `catan_${kind}`,
      turn: 0,
      finished: false,
      catan: {
        players: [
          { resources: Array(8).fill(0) },
          { resources: [2, 0, 0, 0, 0, 3, 0, 0] },
        ],
        citiesKnights: {
          pending: {
            kind,
            players: [actor],
            target: 0,
            resources: [1, 0, 0, 0, 0, 1, 0, 0],
            progress: [4, 4, 8],
            knight: { strength: 3, active: true },
          },
          knights: [
            { owner: 1, vertex: 3, strength: 2 },
            { owner: 1, vertex: 4, strength: 2 },
            { owner: 0, vertex: 5, strength: 3 },
          ],
        },
        diplomacyPlacements: [7],
        treasonPlacements: [9],
      },
    },
  };
}
const choose = (o = {}) => ({
  bundle: Array(8).fill(0),
  color: null,
  card: null,
  map: null,
  skip: false,
  ...o,
});
test("responses belong to the pending actor, including private hand data and queued hand counts", () => {
  const r = fixture("guild_dues"),
    s = choose({ bundle: [1, 0, 0, 0, 0, 1, 0, 0] });
  assert.equal(progressChoiceDue(r.game.catan, 1), 2);
  assert.deepEqual(progressChoiceAction(r, s), {
    type: "catan_guild_dues",
    take: s.bundle,
  });
  assert.deepEqual(progressChoiceHand(r.game.catan, 0), []);
  for (const invalid of [
    { you: 0 },
    { spectating: true },
    { status: "finished" },
  ])
    assert.equal(progressChoiceAction({ ...r, ...invalid }, s), null);
  r.game.phase = "catan_turn";
  assert.equal(progressChoiceAction(r, s), null);
});
test("resource responses distinguish taking, giving and discarding; no over-selection, negative or fractional amounts", () => {
  for (const kind of ["guild_dues", "wedding", "sabotage"]) {
    const r = fixture(kind),
      g = r.game.catan;
    const bundle =
      kind === "guild_dues"
        ? [1, 0, 0, 0, 0, 1, 0, 0]
        : [0, 0, 0, 0, 0, 2, 0, 0];
    assert.equal(progressChoiceDue(g, 1), 2); // Sabotage rounds 5/2 down.
    assert.deepEqual(progressChoiceAction(r, choose({ bundle })), {
      type: `catan_${kind}`,
      [kind === "guild_dues" ? "take" : "give"]: bundle,
    });
    for (const invalid of [
      [],
      [2, 0, 0, 0, 0, 1, 0, 0],
      [0, 2, 0, 0, 0, 0, 0, 0],
      [-1, 0, 0, 0, 0, 3, 0, 0],
      [0.5, 0, 0, 0, 0, 1.5, 0, 0],
    ])
      assert.equal(progressChoiceAction(r, choose({ bundle: invalid })), null);
    assert.equal(progressChoiceAction(r, choose({ skip: true })), null);
    if (kind !== "guild_dues") {
      g.players[1].resources = [0, 0, 0, 0, 0, 1, 0, 0];
      assert.equal(progressChoiceDue(g, 1), kind === "sabotage" ? 0 : 1);
    }
  }
});
test("commercial harbor only trades an owned commodity; espionage only takes a shown non-VP card", () => {
  const r = fixture("commercial_harbor");
  assert.deepEqual(progressChoiceAction(r, choose({ color: 5 })), {
    type: "catan_commercial_harbor",
    color: 5,
  });
  for (const color of [null, 0, 6, 7, 8])
    assert.equal(progressChoiceAction(r, choose({ color })), null);
  const e = fixture("espionage");
  assert.deepEqual(progressChoiceAction(e, choose({ card: 4 })), {
    type: "catan_espionage",
    card: 4,
  });
  for (const card of [null, 0, 9, 23, 24])
    assert.equal(progressChoiceAction(e, choose({ card })), null);
  assert.deepEqual(progressChoiceAction(e, choose({ skip: true })), {
    type: "catan_espionage",
    choice: "skip",
  });
});
test("map responses reject stale selections and respect knight inventory without requiring politics level three", () => {
  const d = fixture("diplomacy");
  assert.deepEqual(
    progressChoiceAction(d, choose({ map: { type: "diplomacy", id: 7 } })),
    { type: "catan_diplomacy", edge: 7 },
  );
  for (const map of [
    { type: "road", id: 7 },
    { type: "diplomacy", id: 8 },
    null,
  ])
    assert.equal(progressChoiceAction(d, choose({ map })), null);
  const r = fixture("treason_remove");
  assert.deepEqual(
    progressChoiceAction(r, choose({ map: { type: "treason_remove", id: 3 } })),
    { type: "catan_treason_remove", vertex: 3 },
  );
  assert.equal(
    progressChoiceAction(r, choose({ map: { type: "treason_remove", id: 5 } })),
    null,
  );
  assert.equal(progressChoiceAction(r, choose({ skip: true })), null);
  const p = fixture("treason_place"),
    map = { type: "treason_place", id: 9 };
  assert.deepEqual(treasonStrengths(p.game.catan, 1), [1, 3]);
  assert.deepEqual(progressChoiceAction(p, choose({ map, color: 3 })), {
    type: "catan_treason_place",
    vertex: 9,
    color: 3,
  });
  for (const color of [null, 0, 2, 4])
    assert.equal(progressChoiceAction(p, choose({ map, color })), null);
  assert.deepEqual(progressChoiceAction(p, choose({ skip: true })), {
    type: "catan_treason_place",
    choice: "skip",
  });
});
