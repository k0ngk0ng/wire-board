import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { mkdir, unlink, writeFile } from "node:fs/promises";
import { mapGuide, characterGuide, weaponGuide, isMeleeWeapon } from "../web/content.ts";

const maps = ["no-talk", "full-house", "push-it", "overdose", "tension", "neighbors", "highball", "exposed", "hot-heavy"];
const characters = ["jacket", "biker", "bodyguard", "father", "richter", "producer", "beard", "hobo"];
const weapons = ["pistol", "uzi", "m16", "shotgun", "knife", "bat", "katana", "magnum", "mp5", "double-barrel", "axe", "machete"];

test("content choices cover nine real maps, eight cosmetic characters and twelve distinct weapons", () => {
  assert.deepEqual(mapGuide.map(item => item.id), maps);
  assert.deepEqual(characterGuide.map(item => item.id), characters);
  assert.deepEqual(weaponGuide.map(item => item.id), weapons);
  for (const group of [mapGuide, characterGuide, weaponGuide]) {
    assert.equal(new Set(group.map(item => item.id)).size, group.length);
    assert.ok(group.every(item => item.name && item.description.length >= 12));
  }
  assert.equal(weaponGuide.filter(item => item.group === "gun").length, 7);
  assert.equal(weaponGuide.filter(item => item.group === "melee").length, 5);
  assert.equal(isMeleeWeapon("axe"), true);
  assert.equal(isMeleeWeapon("machete"), true);
  assert.equal(isMeleeWeapon("magnum"), false);
  assert.equal(isMeleeWeapon(""), true);
});

test("the actual rules component renders every choice and the multiplayer differences in folding groups", async () => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const webRequire = createRequire(new URL("../../../web/package.json", import.meta.url));
  const { build } = webRequire("esbuild");
  const directory = new URL("../../../.local/hotlinemiami/verification/web/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const output = new URL("rules-render.cjs", directory);
  const bundle = await build({
    stdin: { contents: "import {createElement} from 'react';import {renderToStaticMarkup} from 'react-dom/server';import {Rules} from '../games/hotlinemiami/web/Rules.tsx';export const html=renderToStaticMarkup(createElement(Rules));", resolveDir: root + "web", loader: "tsx" },
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
  const { html } = require(fileURLToPath(output));
  await writeFile(new URL("rules-render.html", directory), html + "\n");
  await unlink(output);
  assert.ok((html.match(/<details/g) ?? []).length >= 4);
  for (const id of maps) assert.ok(html.includes(`data-hotlinemiami-rule-map="${id}"`), id);
  for (const id of characters) assert.ok(html.includes(`data-hotlinemiami-rule-character="${id}"`), id);
  for (const id of weapons) assert.ok(html.includes(`data-hotlinemiami-rule-weapon="${id}"`), id);
  const text = html.replace(/<[^>]+>/g, "");
  for (const phrase of ["碰撞体相同", "不能装填", "倒地", "处决", "出生保护", "和局", "触屏", "手柄", "不会自动托管"]) assert.ok(text.includes(phrase), phrase);
});
