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

test("phone and Duo cameras preserve square pixels and a readable painted body instead of scaling the whole combat view", () => {
  for (const [width, height] of [[320, 320], [320, 568], [320, 740], [890, 626], [626, 890], [820, 1180], [1180, 820]]) {
    const camera = cameraForPlayer({ x: 640 * 256, y: 400 * 256 }, 1500, 800, viewport, { width, height, minimumScale: 2, insets: { top: 96, bottom: 128, left: 0, right: 0 } });
    const sx = width / camera.width, sy = height / camera.height;
    assert.equal(Math.round(sx / sy * 1000), 1000);
    assert.ok(Math.round(13 * Math.min(sx, sy) * 1000) >= 24000);
    assert.ok(Math.round(camera.width * 1000) <= viewport.width * 1000 && Math.round(camera.height * 1000) <= viewport.height * 1000);
  }
});

test("safe camera padding keeps a real character body out of top HUD and bottom sticks at map corners", () => {
  const layout = { width: 320, height: 568, minimumScale: 2, insets: { top: 96, bottom: 128, left: 0, right: 0 } };
  for (const [x, y] of [[24, 24], [1476, 24], [24, 776], [1476, 776]]) {
    const camera = cameraForPlayer({ x: x * 256, y: y * 256 }, 1500, 800, viewport, layout);
    const sx = layout.width / camera.width, sy = layout.height / camera.height;
    const center = { x: (x - camera.x) * sx, y: (y - camera.y) * sy };
    assert.ok(Math.round(center.y - 12 * sy) >= layout.insets.top);
    assert.ok(Math.round(center.y + 12 * sy) <= layout.height - layout.insets.bottom);
    const target = pointerWorldPoint(center.x / layout.width, center.y / layout.height, camera);
    assert.equal(Math.round(target.x * 256), x * 256);
    assert.equal(Math.round(target.y * 256), y * 256);
  }
});
