import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const lint = (file, source) => spawnSync(process.execPath, ["games/yuyuhakusho/build/lint.mjs", "--stdin", file], { cwd: root, input: source, encoding: "utf8" });

test("controller input lint requires quantized comparison boundaries", () => {
  const file = "web/src/controllers/pointer-controller.ts";
  assert.equal(lint(file, "const offset = Math.round(position * 1000); if (offset < 2200) {};").status, 0);
  for (const source of ["if (offset < 0.22) {};", "const slope = Math.tan(25); if (offset > slope) {};"]) assert.notEqual(lint(file, source).status, 0);
});

test("combo browser artifacts stay under their declared game directories", () => {
  const file = "web/tests/controller-combos-browser.mjs";
  for (const folder of [".local/yuyuhakusho/controller-combos", ".local/contra/controller-combos"]) assert.equal(lint(file, `const output = ${JSON.stringify(folder)};`).status, 0);
  for (const source of ['const output = ".local/md-combos";', 'const output = "/tmp/combos";', "const output = tmpdir();"]) assert.notEqual(lint(file, source).status, 0);
});
