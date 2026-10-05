import test from "node:test";
import assert from "node:assert/strict";
import { catanCityMotion } from "../src/catan-city-motion.ts";
const knight = (owner, vertex, active = true) => ({
  owner,
  vertex,
  strength: 1,
  active,
});
function room() {
  return {
    id: "table",
    version: 10,
    you: 0,
    status: "playing",
    game: {
      catan: {
        rollId: 4,
        setupStep: 6,
        players: [{}, {}, {}],
        tiles: [{ id: 0 }, { id: 1 }],
        vertices: Array.from({ length: 5 }, (_, id) => ({
          id,
          level: 0,
          owner: -1,
          x: id * 40,
          y: 0,
        })),
        citiesKnights: {
          knights: [knight(0, 0), knight(1, 2)],
          walls: [],
          metropolises: [-1, -1, -1],
          merchant: { owner: 0, tile: 0 },
          barbarianPosition: 6,
          invasions: 0,
          players: Array.from({ length: 3 }, () => ({
            defenderPoints: 0,
            progressPoints: 0,
            improvements: [0, 0, 0],
          })),
        },
      },
    },
  };
}
function next(r) {
  const n = structuredClone(r);
  n.version++;
  return n;
}
test("knight displacement follows attacker, and subsequent retreat follows the displaced knight", () => {
  const a = room(),
    b = next(a),
    k = b.game.catan.citiesKnights;
  k.knights = [knight(0, 2, false)];
  k.pending = { kind: "knight_retreat", knight: knight(1, 2) };
  assert.deepEqual(catanCityMotion(a, b).moves, [{ from: 0, to: 2 }]);
  const c = next(b);
  c.game.catan.citiesKnights.knights.push(knight(1, 4));
  delete c.game.catan.citiesKnights.pending;
  assert.deepEqual(catanCityMotion(b, c).moves, [{ from: 2, to: 4 }]);
});
test("state changes show recruitment, city loss, merchant transfer, improvement and finished invasion", () => {
  const a = room(),
    b = next(a),
    k = b.game.catan.citiesKnights;
  a.game.catan.vertices[3].level = 2;
  b.game.catan.vertices[3].level = 1;
  k.knights.push(knight(2, 4, false));
  k.merchant = { owner: 1, tile: 1 };
  k.invasions = 1;
  k.barbarianPosition = 0;
  k.players[1].improvements[0] = 1;
  const m = catanCityMotion(a, b);
  assert.deepEqual(m.merchant, { from: 0, to: 1 });
  assert.deepEqual(m.ship, { from: 6, to: 0, attack: true });
  assert.ok(m.flashes.some((f) => f.vertex === 3 && f.loss));
  assert.ok(m.flashes.some((f) => f.vertex === 4 && !f.loss));
  assert.deepEqual(m.players, [1]);
});
test("repeats, rematches, initial setup, different viewers and missed snapshots never invent movement", () => {
  const a = room();
  for (const change of [
    (b) => (b.version = 10),
    (b) => (b.version = 13),
    (b) => (b.id = "other"),
    (b) => (b.you = 1),
    (b) => (b.spectating = true),
    (b) => (b.game.catan.rollId = 0),
    (b) => (b.game.catan.setupStep = 0),
  ]) {
    const b = next(a);
    change(b);
    assert.equal(catanCityMotion(a, b), null);
  }
});
test("progress flights use only new contiguous public events; missing history is not replayed", () => {
  const a = room(),
    b = next(a);
  a.game.catan.citiesKnights.progressEventId = 4;
  const k = b.game.catan.citiesKnights;
  k.progressEventId = 6;
  k.progressEvents = [
    { id: 4 },
    { id: 5, kind: "draw", player: 2, track: 1, count: 1 },
    { id: 6, kind: "transfer", player: 0, other: 2, track: -1, count: 1 },
  ];
  const m = catanCityMotion(a, b);
  assert.deepEqual(m.cards, k.progressEvents.slice(1));
  assert.ok(m.cards.every((e) => e.card === undefined));
  k.progressEventId = 7;
  k.progressEvents = [{ id: 5 }, { id: 7 }];
  assert.deepEqual(catanCityMotion(a, b).cards, []);
  k.progressEvents = k.progressEvents.slice(2);
  assert.deepEqual(catanCityMotion(a, b).cards, []);
});
