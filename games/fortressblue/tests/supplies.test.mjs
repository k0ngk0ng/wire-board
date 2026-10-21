// Failure scenarios specified before frontend implementation: future supply
// draws early; crate teleports directly to ground; inventory precedes contact;
// refresh/reduced-motion rerolls positions; sprite indices escape original
// frame groups; missing media hides reward; original direction is mirrored.
import test from "node:test";
import assert from "node:assert/strict";
import { supplyPose, supplySpriteFrame } from "../web/supplies.ts";
const drop = {
  id: 1,
  itemId: 0,
  x: 500,
  y: 750,
  fromX: 460,
  at: 1000,
  dropAt: 2000,
  landAt: 4000,
  landed: false,
};
test("authoritative delivery is hidden before start and falls continuously until landing", () => {
  assert.equal(supplyPose(drop, 999), null);
  const approach = supplyPose(drop, 1500);
  assert.equal(approach.crate, null);
  assert.ok(approach.helicopter.y > 0 && approach.helicopter.y < 50);
  const released = supplyPose(drop, 2000);
  assert.equal(released.crate.x, 460);
  assert.equal(released.crate.y, 45);
  const mid = supplyPose(drop, 3000);
  assert.equal(mid.crate.x, 480);
  assert.equal(mid.crate.y, 45 + (750 - 45) * 0.25);
  assert.equal(mid.crate.airborne, true);
  const landed = supplyPose(drop, 4000);
  assert.equal(landed.crate.x, 500);
  assert.equal(landed.crate.y, 750);
  assert.equal(landed.crate.airborne, false);
  assert.equal(landed.helicopter, null);
  assert.deepEqual(supplyPose(drop, 3000), mid);
});
test("all frame selections preserve source rectangles and freeze only animation in reduced motion", () => {
  for (let now = 0; now < 10000; now += 29) {
    for (const kind of ["crate", "helicopter"]) {
      const f = supplySpriteFrame(kind, now, false, kind === "crate");
      assert.ok(f.w > 0 && f.h > 0);
      assert.ok(f.index >= 0 && f.index < (kind === "crate" ? 9 : 10));
    }
  }
  assert.equal(supplySpriteFrame("crate", 9999, true, false).index, 0);
  assert.equal(supplySpriteFrame("helicopter", 9999, true).index, 0);
});
