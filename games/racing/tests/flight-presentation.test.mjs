import test from "node:test";
import assert from "node:assert/strict";
import { classicSkidStrength } from "../web/classic-skids.ts";

test("airborne braking and sliding cannot draw tyre contact marks", () => {
  const car = { vx: 600, vy: 80, angle: 0, speed: 605, brake: true, drift: true, offroad: false, finishAt: 0, resetUntil: 0, airborne: true, motion: { brake: 1, handbrake: 1, longAccel: -10 } };
  assert.deepEqual(classicSkidStrength(car, 1000), { front: 0, rear: 0, cause: "none" });
  assert(classicSkidStrength({ ...car, airborne: false }, 1000).rear > .5);
});
