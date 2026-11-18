import test from "node:test";
import assert from "node:assert/strict";
import { createPredictionController } from "../web/prediction.ts";

function setup(configuration = {}) {
  let wall = 0;
  let serial = 0;
  const callbacks = new Map();
  const frames = [];
  const sent = [];
  const recoveries = [];
  const canonical = [];
  const runtime = {
    clone: state => structuredClone(state),
    project: state => structuredClone(state),
    advance: (state, milliseconds) => ({ ...state, clock: state.clock + milliseconds, x: state.x + milliseconds * state.power }),
    apply: (state, actor, action) => {
      const next = structuredClone(state);
      if (action.type === "upgrade") {
        if (next.gold < 40) throw Error("金币不足");
        next.gold -= 40;
        next.power++;
        next.events.push({ type: "upgrade", actor, id: next.events.length + 1 });
      } else if (action.type === "value") next.value = action.value;
      else if (action.type === "gain") next.gold += action.value;
      else throw Error("invalid action");
      return next;
    },
  };
  const controller = createPredictionController({
    runtime,
    actor: 0,
    now: () => wall,
    requestFrame: callback => { callbacks.set(++serial, callback); return serial; },
    cancelFrame: id => callbacks.delete(id),
    readClock: state => state.clock,
    send: (action, clientInputID) => sent.push({ action, clientInputID, at: wall }),
    onFrame: (view, metadata) => frames.push({ view: structuredClone(view), metadata }),
    onAuthority: (view, metadata) => canonical.push({ view, metadata }),
    requestSync: reason => recoveries.push(reason),
    ...configuration,
  });
  const initial = { clock: 0, x: 0, power: 1, gold: 100, value: 0, events: [] };
  if (!configuration.skipReset) controller.reset({ state: initial, epoch: 1, sequence: 0, logicalMS: 0 });
  const at = value => {
    wall = value;
    const next = [...callbacks.values()];
    callbacks.clear();
    for (const callback of next) callback(value);
  };
  return { controller, runtime, at, frames, sent, recoveries, canonical, initial, callbacks, setWall(value) { wall = value; } };
}

test("native frames immediately predict original actions while authority stays immutable", () => {
  const state = setup();
  state.at(0);
  state.at(1000 / 60);
  const input = state.controller.input({ type: "upgrade" }, "local:1");
  assert.equal(input.accepted, true);
  assert.equal(state.controller.view().gold, 60);
  assert.equal(state.initial.gold, 100);
  assert.equal(state.sent.length, 1);
  assert.equal(state.sent[0].at, 1000 / 60);
  for (let frame = 2; frame <= 12; frame++) state.at(frame * 1000 / 60);
  assert.equal(state.controller.sample().predictedMS, 200);
  assert.ok(state.frames.filter(frame => frame.metadata.phase === "frame").length >= 12);
  assert.equal(state.controller.sample().pendingInputs, 1);
  state.controller.destroy();
});

test("confirmed own input is removed before replay so it never spends twice", () => {
  const state = setup();
  state.controller.input({ type: "upgrade" }, "local:1");
  state.at(50);
  state.controller.acknowledge("local:1", true);
  assert.equal(state.controller.sample().pendingInputs, 1);
  state.setWall(100);
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 2, logicalMS: 100, operations: [{ type: "action", actor: 0, clientInputID: "local:1", action: { type: "upgrade" } }, { type: "tick", milliseconds: 100 }] });
  assert.equal(state.controller.sample().pendingInputs, 0);
  assert.equal(state.controller.view().gold, 60);
  assert.equal(state.controller.view().events.length, 1);
  assert.equal(state.canonical.at(-1).metadata.confirmedInputIDs[0], "local:1");
  state.controller.destroy();
});

