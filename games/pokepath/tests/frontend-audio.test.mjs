import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdir, writeFile } from "node:fs/promises";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));

test("fire cues follow species facts, area deduplication and all five mute groups", async () => {
  const directory = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const target = new URL("audio-cues.mjs", directory);
  await require("esbuild").build({ entryPoints: [new URL("../web/audio-cues.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { audioCues } = await import(target.href);
  const view = { team: [{ id: 917, species: "charmander" }, { id: 918, species: "araquanid" }, { id: 919, species: "abra" }], towers: [{ id: 11, pokemonID: 917 }, { id: 12, pokemonID: 918 }, { id: 13, pokemonID: 919 }], progression: { settings: { mute: [false, false, false, false, false] } } };
  const catalog = { species: { charmander: { attackType: "single", facts: { projectileSound: "projectile1" } }, araquanid: { attackType: "area", facts: { projectileSound: "hit4" } }, abra: { attackType: "beam", facts: { projectileSound: "laser" } } } };
  const events = [{ id: 1, type: "fire", source: 11, at: 20 }, { id: 2, type: "fire", source: 12, at: 20 }, { id: 3, type: "fire", source: 12, at: 20 }, { id: 4, type: "fire", source: 13, at: 20 }, { id: 5, type: "teleport", at: 20 }, { id: 6, type: "shiny", at: 20 }];
  const all = audioCues(view, catalog, events);
  assert.equal(all.length, 5);
  assert.deepEqual(all.map(cue => cue.group), [0, 3, 4, 1, 2]);
  assert.ok(all[0].path.endsWith("effect/projectile1.wav"));
  view.progression.settings.mute = [true, true, true, true, true];
  assert.deepEqual(audioCues(view, catalog, events), []);
});
