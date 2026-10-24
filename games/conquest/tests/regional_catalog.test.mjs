// Before regional catalog implementation: both creation UI and saved options
// must select world by default, and all four actual map choices must render.
import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { mkdirSync, writeFileSync, rmSync } from "node:fs";
import path from "node:path";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(path.join(root, "web/package.json"));
const { build } = require("esbuild");
test("creation defaults to world and offers the four independent maps", async () => {
  const out = path.join(root, ".local/conquest-regional/catalog");
  mkdirSync(out, { recursive: true });
  const result = await build({
    stdin: {
      contents: `import React from 'react';import{renderToStaticMarkup}from'react-dom/server';import game from'../games/conquest/web/module.tsx';export const options=game.defaultOptions;export const html=renderToStaticMarkup(React.createElement(game.Options,{value:options,onChange:()=>{},capacity:2,disabled:false}));`,
      resolveDir: path.join(root, "web"),
      sourcefile: "regional.tsx",
      loader: "tsx",
    },
    nodePaths: [path.join(root, "web/node_modules")],
    bundle: true,
    platform: "node",
    format: "cjs",
    jsx: "automatic",
    write: false,
    loader: { ".css": "empty" },
  });
  const file = path.join(out, "render.cjs");
  writeFileSync(file, result.outputFiles[0].contents);
  try {
    const d = require(file);
    assert.equal(d.options.mapId, "world");
    for (const id of ["world", "europe", "usa", "china"])
      assert.match(d.html, new RegExp('value="' + id + '"'));
  } finally {
    rmSync(file, { force: true });
  }
});