test("another seat's identical action does not acknowledge the local input", () => {
  const state = setup();
  state.controller.input({ type: "upgrade" }, "local:1");
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 1, logicalMS: 0, operations: [{ type: "action", actor: 1, clientInputID: "local:1", action: { type: "upgrade" } }] });
  assert.equal(state.controller.sample().pendingInputs, 1);
  assert.equal(state.controller.view().gold, 20);
  state.controller.acknowledge("local:1", false);
  assert.equal(state.controller.view().gold, 60);
  assert.equal(state.controller.sample().pendingInputs, 0);
  assert.equal(state.sent.length, 1);
  state.controller.destroy();
});

test("full snapshot receipts, duplicate updates and rejection preserve the displayed clock phase", () => {
  const state = setup();
  state.controller.input({ type: "upgrade" }, "local:1");
  for (let frame = 0; frame <= 9; frame++) state.at(frame * 1000 / 60);
  const shown = state.controller.sample().predictedMS;
  const confirmed = state.runtime.advance(state.runtime.apply(state.initial, 0, { type: "upgrade" }), 100);
  state.controller.receive({ epoch: 1, sequence: 2, logicalMS: 100, state: confirmed, acceptedInputs: ["local:1"] });
  assert.equal(state.controller.view().gold, 60);
  assert.ok(state.controller.sample().predictedMS >= shown);
  const before = state.controller.sample();
  state.setWall(170);
  state.controller.receive({ epoch: 1, sequence: 2, logicalMS: 100, state: confirmed, acceptedInputs: ["local:1"] });
  assert.equal(state.controller.sample().anchorWallMS, before.anchorWallMS);
  state.controller.destroy();
});

test("new inputs are never replayed into an already displayed historical frame", () => {
  const state = setup();
  state.at(150);
  const shown = state.controller.sample().predictedMS;
  state.controller.input({ type: "upgrade" }, "local:1");
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 1, logicalMS: 100, operations: [{ type: "tick", milliseconds: 100 }] });
  assert.ok(state.controller.sample().predictedMS >= shown);
  assert.equal(state.controller.sample().pending[0].atMS, shown);
  assert.equal(state.controller.view().x, shown);
  state.controller.destroy();
});

test("prediction lead and pending input windows stay bounded on a slow connection", () => {
  const state = setup();
  for (let frame = 0; frame <= 30; frame++) state.at(frame * 1000 / 60);
  assert.ok(state.controller.sample().predictedMS <= 200);
  for (let index = 0; index < 8; index++) assert.equal(state.controller.input({ type: "value", value: index }, `local:${index}`).accepted, true);
  assert.equal(state.controller.input({ type: "value", value: 9 }, "local:9").accepted, false);
  assert.equal(state.controller.sample().pendingInputs, 8);
  state.controller.destroy();
});

test("coalesced absolute inputs keep their latest value without dropping the distinct change", () => {
  const state = setup();
  for (let index = 0; index < 20; index++) state.controller.input({ type: "value", value: index }, `input:${index}`, { coalesceKey: "volume:input" });
  assert.equal(state.controller.sample().pendingInputs, 1);
  assert.equal(state.controller.view().value, 19);
  state.controller.input({ type: "value", value: 19 }, "change:1");
  assert.equal(state.controller.sample().pendingInputs, 2);
  state.controller.destroy();
});

test("a fixed delayed stream settles its display phase after reaching the unchanged lead cap", () => {
  for (const rate of [57, 60, 144]) {
    const state = setup();
    const events = [];
    for (let index = 1; index <= 70; index++) events.push({ at: index * 100 + (index > 20 ? 120 : 0), sequence: index });
    for (let index = 0; index < 7 * rate; index++) events.push({ at: 4.3 + index * 1000 / rate });
    events.sort((a, b) => a.at - b.at || (b.sequence ?? 0) - (a.sequence ?? 0));
    const samples = [];
    try {
      for (const event of events) {
        state.setWall(event.at);
        if (event.sequence) state.controller.receive({ epoch: 1, fromSequence: event.sequence - 1, sequence: event.sequence, logicalMS: event.sequence * 100, operations: [{ type: "tick", milliseconds: 100 }] });
        else {
          state.at(event.at);
          if (event.at >= 4000) samples.push({ at: event.at, ...state.controller.sample() });
        }
        assert.ok(state.controller.sample().leadMS <= 200);
        assert.equal(state.controller.view().x, state.controller.sample().predictedMS);
      }
      const intervals = samples.slice(1).map((sample, index) => sample.predictedMS - samples[index].predictedMS);
      assert.ok(intervals.every(value => value >= Math.floor(1000 / rate) - 1 && value <= Math.ceil(1000 / rate) + 1), `${rate} Hz retained capped time and stalled: ${intervals.slice(0, 24)}`);
      assert.deepEqual(state.recoveries, []);
    } finally { state.controller.destroy(); }
  }
});

