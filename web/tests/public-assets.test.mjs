import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { preparePublicAssets } from "../scripts/public-assets.mjs";

function fixture(t) {
  const workspace = fileURLToPath(new URL("../../", import.meta.url));
  const directory = path.join(workspace, ".local/hotlinemiami/verification/public-assets");
  mkdirSync(directory, { recursive: true });
  const root = mkdtempSync(path.join(directory, "fixture-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const write = (file, data) => {
    const target = path.join(root, file);
    mkdirSync(path.dirname(target), { recursive: true });
    writeFileSync(target, data);
  };
  write("games/hotlinemiami/assets/public/hotlinemiami/runtime.txt", "restored-old-runtime");
  write("games/anothergame/assets/public/anothergame/map.txt", "preserved-other-game");
  write("generated/hotlinemiami/runtime.txt", "compiled-source-bound-runtime");
  return { root, write, generated: path.join(root, "generated") };
}

test("a generated game delivery replaces its restored source assets without duplicate public paths", t => {
  const { root, generated } = fixture(t);
  const output = preparePublicAssets(root, [generated], ["hotlinemiami"]);
  assert.equal(readFileSync(path.join(output, "hotlinemiami/runtime.txt"), "utf8"), "compiled-source-bound-runtime");
  assert.equal(readFileSync(path.join(output, "anothergame/map.txt"), "utf8"), "preserved-other-game");
  assert.equal(readFileSync(path.join(root, "games/hotlinemiami/assets/public/hotlinemiami/runtime.txt"), "utf8"), "restored-old-runtime");
});

test("undeclared duplicate ownership remains an error", t => {
  const { root, generated } = fixture(t);
  assert.throws(() => preparePublicAssets(root, [generated]), /Duplicate public asset/);
});

test("declared generated ownership requires a real game and its generated asset tree", t => {
  const { root, generated } = fixture(t);
  assert.throws(() => preparePublicAssets(root, [generated], ["../anothergame"]), /Invalid generated game/);
  assert.throws(() => preparePublicAssets(root, [generated], ["anothergame"]), /Missing generated public assets/);
});
