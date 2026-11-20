import assert from "node:assert/strict";
import { test } from "node:test";
import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "../../../..");
for (const path of ["cache/go", "tmp", "verification/maps"]) mkdirSync(resolve(root, ".local/hotlinemiami", path), { recursive: true });

test("recovered rule source reproducibly generates registered Go maps", () => {
  const result = spawnSync("go", ["run", "./games/hotlinemiami/cmd/maps", "--check"], {
    cwd: root,
    env: { ...process.env, GOTELEMETRY: "off", GOCACHE: resolve(root, ".local/hotlinemiami/cache/go"), GOTMPDIR: resolve(root, ".local/hotlinemiami/tmp") },
    encoding: "utf8",
  });
  assert.equal(result.status, 0, result.stderr);
});
