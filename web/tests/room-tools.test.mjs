import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdir, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const require = createRequire(new URL("../package.json", import.meta.url));
const root = fileURLToPath(new URL("../../", import.meta.url));
const out = path.join(root, ".local/room-tools-followup/verification");
await mkdir(out, { recursive: true });
const result = await require("esbuild").build({
  stdin: {
    contents:
      "import React from 'react';import {renderToStaticMarkup} from 'react-dom/server';import {SaveGameControl} from './src/ui/saved-games';export const render=(room,iconOnly=true,busy=false)=>renderToStaticMarkup(<SaveGameControl room={room} userID='self' busy={busy} iconOnly={iconOnly} command={()=>{}}/>);",
    resolveDir: path.join(root, "web"),
    loader: "tsx",
  },
  bundle: true,
  write: false,
  loader: { ".css": "empty" },
  platform: "node",
  format: "cjs",
  jsx: "automatic",
  plugins: [
    {
      name: "registry-boundary",
      setup(build) {
        build.onResolve({ filter: /^\.\.\/game-api\/registry$/ }, () => ({
          path: "registry",
          namespace: "fixture",
        }));
        build.onLoad({ filter: /.*/, namespace: "fixture" }, () => ({
          contents: "export const gameName=()=>'';",
        }));
      },
    },
  ],
});
const fixture = path.join(out, "save-controls.cjs");
await writeFile(fixture, result.outputFiles[0].contents);
const { render } = require(fixture);
const room = {
  supportsSave: true,
  supportsCheckpoint: true,
  checkpointSavedAt: 1,
  status: "playing",
  seats: [{ id: "self" }, { id: "peer" }],
  game: { restartable: true },
  saveVotes: [],
};
const evidence = [];
test("all save, checkpoint and restart toolbar actions can render as icons", () => {
  for (const [changes, labels] of [
    [{}, ["读取存档", "重新开始", "保存并退出"]],
    [{ saveVotes: ["self"] }, ["读取存档", "重新开始", "撤回保存"]],
    [
      { saveVotes: ["peer"], game: { paused: true, restartable: true } },
      ["读取存档", "重新开始", "同意保存", "取消保存"],
    ],
  ]) {
    const html = render({ ...room, ...changes });
    const buttons = [
      ...html.matchAll(/<button\b[^>]*>[\s\S]*?<\/button>/g),
    ].map((match) => match[0]);
    assert.equal(buttons.length, labels.length);
    for (const [index, button] of buttons.entries()) {
      assert.match(button, /room-tool-button/);
      assert.ok(button.includes('aria-label="' + labels[index] + '"'));
      assert.match(button, /<svg/);
      assert.equal(
        button
          .replace(/<span class="room-tool-tooltip"[^>]*>[\s\S]*?<\/span>/g, "")
          .replace(/<[^>]+>/g, "")
          .trim(),
        "",
      );
    }
    evidence.push({ changes, html });
  }
});
test("disabled save controls stay disabled and normal presentation keeps its labels", () => {
  const html = render(room, true, true);
  assert.equal([...html.matchAll(/<button\b[^>]*disabled=""/g)].length, 3);
  const normal = render(room, false);
  assert.doesNotMatch(normal, /room-tool-button/);
  assert.match(normal, /保存并退出/);
  evidence.push({ html, normal });
});
test.after(async () => {
  await writeFile(
    path.join(out, "save-controls.json"),
    JSON.stringify({ passed: true, evidence }, null, 2) + "\n",
  );
});
