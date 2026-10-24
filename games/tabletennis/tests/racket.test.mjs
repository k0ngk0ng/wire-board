// Specified before client spatial controls: screen/world mismatch, reversed
// placement, device-dependent reach, and visual prediction of a missed ball.
import test from "node:test";
import assert from "node:assert/strict";
import * as Three from "../../../web/node_modules/three/build/three.module.js";
import { cameraSetup, racketFromScreen } from "../web/view-space.ts";
import { canReach, BallDisplay } from "../web/prediction.ts";
import { sampleFlight } from "../web/timeline.ts";
const view = {
  options: { teams: [0, 1] },
  ends: 0,
  epoch: "one",
  rally: 1,
  shot: 1,
  phase: "rally",
  flight: {
    contactStart: 1000,
    contactEnd: 1600,
    segments: [
      {
        at: 1000,
        until: 2000,
        p: { x: 0.6, y: 1.1, z: 0.9 },
        v: { x: 0, y: 0, z: 0 },
        a: { x: 0, y: 0, z: 0 },
      },
    ],
  },
};
test("finger X maps to the actual Three.js projection on desktop and narrow phones", () => {
  for (const [w, h] of [
    [294, 562],
    [364, 624],
    [1120, 700],
    [1424, 850],
  ]) {
    const c = cameraSetup(w, h),
      camera = new Three.PerspectiveCamera(c.fov, w / h, 0.1, 100);
    camera.position.set(0, c.y, c.z);
    camera.lookAt(0, c.targetY, -1.1);
    camera.updateMatrixWorld();
    for (const x of [-0.7, -0.3, 0, 0.3, 0.7]) {
      const point = { x, y: 1.1, z: 0.9 };
      const shown = new Three.Vector3(
        x * 2.75,
        point.y * 2.75,
        point.z * 2.75,
      ).project(camera);
      assert.ok(
        Math.abs(racketFromScreen(shown.x, point, w, h) - x / 0.9) < 1e-8,
      );
    }
  }
});
test("light assistance is bounded and far-side placement is mirrored", () => {
  assert.equal(canReach(view, 0, { racketX: -0.7 }, 1130), false);
  assert.equal(canReach(view, 0, { racketX: 0.45 }, 1130), true);
  const mirrored = {
    ...view,
    ends: 1,
    flight: {
      ...view.flight,
      segments: [
        { ...view.flight.segments[0], p: { x: -0.6, y: 1.1, z: -0.9 } },
      ],
    },
  };
  assert.equal(canReach(mirrored, 0, { racketX: 0.45 }, 1130), true);
  assert.equal(canReach(mirrored, 0, { racketX: -0.7 }, 1130), false);
});
test("a whiff leaves the incoming ball intact instead of predicting a return", () => {
  const d = new BallDisplay();
  d.start(
    view,
    0,
    { kind: "drive", aim: 0, power: 0.5, racketX: -0.7 },
    1130,
    "same",
  );
  assert.equal(d.active, false);
  assert.deepEqual(
    d.sample(view, 1200, "same"),
    sampleFlight(view.flight, 1200),
  );
});
