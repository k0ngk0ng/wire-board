import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdir, unlink, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { ammoLabel, battleHint } from "../web/hud.ts";

const player = extra => ({ weapon: "pistol", ammo: 0, alive: true, downUntil: 0, protectedUntil: 0, ...extra });

test("empty firearms explain the existing throw and pickup actions", () => {
  const hint = battleHint(player({}), 100, false);
  assert.equal(hint.kind, "empty");
  assert.equal(hint.text, "弹药用尽：投掷空枪，拾取其他武器");
  assert.equal(ammoLabel("pistol", 0), "0 发");
  assert.equal(battleHint(player({ ammo: 1 }), 100, false).kind, "controls");
});

test("unarmed and melee states never ask the player to reload an empty firearm", () => {
  for (const weapon of ["knife", "bat", "katana", "axe", "machete"]) {
    assert.equal(ammoLabel(weapon, 0), "近战");
    assert.equal(battleHint(player({ weapon }), 100, false).kind, "controls");
  }
  assert.equal(ammoLabel("", 0), "近战");
  assert.equal(battleHint(player({ weapon: "" }), 100, false).text, "徒手：近身击倒，或拾取武器");
});

test("death, protection, knockdown and finish take precedence over an obsolete empty-gun hint", () => {
  assert.equal(battleHint(player({ alive: false }), 100, false).text, "复活时恢复起始装备");
  assert.equal(battleHint(player({ protectedUntil: 101 }), 100, false).text, "出生保护 · 攻击后解除");
  assert.equal(battleHint(player({ downUntil: 101 }), 100, false).text, "倒地中");
  assert.equal(battleHint(player({}), 100, true).text, "对局结束");
  assert.equal(battleHint(undefined, 100, false).text, "观战中");
});

test("the existing bottom bar renders clear empty, unarmed and respawn messages without new controls", async () => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const webRequire = createRequire(new URL("../../../web/package.json", import.meta.url));
  const { build } = webRequire("esbuild");
  const directory = new URL("../../../.local/hotlinemiami/verification/web/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const output = new URL("hud-render.cjs", directory);
  const bundle = await build({
    stdin: { contents: "import {createElement} from 'react';import {renderToStaticMarkup} from 'react-dom/server';import {BottomHint} from '../games/hotlinemiami/web/BottomHint.tsx';export function render(player){return renderToStaticMarkup(createElement(BottomHint,{player,frame:100,finished:false}));}", resolveDir: root + "web", loader: "tsx" },
    bundle: true,
    platform: "node",
    nodePaths: [root + "web/node_modules"],
    format: "cjs",
    jsx: "automatic",
    minify: true,
    legalComments: "none",
    write: false,
  });
  await writeFile(output, bundle.outputFiles[0].contents);
  const require = createRequire(import.meta.url);
  const { render } = require(fileURLToPath(output));
  const samples = [render(player({})), render(player({ weapon: "" })), render(player({ alive: false })), render(player({ ammo: 8 }))];
  await writeFile(new URL("hud-render.html", directory), samples.join("\n") + "\n");
  await unlink(output);
  for (const html of samples) {
    assert.ok(html.includes('class="hm-bottom-line"'));
    assert.ok(html.includes("击杀 +1"));
    assert.equal(/<button|role="dialog"|<input/.test(html), false);
  }
  assert.ok(samples[0].includes("弹药用尽：投掷空枪，拾取其他武器"));
  assert.ok(samples[1].includes("徒手：近身击倒，或拾取武器"));
  assert.ok(samples[2].includes("复活时恢复起始装备"));
});
