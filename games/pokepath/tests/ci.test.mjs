import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { promisify } from "node:util";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const execute = promisify(execFile);
const files = [
  "native-architecture", "native-reference", "rules-revision", "lint",
  "native-runtime-build-e2e", "native-runtime-unicode", "native-attack-modes-wasm", "tower-order-wasm-e2e",
  "prediction-handles", "prediction", "prediction-session-e2e", "prediction-frame-cache-e2e", "personal-view",
  "damage-labels", "frontend-audio", "frontend-classic", "frontend-language", "frontend-native", "frontend-render",
  "keyboard-command", "keyboard-controller", "pointer-aim", "presentation-proxy", "ui-delivery", "world-animations",
].map(name => `games/pokepath/tests/${name}.test.mjs`);

test("clean checkout delivery checks preserve native trace order and serial Wasm budgets", { timeout: 600000 }, async () => {
  assert.equal(new Set(files).size, files.length);
  const output = join(root, ".local/pokepath/verification/ci");
  await mkdir(output, { recursive: true });
  const environment = { ...process.env };
  delete environment.NODE_TEST_CONTEXT;
  for (const [name, local] of [["GOCACHE", ".local/go-build"], ["GOMODCACHE", ".local/go-mod"], ["GOTMPDIR", ".local/tmp"], ["TMPDIR", ".local/tmp"], ["TEST_TELEMETRY_DIR", ".local/go-telemetry"], ["npm_config_cache", ".local/npm"]]) {
    environment[name] = join(root, local);
    await mkdir(environment[name], { recursive: true });
  }
  const report = { passed: false, scope: "Independent deliverable checks from a clean checkout; original-browser and pixel research stay in local acceptance artifacts", checks: [] };
  const commands = [
    [process.execPath, ["games/pokepath/build/rules-revision.mjs", "--public-only"]],
    [process.execPath, ["games/pokepath/build/rules-revision.mjs", "--check"]],
    ["go", ["test", "./games/pokepath/engine", "./games/pokepath/tests", "-run", "^(TestCoreTowerOrderingReferenceFixtureE2E|TestNativeAttackModeProducesGoWasmParityTracesE2E)$", "-count=1"]],
    [process.execPath, ["--experimental-strip-types", "--test", "--test-concurrency=1", ...files]],
    [process.execPath, ["games/pokepath/build/lint.mjs"]],
  ];
  try {
    for (const [index, [command, args]] of commands.entries()) {
      const result = await execute(command, args, { cwd: root, env: environment, timeout: 540000, maxBuffer: 16 << 20 });
      const log = join(output, `${index}.log`);
      await writeFile(log, result.stdout + result.stderr);
      if (command === process.execPath && args.includes("--test")) {
        assert.doesNotMatch(result.stdout + result.stderr, /called recursively/);
        const count = result.stdout.match(/(?:tests |# tests )([0-9]+)/);
        assert.ok(count && Number(count[1]) >= files.length, "the delivery child must execute its selected tests");
      }
      report.checks.push({ command: [command, ...args], log, passed: true });
    }
    report.passed = true;
  } catch (error) {
    report.error = { message: error.message, stdout: error.stdout, stderr: error.stderr };
    throw error;
  } finally {
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
