import assert from "node:assert/strict";
import test from "node:test";
import { CLASSIC_CAMERA_MODES, CLASSIC_DRAW_DISTANCES, classicCameraPlan, classicMirrorViewport } from "../web/classic-camera-modes.ts";

// Before implementation: C must cycle genuinely different cameras; holding B
// must preserve the selected mode and releasing it must restore the same eye.
// Mounted views must use model-local coordinates and a short near plane; native
// cockpit eye data must not be replaced by a generic fake dashboard position.
// Mirror rectangles must remain inside narrow screens and exclude zero sizes.
const dimensions = [1.9, 1.3, 4.57], cockpitEye = [.37, 1.09, -.43];

test("four named modes provide distinct near/far chase and real mounted views", () => {
  assert.deepEqual(CLASSIC_CAMERA_MODES, ["近追车", "远追车", "引擎盖", "驾驶舱"]);
  const plans = CLASSIC_CAMERA_MODES.map((_, mode) => classicCameraPlan(mode, false, false, dimensions, cockpitEye));
  assert.equal(plans[0].kind, "chase");
  assert.equal(plans[1].kind, "chase");
  assert.ok(plans[1].distance > plans[0].distance * 1.3);
  assert.equal(plans[2].kind, "hood");
  assert.ok(plans[2].eye[2] > dimensions[2] * .25);
  assert.ok(plans[2].eye[1] > dimensions[1] * .5);
  assert.equal(plans[3].kind, "cockpit");
  assert.deepEqual(plans[3].eye, cockpitEye);
  assert.equal(plans[3].near, .05);
  assert.equal(plans[2].hideExterior, false);
  assert.equal(plans[3].hideExterior, true);
});

test("B is a reversible view direction for every persisted C mode", () => {
  for (let mode = 0; mode < CLASSIC_CAMERA_MODES.length; mode++) {
    const front = classicCameraPlan(mode, false, false, dimensions, cockpitEye);
    const rear = classicCameraPlan(mode, true, false, dimensions, cockpitEye);
    assert.equal(front.mode, mode);
    assert.equal(rear.mode, mode);
    assert.equal(rear.direction, -front.direction);
    assert.deepEqual(classicCameraPlan(mode, false, false, dimensions, cockpitEye), front);
    if (mode === 3) assert.deepEqual(rear.eye, cockpitEye);
    else if (mode === 2) assert.ok(rear.eye[2] < 0);
  }
});

test("invalid modes safely use near chase and drawing ranges remain explicit metres", () => {
  for (const mode of [undefined, -1, 4, Infinity, NaN, .5])
    assert.equal(classicCameraPlan(mode, false, false, dimensions, cockpitEye).mode, 0);
  assert.deepEqual(CLASSIC_DRAW_DISTANCES, [1200, 1800, 2400]);
  assert.throws(() => classicCameraPlan(3, false, false, dimensions), /驾驶舱/);
});

test("live rear-view inset has a bounded landscape rectangle even on a 320px screen", () => {
  for (const [width, height] of [[320,180],[320,568],[1366,768],[2560,1440]]) {
    const r = classicMirrorViewport(width, height);
    assert.ok(r.width > 0 && r.height > 0);
    assert.ok(r.x >= 0 && r.y >= 0 && r.x + r.width <= width && r.y + r.height <= height);
    assert.ok(r.width / r.height >= 2.5);
    assert.ok(r.width * r.height <= width * height * .12);
  }
  assert.equal(classicMirrorViewport(0, 0), null);
});
