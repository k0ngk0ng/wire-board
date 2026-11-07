// Failure inventory BEFORE implementation: quick direction changes queue stale
// steering; a slow request drops release; heartbeat starves or spams; reconnect,
// takeover or blur replays held controls; late acknowledgement unlocks a new
// room request; server seq moves forward and client duplicates an old seq.
import test from "node:test";
import assert from "node:assert/strict";
import { LatestDrive, neutral, wheelSteer } from "../web/input.ts";
const sync = (q, extra = {}) =>
  q.sync({
    scope: "r:epoch:0",
    epoch: "epoch",
    seq: 0,
    enabled: true,
    ...extra,
  });
test("one pending request retains only latest intent including release", () => {
  const q = new LatestDrive();
  sync(q);
  q.set({ ...neutral, throttle: 1, steer: -1 });
  const first = q.next(0);
  assert.equal(first.seq, 1);
  assert.equal(first.steer, -1);
  for (let i = 0; i < 30; i++)
    q.set({ ...neutral, throttle: 1, steer: i / 30 });
  assert.equal(q.next(1000), null);
  q.set(neutral);
  q.complete(first.token);
  const release = q.next(1000);
  assert.equal(release.seq, 2);
  assert.equal(release.throttle, 0);
  assert.equal(release.steer, 0);
  q.complete(release.token);
  assert.equal(q.next(1001), null);
});
test("10Hz changed input with 300ms heartbeat and immediate release", () => {
  const q = new LatestDrive();
  sync(q);
  q.set({ ...neutral, throttle: 1 });
  let r = q.next(0);
  q.complete(r.token);
  q.set({ ...neutral, throttle: 1, steer: 1 });
  assert.equal(q.next(99), null);
  r = q.next(100);
  assert.ok(r);
  q.complete(r.token);
  assert.equal(q.next(399), null);
  r = q.next(400);
  assert.ok(r);
  q.complete(r.token);
  q.set(neutral);
  r = q.next(401);
  assert.ok(r);
  q.complete(r.token);
  assert.equal(q.next(402), null);
});
test("room changes, disabled controls and restored epochs clear held input", () => {
  const q = new LatestDrive();
  sync(q);
  q.set({ ...neutral, throttle: 1, drift: true });
  const old = q.next(0);
  sync(q, { scope: "new:epoch:1", seq: 12 });
  assert.deepEqual(q.value, neutral);
  q.set({ ...neutral, steer: 1 });
  const fresh = q.next(1);
  assert.equal(fresh.seq, 13);
  q.complete(old.token);
  assert.equal(q.next(1000), null);
  q.complete(fresh.token);
  sync(q, { scope: "new:epoch:1", seq: 13, enabled: false });
  q.set({ ...neutral, throttle: 1 });
  assert.deepEqual(q.value, neutral);
  assert.equal(q.next(2000), null);
});
test("accepted server sequence and invalid numeric controls are normalized", () => {
  const q = new LatestDrive();
  sync(q, { seq: 99 });
  q.set({ steer: Infinity, throttle: NaN, brake: 0, drift: 1 });
  const r = q.next(0);
  assert.equal(r.seq, 100);
  assert.equal(r.steer, 0);
  assert.equal(r.throttle, 0);
  assert.equal(r.brake, false);
  assert.equal(r.drift, true);
  q.complete(r.token);
  sync(q, { seq: 111 });
  q.set({ ...neutral, steer: -9, throttle: 8 });
  const next = q.next(100);
  assert.equal(next.seq, 112);
  assert.equal(next.steer, -1);
  assert.equal(next.throttle, 1);
});
test("wheel absolute position remains bounded outside pointer capture and has a neutral deadzone", () => {
  assert.equal(wheelSteer(50, 0, 100), 0);
  assert.equal(wheelSteer(51, 0, 100), 0);
  assert.equal(wheelSteer(-100, 0, 100), -1);
  assert.equal(wheelSteer(200, 0, 100), 1);
  assert.ok(wheelSteer(75, 0, 100) > 0.4);
});
test("released foreground controls do not heartbeat away the server idle takeover", () => {
  const q = new LatestDrive();
  sync(q);
  const r = q.next(0);
  q.complete(r.token);
  assert.equal(q.next(16000), null);
});
test("temporary control revocation cannot reuse an in-flight sequence", () => {
  const q = new LatestDrive();
  sync(q);
  q.set({ ...neutral, throttle: 1 });
  const old = q.next(0);
  sync(q, { enabled: false });
  sync(q, { enabled: true });
  q.set({ ...neutral, steer: 1 });
  const next = q.next(100);
  assert.ok(next.seq > old.seq);
  q.complete(old.token);
  assert.equal(q.next(1000), null);
});
test("transport epoch reset clears controls without reusing a delayed race input sequence", () => {
  const q = new LatestDrive();
  sync(q, { identity: "room:race:seat" });
  q.set({ ...neutral, throttle: 1 });
  const old = q.next(0);
  sync(q, { scope: "room:epoch:1", identity: "room:race:seat", seq: 0 });
  const release = q.next(1);
  assert.equal(release.throttle, 0);
  assert.ok(release.seq > old.seq);
  q.complete(old.token);
  assert.equal(q.next(400), null);
  q.complete(release.token);
});

test('brake pressure is retained and changes produce new input packets',()=>{
 const q=new LatestDrive();sync(q);q.set({...neutral,brake:true,brakeAmount:.25});const a=q.next(0);assert.equal(a.brakeAmount,.25);q.complete(a.token);
 q.set({...neutral,brake:true,brakeAmount:.75});const b=q.next(120);assert.equal(b.brakeAmount,.75);q.complete(b.token);
 q.set({...neutral,brake:true});assert.equal(q.value.brakeAmount,1);q.set(neutral);assert.equal(q.value.brakeAmount,0);
});
