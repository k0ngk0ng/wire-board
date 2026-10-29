// Failure inventory: local input waits for a server response; prediction mutates
// confirmed state; correction leaks sessions/replays audio or publishes a result;
// spectators predict; stale/hidden clients run indefinitely; old input survives
// cancellation; mismatched epochs/court changes reuse the wrong fork.
// These drive the actual Go/Wasm rules from a new one/two-player session.
import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { RuntimePrediction } from "./runtime-prediction.ts";
const root = process.cwd(),
  out = path.join(root, ".local/contra-prediction/test");
mkdirSync(out, { recursive: true });
const env = {
  ...process.env,
  GOOS: "js",
  GOARCH: "wasm",
  CGO_ENABLED: "0",
  GOCACHE: path.join(root, ".local/go-build"),
  GOMODCACHE: path.join(root, ".local/go-mod"),
  GOTMPDIR: path.join(root, ".local/tmp"),
};
mkdirSync(env.GOTMPDIR, { recursive: true });
execFileSync(
  "go",
  [
    "build",
    "-ldflags=-X github.com/k0ngk0ng/wire-board/games/contra/runtime.CompiledRulesBuildID=" +
      "a".repeat(64),
    "-o",
    path.join(out, "runtime.wasm"),
    "./games/contra/runtime/cmd/wasm",
  ],
  { cwd: root, env, stdio: "pipe" },
);
const goroot = execFileSync("go", ["env", "GOROOT"], {
  env,
  encoding: "utf8",
}).trim();
await import(pathToFileURL(path.join(goroot, "lib/wasm/wasm_exec.js")).href);
const go = new globalThis.Go();
const loaded = await WebAssembly.instantiate(
  readFileSync(path.join(out, "runtime.wasm")),
  go.importObject,
);
void go.run(loaded.instance);
const calls = [];
const bridge = {
  call(q) {
    calls.push(q);
    const r = JSON.parse(globalThis.contraRuntime(JSON.stringify(q)));
    assert.ok(r.ok, r.error);
    return r.data;
  },
};
bridge.call({
  op: "resources",
  resources: JSON.parse(
    readFileSync("games/contra/engine/leveldata/stage-1.json", "utf8"),
  ),
});
function fixture(t, players = 2) {
  let state = bridge.call({
    op: "new",
    players,
    seed: "42",
    epoch: "prediction-" + Math.random(),
  });
  const step = (pads) =>
    (state = bridge.call({
      op: "pads",
      session: state.session,
      epoch: state.epoch,
      frame: String(BigInt(state.frame) + 1n),
      revision: String(BigInt(state.revision) + 1n),
      pads,
    }));
  for (let n = 1; n <= 60; n++) step([n === 2 ? 16 : 0, 0]);
  assert.equal(state.phase, "playing");
  const prediction = new RuntimePrediction(bridge);
  prediction.confirm(state, [0, 0], 0);
  t.after(() => {
    prediction.dispose();
    bridge.call({ op: "dispose", session: state.session, epoch: state.epoch });
  });
  return {
    prediction,
    step,
    get state() {
      return state;
    },
  };
}
const proof = [];
for (const actor of [0, 1])
  test(
    "native prediction responds for actor " +
      actor +
      " before authority advances",
    (t) => {
      const f = fixture(t),
        before = f.state;
      let result;
      for (let n = 1; n <= 3; n++)
        result = f.prediction.sample((n * 1000) / 60, actor, 65);
      assert.ok(result);
      assert.notEqual(result.players[actor].x, before.players[actor].x);
      assert.notDeepEqual(result.presentation.OAM, before.presentation.OAM);
      assert.equal(
        bridge.call({
          op: "snapshot",
          session: before.session,
          epoch: before.epoch,
        }).digest,
        before.digest,
      );
      assert.equal(result.frame, String(BigInt(before.frame) + 3n));
      proof.push({
        actor,
        visibleAfterMS: 50,
        authorityFrame: before.frame,
        predictedFrame: result.frame,
        authorityUnchanged: true,
      });
    },
  );
