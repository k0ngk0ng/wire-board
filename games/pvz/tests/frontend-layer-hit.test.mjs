import assert from "node:assert/strict";
import test from "node:test";
import { plantKind, plantsAt } from "../web/scene.ts";

// Failure cases were listed before this targeted, low-risk geometry fix. This
// fixture represents the public layered board, including a teammate's cannon.
const plant = (id, kind, row, col, layer, owner = 0) => Object.freeze({ id, kind, row, col, layer, owner, hp: 300, maxHP: 300, readyAt: 0 });
const view = Object.freeze({ plants: Object.freeze([
  plant(1, "flowerpot", 2, 3, "support"),
  plant(2, "flowerpot", 2, 4, "support"),
  plant(3, "cobcannon", 2, 3, "body", 2),
  plant(4, "wallnut", 2, 5, "body"),
]) });

test("both cannon halves find its body and their own base", () => {
  assert.deepEqual(plantsAt(view, 2, 3).map(plant => plant.id), [1, 3]);
  assert.deepEqual(plantsAt(view, 2, 4).map(plant => plant.id), [2, 3]);
});
test("footprint does not capture unrelated neighboring cells or rows", () => {
  assert.deepEqual(plantsAt(view, 2, 5).map(plant => plant.id), [4]);
  assert.deepEqual(plantsAt(view, 1, 4), []);
  assert.deepEqual(plantsAt(view, 2, 2), []);
  assert.equal(plantsAt(view, 2, 4).find(plant => plant.id === 3), view.plants[2]);
});
test("imitater renders its authoritative kind until transformation commits", () => {
  const pending = { ...plant(5, "imitater", 0, 0, "body"), targetKind: "peashooter", imitated: true };
  assert.equal(plantKind({ ...pending, state: "imitating" }), "imitater");
  assert.equal(plantKind(pending), "imitater");
  assert.equal(plantKind({ ...pending, kind: "peashooter", state: "ready" }), "peashooter");
});
