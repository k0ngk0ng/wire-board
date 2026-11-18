import test from "node:test";
import assert from "node:assert/strict";
import { createPredictionController } from "../web/prediction.ts";

function setup() {
  const live = new Map();
  const callbacks = new Map();
  let nextID = 0;
  let frameID = 0;
  let wall = 0;
  const create = value => {
    const id = ++nextID;
    live.set(id, structuredClone(value));
    return Object.freeze({ id, clock: value.clock, paused: false });
  };
  const read = handle => {
    const value = live.get(handle.id);
    assert.ok(value, `use after release of handle ${handle.id}`);
    return value;
  };
  const runtime = {
    clone: handle => create(read(handle)),
    project: handle => structuredClone(read(handle)),
    advance: (handle, milliseconds) => create({ ...read(handle), clock: read(handle).clock + milliseconds }),
    apply: (handle, actor, action) => create({ ...read(handle), gold: read(handle).gold - Number(action.count ?? 1) }),
    release(handle) {
      assert.ok(live.delete(handle.id), `double release of handle ${handle.id}`);
    },
  };
  const controller = createPredictionController({ runtime, actor: 0, readClock: handle => handle.clock, now: () => wall, requestFrame: callback => { callbacks.set(++frameID, callback); return frameID; }, cancelFrame: id => callbacks.delete(id), send() {}, onFrame() {}, requestSync() {} });
  const imported = create({ clock: 0, gold: 100 });
  controller.reset({ state: imported, epoch: 1, sequence: 0, logicalMS: 0 });
  runtime.release(imported);
  return { controller, runtime, live, create, at(value) { wall = value; const list = [...callbacks.values()]; callbacks.clear(); for (const callback of list) callback(value); } };
}

test("opaque prediction keeps only authority and one current prediction handle", () => {
  const state = setup();
  assert.equal(state.live.size, 2);
  for (let frame = 0; frame <= 120; frame++) {
    state.at(frame * 1000 / 60);
    assert.equal(state.live.size, 2, `frame ${frame}`);
  }
  state.controller.input({ type: "upgrade" }, "handle:1");
  assert.equal(state.live.size, 2);
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 3, logicalMS: 100, operations: [{ type: "tick", milliseconds: 50 }, { type: "action", action: { type: "upgrade", epoch: 1 }, actor: 0, clientInputID: "handle:1" }, { type: "tick", milliseconds: 50 }] });
  assert.equal(state.live.size, 2);
  state.controller.destroy();
  assert.equal(state.live.size, 0);
});

test("failed authority candidates are released without invalidating the previous baseline", () => {
  const state = setup();
  const authority = state.controller.authorityState();
  state.controller.receive({ epoch: 1, fromSequence: 0, sequence: 1, logicalMS: 99, operations: [{ type: "tick", milliseconds: 100 }] });
  assert.equal(state.controller.authorityState(), authority);
  assert.equal(state.live.size, 2);
  state.controller.destroy();
  assert.equal(state.live.size, 0);
});

test("reset never consumes the caller's imported handle and disposes the previous world", () => {
  const state = setup();
  const imported = state.create({ clock: 400, gold: 33 });
  state.controller.reset({ state: imported, epoch: 2, sequence: 10, logicalMS: 400 });
  assert.equal(state.live.size, 3);
  state.runtime.release(imported);
  assert.equal(state.live.size, 2);
  assert.equal(state.controller.view().gold, 33);
  state.controller.destroy();
  state.controller.destroy();
  assert.equal(state.live.size, 0);
});
