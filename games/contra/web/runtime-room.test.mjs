// Before room transport: a new browser starts input sequence at one; stage-ready
// overtakes an in-flight input; a stale rejection kills the Wasm or resends an old
// button; observers send actions; stale room/actor identity crosses a remount;
// a clipped revision ring is treated as complete. These isolate transport only.
import test from "node:test";
import assert from "node:assert/strict";
import { createRoomTransport } from "./runtime-room.ts";
function fixture() {
  const snapshot = {
    session: 0,
    epoch: "e",
    frame: "3",
    revision: "3",
    digest: "d",
    resources: "b",
    rulesBuildID: "r",
    level: 0,
  };
  let binding = {
    room: { id: "room", version: 7, status: "playing", you: 0, seats: [{}] },
    view: {
      runtimeBuildID: "asset-a",
      epoch: "e",
      actor: 0,
      inputs: [{ sequence: 40, held: 0, automated: false }],
      snapshot,
      revision: "3",
      frames: [],
      stage: { level: 0, path: "stage-1.json", sha256: "stage" },
      bundle: { id: "b" },
      rulesBuildID: "r",
    },
  };
  const calls = [];
  let behavior = async () => {};
  let syncCount = 0;
  const transport = createRoomTransport({
    runtimeBuildID: "asset-a",
    roomID: "room",
    epoch: "e",
    actor: 0,
    current: () => binding,
    act: async (action) => {
      calls.push(action);
      await behavior(action);
    },
    sync: async () => {
      syncCount++;
      return {
        version: binding.room.version,
        view: binding.view,
        checkpoint: {
          version: 1,
          epoch: "e",
          rulesBuildID: "r",
          resources: "b",
          digest: "d",
          state: { revision: binding.view.revision },
        },
      };
    },
    prepare: async () => {},
  });
  return {
    transport,
    calls,
    get binding() {
      return binding;
    },
    set binding(v) {
      binding = v;
    },
    setBehavior: (f) => {
      behavior = f;
    },
    syncCount: () => syncCount,
  };
}
test("input and stage-ready share the restored sequence and one ordered send chain", async () => {
  const f = fixture();
  let complete;
  f.setBehavior(() => new Promise((r) => (complete = r)));
  const first = f.transport.input(1, 1);
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(f.calls[0].sequence, 41);
  f.binding.view.stageLoading = { level: 0, ready: [false] };
  const ready = f.transport.ready();
  await Promise.resolve();
  assert.equal(f.calls.length, 1);
  f.setBehavior(async () => {});
  complete();
  await first;
  await ready;
  assert.deepEqual(
    f.calls.map((a) => [a.type, a.sequence]),
    [
      ["input", 41],
      ["stage-ready", 42],
    ],
  );
  assert.equal(f.calls[1].bundleID, "b");
  assert.equal(f.calls[1].rulesBuildID, "r");
  assert.equal(f.calls[1].stageSHA256, "stage");
});
test("stale input synchronizes and is dropped instead of replaying old buttons", async () => {
  const f = fixture();
  f.setBehavior(async () => {
    f.binding.view.inputs[0].sequence = 80;
    throw Object.assign(Error("stale or conflicting input sequence"), {
      status: 400,
    });
  });
  await f.transport.input(1, 1);
  assert.equal(f.syncCount(), 1);
  assert.equal(f.calls.length, 1);
  f.setBehavior(async () => {});
  await f.transport.input(0, 2);
  assert.equal(f.calls[1].sequence, 81);
  assert.equal(f.calls[1].held, 0);
});
test("observers and initial loading do not issue controller commands", async () => {
  const f = fixture();
  f.binding.view.actor = -1;
  await f.transport.input(64, 1);
  await f.transport.release(2);
  assert.equal(f.calls.length, 0);
  f.binding.view.actor = 0;
  f.binding.room.loading = { token: "initial" };
  await f.transport.input(64, 3);
  await f.transport.release(4);
  assert.equal(f.calls.length, 0);
});
test("a clipped compact ring requests an explicit checkpoint without replaying a gap", async () => {
  const f = fixture();
  f.binding.view.revision = "200";
  f.binding.view.snapshot = { ...f.binding.view.snapshot, revision: "200" };
  f.binding.view.frames = [
    {
      kind: "pads",
      revision: "199",
      frame: "199",
      pads: [0, 0],
      digest: "x",
      level: 0,
    },
    {
      kind: "pads",
      revision: "200",
      frame: "200",
      pads: [0, 0],
      digest: "d",
      level: 0,
    },
  ];
  const response = await f.transport.poll("3");
  assert.ok(response.checkpoint);
  assert.equal(response.frames.length, 0);
  assert.equal(f.syncCount(), 1);
});
test("release is allowed during autoplay while input stays disabled", async () => {
  const f = fixture();
  f.binding.room.seats[0].autoPlay = true;
  f.binding.view.inputs[0].automated = true;
  await f.transport.input(64, 1);
  await f.transport.release(2);
  assert.deepEqual(
    f.calls.map((a) => a.type),
    ["release"],
  );
});
test("paused ownership uses current platform seat flags instead of last normal-tick automated projection", async () => {
  const f = fixture();
  f.binding.view.inputs[0].automated = true;
  await f.transport.input(8, 1);
  assert.equal(f.calls[0]?.type, "input");
  f.binding.view.inputs[0].automated = false;
  f.binding.room.seats[0].autoPlay = true;
  await f.transport.input(8, 2);
  assert.equal(f.calls.length, 1);
});
test("room updates and checkpoint recovery reject a different runtime artifact with unchanged rule identity", async () => {
  const f = fixture();
  f.binding.view.runtimeBuildID = "asset-b";
  await assert.rejects(() => f.transport.poll("3"), /运行程序版本/);
  await assert.rejects(() => f.transport.checkpoint(), /运行程序版本/);
});

