import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { sampleProjectile } from "../web/motion.ts";
test("arrows fly after release, arrive at the damage frame and disappear after contact", () => {
  const p = {
    kind: "arrow",
    from: [517, 384],
    to: [101, 150],
    launchAt: 300,
    flightDuration: 400,
    impactAt: 700,
    lingerDuration: 120,
  };
  assert.equal(sampleProjectile(p, 299), null);
  const mid = sampleProjectile(p, 500);
  assert.equal(mid.phase, "flight");
  assert.deepEqual([mid.x, mid.y], [309, 267]);
  const hit = sampleProjectile(p, 700);
  assert.equal(hit.phase, "stick");
  assert.deepEqual([hit.x, hit.y], [101, 150]);
  assert.equal(sampleProjectile(p, 821), null);
});
test("boomerang returns as a projectile while its thrower remains stationary", () => {
  const p = {
    kind: "boomerang",
    from: [500, 380],
    to: [100, 160],
    launchAt: 100,
    flightDuration: 300,
    impactAt: 400,
    lingerDuration: 100,
    returnDuration: 300,
  };
  const back = sampleProjectile(p, 650);
  assert.equal(back.phase, "return");
  assert.deepEqual([back.x, back.y], [300, 270]);
  assert.equal(sampleProjectile(p, 801), null);
});
test("native directional arrows, shadows and stones are declared with immutable metadata", () => {
  const art = JSON.parse(
    readFileSync(new URL("../web/projectile-art.json", import.meta.url)),
  );
  assert.equal(art.basePath, "assets");
  for (const id of [
    ...Array.from({ length: 16 }, (_, i) => 25630 + i),
    ...Array.from({ length: 16 }, (_, i) => 25650 + i),
    25785,
    25786,
  ]) {
    const f = art.bitmaps[id];
    assert.ok(f.width > 0 && f.height > 0);
    assert.ok(art.resources[f.file].bytes > 0);
    assert.match(art.resources[f.file].sha256, /^[a-f0-9]{64}$/);
  }
});
