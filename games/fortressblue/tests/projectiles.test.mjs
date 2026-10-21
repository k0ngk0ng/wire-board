// Failure inventory first: primary and special share pixels; native directional
// sprites are rotated/mirrored; animation crosses the 32-frame weapon group;
// delayed ion arc appears before launch; early blast timestamp hides later arcs.
import test from "node:test";
import assert from "node:assert/strict";
import {
  projectileFrame,
  projectilePaths,
  effectFrame,
  blastTimestamp,
} from "../web/projectiles.ts";
import { arcAt } from "../web/timeline.ts";
test("fourteen primary/special groups preserve native direction and animation", () => {
  for (let tank = 1; tank <= 14; tank++) {
    for (const [angle, dir] of [
      [0, 0],
      [-Math.PI / 2, 2],
      [Math.PI, 4],
      [Math.PI / 2, 6],
    ]) {
      const p = projectileFrame(tank, "primary", angle, 100, false),
        s = projectileFrame(tank, "special", angle, 100, false);
      assert.equal(p.index, dir * 4 + 1);
      assert.equal(s.index, p.index + 32);
      assert.equal(p.path, s.path);
      assert.ok(p.w > 0 && p.h > 0 && s.w > 0 && s.h > 0);
    }
    assert.equal(projectileFrame(tank, "special", 0, 999, true).index, 32);
    assert.equal(projectilePaths(tank).length, 1);
  }
});
test("per-arc launch hides later stages independently", () => {
  const arc = {
    path: [
      { x: 0, y: 0 },
      { x: 0, y: 100 },
    ],
    pathTimes: [1000, 1500],
    impactAt: 2500,
    launchAt: 2000,
  };
  assert.equal(arcAt(arc, 1000, 1999, 1000), null);
  assert.ok(arcAt(arc, 1000, 2200, 1000));
  assert.equal(arcAt(arc, 1000, 2500, 1000), null);
});

test("an early impact hides only its own projectile and blast times survive shared coordinates", () => {
  const base = {
    path: [
      { x: 0, y: 0 },
      { x: 100, y: 100 },
    ],
    launchAt: 1100,
  };
  const arcs = [
    { ...base, impactAt: 1500 },
    { ...base, impactAt: 2200 },
  ];
  assert.equal(arcs.filter((a) => arcAt(a, 1000, 1600, 1100)).length, 1);
  const events = [{ type: "explode", x: 10, y: 20, at: 2100 }];
  assert.equal(
    blastTimestamp({ x: 10, y: 20, at: 1500 }, { impactAt: 2200 }, events),
    1500,
  );
  assert.equal(
    blastTimestamp({ x: 10, y: 20 }, { impactAt: 2200 }, events),
    2100,
  );
  assert.equal(blastTimestamp({ x: 11, y: 22 }, { impactAt: 2200 }, []), 2200);
});
test("native mines, reticle blink and ion beam stay in separate frame groups", () => {
  assert.equal(effectFrame("homming", 700, false), null);
  assert.equal(effectFrame("homming", 700, true).index, 0);
  for (let at = 0; at < 2000; at += 31) {
    assert.ok(effectFrame("mine", at, false).index < 4);
    assert.ok(effectFrame("ion", at, false).index < 10);
    assert.ok(effectFrame("ion", at, false, true).index >= 10);
    assert.ok(effectFrame("ion", at, false, true).index < 18);
  }
});
