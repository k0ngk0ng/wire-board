import test from "node:test";
import assert from "node:assert/strict";
import {
  transportCanAct,
  transportEdges,
  transportSelectedAction,
} from "../src/catan-transport-state.ts";
import {
  catanSavedVictoryTarget,
  catanRuleContext,
} from "../src/catan-rule-context.ts";
const room = () => ({
  status: "playing",
  you: 0,
  seats: [{}],
  game: {
    turn: 0,
    phase: "catan_transport_move",
    catan: {
      players: [{}],
      transport: {
        canAct: true,
        barbarianPending: false,
        barbarianSequence: 9,
        state: { sequence: 14, travel: { pending: -1 } },
        choices: {
          steps: [{ edge: 3, mp: 2, toll: 1, pay: 1 }],
          relocate: [5, 6],
        },
      },
    },
  },
});
test("transport movement only uses server legal edges and correct persisted response sequence", () => {
  const r = room();
  assert.deepEqual(transportEdges(r), [3]);
  assert.deepEqual(transportSelectedAction(r, { edge: 3, piece: null }), {
    type: "catan_transport_step",
    offer: 14,
    edge: 3,
  });
  assert.equal(transportSelectedAction(r, { edge: 5, piece: null }), null);
  r.game.catan.transport.state.travel.pending = 2;
  assert.deepEqual(transportEdges(r), [5, 6]);
  assert.deepEqual(transportSelectedAction(r, { edge: 6, piece: null }), {
    type: "catan_transport_relocate",
    offer: 14,
    edge: 6,
  });
  r.game.catan.transport.barbarianPending = true;
  assert.equal(transportSelectedAction(r, { edge: 6, piece: null }), null);
  assert.equal(transportSelectedAction(r, { edge: 6, piece: 3 }), null);
  assert.deepEqual(transportSelectedAction(r, { edge: 5, piece: 1 }), {
    type: "catan_transport_barbarian",
    offer: 9,
    edge: 5,
    card: 1,
  });
});
test("observers, other seats, eliminated players, autoplay and finished rooms cannot operate transport", () => {
  for (const change of [
    { spectating: true },
    { you: -1 },
    { you: 1 },
    { status: "finished" },
    { seats: [{ autoPlay: true }] },
  ]) {
    const r = { ...room(), ...change };
    assert.equal(transportCanAct(r), false);
    assert.deepEqual(transportEdges(r), []);
    assert.equal(transportSelectedAction(r, { edge: 3, piece: null }), null);
  }
  const r = room();
  r.game.catan.players[0].eliminated = true;
  assert.equal(transportCanAct(r), false);
  r.game.catan.players[0].eliminated = false;
  r.game.finished = true;
  assert.equal(transportCanAct(r), false);
});
test("saved transport rules override ordinary ten-point fallback", () => {
  const r = room();
  assert.equal(catanSavedVictoryTarget(r.game.catan), 13);
  assert.equal(catanRuleContext(r).transport, true);
  assert.equal(catanRuleContext(r).target, 13);
});
