import { test } from "node:test";
import assert from "node:assert/strict";
import {
  classInfo,
  classOrder,
  clamp,
  local,
  mapNames,
  sources,
} from "../web/types.ts";

test("aim and movement controls stay inside server ranges", () => {
  assert.equal(clamp(3, 5, 85), 5);
  assert.equal(clamp(86, 5, 85), 85);
  assert.equal(clamp(20, 20, 100), 20);
  assert.equal(clamp(101, 20, 100), 100);
});

test("asset URLs prefer the configured base and always retain a local fallback", () => {
  assert.deepEqual(sources("https://cdn.example.test/bga/v1", "shot.wav"), [
    "https://cdn.example.test/bga/v1/fortressblue/v1/audio/shot.wav",
    "/fortressblue/v1/audio/shot.wav",
  ]);
  assert.deepEqual(sources("", "shot.wav"), ["/fortressblue/v1/audio/shot.wav"]);
  assert.equal(local("battle.wav"), "/fortressblue/v1/audio/battle.wav");
});

test("metadata covers every map and class without duplicate ids", () => {
  assert.equal(Object.keys(mapNames).length, 6);
  assert.equal(new Set(classOrder).size, 5);
  for (const id of classOrder) {
    assert.ok(classInfo[id]);
    assert.ok(classInfo[id].maxHp > 0);
    assert.ok(classInfo[id].move > 0);
  }
});
