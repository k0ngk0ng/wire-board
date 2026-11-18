import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import { join } from "node:path";
import { buildBrowserRuntime } from "../build/browser-runtime.mjs";
import * as nativeModule from "../web/native-runtime.ts";
import { createPredictionController, predictionBudgets } from "../web/prediction.ts";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = join(root, ".local/pokepath/verification");
mkdirSync(output, { recursive: true });
const report = { command: "node --test games/pokepath/tests/prediction-session-e2e.test.mjs", passed: false, scope: "Independent Go/Wasm opaque prediction states; real authenticated Canvas FPS is measured separately", scenarios: [] };
let setupPromise;

async function setup() {
  setupPromise ??= (async () => {
    assert.equal(typeof nativeModule.createNativePredictionRuntime, "function", "the opaque runtime factory must be testable without a DOM loader");
    const artifact = buildBrowserRuntime();
    await import(pathToFileURL(join(artifact.runtimeRoot, "wasm_exec.js")).href);
    const go = new globalThis.Go();
    const wasm = await WebAssembly.instantiate(readFileSync(join(artifact.runtimeRoot, "engine.wasm")), go.importObject);
    void go.run(wasm.instance);
    const bridge = globalThis.wireBoardPokepathNative;
    const request = value => {
      const response = JSON.parse(bridge.request(JSON.stringify(value)));
      if (!response.ok) throw Error(response.error);
      return response.value;
    };
    const catalog = JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json"), "utf8"));
    request({ op: "installCatalog", data: catalog });
    request({ op: "installRoute", data: JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/routes/0.json"), "utf8")) });
    return { request, session: () => nativeModule.createNativePredictionRuntime(request, () => catalog), newState: () => request({ op: "new", options: { starter: "charmander" }, players: 2, seed: 123456, now: 1730000000000 }), buildSHA256: artifact.manifest.buildSHA256 };
  })();
  return setupPromise;
}

const count = runtime => typeof runtime.liveStates === "function" ? runtime.liveStates() : runtime.liveStates;
const percentile = (values, fraction) => { const sorted = [...values].sort((a, b) => a - b); return sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * fraction))] ?? 0; };

test("opaque Wasm frames expose the same fractional phase used by the live board", async context => {
  const value = await setup();
  const runtime = value.session();
  context.after(() => runtime.dispose());
  let state = value.request({ op: "new", options: { starter: "charmander", speed: 1 }, players: 2, seed: 123456, now: 1730000000000 });
  for (const sample of [{ elapsed: 17, step: 1, interpolation: .02 }, { elapsed: 33, step: 1, interpolation: .98 }, { elapsed: 34, step: 2, interpolation: .04 }, { elapsed: 50, step: 3, interpolation: 0 }]) {
    state = value.request({ op: "advance", state, milliseconds: sample.elapsed - state.clock });
    const handle = runtime.import(state);
    const actual = nativeModule.readBattlePhysics(handle);
    assert.equal(actual.step, sample.step);
    assert.ok(Math.abs(actual.interpolation - sample.interpolation) < 1e-12);
    runtime.release(handle);
  }
});

test("slow and fast playback keep the original real frame phase in retained handles", async context => {
  const value = await setup();
  const runtime = value.session();
  context.after(() => runtime.dispose());
  for (const speed of [.4, .8, 1.2, 1.7, 2.5, 5]) {
    let state = value.request({ op: "new", options: { starter: "charmander", speed }, players: 2, seed: 123456, now: 1730000000000 });
    state = value.request({ op: "advance", state, milliseconds: 17 });
    const handle = runtime.import(state);
    const actual = nativeModule.readBattlePhysics(handle);
    assert.equal(actual.step, 1);
    assert.ok(Math.abs(actual.interpolation - .02) < 1e-12);
    assert.equal(handle.physicsClock, Math.floor(speed * 1000 / 60));
    runtime.release(handle);
  }
});

