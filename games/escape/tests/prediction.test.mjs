import { test } from "node:test";
import assert from "node:assert/strict";
import { Motion } from "../web/prediction.ts";
const player = (extra = {}) => ({
  seat: 0,
  room: "r4",
  x: 400,
  y: 400,
  dx: 0,
  dy: 0,
  hp: 100,
  seq: 0,
  facing: "down",
  ready: false,
  ...extra,
});
test("native movement is visible before any network response and stops on release", () => {
  const m = new Motion();
  m.update(player(), 0, "epoch", 0);
  m.input(1, 0, 0);
  assert.ok(m.sample(20).x > 402);
  const held = m.sample(30).x;
  m.input(0, 0, 30);
  assert.equal(m.sample(100).x, held);
});
test("missing snapshots bound prediction, duplicate frames do not extend its life", () => {
  const m = new Motion();
  m.update(player(), 0, "epoch", 0);
  m.input(1, 0, 0);
  m.update(player(), 0, "epoch", 150);
  assert.ok(m.sample(2000).x <= 439);
});
test("room, life and epoch changes discard previous movement", () => {
  const m = new Motion();
  m.update(player(), 0, "epoch", 0);
  m.input(1, 0, 0);
  m.sample(50);
  m.update(player({ room: "r1", x: 180, y: 420 }), 100, "epoch", 100);
  assert.equal(m.sample(100).x, 180);
  m.update(player({ room: "r1", x: 180, y: 420, hp: 0 }), 200, "epoch", 200);
  m.input(1, 0, 200);
  assert.equal(m.sample(220).x, 180);
});
test("walk targets respect the same world boundary as authoritative travel", () => {
  const m = new Motion();
  m.update(player({ x: 850, goal: { x: 855, y: 400 } }), 0, "epoch", 0);
  assert.equal(m.sample(100).x, 855);
});
test("an explicit release cancels an earlier click target locally", () => {
  const m = new Motion();
  m.update(player({ goal: { x: 700, y: 400 } }), 0, "epoch", 0);
  m.input(1, 0, 10);
  m.input(0, 0, 40);
  const x = m.sample(40).x;
  assert.equal(m.sample(100).x, x);
});
test("a later acknowledged click target resumes smooth travel after keyboard use", () => {
  const m = new Motion();
  m.update(player(), 0, "epoch", 0);
  m.input(1, 0, 0);
  m.input(0, 0, 40);
  m.update(
    player({ x: 410, goal: { x: 700, y: 400 }, seq: 3 }),
    200,
    "epoch",
    200,
  );
  assert.ok(m.sample(250).x > m.sample(200).x);
});
