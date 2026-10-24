// Direct-contact failure cases are recorded in CONTRACT.md before implementation.
import test from "node:test";
import assert from "node:assert/strict";
import { PaddleMotion } from "../web/motion.ts";
test("mouse movement needs no button, and speed determines drive power", () => {
  const slow = new PaddleMotion(),
    fast = new PaddleMotion();
  for (const [m, dt] of [
    [slow, 70],
    [fast, 15],
  ]) {
    m.move(180, 250, 1000);
    m.move(182, 230, 1000 + dt);
  }
  const a = slow.sample(1070, 360),
    b = fast.sample(1015, 360);
  assert.equal(a.stroke.kind, "drive");
  assert.equal(b.stroke.kind, "drive");
  assert.ok(b.stroke.power > a.stroke.power + 0.35);
  assert.ok(b.velocity.y < a.velocity.y);
  assert.equal(b.moving, true);
});
test("backward movement chops, resting paddle blocks, entry never invents speed", () => {
  const m = new PaddleMotion();
  m.move(100, 100, 1000);
  assert.equal(m.sample(1000, 360).moving, false);
  assert.equal(m.sample(1000, 360).settled, false);
  assert.equal(m.sample(1080, 360).settled, true);
  m.move(100, 125, 1020);
  assert.equal(m.sample(1020, 360).stroke.kind, "chop");
  assert.equal(m.sample(1200, 360).stroke.kind, "block");
  assert.deepEqual(m.sample(1200, 360).velocity, { x: 0, y: 0 });
});
test("cancel and discontinuous samples discard velocity, scaling preserves strokes", () => {
  const a = new PaddleMotion(),
    b = new PaddleMotion();
  a.move(100, 150, 1000);
  a.move(110, 120, 1040);
  b.move(200, 300, 1000);
  b.move(220, 240, 1040);
  assert.deepEqual(a.sample(1040, 300).stroke, b.sample(1040, 600).stroke);
  a.clear();
  assert.equal(a.sample(1040, 300).active, false);
  b.move(500, 900, 2000);
  assert.equal(b.sample(2000, 600).moving, false);
});
