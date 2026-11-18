// Test first. Boundary failures: wait for ACK before rendering, a duplicate
// packet rebases the clock, a late server frame doubles a tap, received ACK is
// mistaken for applied ACK, held input changes the 100ms phase, old actions
// replay after a gap/epoch/menu/leader change, or reconciliation snaps through
// obstacles. The original-rule and real browser E2E verify the actual motor.
import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
const { build } = createRequire(new URL("../../../web/package.json", import.meta.url))("esbuild");
const bundled = await build({ entryPoints: [new URL("../web/field-prediction.ts", import.meta.url).pathname], bundle: true, write: false, format: "esm", target: "es2022" });
const { FieldPrediction } = await import("data:text/javascript;base64," + Buffer.from(bundled.outputFiles[0].contents).toString("base64"));
function view({ frame = 10, camera = [0, 0], appliedSeq = 0, seq = 0, epoch = 1, phase = "field", leader = 0, cells = Buffer.alloc(289), step = 0 } = {}) {
  return { epoch, phase, leader, scene: 2, seq, appliedSeq, field: {
    frame, camera, anchor: [camera[0] + 160, camera[1] + 112], offset: [160, 112], layer: 0, direction: 0, step, party: 1, followers: 0, radius: 8,
    trail: Array.from({ length: 5 }, () => [160, 112, 0]), checks: Array.from({ length: 5 }, () => [160, 112]), cells: cells.toString("base64"), actors: [[0, 0, 2, camera[0] + 160, camera[1] + 112, 0, 3, 0]],
  }};
}
function fixture() {
  const prediction = new FieldPrediction(), initial = view();
  prediction.update(initial, 0, 0);
  return { prediction, initial };
}
const action = (seq, direction = 4, keys = 0, epoch = 1) => ({ seq, direction, keys, epoch });

test("tap is immediate, remains independent of received ACK, and reconciles once applied", () => {
  const { prediction: p, initial } = fixture(), original = JSON.stringify(initial);
  p.input(action(1, 4, 4), 1);
  assert.deepEqual(p.current.camera, [-16, 8]);
  p.input(action(2), 2);
  p.update(view({ seq: 2 }), 0, 20);
  assert.deepEqual(p.current.camera, [-16, 8]);
  p.update(view({ frame: 11, seq: 2 }), 0, 100);
  assert.deepEqual(p.current.camera, [-16, 8], "unapplied tap doubled after late frame");
  p.update(view({ frame: 12, camera: [-16, 8], seq: 2, appliedSeq: 2 }), 0, 180);
  assert.equal(p.pending, 0);
  assert.equal(p.current, undefined);
  assert.equal(JSON.stringify(initial), original);
});

test("100ms held phase survives renewal and duplicate packets", () => {
  const { prediction: p } = fixture();
  p.input(action(1, 4, 4), 0);
  p.update(view({ seq: 1 }), 0, 60);
  p.input(action(2, 0), 110);
  assert.deepEqual(p.current.camera, [-32, 16]);
  p.sample(199); assert.deepEqual(p.current.camera, [-32, 16]);
  p.sample(201); assert.deepEqual(p.current.camera, [-48, 24]);
  p.sample(350); assert.equal(p.current, undefined, "stale authority allowed unlimited movement");
  p.update(view({ frame: 14 }), 0, 360); p.sample(500);
  assert.equal(p.current, undefined, "old held input replayed after recovery");
});

test("rapid direction edges stay ordered after the last displayed local frame", () => {
  const { prediction: p } = fixture();
  p.input(action(1, 4, 4), 10); p.input(action(2, 4, 32), 20);
  assert.deepEqual(p.current.camera, [-16, 8]);
  p.sample(110); assert.deepEqual(p.current.camera, [0, 16]);
  p.input(action(3), 115);
  p.update(view({ frame: 11, camera: [-16, 8], seq: 3, appliedSeq: 1 }), 0, 120);
  assert.deepEqual(p.current.camera, [0, 16]);
});

test("a tap applied earlier than its predicted target is retired once, after a local idle frame", () => {
  const { prediction: p } = fixture();
  p.input(action(1, 4, 4), 0); p.input(action(2), 1); p.sample(100);
  p.update(view({ frame: 11, camera: [-16, 8], seq: 2, appliedSeq: 2 }), 0, 150);
  p.input(action(3, 4, 16), 210); p.input(action(4), 211);
  p.update(view({ frame: 12, camera: [0, 0], seq: 4, appliedSeq: 4 }), 0, 250);
  assert.equal(p.pending, 0, "an already applied tap was replayed into another native frame");
  assert.equal(p.current, undefined);
});

test("an idle gesture frame cannot delay a fresh tap after 100ms without movement", () => {
  const { prediction: p } = fixture();
  p.input(action(1, 4, 4), 0); p.input(action(2), 1); p.sample(100);
  p.update(view({ frame: 11, camera: [-16, 8], seq: 2, appliedSeq: 2 }), 0, 110);
  p.input(action(3, 4, 16), 150);
  assert.deepEqual(p.current?.camera, [0, 0], "idle phase held a fresh native direction for another frame");
});

test("original collision and trigger cells bound local movement", () => {
  const cells = Buffer.alloc(289); cells[(8 + 1) * 17 + 8] = 1;
  const p = new FieldPrediction(); p.update(view({ cells }), 0, 0);
  p.input(action(1, 4, 4), 1); assert.deepEqual(p.current.camera, [0, 0]);
  const stopped = Buffer.alloc(289); stopped[8 * 17 + 8] = 2;
  p.update(view({ epoch: 2, cells: stopped }), 0, 10);
  p.input(action(1, 4, 4, 2), 11);
  assert.equal(p.pending, 0); assert.equal(p.current, undefined);
});

test("phase, leader, epoch and explicit cancellation discard old predictions", () => {
  for (const next of [view({ phase: "menu" }), view({ leader: 1 }), view({ epoch: 2 })]) {
    const { prediction: p } = fixture(); p.input(action(1, 4, 4), 1);
    p.update(next, 0, 20); p.sample(200);
    assert.equal(p.current, undefined); assert.equal(p.pending, 0);
  }
  const { prediction: p } = fixture(); p.input(action(1, 4, 4), 1); p.clear();
  assert.equal(p.current, undefined); p.sample(200); assert.equal(p.current, undefined);
});

test("authority correction rolls back rules and converges within a bounded visual correction", () => {
  const { prediction: p } = fixture(); p.input(action(1, 4, 4), 1); p.input(action(2), 2);
  p.update(view({ frame: 11, seq: 2, appliedSeq: 2 }), 0, 50);
  assert.deepEqual(p.current.camera, [-16, 8]);
  p.sample(90);
  assert.ok(p.current.camera[0] > -16 && p.current.camera[0] < 0);
  p.sample(140); assert.equal(p.current, undefined);
  p.input(action(3, 4, 32), 150); assert.deepEqual(p.current.camera, [16, 8]);
});
