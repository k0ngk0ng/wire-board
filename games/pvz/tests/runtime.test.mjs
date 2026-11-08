// Specified before the bridge/build implementation. Exercises the actual Go
// binary and actual compiled Wasm through their JSON boundary, including
// snapshot recovery, invalid requests and long deterministic replay.
import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { execFileSync, spawn } from "node:child_process";
import { createInterface } from "node:readline";
import { webcrypto } from "node:crypto";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const out = path.join(root, ".local/pvz/runtime-e2e");
mkdirSync(out, { recursive: true });
const report = { passed: false, command: "node games/pvz/tests/runtime.test.mjs", scope: "native Go executable and compiled browser Wasm; same finite scenario, actions, ticks, validation and digests", steps: [] };
let native;
try {
  const { preparePvZRuntime } = await import("../build/runtime.mjs");
  const publicRoot = preparePvZRuntime(root);
  const directory = path.join(publicRoot, "pvz/v1/runtime");
  const env = { ...process.env, GOTOOLCHAIN: "local", GOCACHE: process.env.GOCACHE || path.join(root, ".local/go-build"), GOMODCACHE: process.env.GOMODCACHE || path.join(root, ".local/go-mod"), GOTMPDIR: path.join(root, ".local/tmp"), TMPDIR: path.join(root, ".local/tmp") };
  for (const p of [env.GOCACHE, env.GOMODCACHE, env.GOTMPDIR]) mkdirSync(p, { recursive: true });
  const binary = path.join(out, "pvz-native");
  execFileSync("go", ["build", "-trimpath", "-buildvcs=false", "-o", binary, "./games/pvz/cmd/runtime-stdio"], { cwd: root, env, stdio: "pipe" });
  native = spawn(binary, [], { cwd: root, env, stdio: ["pipe", "pipe", "pipe"] });
  const lines = createInterface({ input: native.stdout });
  const queue = [], buffered = [];
  lines.on("line", line => { const next = queue.shift(); if (next) next(JSON.parse(line)); else buffered.push(JSON.parse(line)); });
  const callNative = request => new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(Error("native response timed out")), 10000);
    const done = value => { clearTimeout(timer); resolve(value); };
    if (buffered.length) done(buffered.shift()); else queue.push(done);
    native.stdin.write(JSON.stringify(request) + "\n");
  });
  globalThis.crypto ||= webcrypto;
  vm.runInThisContext(readFileSync(path.join(directory, "wasm_exec.js"), "utf8"));
  const go = new globalThis.Go();
  const { instance } = await WebAssembly.instantiate(readFileSync(path.join(directory, "pvz.wasm")), go.importObject);
  void go.run(instance);
  assert.equal(typeof globalThis.pvzWasm, "function", "browser bridge failed to start");
  const compare = async request => {
    const a = await callNative(request), b = JSON.parse(globalThis.pvzWasm(JSON.stringify(request)));
    assert.deepEqual(b, a, "native/Wasm divergence for " + JSON.stringify(request));
    report.steps.push({ request, ok: a.ok, digest: a.digest, tick: a.state?.tick, phase: a.state?.phase });
    return a;
  };
  let state = (await compare({ op: "new", id: "authority", players: 4, epoch: "pvz-runtime-parity", options: { mode: "team", teams: [0, 1, 0, 1], prepSeconds: 20, roundSeconds: 360 } })).state;
  assert.equal(state.players.length, 4);
  for (const [actor, row] of [[0, 0], [2, 1]]) {
    const result = await compare({ op: "apply", id: "authority", actor, action: { type: "plant", epoch: state.epoch, seq: state.players[actor].seq + 1, kind: "sunflower", row, col: 0 } });
    assert.equal(result.ok, true); state = result.state;
  }
  const before = state;
  const invalid = await compare({ op: "apply", id: "authority", actor: 1, action: { type: "shovel", epoch: state.epoch, seq: 1, plantId: state.plants[0].id } });
  assert.equal(invalid.ok, false);
  assert.deepEqual((await compare({ op: "inspect", id: "authority" })).state, before, "rejected action mutated runtime");
  state = (await compare({ op: "step", id: "authority", ticks: 400 })).state;
  assert.equal(state.phase, "battle");
  state = (await compare({ op: "apply", id: "authority", actor: 1, action: { type: "spawn", epoch: state.epoch, seq: state.players[1].seq + 1, kind: "zombie", row: 0 } })).state;
  assert.ok(state.zombies.length > 0);
  await compare({ op: "load", id: "recovered", snapshot: state });
  for (let n = 0; n < 24; n++) {
    const a = await compare({ op: "step", id: "authority", ticks: 100 });
    const b = await compare({ op: "step", id: "recovered", ticks: 100 });
    assert.equal(a.digest, b.digest, "snapshot replay diverged");
  }
  assert.equal((await compare({ op: "step", id: "authority", ticks: 1000000 })).ok, false, "unbounded prediction accepted");
  assert.equal((await compare({ op: "load", id: "invalid", snapshot: {} })).ok, false, "invalid snapshot accepted");
  // Controlled profile-4 entry snapshot; all subsequent transitions use the
  // public runtime API and ordinary prices, phases and deployment cooldowns.
  const opening = (await compare({ op: "new", id: "open", players: 2, epoch: "pvz-open-parity", options: { prepSeconds: 10, roundSeconds: 180 } })).state;
  opening.rulesVersion = 4;
  let opened = await compare({ op: "load", id: "open", snapshot: opening });
  assert.ok(!opened.state.players[0].deck.includes("starfruit"));
  const act = async (actor, action) => compare({ op: "apply", id: "open", actor, action: { ...action, epoch: opened.state.epoch, seq: opened.state.players[actor].seq + 1 } });
  opened = await act(0, { type: "plant", kind: "starfruit", row: 0, col: 2 });
  assert.ok(opened.ok);
  opened = await compare({ op: "step", id: "open", ticks: 200 });
  const wallet = opened.state.players[0].money;
  opened = await act(0, { type: "deck", kinds: ["sunflower"], imitateKind: "squash" });
  assert.ok(opened.ok);
  assert.equal(opened.state.players[0].money, wallet);
  opened = await act(1, { type: "spawn", kind: "flag", row: 1 });
  assert.ok(opened.ok);
  const paid = opened;
  assert.equal((await act(1, { type: "spawn", kind: "backupdancer", row: 2 })).ok, false);
  assert.equal((await compare({ op: "inspect", id: "open" })).digest, paid.digest);
  opened = await compare({ op: "step", id: "open", ticks: 60 });
  opened = await act(1, { type: "spawn", kind: "backupdancer", row: 2 });
  assert.ok(opened.ok);
  await compare({ op: "load", id: "open-restored", snapshot: opened.state });
  const advanced = await compare({ op: "step", id: "open", ticks: 400 });
  assert.equal((await compare({ op: "step", id: "open-restored", ticks: 400 })).digest, advanced.digest);
  for (const id of ["open", "open-restored"]) assert.equal((await compare({ op: "drop", id })).ok, true);
  if (process.env.PVZ_RUNTIME_BALANCE_REPLAY) {
    const file = process.env.PVZ_RUNTIME_BALANCE_REPLAY;
    const replay = JSON.parse(readFileSync(file, "utf8"));
    assert.equal(replay.initial.rulesVersion, 3);
    let balanced = await compare({ op: "load", id: "balanced", snapshot: replay.initial });
    for (const item of replay.actions) {
      if (balanced.state.tick < item.tick) balanced = await compare({ op: "step", id: "balanced", ticks: item.tick - balanced.state.tick });
      balanced = await compare({ op: "apply", id: "balanced", actor: item.actor, action: item.action });
      assert.equal(balanced.ok, true, "balanced replay action became illegal");
    }
    if (balanced.state.tick < replay.finalTick) balanced = await compare({ op: "step", id: "balanced", ticks: replay.finalTick - balanced.state.tick });
    assert.equal(balanced.digest, replay.finalDigest, "balanced profile changed its complete recorded outcome");
    report.balancedReplay = { file: path.relative(root, file), finalDigest: balanced.digest, actions: replay.actions.length };
    assert.equal((await compare({ op: "drop", id: "balanced" })).ok, true);
  }
  for (const id of ["authority", "recovered"]) assert.equal((await compare({ op: "drop", id })).ok, true);
  assert.equal((await compare({ op: "inspect", id: "authority" })).ok, false);
  report.passed = true;
} catch (error) {
  report.error = error.stack;
  process.exitCode = 1;
} finally {
  native?.stdin.end();
  writeFileSync(path.join(out, "result.json"), JSON.stringify(report, null, 2) + "\n");
  console.log(JSON.stringify({ passed: report.passed, steps: report.steps.length, result: path.join(out, "result.json"), error: report.error }));
}
