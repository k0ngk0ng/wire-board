import test from "node:test";
import assert from "node:assert/strict";
import { cameraForPlayer, pointerWorldPoint } from "../web/renderer.ts";

const viewport = { width: 576, height: 360 };

test("mouse aim stays under the screen pointer as the tracking camera moves", () => {
  const first = cameraForPlayer({ x: 320 * 256, y: 240 * 256 }, 640, 480, viewport);
  const next = cameraForPlayer({ x: 324 * 256, y: 240 * 256 }, 640, 480, viewport);
  const a = pointerWorldPoint(0.75, 0.5, first);
  const b = pointerWorldPoint(0.75, 0.5, next);
  assert.equal(a.x - 320, b.x - 324);
  assert.equal((a.x - first.x) / first.width, 0.75);
  assert.equal((b.x - next.x) / next.width, 0.75);
});

test("camera clamps at the original map bounds and preserves one combat viewport", () => {
  for (const [width, height] of [[640, 480], [800, 640], [1500, 640]]) {
    assert.deepEqual(cameraForPlayer({ x: 0, y: 0 }, width, height, viewport), { x: 0, y: 0, width: 576, height: 360 });
    assert.deepEqual(cameraForPlayer({ x: width * 256, y: height * 256 }, width, height, viewport), { x: width - 576, y: height - 360, width: 576, height: 360 });
  }
  assert.throws(() => cameraForPlayer({ x: 0, y: 0 }, 640, 480));
});