// Protocol-v2 failures: bootstrap passes a header to the renderer; a persistent
// recovery marker triggers sync every poll; one unready peer disables the ready
// human; compact input records incorrectly require a network framebuffer/audio.
test("ready humans keep control while another player loads; only own loading blocks input", async () => {
  const f = fixture();
  f.binding.room.seats = [{}, {}];
  f.binding.view.stageLoading = { level: 0, ready: [true, false] };
  assert.equal(f.transport.inputEnabled(), true);
  await f.transport.input(65, 1);
  assert.equal(f.calls.at(-1).held, 65);
  f.binding.view.stageLoading.ready[0] = false;
  assert.equal(f.transport.inputEnabled(), false);
  await f.transport.input(1, 2);
  assert.equal(f.calls.length, 1);
});
test("a compact header bootstraps from one explicit full checkpoint", async () => {
  const f = fixture();
  f.binding.view.snapshot = {
    ...f.binding.view.snapshot,
    presentation: { Patterns: [] },
    audio: { events: [] },
  };
  const initial = await f.transport.initial();
  assert.ok(initial.presentation);
  assert.ok(initial.audio);
  assert.equal(f.syncCount(), 1);
  const checkpoint = await f.transport.checkpoint();
  assert.equal(checkpoint.epoch, "e");
  assert.equal(
    f.syncCount(),
    1,
    "bootstrap checkpoint must be reused by Wasm restore",
  );
});
test("incremental pad records work without map or audio and recovery is consumed once", async () => {
  const f = fixture();
  f.binding.view.protocol = 2;
  f.binding.view.revision = "4";
  f.binding.view.snapshot = { ...f.binding.view.snapshot, revision: "4" };
  f.binding.view.frames = [
    {
      kind: "pads",
      revision: "4",
      frame: "4",
      pads: [65, 0],
      digest: "d",
      level: 0,
    },
  ];
  const delta = await f.transport.poll("3");
  assert.equal(delta.frames.length, 1);
  assert.equal(f.syncCount(), 0);
  assert.equal(delta.snapshot.presentation, undefined);
  assert.equal(delta.snapshot.audio, undefined);
  f.binding.view.revision = "200";
  f.binding.view.snapshot = { ...f.binding.view.snapshot, revision: "200" };
  f.binding.view.resync = true;
  f.binding.view.frames = [];
  assert.ok((await f.transport.poll("4")).checkpoint);
  assert.equal(f.syncCount(), 1);
  await f.transport.checkpoint();
  const caughtUp = await f.transport.poll("200");
  assert.equal(caughtUp.checkpoint, undefined);
  assert.deepEqual(caughtUp.frames, []);
  assert.equal(f.syncCount(), 1);
});
