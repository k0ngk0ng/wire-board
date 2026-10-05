import { test } from "node:test";
import assert from "node:assert/strict";
import {
  catanCardEventAction,
  catanCardEventActor,
  catanCardEventMapMode,
} from "../src/catan-card-event-state.ts";
const selection = (s = {}) => ({ color: null, target: null, map: null, ...s });
function fixture(kind) {
  return {
    status: "playing",
    you: 1,
    spectating: false,
    game: {
      phase: "catan_card_event",
      turn: 0,
      finished: false,
      catan: {
        cardEvent: { kind, players: [1, 2], production: 6 },
        players: [{}, { resources: [2, 0, 1, 0, 0, 1, 0, 0] }, {}],
        bank: [19, 0, 18, 19, 19, 11, 12, 12],
        legal: {
          eventResources: [0, 2, 3, 4],
          eventGifts: [0, 2, 5],
          eventTargets: [0, 2],
          earthquakeRoads: [7, 9],
          fleeDeserts: [3, 8],
        },
      },
    },
  };
}
test("event responder can act outside their turn; stale actor, phase and inactive viewers cannot", () => {
  const r = fixture("plentiful_year"),
    s = selection({ color: 0 });
  assert.equal(catanCardEventActor(r), true);
  assert.deepEqual(catanCardEventAction(r, s), {
    type: "catan_event_resource",
    take: [1, 0, 0, 0, 0],
  });
  for (const changes of [
    { you: 0 },
    { you: -1 },
    { spectating: true },
    { status: "closed" },
  ])
    assert.equal(catanCardEventAction({ ...r, ...changes }, s), null);
  r.game.catan.players[1].eliminated = true;
  assert.equal(catanCardEventAction(r, s), null);
  delete r.game.catan.players[1].eliminated;
  r.game.finished = true;
  assert.equal(catanCardEventAction(r, s), null);
  r.game.finished = false;
  r.game.catan.cardEvent.players = [2];
  assert.equal(catanCardEventAction(r, s), null);
  r.game.catan.cardEvent.players = [1];
  r.game.phase = "catan_turn";
  assert.equal(catanCardEventAction(r, s), null);
});
test("awards allow ordinary resources only and recheck bank stock and server legal choices", () => {
  for (const kind of ["plentiful_year", "calm_seas", "tournament"]) {
    const r = fixture(kind);
    for (const color of [null, -1, 1, 5, 8, 0.5])
      assert.equal(catanCardEventAction(r, selection({ color })), null);
    assert.equal(catanCardEventAction(r, selection({ color: 2 })).take[2], 1);
    r.game.catan.bank[2] = 0;
    assert.equal(catanCardEventAction(r, selection({ color: 2 })), null);
    r.game.catan.legal.eventResources = [];
    assert.equal(catanCardEventAction(r, selection({ color: 0 })), null);
  }
});
test("gifts allow owned commodities, helpful neighbor additionally requires a legal recipient", () => {
  for (const kind of ["good_neighbors", "helpful_neighbor"]) {
    const r = fixture(kind),
      s = selection({ color: 5, target: 2 });
    assert.deepEqual(catanCardEventAction(r, s), {
      type: "catan_event_gift",
      give: [0, 0, 0, 0, 0, 1, 0, 0],
      ...(kind === "helpful_neighbor" ? { target: 2 } : {}),
    });
    for (const color of [null, -1, 1, 6, 8, 2.5])
      assert.equal(
        catanCardEventAction(r, selection({ color, target: 2 })),
        null,
      );
    if (kind === "helpful_neighbor")
      for (const target of [null, 1, 9])
        assert.equal(
          catanCardEventAction(r, selection({ color: 5, target })),
          null,
        );
    r.game.catan.players[1].resources[5] = 0;
    assert.equal(catanCardEventAction(r, s), null);
  }
});
test("map confirms reject changed locations, stale modes and nonresponding viewers", () => {
  for (const [kind, id, field] of [
    ["earthquake", 7, "edge"],
    ["robber_flees", 3, "tile"],
  ]) {
    const r = fixture(kind);
    assert.equal(catanCardEventMapMode(r), kind);
    assert.deepEqual(
      catanCardEventAction(r, selection({ map: { type: kind, id } })),
      { type: `catan_${kind}`, [field]: id },
    );
    for (const map of [null, { type: "road", id }, { type: kind, id: 99 }])
      assert.equal(catanCardEventAction(r, selection({ map })), null);
    r.you = 0;
    assert.equal(catanCardEventMapMode(r), "");
    assert.equal(
      catanCardEventAction(r, selection({ map: { type: kind, id } })),
      null,
    );
  }
});
test("theft uses public legal targets; only explicitly optional conflict may be skipped", () => {
  for (const kind of ["conflict", "trade_advantage"]) {
    const r = fixture(kind);
    assert.deepEqual(catanCardEventAction(r, selection({ target: 2 })), {
      type: "catan_event_steal",
      target: 2,
    });
    for (const target of [null, 1, 9])
      assert.equal(catanCardEventAction(r, selection({ target })), null);
    assert.equal(catanCardEventAction(r, selection({ skip: true })), null);
    r.game.catan.cardEvent.canSkip = true;
    assert.deepEqual(
      catanCardEventAction(r, selection({ skip: true })),
      kind === "conflict" ? { type: "catan_event_skip" } : null,
    );
    r.game.catan.legal.eventTargets = [];
    assert.equal(catanCardEventAction(r, selection({ target: 2 })), null);
  }
});
