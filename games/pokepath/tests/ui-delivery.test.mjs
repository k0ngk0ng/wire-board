import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";

test("independent language facts are published through the verified resource index", async () => {
  const root = new URL("../assets/public/pokepath/v1/", import.meta.url);
  const bytes = await readFile(new URL("ui-text.json", root));
  const resources = JSON.parse(await readFile(new URL("resources.json", root), "utf8"));
  const record = resources.files.find(file => file.path === "ui-text.json");
  assert.ok(record, "the language loader cannot read an unregistered file");
  assert.equal(record.bytes, bytes.length);
  assert.equal(record.sha256, createHash("sha256").update(bytes).digest("hex"));
  assert.equal(record.mime, "application/json");
  const facts = JSON.parse(bytes);
  assert.equal(facts.format, "pokepath-ui-text-1");
  assert.equal(facts.source.version, "1.7.3");
  assert.equal(facts.text.ui.nextWave[0], "Next wave");
  assert.equal(facts.text.ui.nextWave[8], "下一波");
  assert.equal(facts.text.ui.nextWave.length, 10);
  assert.equal(Object.keys(facts.text.interfaceOptions.targetModes).length, 27);
  assert.equal(Object.keys(facts.text.interfaceOptions.effectOpacityLabels).length, 7);
  assert.equal(Object.keys(facts.text.interfaceOptions.muteLabels).length, 2);
  assert.equal(facts.text.interfaceOptions.shortcuts.key[8][0], "Space");
  assert.equal(facts.text.interfaceOptions.shortcuts.key[8][8], "空格");
  assert.equal(facts.text.interfaceOptions.unownDialogue[0][0], "Can you read me?");
  assert.equal(facts.text.interfaceOptions.unownDialogue[0][8], "Can you read me?");
  assert.ok(facts.source.files.some(file => file.path === "src/js/game/UI.js"));
  const manifest = JSON.parse(await readFile(new URL("../data/manifest.json", import.meta.url), "utf8"));
  assert.equal(manifest.factFiles.some(file => file.path.includes("ui-text")), false, "UI translations must not change independent game-rule facts");
});

test("independent multiplayer saves advance the server rollback generation", async () => {
  const dockerfile = await readFile(new URL("../../../Dockerfile", import.meta.url), "utf8");
  const generation = dockerfile.match(/io\.github\.k0ngk0ng\.wire-board\.storage-generation="([0-9]+)"/);
  assert.ok(generation && Number(generation[1]) >= 4);
});