test("time held at the prediction cap is discarded before authoritative recovery", () => {
  const state = setup();
  try {
    for (let index = 0; index <= 54; index++) state.at(index * 1000 / 60);
    assert.equal(state.controller.sample().predictedMS, 200);
    state.setWall(920);
    state.controller.receive({ epoch: 1, sequence: 9, logicalMS: 900, state: state.runtime.advance(state.initial, 900) });
    assert.equal(state.controller.sample().predictedMS, 900);
    state.at(936);
    assert.equal(state.controller.sample().predictedMS, 916);
    assert.equal(state.controller.sample().leadMS, 16);
  } finally { state.controller.destroy(); }
});

test("epoch changes, gaps, visibility and lost control clear stale prediction without replaying inputs", () => {
  const state = setup();
  state.controller.input({ type: "upgrade" }, "local:1");
  state.controller.suspend();
  assert.equal(state.controller.sample().pendingInputs, 0);
  state.controller.resume();
  assert.equal(state.controller.sample().waiting, true);
  assert.equal(state.recoveries.length, 1);
  state.controller.reset({ state: state.initial, epoch: 2, sequence: 4, logicalMS: 0 });
  assert.equal(state.controller.acknowledge("local:1", false), false);
  state.controller.receive({ epoch: 2, fromSequence: 1, sequence: 5, logicalMS: 100, operations: [{ type: "tick", milliseconds: 100 }] });
  assert.equal(state.controller.sample().waiting, true);
  assert.equal(state.recoveries.length, 2);
  state.controller.reset({ state: state.initial, epoch: 3, sequence: 0, logicalMS: 0 });
  state.controller.setControllable(false);
  assert.equal(state.controller.input({ type: "upgrade" }, "local:2").accepted, false);
  state.controller.destroy();
  assert.equal(state.callbacks.size, 0);
});

test("an invalid authority update cannot replace the last valid canonical state", () => {
  const state = setup();
  const original = structuredClone(state.controller.authorityState());
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 1, logicalMS: 101, operations: [{ type: "tick", milliseconds: 100 }] });
  assert.equal(state.controller.sample().waiting, true);
  assert.deepEqual(state.controller.authorityState(), original);
  assert.equal(state.recoveries.length, 1);
  state.controller.destroy();
});

test("pause and resume never advance the paused offline duration", () => {
  const state = setup();
  state.at(50);
  state.controller.setPaused(true);
  for (const at of [100, 200, 500]) state.at(at);
  assert.equal(state.controller.sample().predictedMS, 50);
  state.controller.setPaused(false);
  state.at(510);
  assert.equal(state.controller.sample().predictedMS, 60);
  state.controller.destroy();
});

