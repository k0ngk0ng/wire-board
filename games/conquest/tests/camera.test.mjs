import test from "node:test";
import assert from "node:assert/strict";
import {
  fitCamera,
  zoomCamera,
  panCamera,
  mapPoint,
  viewPoint,
  locateCamera,
  pinchCamera,
  focusCamera,
  visibleCameraRect,
} from "../web/camera.ts";

const near = (a, b) => assert.ok(Math.abs(a - b) < 0.001, `${a} != ${b}`);
const world = { width: 1200, height: 680 };
const viewport = { width: 600, height: 400 };

test("fit preserves geography aspect and centres the entire map", () => {
  const c = fitCamera(world, viewport);
  near(c.scale, 0.5);
  near(c.x, 0);
  near(c.y, 30);
  const a = viewPoint(c, { x: 0, y: 0 }),
    b = viewPoint(c, { x: 1200, y: 680 });
  near(a.x, 0);
  near(b.x, 600);
  near(b.y - a.y, 340);
});
test("wheel zoom preserves an off-centre mouse anchor", () => {
  const c = fitCamera(world, viewport),
    anchor = { x: 175, y: 125 };
  const before = mapPoint(c, anchor),
    next = zoomCamera(c, 2, anchor, world, viewport);
  const after = mapPoint(next, anchor);
  near(before.x, after.x);
  near(before.y, after.y);
});
test("zoom minimum and maximum are bounded for huge wheel inputs", () => {
  const c = fitCamera(world, viewport);
  const small = zoomCamera(c, 0.0001, { x: 300, y: 200 }, world, viewport);
  const huge = zoomCamera(c, 100000, { x: 300, y: 200 }, world, viewport);
  near(small.scale, 0.5);
  near(huge.scale, 16);
});
test("320px viewports can magnify small real map regions beyond the old eight-times limit", () => {
  const phone = { width: 320, height: 480 };
  const initial = fitCamera(world, phone);
  const enlarged = zoomCamera(initial, 10000, { x: 160, y: 240 }, world, phone);
  near(enlarged.scale / initial.scale, 32);
  const pinch = pinchCamera(
    initial,
    { a: { x: 140, y: 220 }, b: { x: 180, y: 220 } },
    { a: { x: -5000, y: 220 }, b: { x: 5000, y: 220 } },
    world,
    phone,
  );
  near(pinch.scale, enlarged.scale);
});
test("pan has bounded quarter-viewport padding without losing the map", () => {
  const c = zoomCamera(
    fitCamera(world, viewport),
    3,
    { x: 300, y: 200 },
    world,
    viewport,
  );
  const left = panCamera(c, -100000, -100000, world, viewport);
  near(
    left.x,
    viewport.width - world.width * left.scale - viewport.width * 0.25,
  );
  near(
    left.y,
    viewport.height - world.height * left.scale - viewport.height * 0.25,
  );
  const right = panCamera(c, 100000, 100000, world, viewport);
  near(right.x, viewport.width * 0.25);
  near(right.y, viewport.height * 0.25);
  for (const moved of [left, right]) {
    const rect = visibleCameraRect(moved, world, viewport);
    assert.ok(rect.width > 0 && rect.height > 0);
  }
});
test("initial full-map fit pans immediately, reset is centred, and zero layouts stay finite", () => {
  const c = fitCamera(world, viewport),
    p = panCamera(c, 50, 70, world, viewport);
  near(p.x, c.x + 50);
  near(p.y, c.y + 70);
  const reset = fitCamera(world, viewport);
  near(reset.x, 0);
  near(reset.y, 30);
  const zero = fitCamera(world, { width: 0, height: 0 });
  assert.ok(Object.values(zero).every(Number.isFinite));
});
test("minimap rect clips the actual visible intersection after fit-state pan", () => {
  const c = fitCamera(world, viewport);
  const initial = visibleCameraRect(c, world, viewport);
  assert.deepEqual(initial, { x: 0, y: 0, width: 1200, height: 680 });
  const right = visibleCameraRect(
    panCamera(c, 150, 100, world, viewport),
    world,
    viewport,
  );
  near(right.x, 0);
  near(right.width, 900);
  near(right.y, 0);
  near(right.height, 540);
  const left = visibleCameraRect(
    panCamera(c, -150, -100, world, viewport),
    world,
    viewport,
  );
  near(left.x, 300);
  near(left.width, 900);
  assert.ok(left.height > 0 && left.height <= 680);
});
test("zoom keeps a visible point anchored after a full-map drag", () => {
  const c = panCamera(fitCamera(world, viewport), 75, 35, world, viewport),
    anchor = { x: 200, y: 200 };
  const before = mapPoint(c, anchor),
    after = mapPoint(zoomCamera(c, 1.4, anchor, world, viewport), anchor);
  near(before.x, after.x);
  near(before.y, after.y);
});
test("minimap positioning centres an interior geographic point", () => {
  const c = zoomCamera(
    fitCamera(world, viewport),
    4,
    { x: 300, y: 200 },
    world,
    viewport,
  );
  const moved = locateCamera(c, { x: 550, y: 300 }, world, viewport);
  const centre = viewPoint(moved, { x: 550, y: 300 });
  near(centre.x, 300);
  near(centre.y, 200);
});
test("pinch keeps starting midpoint geography under moving midpoint", () => {
  const c = zoomCamera(
    fitCamera(world, viewport),
    2,
    { x: 300, y: 200 },
    world,
    viewport,
  );
  const start = { a: { x: 200, y: 150 }, b: { x: 400, y: 250 } },
    end = { a: { x: 170, y: 140 }, b: { x: 430, y: 280 } };
  const mid = { x: 300, y: 200 },
    newMid = { x: 300, y: 210 };
  const p = mapPoint(c, mid),
    next = pinchCamera(c, start, end, world, viewport),
    q = mapPoint(next, newMid);
  near(p.x, q.x);
  near(p.y, q.y);
});
test("coincident fingers cannot produce Infinity or an inverted camera", () => {
  const c = fitCamera(world, viewport),
    same = { a: { x: 100, y: 100 }, b: { x: 100, y: 100 } };
  const next = pinchCamera(c, same, same, world, viewport);
  assert.ok(Object.values(next).every(Number.isFinite));
  assert.ok(next.scale > 0);
});

