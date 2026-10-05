import test from "node:test";
import assert from "node:assert/strict";
import {
  caravanGeometry,
  caravanPick,
  caravanSelected,
  caravanResponder,
  caravanCanRespond,
  caravanBonus,
} from "../src/catan-caravans-state.ts";
import { catanSavedVictoryTarget } from "../src/catan-rule-context.ts";

test("south-facing original wagon rotates to all six actual travel directions", () => {
  const labels = ["朝东南", "朝南", "朝西南", "朝西北", "朝北", "朝东北"];
  for (let i = 0; i < 6; i++) {
    const angle = ((30 + i * 60) * Math.PI) / 180;
    const g = {
      vertices: [
        { x: 100, y: 200 },
        { x: 100 + 46 * Math.cos(angle), y: 200 + 46 * Math.sin(angle) },
      ],
      edges: [{ a: 0, b: 1 }],
    };
    const p = caravanGeometry(g, { edge: 0, from: 0 }),
      r = caravanGeometry(g, { edge: 0, from: 1 });
    assert.equal(p.direction, labels[i]);
    assert.equal(r.direction, labels[(i + 3) % 6]);
    assert.equal(p.x, r.x);
    assert.equal(p.y, r.y);
    const rotation = (p.rotation * Math.PI) / 180;
    assert.ok(Math.abs(-Math.sin(rotation) - p.dx) < 1e-10);
    assert.ok(Math.abs(Math.cos(rotation) - p.dy) < 1e-10);
    assert.ok(Math.abs(p.dx + r.dx) < 1e-10 && Math.abs(p.dy + r.dy) < 1e-10);
  }
  assert.equal(
    caravanGeometry({ edges: [], vertices: [] }, { edge: 0, from: 0 }),
    null,
  );
  assert.equal(
    caravanGeometry(
      {
        edges: [{ a: 0, b: 1 }],
        vertices: [
          { x: 0, y: 0 },
          { x: 0, y: 0 },
        ],
      },
      { edge: 0, from: 0 },
    ),
    null,
  );
});

test("a shared edge never silently picks one of two legal directions", () => {
  const g = {
    caravans: {
      choices: [
        { edge: 4, from: 2 },
        { edge: 4, from: 3 },
        { edge: 8, from: 9 },
      ],
    },
  };
  const pick = caravanPick(g, 4);
  assert.deepEqual(pick, { edge: 4, from: null });
  assert.equal(caravanSelected(g, pick), undefined);
  assert.deepEqual(caravanSelected(g, { edge: 4, from: 3 }), {
    edge: 4,
    from: 3,
  });
  assert.deepEqual(caravanPick(g, 8), { edge: 8, from: 9 });
  assert.equal(caravanPick(g, 99), null);
  assert.equal(caravanSelected(g, { edge: 4, from: 99 }), undefined);
});

test("response permissions follow the bidder without replacing the action owner", () => {
  const room = {
    status: "playing",
    you: 2,
    game: {
      turn: 0,
      catan: {
        players: [{}, {}, {}],
        caravans: { actor: 2, pending: { kind: "vote" }, canAct: true },
      },
    },
  };
  assert.equal(caravanResponder(room), 2);
  assert.equal(caravanCanRespond(room), true);
  assert.equal(caravanCanRespond({ ...room, you: 0 }), false);
  assert.equal(caravanCanRespond({ ...room, spectating: true }), false);
  assert.equal(caravanCanRespond({ ...room, you: -1 }), false);
  assert.equal(caravanResponder({ ...room, status: "finished" }), undefined);
  assert.equal(
    caravanCanRespond({ ...room, game: { ...room.game, finished: true } }),
    false,
  );
  const after = structuredClone(room);
  delete after.game.catan.caravans.pending;
  assert.equal(caravanResponder(after), undefined);
  assert.equal(caravanCanRespond(after), false);
  assert.equal(catanSavedVictoryTarget(room.game.catan), 12);
});

test("the public building badge marks one bonus for two or three incident wagons", () => {
  const g = {
    edges: [
      { a: 0, b: 1 },
      { a: 1, b: 2 },
      { a: 1, b: 3 },
    ],
    caravans: { wagons: [{ edge: 0, from: 0 }] },
  };
  assert.equal(caravanBonus(g, 1), false);
  g.caravans.wagons.push({ edge: 1, from: 1 });
  assert.equal(caravanBonus(g, 1), true);
  assert.equal(caravanBonus(g, 0), false);
  g.caravans.wagons.push({ edge: 2, from: 3 });
  assert.equal(caravanBonus(g, 1), true);
});