test("a slow peer's bounded lead and local inputs cannot stall a fast peer", () => {
  const fast = setup();
  const slow = setup({ actor: 1 });
  for (let frame = 0; frame <= 30; frame++) {
    const at = frame * 1000 / 60;
    fast.setWall(at);
    if (frame > 0 && frame % 6 === 0) {
      const step = frame / 6;
      fast.controller.receive({ epoch: 1, fromSequence: step - 1, sequence: step, logicalMS: step * 100, operations: [{ type: "tick", milliseconds: 100 }] });
    }
    fast.at(at);
    slow.at(at);
  }
  assert.equal(fast.controller.sample().authorityMS, 500);
  assert.equal(slow.controller.sample().authorityMS, 0);
  assert.equal(slow.controller.sample().predictedMS, 200);
  assert.deepEqual(fast.recoveries, []);
  slow.controller.receive({ epoch: 1, fromSequence: 0, sequence: 5, logicalMS: 500, operations: Array.from({ length: 5 }, () => ({ type: "tick", milliseconds: 100 })) });
  assert.equal(slow.controller.sample().predictedMS, 500);
  slow.controller.input({ type: "upgrade" }, "slow:1");
  assert.equal(slow.controller.view().gold, 60);
  assert.equal(fast.controller.view().gold, 100);
  fast.controller.destroy();
  slow.controller.destroy();
});

test("authority catchup after a local display pause resumes between network messages", () => {
  const state = setup();
  state.at(50);
  state.controller.setPaused(true);
  state.at(500);
  state.controller.setPaused(false);
  const caughtUp = state.runtime.advance(state.initial, 500);
  state.controller.receive({ epoch: 1, sequence: 5, logicalMS: 500, state: caughtUp });
  assert.equal(state.controller.sample().predictedMS, 500);
  state.at(516);
  assert.equal(state.controller.sample().predictedMS, 516);
  const anchor = state.controller.sample().anchorWallMS;
  state.setWall(600);
  state.controller.receive({ epoch: 1, fromSequence: 5, sequence: 6, logicalMS: 600, operations: [{ type: "tick", milliseconds: 100 }] });
  state.at(616);
  assert.equal(state.controller.sample().predictedMS, 616);
  assert.equal(state.controller.sample().anchorWallMS, anchor);
  state.controller.destroy();
});

test("copy cost uses a native measurement clock independently of rule/display time", () => {
  let measurement = 0;
  const costs = [];
  const state = setup({ measureNow: () => measurement++, onCost: value => costs.push(value) });
  assert.ok(costs.filter(value => value.phase === "clone").every(value => value.milliseconds === 1));
  state.controller.destroy();
});

test("authority verification happens before a candidate canonical state is committed", () => {
  const state = setup({ validateAuthority: (canonical, view, verification) => canonical.gold === verification.gold && view.power === verification.power });
  const original = structuredClone(state.controller.authorityState());
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 1, logicalMS: 100, verification: { gold: 99, power: 1 }, operations: [{ type: "tick", milliseconds: 100 }] });
  assert.equal(state.controller.sample().waiting, true);
  assert.deepEqual(state.controller.authorityState(), original);
  state.controller.destroy();
});

test("an initial delta requests one full snapshot and duplicate deltas do not flood recovery", () => {
  const state = setup({ skipReset: true });
  const update = { epoch: 1, fromSequence: 0, sequence: 1, logicalMS: 100, operations: [{ type: "tick", milliseconds: 100 }] };
  state.controller.receive(update);
  state.controller.receive(update);
  assert.equal(state.recoveries.length, 1);
  state.controller.destroy();
});

test("scaled physical step phase is supplied by the native rule state", () => {
  const state = setup({ readPhysics: world => ({ step: Math.floor(world.clock * 0.8 / 20), interpolation: world.clock * 0.8 % 20 / 20 }) });
  state.at(33);
  const last = state.frames.at(-1).metadata;
  assert.ok(Math.abs(last.interpolation - 0.32) < 1e-9);
  state.controller.destroy();
});

test("display metadata preserves the authority cursor for arrival-to-paint measurements", () => {
  const state = setup();
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 1, logicalMS: 100, operations: [{ type: "tick", milliseconds: 100 }] });
  const metadata = state.frames.at(-1).metadata;
  assert.equal(metadata.epoch, 1);
  assert.equal(metadata.authoritySequence, 1);
  assert.equal(metadata.authorityMS, 100);
  state.controller.destroy();
});
