import test from "node:test";
import assert from "node:assert/strict";
import { MotionPrediction } from "../web/prediction.ts";
import { aimAngle, preparePredictedInput } from "../web/camera-input.ts";
import { cameraForPlayer, pointerWorldPoint } from "../web/renderer.ts";
import { PosePresentation } from "../web/pose-presentation.ts";

const motion = (x, y, aim, mx, my, frames) => [x + Math.trunc(mx * 3 * 256 * frames / 1000), y + Math.trunc(my * 3 * 256 * frames / 1000), aim];
const viewport = { width: 576, height: 360 };

for (const dimensions of [{ width: 640, height: 480 }, { width: 800, height: 640 }]) {
  test(`camera-edge aim uses the final immediate movement pose at ${dimensions.width}px`, () => {
    const initial = { x: 289 * 256, y: 300 * 256, aim: 0, life: 1, alive: true, downUntil: 0, appliedSeq: 0 };
    const pointer = { x: 0.52, y: 0.25 };
    const beforePoint = pointerWorldPoint(pointer.x, pointer.y, cameraForPlayer(initial, dimensions.width, dimensions.height, viewport));
    const beforeAim = aimAngle(Math.round(beforePoint.x * 256) - initial.x, Math.round(beforePoint.y * 256) - initial.y);
    const prediction = new MotionPrediction(motion);
    prediction.confirm(initial, 0, 0, "epoch");
    const input = { type: "input", epoch: "epoch", seq: 1, life: 1, viewFrame: 0, moveX: -1000, moveY: 0, aim: beforeAim, fire: false, throw: false, pickup: false, finish: false, interact: false };
    preparePredictedInput(prediction, input, 8, pointer, dimensions, viewport);
    const shown = prediction.sample(8);
    assert.equal(shown.x, 286 * 256);
    assert.equal(prediction.predictedFrame, 1);
    const shownPoint = pointerWorldPoint(pointer.x, pointer.y, cameraForPlayer(shown, dimensions.width, dimensions.height, viewport));
    const expected = aimAngle(Math.round(shownPoint.x * 256) - shown.x, Math.round(shownPoint.y * 256) - shown.y);
    assert.ok(Math.abs(expected - beforeAim) > 2);
    assert.equal(input.aim, expected);
    assert.equal(shown.aim, expected);
  });
}

test("the transmitted aim follows the final displayed pose during a small correction", () => {
  const initial = { x: 289 * 256, y: 300 * 256, aim: 0, life: 1, alive: true, downUntil: 0, appliedSeq: 0 };
  const prediction = new MotionPrediction(motion);
  const presentation = new PosePresentation();
  prediction.confirm(initial, 0, 0, "epoch");
  presentation.reconcile({ ...initial, x: initial.x - 768 }, initial, 0);
  const input = { type: "input", epoch: "epoch", seq: 1, life: 1, viewFrame: 0, moveX: -1000, moveY: 0, aim: 0, fire: false, throw: false, pickup: false, finish: false, interact: false };
  const pointer = { x: 0.52, y: 0.25 };
  preparePredictedInput(prediction, input, 8, pointer, { width: 800, height: 640 }, viewport, (logical, at) => presentation.sample(logical, at));
  const logical = prediction.sample(8);
  const shown = presentation.sample(logical, 8);
  const point = pointerWorldPoint(pointer.x, pointer.y, cameraForPlayer(shown, 800, 640, viewport));
  const expected = aimAngle(Math.round(point.x * 256) - shown.x, Math.round(point.y * 256) - shown.y);
  assert.equal(logical.x, 286 * 256);
  assert.equal(input.aim, expected);
});

test("portrait aim shares the render camera zoom and safe-area offset after immediate movement", () => {
  const prediction = new MotionPrediction(motion);
  const initial = { x: 320 * 256, y: 300 * 256, aim: 0, life: 1, alive: true, downUntil: 0, appliedSeq: 0 };
  prediction.confirm(initial, 0, 0, "portrait");
  const input = { type: "input", epoch: "portrait", seq: 1, life: 1, viewFrame: 0, moveX: -1000, moveY: 0, aim: 0, fire: false, throw: false, pickup: false, finish: false, interact: false };
  const layout = { width: 320, height: 568, minimumScale: 2, insets: { top: 100, bottom: 112, left: 0, right: 0 } };
  preparePredictedInput(prediction, input, 8, { x: 0.75, y: 0.25 }, { width: 800, height: 640 }, viewport, undefined, layout);
  assert.equal(input.aim, aimAngle(40 * 256, -68 * 256));
  assert.equal(prediction.sample(8).aim, input.aim);
});
