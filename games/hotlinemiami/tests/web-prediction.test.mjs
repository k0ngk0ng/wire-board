import test from "node:test";
import assert from "node:assert/strict";
import { MotionPrediction, EventCursor } from "../web/prediction.ts";

const player = (extra = {}) => ({ x: 0, y: 0, aim: 0, life: 1, alive: true, downUntil: 0, appliedSeq: 0, ...extra });
const input = (extra = {}) => ({ seq: 1, moveX: 1000, moveY: 0, aim: 0, ...extra });
const motion = (x, y, aim, mx, my, frames) => [x + Math.trunc(mx * frames / 1000), y + Math.trunc(my * frames / 1000), aim];

test("local movement is visible before an authority acknowledgement", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player(), 0, 0, "epoch");
  p.input(input(), 0);
  assert.equal(p.sample(34).x, 2);
});

test("prediction has a nine-frame bound", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player(), 0, 0, "epoch");
  p.input(input(), 0);
  assert.equal(p.sample(1000).x, 9);
});

test("confirmation replays only unapplied input and retains display phase", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player(), 0, 0, "epoch");
  p.input(input(), 0);
  assert.equal(p.sample(34).x, 2);
  p.confirm(player({ x: 2, appliedSeq: 1 }), 2, 35, "epoch");
  assert.equal(p.sample(51).x, 3);
  p.confirm(player({ x: 1, appliedSeq: 0 }), 1, 52, "epoch");
  assert.equal(p.sample(68).x, 4);
});

test("death and new life cancel prior movement", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player(), 0, 0, "epoch");
  p.input(input(), 0);
  p.confirm(player({ x: 4, alive: false }), 4, 67, "epoch");
  assert.equal(p.sample(100).x, 4);
  p.confirm(player({ x: 20, life: 2 }), 6, 101, "epoch");
  assert.equal(p.sample(150).x, 20);
});

test("execution movement remains locked until its authoritative frame", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player({ busyUntil: 3 }), 0, 0, "epoch");
  p.input(input(), 0);
  assert.equal(p.sample(34).x, 0);
  assert.equal(p.sample(67).x, 2);
});

test("loading barriers do not leave the display clock ahead of the live match", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player(), 0, 0, "epoch");
  p.setEnabled(false, 0);
  assert.equal(p.sample(1000).x, 0);
  assert.equal(p.predictedFrame, 0);
  p.setEnabled(true, 1000);
  p.input(input(), 1000);
  assert.equal(p.sample(1017).x, 1);
  assert.equal(p.predictedFrame, 1);
});

test("authority one or two frames ahead still consumes fresh movement on the next display", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player(), 0, 0, "epoch");
  p.sample(50);
  p.confirm(player({ x: 20 }), 5, 51, "epoch");
  p.input(input(), 52);
  assert.ok(p.sample(67).x > 20);
});

test("first local input consumes one visible step without accelerating high refresh sampling", () => {
  const p = new MotionPrediction(motion);
  p.confirm(player(), 0, 0, "epoch");
  for (let at = 8; at <= 96; at += 8) {
    p.input(input({ seq: at }), at - 1);
    const result = p.sample(at);
    assert.ok(result.x > 0);
    assert.ok(p.predictedFrame <= Math.trunc(at * 60 / 1000) + 1);
  }
});

test("event cursor skips initial history and out-of-order duplicates", () => {
  const events = new EventCursor();
  assert.deepEqual(events.consume("e", [{ id: 1 }, { id: 2 }]), []);
  assert.deepEqual(events.consume("e", [{ id: 2 }, { id: 3 }]), [{ id: 3 }]);
  assert.deepEqual(events.consume("e", [{ id: 1 }, { id: 2 }]), []);
  assert.deepEqual(events.consume("new", [{ id: 10 }]), []);
});