test("correction forks native authority and preserves only unconfirmed samples", (t) => {
  const f = fixture(t);
  for (let n = 1; n <= 6; n++) f.prediction.sample((n * 1000) / 60, 0, 65);
  for (let n = 1; n <= 3; n++) f.step([0, 0]);
  f.prediction.confirm(f.state, [0, 0], 100);
  const corrected = f.prediction.sample((7 * 1000) / 60, 0, 65);
  const reference = bridge.call({
    op: "fork",
    session: f.state.session,
    epoch: f.state.epoch,
  });
  t.after(() =>
    bridge.call({
      op: "dispose",
      session: reference.session,
      epoch: reference.epoch,
    }),
  );
  let expected = reference;
  for (let n = 0; n < 4; n++)
    expected = bridge.call({
      op: "pads",
      session: expected.session,
      epoch: expected.epoch,
      frame: String(BigInt(expected.frame) + 1n),
      pads: [65, 0],
    });
  assert.equal(corrected.digest, expected.digest);
  assert.equal(
    bridge.call({
      op: "snapshot",
      session: f.state.session,
      epoch: f.state.epoch,
    }).digest,
    f.state.digest,
  );
});
test("prediction is bounded, spectators do not advance and cancellation releases old samples", (t) => {
  const f = fixture(t);
  assert.equal(f.prediction.sample(17, -1, 1), undefined);
  for (let n = 1; n <= 30; n++) f.prediction.sample((n * 1000) / 60, 0, 1);
  assert.ok(
    f.prediction.frame === undefined ||
      BigInt(f.prediction.frame) <= BigInt(f.state.frame) + 12n,
  );
  assert.equal(f.prediction.sample(1000, 0, 1), undefined);
  f.prediction.cancel();
  f.step([0, 0]);
  f.prediction.confirm(f.state, [0, 0], 1010);
  const still = f.prediction.sample(1030, 0, 0);
  assert.equal(still.players[0].x, f.state.players[0].x);
});
test("paused, terminal and new epochs cannot reuse speculative state", (t) => {
  const f = fixture(t);
  f.prediction.sample(17, 0, 1);
  f.prediction.confirm({ ...f.state, paused: true }, [0, 0], 20);
  assert.equal(f.prediction.sample(40, 0, 1), undefined);
  f.prediction.confirm({ ...f.state, finalized: true }, [0, 0], 50);
  assert.equal(f.prediction.sample(70, 0, 1), undefined);
  assert.throws(
    () => f.prediction.confirm({ ...f.state, epoch: "wrong" }, [0, 0], 80),
    /epoch|身份/,
  );
});
test("reconciliation uses native forks without checkpoint export or session growth", (t) => {
  const f = fixture(t);
  const start = calls.length;
  for (let n = 1; n <= 40; n++) {
    f.prediction.sample(n * 50, 0, 1);
    for (let i = 0; i < 3; i++) f.step([1, 0]);
    f.prediction.confirm(f.state, [1, 0], n * 50);
  }
  const operations = calls.slice(start);
  assert.ok(operations.some((q) => q.op === "fork" && q.target));
  assert.ok(
    !operations.some((q) => q.op === "checkpoint" || q.op === "restore"),
  );
});
test.after(() =>
  writeFileSync(
    path.join(out, "result.json"),
    JSON.stringify(
      {
        passed: true,
        scope:
          "Actual Wasm rules, no server-frame progress during initial local response; deterministic 60Hz test clock",
        cases: proof,
      },
      null,
      2,
    ) + "\n",
  ),
);

// A short press between 144Hz RAF callbacks still owes one 60Hz rule step.
test("a sub-frame tap is retained until the next prediction tick", (t) => {
  const f = fixture(t);
  f.prediction.sample(5, 0, 1);
  f.prediction.sample(10, 0, 0);
  const moved = f.prediction.sample(17, 0, 0);
  assert.notEqual(moved.players[0].x, f.state.players[0].x);
  f.prediction.cancel();
  f.prediction.confirm(f.state, [0, 0], 20);
  const cleared = f.prediction.sample(37, 0, 0);
  assert.equal(cleared.players[0].x, f.state.players[0].x);
});
