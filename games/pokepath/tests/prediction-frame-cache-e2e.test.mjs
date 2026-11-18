import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import { join } from "node:path";
import { buildBrowserRuntime } from "../build/browser-runtime.mjs";
import { createNativePredictionRuntime } from "../web/native-runtime.ts";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = join(root, ".local/pokepath/verification/frame-cache");
mkdirSync(output, { recursive: true });
test("render-only projection reuse preserves the complete actual Wasm view and immutable branches", async () => {
  const report = { passed: false, command: "node --experimental-strip-types --test games/pokepath/tests/prediction-frame-cache-e2e.test.mjs", comparisons: 0 };
  const artifact = buildBrowserRuntime();
  await import(pathToFileURL(join(artifact.runtimeRoot, "wasm_exec.js")).href);
  const go = new globalThis.Go();
  const wasm = await WebAssembly.instantiate(readFileSync(join(artifact.runtimeRoot, "engine.wasm")), go.importObject);
  void go.run(wasm.instance);
  const counts = new Map();
  const request = value => {
    counts.set(value.op, (counts.get(value.op) ?? 0) + 1);
    const response = JSON.parse(globalThis.wireBoardPokepathNative.request(JSON.stringify(value)));
    assert.equal(response.ok, true, response.error);
    return response.value;
  };
  const catalog = JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json")));
  request({ op: "installCatalog", data: catalog });
  request({ op: "installRoute", data: JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/routes/0.json"))) });
  const runtime = createNativePredictionRuntime(request, () => catalog);
  const initial = request({ op: "new", options: { starter: "charmander" }, players: 2, seed: 123456, now: 1730000000000 });
  const handles = [];
  const keep = handle => { handles.push(handle); return handle; };
  const compare = handle => {
    const frame = runtime.frame(handle);
    assert.deepEqual(frame, runtime.project(handle));
    assert.equal(Object.isFrozen(frame), true);
    assert.equal(Object.isFrozen(frame.team[0].stats), true);
    report.comparisons++;
    return frame;
  };
  try {
    assert.equal(typeof runtime.frame, "function", "the presentation-only cached projector is unavailable");
    const baseline = keep(runtime.import(initial));
    const first = compare(baseline);
    const projectionsBefore = counts.get("session.project");
    for (let index = 1; index <= 10; index++) {
      const next = keep(runtime.advance(baseline, index));
      assert.equal(next.tick, baseline.tick);
      const frame = runtime.frame(next);
      assert.equal(frame.clock, index);
      assert.equal(frame.remainderMS, next.remainderMS);
      assert.equal(frame.enemies, first.enemies);
    }
    assert.equal(counts.get("session.project"), projectionsBefore);
    assert.throws(() => { first.team[0].stats.power = -1; });
    assert.deepEqual(runtime.export(baseline), initial);
    compare(keep(runtime.advance(baseline, 30)));
    const branch = keep(runtime.apply(baseline, 1, { type: "upgrade", epoch: initial.epoch, pokemonID: initial.team[0].id }));
    const changed = compare(branch);
    assert.notEqual(changed.gold, first.gold);
    assert.equal(runtime.frame(baseline).gold, first.gold);
    assert.throws(() => runtime.apply(baseline, 1, { type: "unknown", epoch: initial.epoch }));
    assert.deepEqual(runtime.export(baseline), initial);
    const paused = keep(runtime.apply(branch, 0, { type: "pause", epoch: initial.epoch }));
    compare(paused);
    compare(keep(runtime.advance(paused, 100)));
    let minute = keep(runtime.advance(baseline, 1));
    minute = keep(runtime.apply(minute, 0, { type: "speed", epoch: initial.epoch }));
    minute = keep(runtime.advance(minute, 59998));
    compare(minute);
    const boundary = keep(runtime.advance(minute, 1));
    assert.equal(boundary.battleFrame, minute.battleFrame + 1);
    assert.equal(boundary.tick, 72000);
    assert.equal(compare(boundary).progression.profileStats.timePlayed, 1);
    const imported = keep(runtime.import({ ...initial, gold: 49 }));
    assert.equal(compare(imported).gold, 49);
    const released = keep(runtime.clone(imported));
    runtime.frame(released);
    runtime.release(released);
    assert.throws(() => runtime.frame(released));
    report.buildSHA256 = artifact.manifest.buildSHA256;
    report.sourceSHA256 = artifact.manifest.sourceSHA256;
    report.passed = true;
  } finally {
    for (const handle of handles) runtime.release(handle);
    runtime.dispose();
    assert.equal(runtime.liveStates, 0);
    writeFileSync(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
