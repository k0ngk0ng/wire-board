import test from "node:test";
import assert from "node:assert/strict";
import { PaddleStream, RemotePaddles } from "../web/paddle-stream.ts";
test("movement coalesces independently of strikes and discards stale identity", async () => {
  const sent = [];
  let release;
  const s = new PaddleStream((a) => {
    sent.push(a);
    return new Promise((r) => {
      release = r;
    });
  });
  s.update("one", "epoch", 0, 0);
  s.offer({ x: 0, y: 0.8, z: 1.2 }, 1000);
  assert.equal(s.flush(1000), true);
  s.offer({ x: 0.2, y: 0.9, z: 1.2 }, 1020);
  s.offer({ x: 0.5, y: 1, z: 1.2 }, 1040);
  assert.equal(s.flush(1100), false);
  release();
  await Promise.resolve();
  assert.equal(s.flush(1100), true);
  assert.equal(sent[1].racketX, 0.5);
  assert.equal(sent[1].motionSeq, 2);
  s.update("two", "epoch2", 1, 4);
  s.offer({ x: -0.3, y: 1, z: 1.3 }, 1200);
  release();
  await Promise.resolve();
  assert.equal(s.flush(1200), true);
  assert.equal(sent[2].epoch, "epoch2");
  assert.equal(sent[2].motionSeq, 5);
  s.clear();
  assert.equal(s.flush(1300), false);
});
test("remote tracks interpolate actual preparation before a hit and stop extrapolating", () => {
  const r = new RemotePaddles();
  r.update(
    "one",
    [{ x: -0.5, y: 0.6, z: 1.2, motionAt: 1000, motionSeq: 1 }],
    1000,
  );
  r.update(
    "one",
    [{ x: 0.5, y: 1.2, z: 1.1, motionAt: 1100, motionSeq: 2 }],
    1100,
  );
  const p = r.sample(0, 1050);
  assert.ok(Math.abs(p.x) < 1e-8);
  assert.ok(Math.abs(p.y - 0.9) < 1e-8);
  r.update("one", [{ x: -0.9, y: 3, z: 1, motionAt: 900, motionSeq: 0 }], 1100);
  assert.deepEqual(r.sample(0, 1050), p);
  assert.deepEqual(r.sample(0, 1500), r.sample(0, 3000));
  r.update(
    "two",
    [{ x: 0, y: 0.8, z: 1.7, motionAt: 2000, motionSeq: 0 }],
    2000,
  );
  assert.equal(r.sample(0, 2000).x, 0);
});
test("returning to the last sent point discards an unsent detour", async () => {
  let release;
  const sent = [];
  const s = new PaddleStream((a) => {
    sent.push(a);
    return new Promise((r) => (release = r));
  });
  s.update("one", "epoch", 0, 0);
  s.offer({ x: 0, y: 1, z: 1.2 }, 1000);
  s.flush(1000);
  s.offer({ x: 1, y: 1, z: 1.2 }, 1010);
  s.offer({ x: 0, y: 1, z: 1.2 }, 1020);
  release();
  await Promise.resolve();
  assert.equal(s.flush(1100), false);
  assert.equal(sent.length, 1);
});
