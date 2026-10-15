// Failure inventory BEFORE implementation: a reconnected mobile tab silently
// resumes auto-throttle; changing autoplay/busy clears a blur pause; refreshing
// a race moves the car before any gesture; a genuinely new race never starts.
import test from "node:test";
import assert from "node:assert/strict";
import { ControlLifecycle } from "../web/lifecycle.ts";
const state = (extra = {}) => ({
  race: "room:race:0",
  transport: 0,
  enabled: true,
  hidden: false,
  phase: "countdown",
  ...extra,
});
test("new countdown may prepare automatic acceleration, ongoing restored race waits for a gesture", () => {
  assert.equal(new ControlLifecycle().sync(state()), false);
  assert.equal(new ControlLifecycle().sync(state({ phase: "racing" })), true);
});
test("blur, visibility and reconnect preserve release until explicit wake", () => {
  const c = new ControlLifecycle();
  c.sync(state());
  c.suspend();
  assert.equal(c.sync(state({ phase: "racing" })), true);
  assert.equal(c.sync(state({ transport: 1, phase: "racing" })), true);
  assert.equal(c.wake(), false);
  assert.equal(c.sync(state({ transport: 2, phase: "racing" })), true);
  c.sync(state({ transport: 2, phase: "racing", hidden: true }));
  assert.equal(c.wake(), true);
  assert.equal(c.sync(state({ transport: 2, phase: "racing" })), true);
  assert.equal(c.wake(), false);
});
test("taking over and busy changes never reapply held auto-throttle, a new countdown can prepare", () => {
  const c = new ControlLifecycle();
  c.sync(state());
  c.sync(state({ enabled: false }));
  assert.equal(c.sync(state({ phase: "racing" })), true);
  assert.equal(c.sync(state({ race: "room:race:1" })), false);
});
// The real offline E2E leaves WebSocket epoch unchanged while HTTP drive fails.
// A restored snapshot must not act as a fresh pedal gesture.
test("HTTP transmission failure latches pause without a WebSocket epoch change", () => {
  const c = new ControlLifecycle();
  c.sync(state({ snapshotAge: 0 }));
  assert.equal(c.wake(), false);
  c.networkFailed();
  assert.equal(c.sync(state({ phase: "racing", snapshotAge: 25 })), true);
  assert.equal(c.sync(state({ phase: "racing", snapshotAge: 0 })), true);
  assert.equal(c.wake(), false);
});
test("stalled snapshots release controls; normal 180ms latency does not pause and recovery requires a gesture", () => {
  const c = new ControlLifecycle();
  assert.equal(c.sync(state({ snapshotAge: 180 })), false);
  assert.equal(c.sync(state({ snapshotAge: 1001 })), true);
  assert.equal(c.wake(), true);
  assert.equal(c.sync(state({ snapshotAge: 180 })), true);
  assert.equal(c.wake(), false);
});
