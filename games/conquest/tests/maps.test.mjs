import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

test("world and Europe map contracts, source identity and offline regeneration", () => {
  const script = fileURLToPath(
    new URL("./map_contract_test.py", import.meta.url),
  );
  const result = spawnSync("python3", ["-B", script], {
    encoding: "utf8",
    timeout: 60_000,
    maxBuffer: 1024 * 1024,
  });
  assert.equal(
    result.status,
    0,
    result.error?.message || result.stdout + result.stderr,
  );
});
