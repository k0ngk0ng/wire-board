// Failure inventory before implementation: source record order is not character
// ID order; hidden characters accidentally selectable; missing sprites or wrong
// frame bounds/anchors; malformed IDs; local fallback mutates immutable v3 paths.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { createHash } from "node:crypto";
import {
  characters,
  selectableCharacters,
  normalizeCharacter,
  characterSourceCandidates,
} from "../web/character-definitions.ts";
const root = new URL("../", import.meta.url);

test("ten profiles preserve source IDs and all six original attributes", () => {
  const source = JSON.parse(
    readFileSync(new URL("data/classic-2002.json", root)),
  ).characterAttributes;
  assert.equal(characters.length, 10);
  assert.deepEqual(
    selectableCharacters.map((c) => c.id),
    [0, 1, 2, 3, 4, 5, 6, 7],
  );
  for (const original of source.characters) {
    const id = Number(original.name.slice(-2)),
      c = characters[id],
      fields = original.fieldsRaw;
    assert.equal(c.id, id);
    assert.deepEqual(
      [
        c.initialSpeed,
        c.maximumSpeed,
        c.initialBombs,
        c.maximumBombs,
        c.initialRange,
        c.maximumRange,
      ],
      fields.slice(1, 7),
    );
    assert.equal(c.hidden, Boolean(fields[0]));
  }
});
test("all ten original characters have both team palettes and valid animation bounds", () => {
  const manifest = JSON.parse(
    readFileSync(new URL("assets/manifest-characters-v4.json", root)),
  );
  assert.equal(manifest.entries.length, 120);
  for (const entry of manifest.entries) {
    const file = new URL("assets/public" + entry.urlPath, root);
    assert.ok(existsSync(file));
    const bytes = readFileSync(file);
    assert.equal(
      createHash("sha256").update(bytes).digest("hex"),
      entry.sha256,
    );
    assert.equal(bytes.readUInt32BE(16), entry.width);
    assert.equal(bytes.readUInt32BE(20), entry.height);
  }
  for (const c of characters) {
    assert.ok(c.name.length > 0);
    for (const clip of Object.values(c.clips)) {
      assert.ok(clip.red !== clip.blue);
      for (const [frame, weight] of clip.sequence) {
        assert.ok(clip.frames[frame]);
        assert.ok(weight > 0);
      }
    }
  }
});
test("v4 character assets and IDs remain independent from legacy v3 cosmetics", () => {
  for (const id of [-1, 10, NaN, undefined, 0.5])
    assert.equal(normalizeCharacter(id), 0);
  assert.equal(normalizeCharacter(9), 9);
  assert.deepEqual(
    characterSourceCandidates(
      "https://assets.test/",
      "character-0-down-red.png",
    ),
    [
      "https://assets.test/bubbleparty/v4/characters/character-0-down-red.png",
      "/bubbleparty/v4/characters/character-0-down-red.png",
    ],
  );
});
