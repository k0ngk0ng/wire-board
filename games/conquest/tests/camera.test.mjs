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
  near(huge.scale, 4);
});
test("pan clamps map to bounds rather than exposing blank oceans outside canvas", () => {
  const c = zoomCamera(
    fitCamera(world, viewport),
    3,
    { x: 300, y: 200 },
    world,
    viewport,
  );
  const left = panCamera(c, -100000, -100000, world, viewport);
  near(left.x, viewport.width - world.width * left.scale);
  near(left.y, viewport.height - world.height * left.scale);
  const right = panCamera(c, 100000, 100000, world, viewport);
  near(right.x, 0);
  near(right.y, 0);
});
test("small map remains centred during pan and zero-sized layouts stay finite", () => {
  const c = fitCamera(world, viewport),
    p = panCamera(c, 50, 70, world, viewport);
  near(p.x, c.x);
  near(p.y, c.y);
  const zero = fitCamera(world, { width: 0, height: 0 });
  assert.ok(Object.values(zero).every(Number.isFinite));
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
