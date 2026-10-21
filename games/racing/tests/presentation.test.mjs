// Failure inventory BEFORE implementation: zoom drifts away from cursor; pan
// moves in the wrong coordinate system; long background gaps extrapolate cars
// through walls; interpolation mutates authoritative laps/ranks or takes the
// long path across the ±π angle boundary; respawns sweep across the whole map.
import test from "node:test";
import assert from "node:assert/strict";
import {
  zoomAt,
  screenToWorld,
  panBy,
  interpolateCar,
  extrapolateCar,
} from "../web/presentation.ts";
const car = (extra = {}) => ({
  seat: 0,
  x: 100,
  y: 100,
  vx: 100,
  vy: 0,
  angle: 3.1,
  speed: 100,
  steer: 0,
  throttle: 1,
  brake: false,
  drift: false,
  skin: 0,
  seq: 1,
  lap: 2,
  checkpoint: 8,
  progress: 900,
  rank: 1,
  finishAt: 0,
  offroad: false,
  lastInputAt: 0,
  idleAt: 0,
  resetUntil: 0,
  ...extra,
});
test("zoom is anchored at the native pointer and pan uses world coordinates", () => {
  const camera = { x: 500, y: 300, scale: 1 },
    view = { width: 640, height: 400 },
    mouse = { x: 390, y: 140 };
  const before = screenToWorld(camera, view, mouse),
    next = zoomAt(camera, view, mouse, 2);
  assert.deepEqual(screenToWorld(next, view, mouse), before);
  assert.deepEqual(panBy(camera, 40, -20), { x: 460, y: 320, scale: 1 });
  assert.equal(zoomAt(next, view, mouse, 100).scale, 2.4);
});
test("interpolation keeps authoritative game fields and shortest rotation without mutation", () => {
  const a = car(),
    b = car({ x: 120, angle: -3.1, lap: 3, rank: 2 });
  const frozen = JSON.stringify([a, b]);
  const midpoint = interpolateCar(a, b, 0.5);
  assert.equal(midpoint.x, 110);
  assert.ok(Math.abs(midpoint.angle) > 3.1);
  assert.equal(midpoint.lap, 3);
  assert.equal(midpoint.rank, 2);
  assert.equal(JSON.stringify([a, b]), frozen);
});
test("remote extrapolation is bounded and finish or rescue never invents motion", () => {
  assert.equal(extrapolateCar(car(), 2000).x, 125);
  assert.equal(extrapolateCar(car({ finishAt: 100 }), 1000).x, 100);
  const a = car(),
    b = car({ x: 900, y: 500, resetUntil: 2000 });
  assert.equal(interpolateCar(a, b, 0.5).x, 900);
});

test('long circuits can zoom out to overview without losing the cursor anchor',()=>{
 const camera={x:3100,y:2200,scale:.15},view={width:1000,height:600},pointer={x:760,y:240};
 const before=screenToWorld(camera,view,pointer),after=zoomAt(camera,view,pointer,.5);
 assert.ok(after.scale<.15);assert.deepEqual(screenToWorld(after,view,pointer),before);
});
