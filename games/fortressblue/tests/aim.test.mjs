import { test } from "node:test";
import assert from "node:assert/strict";
import {
  clamp,
  mapById,
  maps,
  sourceList,
  tankById,
  tanks,
  worldHeight,
} from "../web/types.ts";

test("aim and movement controls stay inside server ranges", () => {
  assert.equal(clamp(3, 0, 90), 3);
  assert.equal(clamp(91, 0, 90), 90);
  assert.equal(clamp(20, 20, 100), 20);
  assert.equal(clamp(101, 20, 100), 100);
});

test("asset URLs prefer CDN and always retain a same-origin fallback", () => {
  assert.deepEqual(sourceList("https://cdn.example.test/bga/v1", "/fortressblue/v2/audio/fire.mp3"), [
    "https://cdn.example.test/bga/v1/fortressblue/v2/audio/fire.mp3",
    "/fortressblue/v2/audio/fire.mp3",
  ]);
  assert.deepEqual(sourceList("", "/fortressblue/v2/audio/fire.mp3"), [
    "/fortressblue/v2/audio/fire.mp3",
  ]);
});

test("original catalog exposes all 17 maps and 14 tanks", () => {
  assert.equal(worldHeight, 960);
  assert.equal(maps.length, 17);
  assert.equal(new Set(maps.map((m) => m.id)).size, 17);
  assert.equal(tanks.length, 14);
  assert.equal(new Set(tanks.map((t) => t.id)).size, 14);
  for (const map of maps) {
    assert.ok(map.width >= 1200);
    assert.ok(map.height === worldHeight);
    assert.ok(map.thumb.startsWith("/fortressblue/v2/maps/"));
    assert.ok(map.fg.startsWith("/fortressblue/v2/maps/"));
    assert.ok(mapById.get(map.id));
  }
  for (const tank of tanks) {
    assert.ok(tank.hp > 0 && tank.primary > 0 && tank.angle > 0);
    assert.ok(tank.sprites.idle && tank.bullet);
    assert.ok(tankById.get(tank.id));
  }
});
