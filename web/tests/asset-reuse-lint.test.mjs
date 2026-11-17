import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

test("publication and media changes enforce language naming, directories and forbidden constructs", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const config = "games/racing/build/asset-reuse-lint.json";
  const rules = JSON.parse(readFileSync(new URL("../../" + config, import.meta.url)));
  for (const file of ["scripts/asset_reuse.py", "scripts/cdn_delivery.py", "scripts/asset_reuse_e2e_test.py", "internal/server/static_media.go", "internal/server/static_media_reuse_e2e_test.go", "web/src/media-assets.ts", "web/src/main.tsx"]) assert(rules.files.includes(file), file);
  const result = spawnSync(process.execPath, ["games/racing/build/lint.mjs", "--config", config, "--self-test"], { cwd: root, encoding: "utf8" });
  assert.equal(result.status, 0, result.stdout + result.stderr);
});
