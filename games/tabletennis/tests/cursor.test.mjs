import test from "node:test";
import assert from "node:assert/strict";
import * as Three from "../../../web/node_modules/three/build/three.module.js";
import { cameraSetup, paddleAtScreen } from "../web/view-space.ts";
import { canReach } from "../web/prediction.ts";

test("paddle centre follows both screen coordinates including edges and serve depth", () => {
  for (const [w, h] of [
    [294, 562],
    [364, 624],
    [1120, 700],
    [1400, 850],
  ]) {
    const c = cameraSetup(w, h),
      camera = new Three.PerspectiveCamera(c.fov, w / h, 0.1, 100);
    camera.position.set(0, c.y, c.z);
    camera.lookAt(0, c.targetY, -1.1);
    camera.updateMatrixWorld();
    for (const z of [0.5, 1.28, 1.7])
      for (const x of [0.02, 0.3, 0.7, 0.98])
        for (const y of [0.02, 0.25, 0.75, 0.98]) {
          const p = paddleAtScreen({ x, y }, z, w, h),
            q = new Three.Vector3(p.x * 2.75, p.y * 2.75, p.z * 2.75).project(
              camera,
            );
          assert.ok(Math.abs((q.x + 1) / 2 - x) * w < 0.01);
          assert.ok(Math.abs((1 - q.y) / 2 - y) * h < 0.01);
        }
  }
});
test("high paddle cannot hit a low ball merely by matching horizontal position", () => {
  const v = {
    phase: "rally",
    ends: 0,
    options: { teams: [0, 1] },
    flight: {
      contactStart: 0,
      segments: [
        {
          at: 0,
          until: 500,
          p: { x: 0.1, y: 0.8, z: 1.2 },
          v: { x: 0, y: 0, z: 0 },
          a: { x: 0, y: 0, z: 0 },
        },
      ],
    },
  };
  assert.equal(
    canReach(v, 0, { racketX: 0.1 / 0.9, racketY: 1.5 }, 100),
    false,
  );
  assert.equal(
    canReach(v, 0, { racketX: 0.1 / 0.9, racketY: 0.88 }, 100),
    true,
  );
});
