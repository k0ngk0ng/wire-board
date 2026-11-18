import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { pathToFileURL, fileURLToPath } from "node:url";
import { join } from "node:path";
import { buildBrowserRuntime } from "../build/browser-runtime.mjs";
import { createPredictionController, predictionBudgets } from "../web/prediction.ts";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = join(root, ".local/pokepath/verification");
mkdirSync(output, { recursive: true });
const report = { command: "node --test games/pokepath/tests/prediction-native-e2e.test.mjs", passed: false, scope: "Independent Go/Wasm rules; formal initial resources and actions, simulated native RAF clocks; real authenticated browser metrics are separate", scenarios: [] };
let runtimePromise;

async function nativeRuntime() {
  runtimePromise ??= (async () => {
    const artifact = buildBrowserRuntime();
    await import(pathToFileURL(join(artifact.runtimeRoot, "wasm_exec.js")).href);
    const go = new globalThis.Go();
    const wasm = await WebAssembly.instantiate(readFileSync(join(artifact.runtimeRoot, "engine.wasm")), go.importObject);
    void go.run(wasm.instance);
    const bridge = globalThis.wireBoardPokepathNative;
    assert.equal(bridge?.ready, true);
    const request = value => {
      const response = JSON.parse(bridge.request(JSON.stringify(value)));
      if (!response.ok) throw Error(response.error);
      return response.value;
    };
    request({ op: "installCatalog", data: JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json"), "utf8")) });
    request({ op: "installRoute", data: JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/routes/0.json"), "utf8")) });
    return {
      buildSHA256: artifact.manifest.buildSHA256,
      create: (options, players, seed, now) => request({ op: "new", options, players, seed, now }),
      apply: (state, actor, action) => request({ op: "apply", state, actor, action }),
      advance: (state, milliseconds) => request({ op: "advance", state, milliseconds }),
      project: state => request({ op: "project", state }),
      clone: state => request({ op: "clone", state }),
    };
  })();
  return runtimePromise;
}

function controllerFor(runtime, state, actor) {
  const callbacks = new Map();
  const costs = [];
  const frames = [];
  const sent = [];
  let wall = 0;
  let serial = 0;
  const origin = performance.now();
  const controller = createPredictionController({
    runtime,
    actor,
    readClock: state => state.clock,
    readPaused: state => state.paused,
    now: () => origin + wall,
    measureNow: () => performance.now(),
    requestFrame: callback => { callbacks.set(++serial, callback); return serial; },
    cancelFrame: id => callbacks.delete(id),
    send: (action, clientInputID) => sent.push({ action, clientInputID }),
    onFrame: (view, metadata) => frames.push({ view, metadata }),
    requestSync: reason => { throw Error(`Unexpected recovery: ${reason}`); },
    onCost: sample => costs.push(sample),
  });
  controller.reset({ state, epoch: state.epoch, sequence: 0, logicalMS: state.clock });
  const at = milliseconds => {
    wall = milliseconds;
    const queued = [...callbacks.values()];
    callbacks.clear();
    for (const callback of queued) callback(origin + milliseconds);
  };
  return { controller, costs, frames, sent, at, setWall(value) { wall = value; }, get wall() { return wall; } };
}

function percentile(values, amount) {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * amount))] ?? 0;
}

test("native placement, predicted upgrade and authority echo use one resource transaction", async () => {
  const runtime = await nativeRuntime();
  let state = runtime.create({ starter: "charmander" }, 2, 123456, 1730000000000);
  let view = runtime.project(state);
  const pokemonID = view.team[0].id;
  const tileID = view.tiles.find(tile => tile.terrain === 1).id;
  state = runtime.apply(state, 0, { type: "deploy", epoch: state.epoch, pokemonID, tileID });
  const before = structuredClone(state);
  const client = controllerFor(runtime, state, 0);
  const result = client.controller.input({ type: "upgrade", pokemonID }, "native:upgrade:1");
  assert.equal(result.accepted, true);
  assert.equal(client.controller.view().team[0].level, 2);
  assert.ok(client.controller.view().gold < view.gold);
  assert.deepEqual(state, before);
  const action = client.sent[0].action;
  state = runtime.apply(state, 0, action);
  state = runtime.apply(state, 1, { type: "wave", epoch: state.epoch });
  state = runtime.advance(state, 100);
  client.controller.receive({ epoch: state.epoch, sequence: 3, fromSequence: 0, logicalMS: 100, operations: [{ type: "action", actor: 0, clientInputID: "native:upgrade:1", action }, { type: "action", actor: 1, action: { type: "wave", epoch: state.epoch } }, { type: "tick", milliseconds: 100 }] });
  assert.equal(client.controller.sample().pendingInputs, 0);
  assert.deepEqual(client.controller.authorityState(), state);
  assert.equal(client.controller.view().gold, state.gold);
  report.scenarios.push({ name: "native-upgrade-confirmation", passed: true, buildSHA256: runtime.buildSHA256, initialGold: before.gold, finalGold: state.gold, level: state.team[0].level });
  client.controller.destroy();
});

test("different native display phases and a delayed peer preserve the same canonical world", async () => {
  const runtime = await nativeRuntime();
  let state = runtime.create({ starter: "froaki" }, 2, 87654321, 1730000000000);
  state = runtime.apply(state, 0, { type: "wave", epoch: state.epoch });
  const fast = controllerFor(runtime, state, 0);
  const slow = controllerFor(runtime, state, 1);
  let sequence = 0;
  for (let frame = 0; frame <= 120; frame++) {
    const milliseconds = frame * 1000 / 60;
    fast.setWall(milliseconds);
    slow.setWall(milliseconds);
    if (frame > 0 && frame % 6 === 0) {
      sequence++;
      state = runtime.advance(state, 100);
      fast.controller.receive({ epoch: state.epoch, sequence, fromSequence: sequence - 1, logicalMS: state.clock, operations: [{ type: "tick", milliseconds: 100 }] });
      if (sequence % 5 === 0) slow.controller.receive({ epoch: state.epoch, sequence, fromSequence: sequence - 5, logicalMS: state.clock, operations: Array.from({ length: 5 }, () => ({ type: "tick", milliseconds: 100 })) });
    }
    fast.at(milliseconds);
    slow.at(milliseconds);
    assert.ok(fast.controller.sample().leadMS <= predictionBudgets.maxLeadMS);
    assert.ok(slow.controller.sample().leadMS <= predictionBudgets.maxLeadMS);
  }
  assert.deepEqual(fast.controller.authorityState(), state);
  assert.deepEqual(slow.controller.authorityState(), state);
  const copies = [...fast.costs, ...slow.costs].filter(value => value.phase === "clone").map(value => value.milliseconds);
  const copyP95 = percentile(copies, 0.95);
  report.scenarios.push({ name: "native-two-peer-phases", passed: copyP95 <= predictionBudgets.normalCopyMS, confirmedClock: state.clock, nativeFrames: [fast.frames.length, slow.frames.length], normalCloneP95MS: copyP95, normalCloneBudgetMS: predictionBudgets.normalCopyMS });
  assert.ok(copyP95 <= predictionBudgets.normalCopyMS, `normal native copy p95 ${copyP95}ms exceeds frozen ${predictionBudgets.normalCopyMS}ms budget`);
  fast.controller.destroy();
  slow.controller.destroy();
});

test("dense native copy is independent and remains inside its frozen budget", async () => {
  const runtime = await nativeRuntime();
  let state = runtime.create({ starter: "charmander" }, 4, 123456, 1730000000000);
  state = runtime.apply(state, 0, { type: "wave", epoch: state.epoch });
  assert.ok(state.enemies.length > 0);
  const first = state.enemies[0];
  const dense = structuredClone(state);
  dense.enemies = Array.from({ length: 200 }, (_, index) => ({ ...structuredClone(first), id: state.nextID + index }));
  dense.nextID = state.nextID + dense.enemies.length;
  const baseline = structuredClone(dense);
  const timings = [];
  let copy;
  for (let index = 0; index < 40; index++) {
    const started = performance.now();
    copy = runtime.clone(dense);
    timings.push(performance.now() - started);
  }
  assert.deepEqual(dense, baseline);
  copy.enemies[0].x += 100;
  assert.equal(dense.enemies[0].x, first.x);
  const p95 = percentile(timings, 0.95);
  const advanceTimes = [];
  const projectTimes = [];
  for (let index = 0; index < 20; index++) {
    const started = performance.now();
    const advanced = runtime.advance(dense, 25);
    const between = performance.now();
    runtime.project(advanced);
    const ended = performance.now();
    advanceTimes.push(between - started);
    projectTimes.push(ended - between);
  }
  report.scenarios.push({ name: "synthetic-dense-copy", scope: "200 cloned formal enemy records with distinct IDs; copy isolation/cost only, not a gameplay or real room performance claim", passed: p95 <= predictionBudgets.denseCopyMS, enemies: dense.enemies.length, stateBytes: Buffer.byteLength(JSON.stringify(dense)), copyP95MS: p95, budgetMS: predictionBudgets.denseCopyMS, advanceP95MS: percentile(advanceTimes, 0.95), projectP95MS: percentile(projectTimes, 0.95) });
  assert.ok(p95 <= predictionBudgets.denseCopyMS, `dense native copy p95 ${p95}ms exceeds frozen ${predictionBudgets.denseCopyMS}ms budget`);
});

test.after(() => {
  report.passed = report.scenarios.length === 3 && report.scenarios.every(value => value.passed);
  writeFileSync(join(output, "prediction-native-result.json"), JSON.stringify(report, null, 2) + "\n");
});
