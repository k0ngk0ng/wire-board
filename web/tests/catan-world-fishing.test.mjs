import test from "node:test";
import assert from "node:assert/strict";
import { worldFishPreview } from "../src/catan-world-fishing-state.ts";

function room() {
  return {
    status: "playing",
    you: 0,
    spectating: false,
    game: {
      turn: 0,
      phase: "catan_world_fish",
      finished: false,
      catan: {
        players: [{}, {}],
        fishing: {
          worldSetup: { index: 0, current: 8, total: 6, remaining: 6 },
        },
        legal: { fishGrounds: [1] },
        edges: [
          { id: 0, a: 0, b: 1, tiles: [0, 2] },
          { id: 1, a: 1, b: 2, tiles: [1, 2] },
          { id: 2, a: 1, b: 3, tiles: [0, 1] },
        ],
        tiles: [
          { id: 0, resource: 0, vertices: [0, 1, 3] },
          { id: 1, resource: 1, vertices: [1, 2, 3] },
          { id: 2, resource: 6, vertices: [0, 1, 2] },
        ],
      },
    },
  };
}
test("ground preview follows the public eligible concave vertex and retains map data", () => {
  const r = room(),
    before = structuredClone(r);
  assert.deepEqual(worldFishPreview(r, 1), {
    number: 8,
    edges: [0, 1],
    vertices: [0, 1, 2],
  });
  assert.deepEqual(r, before);
  for (const e of r.game.catan.edges) delete e.tiles;
  assert.deepEqual(worldFishPreview(r, 1), {
    number: 8,
    edges: [0, 1],
    vertices: [0, 1, 2],
  });
  assert.equal(worldFishPreview(r, 3), null);
  assert.equal(worldFishPreview(r, null), null);
  r.game.catan.edges[1].b = 99;
  assert.equal(worldFishPreview(r, 1), null);
});
test("stale selection, observer, inactive seat, and hidden current face produce no preview", () => {
  for (const change of [
    (r) => (r.spectating = true),
    (r) => (r.you = -1),
    (r) => (r.you = 1),
    (r) => (r.game.turn = 1),
    (r) => (r.game.finished = true),
    (r) => (r.status = "finished"),
    (r) => (r.game.phase = "catan_world_ports"),
    (r) => (r.game.catan.players[0].eliminated = true),
    (r) => (r.game.catan.legal.fishGrounds = []),
    (r) => delete r.game.catan.fishing.worldSetup.current,
  ]) {
    const r = room();
    change(r);
    assert.equal(worldFishPreview(r, 1), null);
  }
});
