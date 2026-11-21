import test from "node:test";
import assert from "node:assert/strict";
import { MotionPrediction } from "../web/prediction.ts";
import { PosePresentation } from "../web/pose-presentation.ts";

const pose = (x, extra = {}) => ({ x, y: 49664, life: 1, alive: true, ...extra });

test("a small authority correction cannot cancel the first visible reverse step", () => {
  const presentation = new PosePresentation();
  presentation.reconcile(pose(96000), pose(96768), 36851);
  assert.equal(presentation.sample(pose(96768), 36851).x, 96000);
  const logical = pose(96000);
  assert.ok(presentation.sample(logical, 36866).x < 96000);
  assert.equal(logical.x, 96000);
  assert.equal(presentation.sample(logical, 36951).x, logical.x);
});

test("the recorded stop and reverse timeline remains responsive without changing logical motion", () => {
  const motion = (x, y, aim, mx, my, frames) => [x + Math.trunc(mx * 768 * frames / 1000), y + Math.trunc(my * 768 * frames / 1000), aim];
  const prediction = new MotionPrediction(motion);
  const presentation = new PosePresentation();
  const player = (x, appliedSeq) => ({ ...pose(x), aim: 0, downUntil: 0, appliedSeq });
  const confirm = (frame, x, seq, at) => {
    const before = prediction.sample(at);
    prediction.confirm(player(x, seq), frame, at, "epoch");
    presentation.reconcile(before, prediction.sample(at), at);
  };
  const input = (seq, moveX, at) => prediction.input({ seq, moveX, moveY: 0, aim: 0 }, at);
  const draw = at => {
    const logical = prediction.sample(at);
    return { logical, shown: presentation.sample(logical, at) };
  };
  confirm(482, 92160, 95, 36650);
  draw(36699.4);
  input(96, 1000, 36702.1);
  draw(36702.1);
  confirm(484, 92160, 95, 36718.566);
  draw(36732.7);
  confirm(486, 92160, 95, 36738.615);
  draw(36749.3);
  input(97, 1000, 36750.745);
  draw(36766);
  confirm(488, 93696, 96, 36779.241);
  draw(36782.7);
  draw(36799.3);
  input(98, 1000, 36800.24);
  input(99, 0, 36803.271);
  draw(36803.271);
  confirm(490, 95232, 97, 36819.272);
  draw(36832.7);
  const before = draw(36849.3);
  confirm(492, 96768, 98, 36851.073);
  input(100, -1000, 36865.2);
  const first = draw(36866.1);
  assert.equal(first.logical.x, before.logical.x);
  assert.ok(first.shown.x < before.shown.x);
  assert.ok(prediction.predictedFrame <= 492 + 9);
  confirm(494, 96768, 99, 36887.851);
  const next = draw(36899.4);
  input(101, -1000, 36900.484);
  confirm(496, 96768, 99, 36907.297);
  const corrected = draw(36916);
  assert.ok(corrected.logical.x > next.logical.x);
  assert.ok(corrected.shown.x < next.shown.x);
});

test("life, death, geometry recovery and large corrections immediately use the logical pose", () => {
  const presentation = new PosePresentation();
  presentation.reconcile(pose(0), pose(768), 0);
  presentation.reconcile(pose(768), pose(4096, { life: 2 }), 1);
  assert.equal(presentation.sample(pose(4096, { life: 2 }), 1).x, 4096);
  presentation.reconcile(pose(4096), pose(4864), 2);
  presentation.reconcile(pose(4864), pose(5120, { alive: false }), 3);
  assert.equal(presentation.sample(pose(5120, { alive: false }), 3).x, 5120);
  presentation.reconcile(pose(5120), pose(5888), 4);
  presentation.reconcile(pose(5888), pose(6656), 5, true);
  assert.equal(presentation.sample(pose(6656), 5).x, 6656);
  presentation.reconcile(pose(6656), pose(8705), 6);
  assert.equal(presentation.sample(pose(8705), 6).x, 8705);
});

test("display correction keeps its integer distance and time boundaries", () => {
  for (const delta of [2047, 2048, 2049]) {
    const presentation = new PosePresentation();
    presentation.reconcile(pose(0), pose(delta), 0);
    assert.equal(presentation.sample(pose(delta), 0).x, delta <= 2048 ? 0 : delta);
    assert.equal(presentation.sample(pose(delta), 100).x, delta);
  }
});
