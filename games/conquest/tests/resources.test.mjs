// Failure inventory precedes fix in FAILURE_SCENARIOS_UI.md. Actual public
// player component: debt (-10000 and -1), zero, hidden opponents/spectators.
import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(path.join(root, "web/package.json"));
const { build } = require("esbuild");
const out = path.join(root, ".local/conquest/resource-render");
mkdirSync(out, { recursive: true });
const result = await build({
  stdin: {
    contents: `import React from 'react';import {renderToStaticMarkup} from 'react-dom/server';import module from '../games/conquest/web/module.tsx';export function render(room,gold,seat=0){const p={seat,gold,territories:2,troops:500,submitted:false,eliminated:false};return renderToStaticMarkup(React.createElement(module.PlayerSummary,{room,data:{players:[p,p],phase:'planning'},seat,assets:''}));}`,
    resolveDir: path.join(root, "web"),
    sourcefile: "conquest-resource-fixture.tsx",
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
const compiled = path.join(out, "render.cjs");
writeFileSync(compiled, result.outputFiles[0].contents);
const { render } = require(compiled),
  evidence = [];
for (const gold of [-10000, -1, 0, 5000])
  test(`own gold ${gold} stays signed and visible`, () => {
    const html = render({ you: 0, spectating: false }, gold);
    assert.ok(
      html.includes(
        `data-cq-gold="0">${gold.toLocaleString("zh-CN")}</strong>`,
      ),
      html,
    );
    evidence.push({ kind: "own", gold, html });
  });
test("opponent hidden value is not represented as a one-gold debt", () => {
  const html = render({ you: 0, spectating: false }, -1, 1);
  assert.ok(html.includes('data-cq-gold="1">?</strong>'), html);
  evidence.push({ kind: "opponent", html });
});
test("spectators never acquire a seat financial identity", () => {
  const html = render({ you: 0, spectating: true }, -1, 0);
  assert.ok(html.includes('data-cq-gold="0">?</strong>'), html);
  evidence.push({ kind: "spectator", html });
});
test.after(() =>
  writeFileSync(
    path.join(out, "result.json"),
    JSON.stringify(
      { scope: "actual PlayerSummary React rendering", evidence },
      null,
      2,
    ),
  ),
);