test("opaque native states retain exact full projections while releasing every replaced state", async context => {
  const value = await setup();
  const runtime = value.session();
  let state = value.newState();
  state = value.request({ op: "apply", state, actor: 1, action: { type: "wave", epoch: state.epoch } });
  const imported = runtime.import(state);
  const callbacks = new Map();
  let serial = 0;
  let wall = 0;
  const costs = [];
  const client = createPredictionController({ runtime, actor: 0, readClock: handle => handle.clock, readPaused: handle => handle.paused, readPhysics: nativeModule.readBattlePhysics, now: () => wall, measureNow: () => performance.now(), requestFrame: callback => { callbacks.set(++serial, callback); return serial; }, cancelFrame: id => callbacks.delete(id), send() {}, onFrame() {}, onCost: sample => costs.push(sample), requestSync: reason => { throw Error(reason); } });
  context.after(() => { client.destroy(); runtime.dispose(); });
  client.reset({ state: imported, epoch: state.epoch, sequence: 0, logicalMS: 0 });
  runtime.release(imported);
  assert.equal(count(runtime), 2);
  for (let frame = 0; frame <= 120; frame++) {
    wall = frame * 1000 / 60;
    if (frame > 0 && frame % 6 === 0) {
      const sequence = frame / 6;
      state = value.request({ op: "advance", state, milliseconds: 100 });
      client.receive({ epoch: state.epoch, sequence, fromSequence: sequence - 1, logicalMS: state.clock, operations: [{ type: "tick", milliseconds: 100 }] });
      assert.deepEqual(runtime.export(client.authorityState()), state);
      assert.deepEqual(runtime.project(client.authorityState()), value.request({ op: "project", state }));
    }
    const list = [...callbacks.values()];
    callbacks.clear();
    for (const callback of list) callback(wall);
    assert.equal(count(runtime), 2, `frame ${frame}`);
  }
  const copies = costs.filter(value => value.phase === "clone").map(value => value.milliseconds);
  const copyP95MS = percentile(copies, 0.95);
  report.scenarios.push({ name: "opaque-native-replay", passed: copyP95MS <= predictionBudgets.normalCopyMS, confirmedClock: state.clock, liveStates: count(runtime), copyP95MS, copyBudgetMS: predictionBudgets.normalCopyMS, buildSHA256: value.buildSHA256 });
  assert.ok(copyP95MS <= predictionBudgets.normalCopyMS);
  client.destroy();
  assert.equal(count(runtime), 0);
  runtime.dispose();
});

test("independent opaque sessions cannot release or project another session's state", async context => {
  const value = await setup();
  const first = value.session();
  const second = value.session();
  context.after(() => { first.dispose(); second.dispose(); });
  const state = value.newState();
  const handle = first.import(state);
  assert.throws(() => second.project(handle));
  second.release(handle);
  assert.deepEqual(first.export(handle), state);
  first.release(handle);
  first.release(handle);
  assert.equal(count(first), 0);
  first.dispose();
  second.dispose();
  report.scenarios.push({ name: "opaque-session-isolation", passed: true });
});

test("dense opaque native frame work avoids full-state JSON round trips", async context => {
  const value = await setup();
  const runtime = value.session();
  context.after(() => runtime.dispose());
  let state = value.newState();
  state = value.request({ op: "apply", state, actor: 0, action: { type: "wave", epoch: state.epoch } });
  const first = state.enemies[0];
  state.enemies = Array.from({ length: 200 }, (_, index) => ({ ...structuredClone(first), id: state.nextID + index }));
  state.nextID += state.enemies.length;
  const baseline = runtime.import(state);
  const copies = [];
  const advances = [];
  const projections = [];
  for (let index = 0; index < 40; index++) {
    const started = performance.now();
    const cloned = runtime.clone(baseline);
    const copied = performance.now();
    const advanced = runtime.advance(cloned, 25);
    const stepped = performance.now();
    runtime.project(advanced);
    const projected = performance.now();
    copies.push(copied - started);
    advances.push(stepped - copied);
    projections.push(projected - stepped);
    runtime.release(cloned);
    runtime.release(advanced);
  }
  assert.equal(count(runtime), 1);
  assert.deepEqual(runtime.export(baseline), state);
  const copyP95MS = percentile(copies, 0.95);
  const advanceP95MS = percentile(advances, 0.95);
  const projectP95MS = percentile(projections, 0.95);
  report.scenarios.push({ name: "synthetic-opaque-dense", scope: "200 cloned formal enemy records; copy/RPC workload only, not gameplay or real browser FPS", passed: copyP95MS <= predictionBudgets.denseCopyMS, enemies: 200, copyP95MS, copyBudgetMS: predictionBudgets.denseCopyMS, advanceP95MS, projectP95MS });
  assert.ok(copyP95MS <= predictionBudgets.denseCopyMS);
  runtime.release(baseline);
  runtime.dispose();
});

