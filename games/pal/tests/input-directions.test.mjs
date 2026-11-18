// Test first. These deterministic input-boundary checks complement the actual
// original-core and browser E2E. Failures: an opposite held key emits a movement
// pulse; releasing it does not immediately restore the remaining direction;
// diagonal edges contain competing directions; releasing one source cancels
// another; held field input repeats key edges; disabled input never releases.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
const { transformSync } = createRequire(new URL("../../../web/package.json", import.meta.url))("esbuild");
const source = readFileSync(new URL("../web/input.ts", import.meta.url), "utf8");
const { code } = transformSync(source, { loader: "ts", format: "esm", target: "es2022" });
const { PalInput } = await import("data:text/javascript;base64," + Buffer.from(code).toString("base64"));

function fixture({ window = 8, acknowledge = async () => {} } = {}) {
  let held = new Set(), enabled = true;
  const actions = [], errors = [];
  const props = {
    data: { epoch: 1, seq: 0, discardedSeq: 0, controlAt: 100000, phase: "field", leader: 0 },
    room: { you: 0, status: "playing", seats: [{}] },
    inputWindow: window,
    act: async action => { actions.push(action); await acknowledge(); },
  };
  const input = new PalInput(() => props, () => enabled, message => errors.push(message));
  input.controller = {
    cancelVersion: 0,
    sample: () => new Set(held),
    clear: () => held.clear(),
  };
  input.update(0);
  return {
    actions, errors, input, props,
    sample(buttons, at) { held = new Set(buttons); input.sample(at); },
    disable() { enabled = false; },
  };
}

test("opposite directions stop without an unwanted movement edge and restore immediately", () => {
  const f = fixture();
  f.sample(["up"], 10);
  f.sample(["up", "down"], 20);
  assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4, 16], [4, 0]]);
  f.sample(["up"], 30);
  assert.deepEqual([f.actions.at(-1).direction, f.actions.at(-1).keys], [4, 16]);
  f.input.dispose();
});

// Transport lifecycle failures fixed before implementation: a queued action
// is stamped when drained instead of captured; stalled snapshots keep renewing
// held levels; clearing/rejecting an input loses its neutral release; transient
// sync failures turn into a permanent Board error; autoplay blocks releases.
test("queued physical actions retain their original authoritative clock", async () => {
  let acknowledge;
  const f = fixture({ window: 1, acknowledge: () => new Promise(resolve => { acknowledge = resolve; }) });
  try {
    f.sample(["down"], 10); f.sample([], 20);
    f.props.data.controlAt = 100100; f.input.update(100);
    assert.equal(f.actions[0].controlAt, 100000);
    acknowledge(); await new Promise(resolve => setImmediate(resolve));
    assert.equal(f.actions[1].controlAt, 100000, "queued release was re-stamped with a fresh clock");
  } finally { f.input.dispose(); acknowledge?.(); }
});

test("stalled authoritative clock clears physical controls and sends one neutral release", () => {
  const f = fixture(); f.sample(["down"], 10); f.sample(["down"], 120);
  f.sample(["down"], 600); f.sample(["down"], 700);
  assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4,4],[0,0],[4,0]]);
  f.props.data.controlAt = 100700;
  f.sample([], 710); f.sample(["up"], 720);
  assert.deepEqual([f.actions.at(-1).direction,f.actions.at(-1).keys],[4,16]);
  f.input.dispose();
});

test("discard feedback removes queued held input but preserves its release", async () => {
  let acknowledge;
  const f = fixture({ window: 1, acknowledge: () => new Promise(resolve => { acknowledge = resolve; }) });
  try {
    f.sample(["down"], 10); f.sample(["down"], 120);
    f.props.data.seq = 2; f.props.data.discardedSeq = 2; f.props.data.controlAt = 100200;
    f.input.update(200);
    acknowledge(); await new Promise(resolve => setImmediate(resolve));
    assert.equal(f.actions.length, 2);
    assert.deepEqual([f.actions[1].seq,f.actions[1].direction,f.actions[1].keys],[3,4,0]);
    assert.deepEqual(f.errors, []);
  } finally { f.input.dispose(); acknowledge?.(); }
});

test("transient input rejection clears controls without a permanent error overlay", async () => {
  const f = fixture({ acknowledge: async () => { throw Object.assign(new Error(""),{transientSync:true}); } });
  f.sample(["down"], 10);
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(f.errors, []);
  assert.deepEqual(f.actions.map(a => [a.direction,a.keys]),[[4,4],[4,0]]);
  f.input.dispose();
});

test("explicit control clear and autoplay still release the previous human direction", () => {
  const f=fixture();f.sample(["right"],10);f.sample(["right"],120);
  f.props.room.seats[0].autoPlay=true;f.disable();f.sample(["right"],130);
  assert.deepEqual([f.actions.at(-1).direction,f.actions.at(-1).keys],[4,0]);
  const count=f.actions.length;f.input.clear();assert.equal(f.actions.length,count);
  f.input.dispose();
});

test("controller cancellation sends a release even after clearing the previous key set", () => {
  const f=fixture();f.sample(["right"],10);f.sample(["right"],120);
  f.input.controller.cancelVersion++;
  f.sample([],130);
  assert.deepEqual([f.actions.at(-1).direction,f.actions.at(-1).keys],[4,0]);
  f.input.dispose();
});

test("opposite held directions release once without a neutral packet every frame", () => {
  const f=fixture();f.sample(["up"],10);f.sample(["up","down"],20);f.sample(["up","down"],30);
  assert.deepEqual(f.actions.map(a=>[a.direction,a.keys]),[[4,16],[4,0]]);
  f.input.dispose();
});

test("one selected direction represents simultaneous diagonal edges", () => {
  const f = fixture();
  f.sample(["up", "right"], 10);
  assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4, 32]]);
  f.input.dispose();
});

test("opposing touch and keyboard release only their own input source", () => {
  const f = fixture();
  f.sample(["right"], performance.now());
  f.input.pressPointer(9, ["left"]);
  assert.deepEqual([f.actions.at(-1).direction, f.actions.at(-1).keys], [4, 0]);
  f.input.releasePointer(9, false);
  assert.deepEqual([f.actions.at(-1).direction, f.actions.at(-1).keys], [4, 32]);
  assert.deepEqual(f.errors, []);
  f.input.dispose();
});

test("field hold renews its level without repeating direction edges and disables once", () => {
  const f = fixture();
  f.sample(["down"], 10);
  f.sample(["down"], 120);
  f.sample(["down"], 330);
  assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4, 4], [0, 0], [0, 0]]);
  f.disable();
  f.sample(["down"], 340);
  f.sample(["down"], 350);
  assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4, 4], [0, 0], [0, 0], [4, 0]]);
  f.input.dispose();
});

test("a delayed HTTP acknowledgement cannot turn a released tap into a held direction", async () => {
  let acknowledge;
  const f = fixture({ window: 1, acknowledge: () => new Promise(resolve => { acknowledge = resolve; }) });
  try {
    f.sample(["down"], 10);
    f.sample([], 20);
    assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4, 4]]);
    acknowledge();
    await new Promise(resolve => setImmediate(resolve));
    assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4, 4], [4, 0]]);
  } finally {
    f.input.dispose();
    acknowledge?.();
  }
});

test("confirmation during the first short movement still leaves no held server level", () => {
  const f = fixture();
  f.sample(["down"], 10);
  f.sample(["down", "confirm"], 20);
  f.sample([], 30);
  assert.deepEqual(f.actions.map(a => [a.direction, a.keys]), [[4, 4], [4, 2], [4, 0]]);
  f.input.dispose();
});
