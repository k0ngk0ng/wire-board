// Failure cases before the fix: a paused match expired after 30 days displays
// the ordinary 24-hour reason; old rooms without pause metadata change copy;
// paused metadata overrides administrator, unpublished or voluntary closure.
import test from "node:test";
import assert from "node:assert/strict";
import { build } from "esbuild";
import { createRequire } from "node:module";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root = fileURLToPath(new URL("../../", import.meta.url));
const out = path.join(root, ".local/module-pause");
mkdirSync(out, { recursive: true });
const result = await build({
  stdin: {
    contents: `
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import {ClosedRoomDescription} from './src/ui/closed-room.tsx';
export function render(room){return renderToStaticMarkup(React.createElement(ClosedRoomDescription,{room}));}
`,
    resolveDir: path.join(root, "web"),
    sourcefile: "closed-room-fixture.tsx",
    loader: "tsx",
  },
  bundle: true,
  platform: "node",
  format: "cjs",
  jsx: "automatic",
  write: false,
});
const compiled = path.join(out, "closed-ui-fixture.cjs");
writeFileSync(compiled, result.outputFiles[0].contents);
const { render } = createRequire(import.meta.url)(compiled);
const evidence = [];
test("expired paused room shows its actual 30-day retention policy", () => {
  const room = {
    status: "closed",
    closeReason: "inactive",
    game: { paused: true, pauseExpiresAt: 1793000000000 },
  };
  const html = render(room);
  assert.equal(
    html,
    "<p>暂停对局连续 30 天无人操作，系统已自动关闭牌桌；本局不计胜负与积分。</p>",
  );
  assert.ok(!html.includes("24 小时"));
  evidence.push({ case: "paused retention expired", room, html });
});
test("normal and historical inactive rooms retain the 24-hour explanation", () => {
  for (const game of [undefined, {}, { paused: false }]) {
    const room = { status: "closed", closeReason: "inactive", game };
    const html = render(room);
    assert.equal(
      html,
      "<p>连续 24 小时无人操作，系统已自动关闭牌桌；本局不计胜负与积分。</p>",
    );
    evidence.push({ case: "ordinary or old room", room, html });
  }
});
test("other closure reasons take precedence over saved pause metadata", () => {
  for (const [closeReason, text] of [
    ["unpublished", "等待中的牌桌已关闭，可以返回大厅选择其他游戏。"],
    ["admin", "本局已中止，不计胜负与积分。"],
    [undefined, "休息一下，或者准备下一局。"],
  ]) {
    const room = { status: "closed", closeReason, game: { paused: true } };
    const html = render(room);
    assert.equal(html, `<p>${text}</p>`);
    evidence.push({ case: "other closure", room, html });
  }
});
test.after(() =>
  writeFileSync(
    path.join(out, "closed-ui-render.json"),
    JSON.stringify(
      {
        scope: "rendered closed-room explanation component used by main.tsx",
        checks: evidence,
      },
      null,
      2,
    ) + "\n",
  ),
);