test("compact enemy defaults restore every full native field without changing retained states", async context => {
  const value = await setup();
  const packets = [];
  const runtime = nativeModule.createNativePredictionRuntime(request => {
    const result = value.request(request);
    if (request.op === "session.project") packets.push({ compact: Boolean(request.compact), bytes: Buffer.byteLength(JSON.stringify(result)), result });
    return result;
  }, () => JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json"))));
  context.after(() => runtime.dispose());
  let state = value.newState();
  state = value.request({ op: "apply", state, actor: 0, action: { type: "wave", epoch: state.epoch } });
  const model = state.enemies[0];
  state.enemies = Array.from({ length: 200 }, (_, index) => {
    const enemy = { ...structuredClone(model), id: state.nextID + index };
    if (index % 4 === 0) Object.assign(enemy, { health: 25.5, armor: 250, maxArmor: 250, shield: 7, invisible: true, canBurn: false, lastIncomingDamage: 4.5, lastPreHitHp: 30, lastHPDamage: 4.5, lastResolvedDamage: 4.5, lastHitEvaded: true, lastHitCritical: true, lastHitTerrain: 4, lastDamageSourceID: state.team[0].id, lastHitBy: state.team[0].id, lastDamageKind: "direct", lastHitAbilityID: "中文特性", lastHitItemID: "引号\"和\\路径" });
    if (index % 7 === 0) Object.assign(enemy, { criticalBlocks: 3, stunApplications: 8, healingEvents: 5, cloudNine: true, passiveFlags: { sample: true }, passiveTimers: { sample: 17 }, passiveValues: { sample: .5 }, passiveCounters: { sample: 2 }, conditions: [{ kind: "slow", durationMs: 2500, remainingMs: 1100, magnitude: .4, stacks: 1, tickElapsedMs: 0, sourcePokemonID: state.team[0].id, sourceItemID: "加速器" }] });
    return enemy;
  });
  state.nextID += state.enemies.length;
  const handle = runtime.import(state);
  const full = value.request({ op: "project", state });
  assert.deepEqual(runtime.project(handle), full);
  const compact = runtime.project(handle);
  assert.deepEqual(compact, full);
  assert.deepEqual(runtime.export(handle), state);
  const short = packets.at(-1), first = packets[0];
  assert.equal(short.compact, true);
  const fraction = short.bytes / first.bytes;
  const timing = [];
  for (let index = 0; index < 30; index++) {
    const begin = performance.now();
    assert.deepEqual(runtime.project(handle), full);
    timing.push(performance.now() - begin);
  }
  report.scenarios.push({ name: "compact-enemy-complete-projection", passed: fraction < .65, enemies: state.enemies.length, fullBytes: first.bytes, compactBytes: short.bytes, fraction, projectP95MS: percentile(timing, .95), buildSHA256: value.buildSHA256 });
  assert.ok(fraction < .65, `compact/full bytes = ${fraction}`);
  compact.enemies[0].conditions.push({ kind: "poison" });
  compact.enemies[1].conditions.push({ kind: "poison" });
  assert.deepEqual(compact.enemies[2].conditions, full.enemies[2].conditions);
  assert.deepEqual(runtime.project(handle), full);
  runtime.release(handle);
});

test("retained geometry is reused for ordinary input and replaced by a custom route", async context => {
  const value = await setup();
  const packets = [];
  const runtime = nativeModule.createNativePredictionRuntime(input => {
    if (input.op === "session.project") packets.push({ ...input });
    return value.request(input);
  }, () => JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json"))));
  context.after(() => runtime.dispose());
  let state = value.newState();
  let handle = runtime.import(state);
  assert.deepEqual(runtime.project(handle), value.request({ op: "project", state }));
  for (const action of [{ type: "pause" }, { type: "resume" }, { type: "custom", settings: { mapId: 0, hearts: 8, health: 0, armor: 0, wave: [{ enemy: "rattata", amount: 200 }] } }, { type: "custom_exit" }]) {
    const normalized = { ...action, epoch: state.epoch };
    state = value.request({ op: "apply", state, actor: 0, action: normalized });
    const next = runtime.apply(handle, 0, normalized);
    runtime.release(handle);
    handle = next;
    const before = packets.length;
    const actual = runtime.project(handle);
    assert.deepEqual(actual, value.request({ op: "project", state }));
    assert.equal(packets.length, before + 1);
    assert.equal(packets.at(-1).compact, action.type === "pause" || action.type === "resume", action.type);
  }
  report.scenarios.push({ name: "geometry-reuse-and-custom-invalidation", passed: true, requests: packets.length, buildSHA256: value.buildSHA256 });
  runtime.release(handle);
});

test("sparse enemy projection preserves signed zero coordinates and damage values", async context => {
  const value = await setup();
  const runtime = nativeModule.createNativePredictionRuntime(input => {
    if (input.op !== "session.import") return value.request(input);
    let raw = JSON.stringify(input);
    for (const field of ["x", "y", "distance", "armor", "worldSandMultiplier"]) raw = raw.replace(`"${field}":0,`, `"${field}":-0,`);
    const result = JSON.parse(globalThis.wireBoardPokepathNative.request(raw));
    assert.ok(result.ok, result.error);
    return result.value;
  }, () => JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json"))));
  context.after(() => runtime.dispose());
  let state = value.newState();
  state = value.request({ op: "apply", state, actor: 0, action: { type: "wave", epoch: state.epoch } });
  state.enemies[0].x = 0;
  state.enemies[0].y = 0;
  const handle = runtime.import(state);
  const full = runtime.project(handle);
  assert.equal(Object.is(full.enemies[0].x, -0), true);
  assert.deepEqual(runtime.project(handle), full);
  runtime.release(handle);
});

test("opaque single owned Pokemon inspection reads Box combat values without mutating the world", async context => {
  const value = await setup();
  const runtime = value.session();
  const other = value.session();
  context.after(() => { runtime.dispose(); other.dispose(); });
  const state = value.newState();
  state.box.push(state.team.pop());
  state.inventory.push({ id: "protein", equippedBy: 0 }, { id: "charizarditeX", equippedBy: 0 });
  state.progression.shopStock = state.progression.shopStock.map(key => state.inventory.some(item => item.id === key) ? "" : key);
  const handle = runtime.import(state);
  const before = runtime.export(handle);
  const inspected = runtime.inspect(handle, state.box[0].id);
  assert.ok(inspected.combatStats);
  assert.equal(inspected.id, state.box[0].id);
  assert.equal(inspected.combatStats.power, state.box[0].stats.power);
  assert.deepEqual(inspected.equipableItems, ["protein"]);
  inspected.combatStats.power = -1;
  assert.equal(runtime.inspect(handle, state.box[0].id).combatStats.power, state.box[0].stats.power);
  assert.deepEqual(runtime.export(handle), before);
  assert.equal(runtime.liveStates, 1);
  const pressured = structuredClone(state);
  pressured.world.pressure = true;
  const locked = runtime.import(pressured);
  assert.deepEqual(runtime.inspect(locked, state.box[0].id).equipableItems ?? [], []);
  runtime.release(locked);
  assert.throws(() => runtime.inspect(handle, 999999));
  assert.throws(() => other.inspect(handle, state.box[0].id));
  runtime.release(handle);
  assert.throws(() => runtime.inspect(handle, state.box[0].id));
  report.scenarios.push({ name: "single-box-combat-inspection", passed: true, buildSHA256: value.buildSHA256 });
});

test.after(() => {
  report.passed = report.scenarios.length === 6 && report.scenarios.every(value => value.passed);
  writeFileSync(join(output, "prediction-session-result.json"), JSON.stringify(report, null, 2) + "\n");
});
