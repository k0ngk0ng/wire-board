import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import * as facts from "../build/extract-reference.mjs";

test("tutorial literals retain localized text and canonical illustration paths", () => {
  const raw = [{ key: "secrets", title: ["Secrets", ...Array(7).fill(""), "秘密"], text: ["Text", ...Array(7).fill(""), "[POKEMON]寶可夢[/POKEMON]"], image: "./src/assets/images/tutorials/secrets.png" }];
  const result = facts.normalizeTutorialFacts(raw);
  assert.equal(result.secrets.name, "秘密");
  assert.equal(result.secrets.description, "[POKEMON]寶可夢[/POKEMON]");
  assert.equal(result.secrets.sprite, "src/assets/images/tutorials/secrets.png");
  assert.throws(() => facts.normalizeTutorialFacts([raw[0], raw[0]]));
  assert.throws(() => facts.normalizeTutorialFacts([{ ...raw[0], image: "../escape.png" }]));
});

test("theme facts resolve root palette references without retaining layout CSS", () => {
  const css = ':root{--white:#fff;--green:#70ac4c;}[data-theme="original"]{--sceneBackground:#2a2b2d;--money:var(--green);--heartImage:url(image.png);} [data-theme="classic"]{--sceneBackground:#333;} [data-theme="halloween"]{--sceneBackground:#111;} [data-theme="monochrome"]{--sceneBackground:#222;} .window{position:fixed;width:720px;}';
  const result = facts.parseThemeColors(css);
  assert.equal(result.original.money, "#70ac4c");
  assert.equal(result.original.sceneBackground, "#2a2b2d");
  assert.equal(result.original.heartImage, undefined);
  assert.equal(result.original.position, undefined);
  assert.deepEqual(Object.keys(result).sort(), ["classic", "halloween", "monochrome", "original"]);
  assert.throws(() => facts.parseThemeColors(':root{--a:var(--b);--b:var(--a);}[data-theme="original"]{--money:var(--a);}'));
});

test("the archived tutorial source remains static data", async () => {
  const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/data/tutorialData.js", import.meta.url), "utf8");
  const raw = facts.parseLiteralDeclarations(source, "tutorialData.js").get("tutorials");
  const result = facts.normalizeTutorialFacts(raw);
  assert.ok(Object.keys(result).length >= 20);
  assert.ok(result.secrets.description.length > 0);
});
