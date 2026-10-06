import test from "node:test";
import assert from "node:assert/strict";
import {
  explorerChoices,
  explorerCanRespond,
  explorerSelectedAction,
  explorerDiscardAction,
  explorerActionKey,
  explorerTarget,
  explorerShipPosition,
  explorerContents,
  explorerActionDescription,
} from "../src/catan-explorer-state.ts";
import { catanColorIndex } from "../src/catan-player-colors.ts";
import { catanResultDescription } from "../src/catan-results.ts";
const room = () => ({
  id: "initial-voyage",
  status: "playing",
  you: 0,
  seats: [{}, {}],
  game: {
    turn: 0,
    phase: "catan_explorer_move",
    catan: {
      hexSize: 62,
      players: [{ resources: [2, 3, 4, 5, 6] }, {}],
      discardDue: [0, 0],
      vertices: [
        { x: 0, y: 0 },
        { x: 100, y: 0 },
        { x: 100, y: 100 },
      ],
      edges: [
        { a: 0, b: 1 },
        { a: 1, b: 2 },
      ],
      tiles: [],
      victoryTarget: 8,
      explorer: {
        sequence: 5,
        choices: [
          { type: "catan_explorer_sail", prompt: 5, slot: 0, targets: [1] },
        ],
        fleet: { positions: [0, -1, -1, 0, -1, -1], turn: { current: -1 } },
        cargo: {
          units: [
            { kind: "ship", index: 0 },
            { kind: "harbor", index: 0 },
          ],
        },
      },
    },
  },
});
test("only present humans can act; observers and autoplay have no controls", () => {
  for (const change of [
    (r) => (r.spectating = true),
    (r) => (r.you = -1),
    (r) => (r.seats[0].autoPlay = true),
    (r) => (r.game.catan.players[0].eliminated = true),
    (r) => (r.game.finished = true),
    (r) => (r.status = "closed"),
  ]) {
    const r = room();
    change(r);
    assert.equal(explorerCanRespond(r), false);
    assert.deepEqual(explorerChoices(r), []);
  }
  const r = room();
  r.game.turn = 1;
  assert.deepEqual(explorerChoices(r), []);
  assert.equal(explorerCanRespond(r), true);
});
test("confirmation revalidates room, prompt, route and live legal choices", () => {
  const r = room(),
    a = r.game.catan.explorer.choices[0],
    pick = { room: r.id, action: structuredClone(a) };
  assert.deepEqual(explorerSelectedAction(r, pick), a);
  assert.equal(explorerSelectedAction({ ...r, id: "different" }, pick), null);
  r.game.catan.explorer.sequence++;
  assert.equal(explorerSelectedAction(r, pick), null);
  r.game.catan.explorer.sequence--;
  r.game.catan.explorer.choices[0].targets = [0, 1];
  assert.equal(explorerSelectedAction(r, pick), null);
  assert.equal(
    explorerActionKey({ type: "catan_road", edge: 0, prompt: 5 }),
    explorerActionKey({ prompt: 5, edge: 0, type: "catan_road" }),
  );
});
test("parallel discard works off turn, checks exact five resources, and carries prompt", () => {
  const r = room();
  r.game.turn = 1;
  r.game.phase = "catan_discard";
  r.game.catan.discardDue[0] = 3;
  assert.deepEqual(explorerDiscardAction(r, [1, 2, 0, 0, 0]), {
    type: "catan_discard",
    prompt: 5,
    tokens: [1, 2, 0, 0, 0],
  });
  for (const bad of [
    [3, 0, 0, 0, 0],
    [0, 0, 0, 0, 0],
    [1.5, 1.5, 0, 0, 0],
    [-1, 4, 0, 0, 0],
    [1, 2],
  ])
    assert.equal(explorerDiscardAction(r, bad), null);
  r.seats[0].autoPlay = true;
  assert.equal(explorerDiscardAction(r, [1, 2, 0, 0, 0]), null);
});
test("same-edge ships have distinct perpendicular offsets; supplies stay off map", () => {
  const g = room().game.catan;
  assert.deepEqual(explorerShipPosition(g, 0), { x: 50, y: -11 });
  assert.deepEqual(explorerShipPosition(g, 3), { x: 50, y: 11 });
  assert.equal(explorerShipPosition(g, 1), null);
  assert.deepEqual(explorerContents(g, "ship", 0), [0]);
  assert.deepEqual(explorerContents(g, "harbor", 0), [1]);
});
test("map targets retain zero IDs and refer to the actual ship or harbor", () => {
  const g = room().game.catan;
  assert.deepEqual(
    explorerTarget(g, {
      type: "catan_explorer_unit",
      choice: "ship",
      target: 0,
    }),
    { kind: "edge", id: 0 },
  );
  assert.deepEqual(
    explorerTarget(g, {
      type: "catan_explorer_unit",
      choice: "harbor",
      target: 0,
    }),
    { kind: "vertex", id: 0 },
  );
  assert.deepEqual(
    explorerTarget(g, { type: "catan_explorer_sail", targets: [0, 1] }),
    { kind: "edge", id: 1 },
  );
  assert.equal(
    explorerTarget(g, { type: "catan_explorer_bank", color: 0, target: 1 }),
    null,
  );
});
test("destructive rebuilding, full cargo replacement and switching ships are explicit", () => {
  const g = room().game.catan;
  assert.match(
    explorerActionDescription(g, { type: "catan_explorer_ship", slot: 0 }),
    /拆回原船及全部货物/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_unit",
      choice: "harbor",
      target: 0,
      cards: [0],
    }),
    /归还.*原移民/,
  );
  g.explorer.fleet.turn.current = 1;
  g.tiles = [{ resource: 8, vertices: [2] }];
  const text = explorerActionDescription(g, {
    type: "catan_explorer_sail",
    slot: 0,
    targets: [1],
  });
  assert.match(text, /发现迷雾后本船停止/);
  assert.match(text, /上一艘船不能继续/);
  assert.match(catanResultDescription(g), /村庄1分、港口2分/);
  assert.equal(catanColorIndex(g, -2), 2);
  assert.equal(catanColorIndex(g, -3), 3);
});