// Before the mobile initial-focus refinement: wide world maps remained a thin
// horizontal strip, while resetting still needs to retain a complete overview.
test("focused world and Europe views cover a tall phone viewport", () => {
  const phone = { width: 294, height: 600 };
  for (const map of [
    { width: 1440, height: 720 },
    { width: 1120, height: 980 },
  ]) {
    const fit = fitCamera(map, phone),
      target = { x: map.width / 2, y: map.height / 2 };
    const focused = focusCamera(fit, target, map, phone);
    near(
      focused.scale,
      Math.max(fit.scale * 2.5, (phone.height / map.height) * 1.05),
    );
    assert.ok(map.height * focused.scale >= phone.height);
    const centre = viewPoint(focused, target);
    near(centre.x, phone.width / 2);
    near(centre.y, phone.height / 2);
    const overview = fitCamera(map, phone);
    near(overview.scale, fit.scale);
  }
});
test("focus preserves a player's closer zoom and respects the global maximum", () => {
  const phone = { width: 320, height: 600 },
    fit = fitCamera(world, phone);
  const close = zoomCamera(fit, 10, { x: 160, y: 300 }, world, phone);
  near(focusCamera(close, { x: 500, y: 300 }, world, phone).scale, close.scale);
  const narrow = { width: 32, height: 4000 },
    overview = fitCamera(world, narrow);
  near(
    focusCamera(overview, { x: 500, y: 300 }, world, narrow).scale,
    overview.scale * 32,
  );
});
