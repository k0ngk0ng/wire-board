import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync, readdirSync, realpathSync, writeFileSync } from "node:fs";
import { join, relative, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const directory = join(root, ".local/pokepath/verification/native-attack-modes");
const frozen = process.env.POKEPATH_PARITY_FROZEN === "1";
const existingRuntimeRoot = process.env.POKEPATH_PARITY_RUNTIME_ROOT;
const expectedRulesRevision = process.env.POKEPATH_PARITY_RULES_REVISION;
const expectedFactsSHA256 = process.env.POKEPATH_PARITY_FACTS_SHA256;
const report = { command: "node --test games/pokepath/tests/native-attack-modes-wasm.test.mjs", passed: false, comparisons: 0, scenarios: [], scope: "Same explicit native Go snapshots and inputs replayed through the actual browser Go/Wasm bridge" };

function reuseRuntime(path) {
  const runtimeRoot = realpathSync(resolve(path));
  const local = relative(realpathSync(root), runtimeRoot);
  assert.ok(local !== ".." && !local.startsWith(`..${sep}`) && !local.startsWith(sep), "existing runtime must belong to this workspace");
  const manifest = JSON.parse(readFileSync(join(runtimeRoot, "manifest.json"), "utf8"));
  assert.equal(manifest.abi, "wire-board-pokepath-go-v1");
  assert.equal(manifest.engine, "wire-board-pokepath-go");
  const paths = new Set(["engine.wasm", "wasm_exec.js", "go-runtime-LICENSE.txt"]);
  for (const entry of manifest.files) {
    assert.ok(paths.delete(entry.path), `unexpected or duplicate runtime file ${entry.path}`);
    const bytes = readFileSync(join(runtimeRoot, entry.path));
    assert.equal(bytes.length, entry.bytes, entry.path);
    assert.equal(createHash("sha256").update(bytes).digest("hex"), entry.sha256, entry.path);
  }
  assert.equal(paths.size, 0, "existing runtime manifest omitted a required artifact");
  for (const entry of manifest.sourceFiles) {
    if (!entry.path.startsWith("games/pokepath/engine/") && !entry.path.startsWith("games/pokepath/cmd/browser/")) continue;
    const bytes = readFileSync(join(root, entry.path));
    assert.equal(bytes.length, entry.bytes, entry.path);
    assert.equal(createHash("sha256").update(bytes).digest("hex"), entry.sha256, `frozen source ${entry.path}`);
  }
  report.reusedExistingRuntime = true;
  report.runtimeRoot = runtimeRoot;
  report.runtimeFiles = manifest.files;
  return { runtimeRoot, manifest };
}

test("native attack modes replay the same full state and projection in Go and Wasm", async context => {
  context.after(() => writeFileSync(join(directory, frozen ? "final-frozen-wasm-parity.json" : "wasm-parity.json"), JSON.stringify(report, null, 2) + "\n"));
  const paths = readdirSync(directory).filter(path => /^parity-[A-Za-z]+\.json$/.test(path)).sort();
  assert.equal(paths.length, 13, "run TestNativeAttackModeProducesGoWasmParityTracesE2E before this replay");
  if (frozen) {
    assert.ok(existingRuntimeRoot, "frozen verification requires an existing runtime and cannot compile one");
    assert.match(expectedRulesRevision ?? "", /^[a-f0-9]{64}$/);
    assert.match(expectedFactsSHA256 ?? "", /^[a-f0-9]{64}$/);
  }
  const artifact = existingRuntimeRoot ? reuseRuntime(existingRuntimeRoot) : (await import("../build/browser-runtime.mjs")).buildBrowserRuntime();
  report.buildSHA256 = artifact.manifest.buildSHA256;
  report.sourceSHA256 = artifact.manifest.sourceSHA256;
  await import(pathToFileURL(join(artifact.runtimeRoot, "wasm_exec.js")).href);
  const go = new globalThis.Go();
  const wasm = await WebAssembly.instantiate(readFileSync(join(artifact.runtimeRoot, "engine.wasm")), go.importObject);
  void go.run(wasm.instance);
  const request = value => {
    const result = JSON.parse(globalThis.wireBoardPokepathNative.request(JSON.stringify(value)));
    assert.equal(result.ok, true, result.error);
    return result.value;
  };
  request({ op: "installCatalog", data: JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json"), "utf8")) });
  request({ op: "installRoute", data: JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/routes/0.json"), "utf8")) });
  if (frozen) {
    const identity = `pokepath-native-1/${expectedRulesRevision}/${expectedFactsSHA256}`;
    const initial = request({ op: "new", options: { starter: "charmander" }, players: 4, seed: 123456, now: 1794919872358 });
    assert.equal(initial.rulesVersion, identity, "existing Wasm does not carry the frozen engine and facts identity");
    report.rulesRevision = expectedRulesRevision;
    report.factsSHA256 = expectedFactsSHA256;
    report.rulesVersion = identity;
  }
  for (const path of paths) {
    const trace = JSON.parse(readFileSync(join(directory, path), "utf8"));
    if (frozen) assert.equal(trace.initial.rulesVersion, report.rulesVersion, `stale native trace ${trace.species}`);
    let state = request({ op: "restore", state: trace.initial });
    assert.deepEqual(state, trace.initial, `${trace.species}: initial full state`);
    for (const [index, operation] of trace.operations.entries()) {
      if (operation.kind === "apply") state = request({ op: "apply", state, actor: operation.actor, action: operation.action });
      else state = request({ op: "advance", state, milliseconds: operation.milliseconds });
      assert.deepEqual(state, operation.state, `${trace.species}: full state after operation ${index}`);
      assert.deepEqual(request({ op: "project", state }), operation.view, `${trace.species}: native projection after operation ${index}`);
      report.comparisons += 2;
    }
    report.scenarios.push({ species: trace.species, passed: true, operations: trace.operations.length, finalClock: state.clock, finalPhysicsClock: state.tick });
  }
  report.passed = true;
});
