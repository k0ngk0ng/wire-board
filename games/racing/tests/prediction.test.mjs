// Failure inventory BEFORE implementation: local steering waits for network;
// long network gaps let prediction escape the authoritative frame; drift and
// braking ignored; local extrapolation writes lap/rank; cars move in countdown,
// after finish or while reset; road rendering and offroad prediction disagree.
import test from "node:test";
import assert from "node:assert/strict";
import { predictCar, roadDistance } from "../web/prediction.ts";
const track = {
  width: 1000,
  height: 1000,
  roadWidth: 160,
  points: [
    { x: 100, y: 100 },
    { x: 900, y: 100 },
    { x: 900, y: 900 },
    { x: 100, y: 900 },
  ],
};
const c = {
  seat: 0,
  x: 300,
  y: 100,
  vx: 150,
  vy: 0,
  angle: 0,
  speed: 150,
  steer: 0,
  throttle: 1,
  brake: false,
  drift: false,
  skin: 0,
  seq: 1,
  lap: 2,
  checkpoint: 1,
  progress: 200,
  rank: 2,
  finishAt: 0,
  offroad: false,
  lastInputAt: 0,
  idleAt: 0,
  resetUntil: 0,
};
const input = { steer: 0, throttle: 1, brake: false, drift: false };
test("prediction reacts immediately yet leaves all scoring and server inputs unchanged", () => {
  const original = JSON.stringify(c),
    n = predictCar(c, { ...input, steer: 1 }, track, 100, 1000);
  assert.ok(n.x > c.x);
  assert.ok(n.angle > 0.1);
  assert.equal(n.lap, 2);
  assert.equal(n.rank, 2);
  assert.equal(n.seq, 1);
  assert.equal(JSON.stringify(c), original);
});
test("250ms cap prevents a long outage predicting a whole race; braking slows and reset freezes", () => {
  assert.deepEqual(
    predictCar(c, input, track, 9000, 1000),
    predictCar(c, input, track, 250, 1000),
  );
  assert.ok(
    predictCar(c, { ...input, throttle: 0, brake: true }, track, 200, 1000)
      .speed < c.speed,
  );
  assert.equal(
    predictCar({ ...c, finishAt: 500 }, input, track, 200, 1000).x,
    c.x,
  );
  assert.equal(
    predictCar({ ...c, resetUntil: 2000 }, input, track, 200, 1000).x,
    c.x,
  );
});
test("closed road distance matches straight sections including last to first", () => {
  assert.equal(roadDistance(track, { x: 100, y: 300 }), 0);
  assert.equal(roadDistance(track, { x: 500, y: 500 }), 400);
  const grass = predictCar({ ...c, y: 400 }, input, track, 200, 1000);
  assert.equal(grass.offroad, true);
  assert.ok(grass.speed <= 110.01);
});
test("offroad sideways velocity is capped and reverse impacts do not produce reverse throttle", () => {
  const grass = predictCar(
    { ...c, y: 400, vx: 90, vy: 100 },
    { ...input, drift: true },
    track,
    20,
    1000,
  );
  assert.ok(grass.speed <= 110.00001);
  const backwards = predictCar(
    { ...c, vx: -100 },
    { ...input, throttle: 0 },
    track,
    20,
    1000,
  );
  assert.ok(backwards.vx >= 0);
});
