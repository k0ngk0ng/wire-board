import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

test("publication and media changes enforce language naming, directories and forbidden constructs", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const config = "games/racing/build/asset-reuse-lint.json";
  const rules = JSON.parse(readFileSync(new URL("../../" + config, import.meta.url)));
  for (const file of ["scripts/asset_reuse.py", "scripts/cdn_delivery.py", "scripts/asset_reuse_e2e_test.py", "internal/server/static_media.go", "internal/server/static_media_reuse_e2e_test.go", "web/src/media-assets.ts", "web/src/main.tsx", "scripts/ci/baseline.cjs", "scripts/ci/baseline.test.cjs"]) assert(rules.files.includes(file), file);
  const result = spawnSync(process.execPath, ["games/racing/build/lint.mjs", "--config", config, "--self-test"], { cwd: root, encoding: "utf8" });
  assert.equal(result.status, 0, result.stdout + result.stderr);
});

test("Go package checks follow each registered game directory", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  for (const folder of [".local/go-build", ".local/go-mod", ".local/racing/dynamics/tmp"]) mkdirSync(root+folder,{recursive:true});
  const inputs = [{path:"games/tetris/tests/http_test.go",source:"package tetris_test\nfunc checkBounds() {}"},{path:"games/tetris/tests/http_test.go",source:"package racing_test\nfunc checkBounds() {}"}];
  const result = spawnSync("go", ["run", "games/racing/build/lint_go.go"], {cwd:root,input:JSON.stringify(inputs),encoding:"utf8",env:{...process.env,GOCACHE:root+".local/go-build",GOMODCACHE:root+".local/go-mod",GOTMPDIR:root+".local/racing/dynamics/tmp",TMPDIR:root+".local/racing/dynamics/tmp",GOTOOLCHAIN:"local",GOTELEMETRY:"off"}});
  assert.equal(result.status,0,result.stderr);
  const checked=JSON.parse(result.stdout);
  assert.deepEqual(checked[0],[]);
  assert(checked[1].includes("package-directory"));
});
