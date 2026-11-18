import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createHash, randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const execute = promisify(execFile);
const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = join(root, ".local/pokepath/verification/native-runtime");
const environment = { ...process.env, GOCACHE: join(root, ".local/go-build"), GOMODCACHE: join(root, ".local/go-mod"), GOTMPDIR: join(root, ".local/tmp"), TMPDIR: join(root, ".local/tmp"), TEST_TELEMETRY_DIR: join(root, ".local/go-telemetry"), GOTOOLCHAIN: "local" };
const sha = bytes => createHash("sha256").update(bytes).digest("hex");

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
