// Failure cases are listed before implementation in MAP_PREVIEW_SCENARIOS.md.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  classicMapGroups,
  classicMapNames,
  classicMapByID,
  previewSources,
  mapChoiceGroups,
} from "../web/classic-map-options.ts";
const source = JSON.parse(
  readFileSync(new URL("../data/classic-2002.json", import.meta.url)),
);

test("112 choices in nine themes map exactly to verified source IDs and images", () => {
  assert.equal(classicMapGroups.length, 9);
  assert.equal(Object.keys(classicMapNames).length, 112);
  const maps = classicMapGroups.flatMap((group) => group.maps);
  assert.equal(new Set(maps.map((map) => map.id)).size, 112);
  for (const original of source.maps) {
    const map = classicMapByID[original.id];
    assert.ok(map, original.id);
    assert.equal(map.sourceName, original.sourceName);
    assert.equal(map.sourceNumber, original.sourceNumber);
    assert.equal(map.preview, original.preview.file);
    assert.equal(map.width, original.preview.width);
    assert.equal(map.height, original.preview.height);
    assert.ok(map.name.startsWith("经典"));
    assert.equal(classicMapNames[map.id], map.name);
    const group = classicMapGroups.find((g) => g.maps.includes(map));
    assert.equal(group.family, original.family);
    assert.ok(!group.label.includes("2002"));
  }
  assert.ok(classicMapByID["classic2002-patrit00"].name.includes("海盗船"));
  assert.ok(classicMapByID["classic2002-ocean00"].name.includes("海洋"));
  assert.ok(classicMapByID["classic2002-xmas00"].name.includes("圣诞"));
  assert.equal(classicMapByID.community10, undefined);
});

test("preview only tries CDN then immutable same-origin path", () => {
  const file = "classic2002-village09.png";
  assert.deepEqual(previewSources("https://cdn.example.test/", file), [
    "https://cdn.example.test/bubbleparty/v4/classic/previews/" + file,
    "/bubbleparty/v4/classic/previews/" + file,
  ]);
  assert.deepEqual(previewSources("", file), [
    "/bubbleparty/v4/classic/previews/" + file,
  ]);
});

test("all four existing maps join their actual themes without changing source maps", () => {
  const choices = mapChoiceGroups.flatMap((group) => group.maps);
  assert.equal(choices.length, 116);
  assert.equal(new Set(choices.map((map) => map.id)).size, 116);
  for (const [id, family, name] of [
    ["community10", "Village", "经典小区10"],
    ["forest", "Forest", "森林公园"],
    ["desert", "Desert", "沙丘营地"],
    ["waterpark", "Ocean", "水乐园"],
  ]) {
    const group = mapChoiceGroups.find((group) =>
      group.maps.some((map) => map.id === id),
    );
    assert.equal(group.family, family);
    assert.equal(group.maps.find((map) => map.id === id).name, name);
  }
  assert.equal(choices.filter((map) => map.id === "random").length, 0);
  assert.ok(
    mapChoiceGroups.every((group) => !/[0-9]|现有|留存/.test(group.label)),
  );
});
