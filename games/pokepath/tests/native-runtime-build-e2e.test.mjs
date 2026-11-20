import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createHash, randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { existsSync } from "node:fs";

const execute = promisify(execFile);
const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = join(root, ".local/pokepath/verification/native-runtime");
const environment = { ...process.env, GOCACHE: join(root, ".local/go-build"), GOMODCACHE: join(root, ".local/go-mod"), GOTMPDIR: join(root, ".local/tmp"), TMPDIR: join(root, ".local/tmp"), TEST_TELEMETRY_DIR: join(root, ".local/go-telemetry"), GOTOOLCHAIN: "local" };
const sha = bytes => createHash("sha256").update(bytes).digest("hex");

test("cache publication after a missing validation is reused and corrupt caches stay intact", { timeout: 120000 }, async () => {
  await mkdir(output, { recursive: true });
  const cacheRoot = join(root, `.local/pokepath/native-runtime-publication-${randomUUID().slice(0, 8)}`);
  await mkdir(cacheRoot, { recursive: true });
  const ready = join(cacheRoot, "reader-ready");
  const release = join(cacheRoot, "reader-release");
  const hook = join(cacheRoot, "reader-gate.mjs");
  await writeFile(hook, `import fs from "node:fs";
import { syncBuiltinESMExports } from "node:module";
const read = fs.readFileSync;
let missing = 0;
fs.readFileSync = function(path, ...args) {
  try { return read.call(this, path, ...args); }
  catch (error) {
    if (String(path).startsWith(process.env.CACHE_RACE_ROOT + "/") && String(path).endsWith("/public/pokepath/v1/native/manifest.json") && error.code === "ENOENT" && ++missing === 2) {
      fs.writeFileSync(process.env.CACHE_RACE_READY, "ready");
      const deadline = Date.now() + 60000;
      while (!fs.existsSync(process.env.CACHE_RACE_RELEASE)) {
        if (Date.now() > deadline) throw Error("Timed out waiting for the concurrent publisher");
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 10);
      }
    }
    throw error;
  }
};
syncBuiltinESMExports();
`);
  const command = ["games/pokepath/build/browser-runtime.mjs", "--cache-root", cacheRoot];
  const env = { ...environment, CACHE_RACE_ROOT: cacheRoot, CACHE_RACE_READY: ready, CACHE_RACE_RELEASE: release };
  const waiting = execute(process.execPath, ["--import", hook, ...command], { cwd: root, env, timeout: 90000, maxBuffer: 8 << 20 }).then(result => ({ result }), error => ({ error }));
  const report = { passed: false, forcedMissingReadBeforeConcurrentPublication: true };
  try {
    const deadline = Date.now() + 60000;
    while (!existsSync(ready)) {
      if (Date.now() > deadline) throw Error("The reader did not reach its missing-validation barrier");
      await new Promise(resolve => setTimeout(resolve, 10));
    }
    const publisher = JSON.parse((await execute(process.execPath, command, { cwd: root, env: environment, timeout: 90000, maxBuffer: 8 << 20 })).stdout);
    await writeFile(release, "published");
    const reader = await waiting;
    if (reader.error) throw reader.error;
    const reused = JSON.parse(reader.result.stdout);
    assert.equal(reused.cached, true);
    assert.equal(reused.runtimeRoot, publisher.runtimeRoot);
    assert.equal(reused.buildSHA256, publisher.buildSHA256);
    for (const entry of publisher.files) {
      const bytes = await readFile(join(publisher.runtimeRoot, entry.path));
      assert.equal(bytes.length, entry.bytes);
      assert.equal(sha(bytes), entry.sha256);
    }
    const damaged = join(publisher.runtimeRoot, "engine.wasm");
    await writeFile(damaged, "invalid-cache-must-stay");
    await assert.rejects(execute(process.execPath, command, { cwd: root, env: environment, timeout: 90000, maxBuffer: 8 << 20 }), /existing runtime cache failed validation/);
    assert.equal(await readFile(damaged, "utf8"), "invalid-cache-must-stay");
    report.passed = true;
    report.reused = reused;
    report.corruptCachePreserved = true;
  } catch (error) {
    report.error = error.stack ?? String(error);
    throw error;
  } finally {
    await writeFile(release, "cleanup");
    await waiting;
    await rm(cacheRoot, { recursive: true, force: true });
    await writeFile(join(output, "publication-race.json"), JSON.stringify(report, null, 2) + "\n");
  }
});

test("concurrent cold browser builds reuse an immutable verified directory", { timeout: 120000 }, async () => {
  await mkdir(output, { recursive: true });
  const cacheRoot = join(root, `.local/pokepath/native-runtime-concurrency-${randomUUID().slice(0, 8)}`);
  const command = ["games/pokepath/build/browser-runtime.mjs", "--cache-root", cacheRoot];
  const report = { command: "node --test games/pokepath/tests/native-runtime-build-e2e.test.mjs", passed: false };
  try {
    const builds = await Promise.all([execute(process.execPath, command, { cwd: root, env: environment, timeout: 90000, maxBuffer: 8 << 20 }), execute(process.execPath, command, { cwd: root, env: environment, timeout: 90000, maxBuffer: 8 << 20 })]);
    const artifacts = builds.map(result => JSON.parse(result.stdout));
    for (const artifact of artifacts) assert.ok(artifact.runtimeRoot.startsWith(cacheRoot + "/"), "the regression must use only its own cold cache");
    assert.equal(artifacts[0].runtimeRoot, artifacts[1].runtimeRoot);
    assert.equal(artifacts[0].sourceSHA256, artifacts[1].sourceSHA256);
    for (const entry of artifacts[0].files) {
      const file = join(artifacts[0].runtimeRoot, entry.path);
      for (let attempt = 0; attempt < 5; attempt++) {
        const bytes = await readFile(file);
        assert.equal(bytes.length, entry.bytes);
        assert.equal(sha(bytes), entry.sha256);
      }
    }
    const repeated = JSON.parse((await execute(process.execPath, command, { cwd: root, env: environment, timeout: 90000, maxBuffer: 8 << 20 })).stdout);
    assert.equal(repeated.cached, true);
    assert.equal(repeated.buildSHA256, artifacts[0].buildSHA256);
    report.artifacts = artifacts;
    report.repeated = repeated;
    report.passed = true;
  } catch (error) {
    report.error = error.stack ?? String(error);
    throw error;
  } finally {
    await rm(cacheRoot, { recursive: true, force: true });
    await mkdir(output, { recursive: true });
    await writeFile(join(output, "concurrent-build.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
