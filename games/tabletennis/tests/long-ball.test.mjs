import test from "node:test";
import assert from "node:assert/strict";
import * as Three from "../../../web/node_modules/three/build/three.module.js";
import { cameraSetup } from "../web/view-space.ts";
import { returnWindowOpen } from "../web/prediction.ts";
test("settled paddle can block immediately inside a short long-ball window", () => {
  const view = {
    phase: "rally",
    flight: { contactStart: 1000 },
    controls: { closeAt: 1070 },
  };
  assert.equal(
    returnWindowOpen(view, 1001, { moving: false, settled: true }),
    true,
  );
  assert.equal(
    returnWindowOpen(view, 999, { moving: true, settled: false }),
    false,
  );
  assert.equal(
    returnWindowOpen(view, 1071, { moving: true, settled: false }),
    false,
  );
  assert.equal(
    returnWindowOpen(view, 1020, { moving: false, settled: false }),
    false,
  );
  assert.equal(
    returnWindowOpen(view, 1020, {
      moving: false,
      settled: false,
      touching: true,
    }),
    true,
  );
});
test("near-court long balls remain in the PC and narrow-phone view", () => {
  for (const [w, h] of [
    [1120, 700],
    [1400, 850],
    [294, 562],
    [364, 624],
  ]) {
    const c = cameraSetup(w, h),
      camera = new Three.PerspectiveCamera(c.fov, w / h, 0.1, 100);
    camera.position.set(0, c.y, c.z);
    camera.lookAt(0, c.targetY, -1.1);
    camera.updateMatrixWorld();
    for (const x of [-0.76, 0, 0.76])
      for (const y of [0.78, 1.1, 1.5])
        for (const z of [1.37, 2.05, 2.25]) {
          const p = new Three.Vector3(x * 2.75, y * 2.75, z * 2.75).project(
            camera,
          );
          assert.ok(
            Math.abs(p.x) < 1 && Math.abs(p.y) < 1,
            JSON.stringify({ w, h, x, y, z, screen: p }),
          );
        }
  }
});
