import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { classicVehicles, classicTrackNames } from "../web/classic-catalog.ts";
import { classicVehicle } from "../web/classic-motion.ts";
import { selectClassicAssets } from "../web/classic-assets.ts";

const base = new URL("../assets/public/racing/v3/", import.meta.url);
const read = name => readFileSync(new URL(name, base));

test("three new original vehicles have distinct models, physics, audio and previews", () => {
  const manifest = JSON.parse(read("manifest.json"));
  assert.equal(classicVehicles.length, 8);
  const source = JSON.parse(read("cars.json")).cars;
  for (const id of ["bmw5", "jxkr", "mclk"]) {
    const car = classicVehicles.find(car => car.id === id);
    assert(car && classicVehicle(id).id === id);
    assert(source[id.toUpperCase()]?.cockpitSource?.length === 2);
    for (const path of [`cars/${id}.glb`, `audio/${id}/engine.json`, `previews/cars/${id}.webp`]) {
      const item = manifest.assets.find(item => item.path === path);
      assert(item, path);
      const bytes = read(path);
      assert.equal(bytes.length, item.bytes);
      assert.equal(createHash("sha256").update(bytes).digest("hex"), item.sha256);
    }
  }
});

test("Germany has its own route and day/night scenes without eager loading other courses", () => {
  assert.equal(typeof classicTrackNames.germany, "string");
  assert.equal(Object.keys(classicTrackNames).length, 5);
  const manifest = JSON.parse(read("manifest.json"));
  const route = JSON.parse(read("tracks/germany.route.json"));
  assert.equal(route.id, "germany");
  assert(route.points.length >= 300 && route.length > 2000);
  assert.notEqual(route.geometryHash, JSON.parse(read("tracks/france.route.json")).geometryHash);
  for (const time of ["day", "night"]) {
    const selected = selectClassicAssets(manifest, "germany", time, ["bmw5", "mclk"]);
    assert(selected.some(item => item.path === `tracks/germany${time === "night" ? "-night" : ""}.glb`));
    assert(!selected.some(item => /tracks\/(france|coastal|park|uk)/.test(item.path)));
    assert(!selected.some(item => item.path === "cars/jxkr.glb"));
  }
  for (const id of Object.keys(classicTrackNames)) {
    const path = `previews/tracks/${id}.webp`;
    assert(manifest.assets.some(item => item.path === path), path);
    assert(read(path).length > 1000);
  }
});
