import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { readFile, mkdir, writeFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";
import { buildBrowserRuntime } from "../build/browser-runtime.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const execute = promisify(execFile);

test("multi-owner Go snapshots replay exactly through the actual browser Wasm bridge", { timeout: 60000 }, async () => {
  const output = join(root, ".local/pokepath/verification/tower-order", `wasm-${randomUUID().slice(0, 8)}`);
  await mkdir(output, { recursive: true });
  const nativePath = join(output, "native.json");
  const report = { passed: false, command: "node --test games/pokepath/tests/tower-order-wasm-e2e.test.mjs", scope: "Complete multi-tower native states and projections replayed through the same independent Go/Wasm engine; browser network timing remains a separate gate", comparisons: 0, cases: [] };
  try {
    await execute("go", ["test", "./games/pokepath/engine", "-run", "^TestCoreTowerOrderingReferenceFixtureE2E$", "-count=1"], { cwd: root, env: { ...process.env, WIRE_BOARD_POKEPATH_TOWER_ORDER_OUT: nativePath }, timeout: 40000 });
    const native = JSON.parse(await readFile(nativePath));
    const artifact = buildBrowserRuntime();
    report.rulesVersion = native.rulesVersion;
    report.runtimeBuildSHA256 = artifact.manifest.buildSHA256;
    report.runtimeSourceSHA256 = artifact.manifest.sourceSHA256;
    await import(pathToFileURL(join(artifact.runtimeRoot, "wasm_exec.js")).href);
    const go = new globalThis.Go();
    const wasm = await WebAssembly.instantiate(await readFile(join(artifact.runtimeRoot, "engine.wasm")), go.importObject);
    void go.run(wasm.instance);
    const request = value => { const response = JSON.parse(globalThis.wireBoardPokepathNative.request(JSON.stringify(value))); assert.equal(response.ok, true, response.error); return response.value; };
    request({ op: "installCatalog", data: JSON.parse(await readFile(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json"))) });
    request({ op: "installRoute", data: JSON.parse(await readFile(join(root, "games/pokepath/assets/public/pokepath/v1/routes/0.json"))) });
    for (const scenario of native.cases) {
      let handle = request({ op: "session.import", state: scenario.initial });
      try {
        assert.deepEqual(request({ op: "session.export", handle: handle.nativeHandle }), scenario.initial);
        report.comparisons++;
        for (const [index, frame] of scenario.frames.entries()) {
          const previous = handle;
          handle = request({ op: "session.advance", handle: previous.nativeHandle, milliseconds: frame.duration });
          request({ op: "session.release", handles: [previous.nativeHandle] });
          assert.deepEqual(request({ op: "session.export", handle: handle.nativeHandle }), frame.state, `${scenario.input.name}: complete state after frame ${index + 1}`);
          assert.deepEqual(request({ op: "session.project", handle: handle.nativeHandle }), frame.view, `${scenario.input.name}: transient projection after frame ${index + 1}`);
          report.comparisons += 2;
        }
      } finally { request({ op: "session.release", handles: [handle.nativeHandle] }); }
      report.cases.push({ name: scenario.input.name, passed: true, frames: scenario.frames.length });
    }
    assert.equal(report.cases.length, 71);
    assert.equal(report.comparisons, 731);
    report.passed = true;
  } finally {
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
    await writeFile(join(root, ".local/pokepath/verification/tower-order/latest-wasm.json"), JSON.stringify({ output, passed: report.passed }) + "\n");
  }
});
