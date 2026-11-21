import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

test("CS16 CI retains checkpoint JSON after command failure without changing other lanes", () => {
  const result = spawnSync("python3", ["-B", "games/cs16/tests/ci_artifacts_e2e_test.py"], { cwd: root, encoding: "utf8" });
  assert.equal(result.status, 0, result.stdout + result.stderr);
});
