import test from "node:test";
import assert from "node:assert/strict";
import { MotionPrediction } from "../web/prediction.ts";
import { aimAngle, preparePredictedInput } from "../web/camera-input.ts";
import { cameraForPlayer, pointerWorldPoint } from "../web/renderer.ts";

const motion = (x, y, aim, mx, my, frames) => [x + Math.trunc(mx * 3 * 256 * frames / 1000), y + Math.trunc(my * 3 * 256 * frames / 1000), aim];

for (const dimensions of [{ width: 640, height: 480 }, { width: 800, height: 640 }]) {
  test(`camera-edge aim uses the final immediate movement pose at ${dimensions.width}px`, () => {
    const initial = { x: 193 * 256, y: 300 * 256, aim: 0, life: 1, alive: true, downUntil: 0, appliedSeq: 0 };
    const pointer = { x: 0.52, y: 0.25 };
    const beforePoint = pointerWorldPoint(pointer.x, pointer.y, cameraForPlayer(initial, dimensions.width, dimensions.height));
    const beforeAim = aimAngle(Math.round(beforePoint.x * 256) - initial.x, Math.round(beforePoint.y * 256) - initial.y);
    const prediction = new MotionPrediction(motion);
    prediction.confirm(initial, 0, 0, "epoch");
    const input = { type: "input", epoch: "epoch", seq: 1, life: 1, viewFrame: 0, moveX: -1000, moveY: 0, aim: beforeAim, fire: false, throw: false, pickup: false, finish: false, interact: false };
    preparePredictedInput(prediction, input, 8, pointer, dimensions);
    const shown = prediction.sample(8);
    assert.equal(shown.x, 190 * 256);
    assert.equal(prediction.predictedFrame, 1);
    const shownPoint = pointerWorldPoint(pointer.x, pointer.y, cameraForPlayer(shown, dimensions.width, dimensions.height));
    const expected = aimAngle(Math.round(shownPoint.x * 256) - shown.x, Math.round(shownPoint.y * 256) - shown.y);
    assert.ok(Math.abs(expected - beforeAim) > 2);
    assert.equal(input.aim, expected);
    assert.equal(shown.aim, expected);
  });
}
