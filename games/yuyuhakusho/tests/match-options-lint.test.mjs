import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const lint = (file, source) =>
  spawnSync(
    process.execPath,
    ["games/yuyuhakusho/build/lint.mjs", "--stdin", file],
    { cwd: root, input: source, encoding: "utf8" },
  );

test("match options language lint rejects comments, naming and directory violations", () => {
  for (const [file, valid, invalid] of [
    [
      "games/yuyuhakusho/web/labels.ts",
      "export const character = 1;",
      [
        "// bad\nexport const character = 1;",
        "const bad_name = 1;",
        "eval('1')",
      ],
    ],
    [
      "games/yuyuhakusho/web/Preparation.tsx",
      "export function Preparation() { return <div />; }",
      [
        "/* bad */ export const character = 1;",
        "export function bad_name() { return <div />; }",
      ],
    ],
    [
      "games/yuyuhakusho/web/network.css",
      ".team { display: grid; }",
      ["/* bad */ .team { display: grid; }"],
    ],
    [
      "games/yuyuhakusho/server/options.go",
      "package yuyuhakusho\nvar character = 1\n",
      [
        "package wrong\n",
        "package yuyuhakusho\n// bad\nvar character = 1\n",
        "package yuyuhakusho\nvar bad_name = 1\n",
        'package yuyuhakusho\nimport _ "unsafe"\n',
      ],
    ],
  ]) {
    assert.equal(lint(file, valid).status, 0, file);
    for (const source of invalid)
      assert.notEqual(lint(file, source).status, 0, `${file}: ${source}`);
  }
  for (const file of [
    "games/yuyuhakusho/web/bad_name.ts",
    "games/yuyuhakusho/other/options.go",
    "games/yuyuhakusho/server/BadName.go",
  ])
    assert.notEqual(lint(file, "").status, 0, file);
});
