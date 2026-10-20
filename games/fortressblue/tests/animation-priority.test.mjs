// Failure scenarios before changing renderer priority: a nonfatal hazard hit
// during Fire's first five frames must not suppress the muzzle animation;
// death still overrides it; impact/fall and damage may take over after launch;
// another seat and old shots without a windup must not inherit this protection.
import test from "node:test";
import assert from "node:assert/strict";
import { tankPose } from "../web/animation.ts";
test("nonfatal damage cannot interrupt firing through the complete muzzle frame", () => {
  const input = {
    alive: true,
    seat: 0,
    now: 5200,
    shot: { seat: 0, at: 5000, launchAt: 5334 },
    damageAt: 5100,
    actions: {
      fire: { durationMs: 667 },
      afterFire: { durationMs: 333 },
      damage: { durationMs: 667 },
      death: { durationMs: 667 },
      dead: { durationMs: 1333 },
    },
    reduced: false,
  };
  assert.deepEqual(tankPose(input), { action: "fire", elapsed: 200 });
  assert.deepEqual(tankPose({ ...input, alive: false, deathAt: 5100 }), {
    action: "death",
    elapsed: 100,
  });
  for (const now of [5334, 5350, 5400])
    assert.deepEqual(tankPose({ ...input, now }), {
      action: "fire",
      elapsed: now - 5000,
    });
  assert.deepEqual(tankPose({ ...input, now: 5401 }), {
    action: "damage",
    elapsed: 301,
  });
  assert.deepEqual(tankPose({ ...input, seat: 1 }), {
    action: "damage",
    elapsed: 100,
  });
  assert.deepEqual(tankPose({ ...input, shot: { seat: 0, at: 5000 } }), {
    action: "damage",
    elapsed: 100,
  });
  assert.deepEqual(
    tankPose({ ...input, now: 5514, fall: { at: 5514, endAt: 6000 } }),
    { action: "fall", elapsed: 0 },
  );
  assert.deepEqual(tankPose({ ...input, damageAt: undefined, now: 5800 }), {
    action: "afterFire",
    elapsed: 133,
  });
});

test("cancelled preparation cannot replay Fire and Item yields to higher priorities", () => {
  const input = {
    alive: true,
    seat: 0,
    now: 5200,
    shot: { seat: 0, at: 5000, launchAt: 5334, cancelledAt: 5100 },
    actions: {
      fire: { durationMs: 667 },
      item: { durationMs: 667 },
      damage: { durationMs: 667 },
    },
    reduced: false,
  };
  assert.deepEqual(tankPose(input), { action: "idle", elapsed: 5200 });
  assert.deepEqual(tankPose({ ...input, itemAt: 5100 }), {
    action: "item",
    elapsed: 100,
  });
  assert.deepEqual(tankPose({ ...input, itemAt: 5100, damageAt: 5150 }), {
    action: "damage",
    elapsed: 50,
  });
  assert.equal(
    tankPose({ ...input, itemAt: 5100, motionAt: 5150 }).action,
    "move",
  );
  assert.equal(
    tankPose({
      ...input,
      itemAt: 5100,
      shot: { ...input.shot, cancelledAt: undefined },
    }).action,
    "fire",
  );
});
