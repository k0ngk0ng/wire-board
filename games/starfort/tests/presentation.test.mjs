// Failure scenarios specified before code: reconnect replays old shots/sounds;
// snapshots repeat events; room changes leak prior events; clock/tick rewind
// animates a stale game; interpolation cuts path corners or extrapolates;
// deleted enemies keep moving; reduced-motion keeps moving; rapid attacks
// flood the shared player; important base/victory feedback is lost.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  freshEvents,
  pointOnPath,
  enemyPoint,
  pickSound,
  assetURL,
  spritePlacement,
  battlefieldTopInset,
} from "../web/presentation.ts";

const path = [
  { x: 0, y: 0 },
  { x: 100, y: 0 },
  { x: 100, y: 100 },
];
const event = (id, type = "shot", kind = "arrow") => ({
  id,
  type,
  kind,
  tick: id,
  x: 10,
  y: 10,
});
const snapshot = (tick, events) => ({ tick, events });

test("first snapshot and a newly joined room never replay history", () => {
  let [cursor, events] = freshEvents(
    null,
    "one",
    snapshot(10, [event(2), event(3)]),
    1000,
  );
  assert.deepEqual(events, []);
  [cursor, events] = freshEvents(cursor, "two", snapshot(20, [event(8)]), 1200);
  assert.deepEqual(events, []);
  assert.equal(cursor.lastID, 8);
});
test("only fresh increasing event ids play, duplicate snapshots stay quiet", () => {
  let [cursor] = freshEvents(null, "one", snapshot(10, [event(2)]), 1000);
  let events;
  [cursor, events] = freshEvents(
    cursor,
    "one",
    snapshot(11, [event(2), event(3), event(4)]),
    2000,
  );
  assert.deepEqual(
    events.map((e) => e.id),
    [3, 4],
  );
  [cursor, events] = freshEvents(
    cursor,
    "one",
    snapshot(11, [event(2), event(3), event(4)]),
    2100,
  );
  assert.deepEqual(events, []);
});
test("reconnect gaps, tick rewind, and excessive replay batches reset safely", () => {
  const [cursor] = freshEvents(null, "one", snapshot(20, [event(4)]), 1000);
  assert.deepEqual(
    freshEvents(cursor, "one", snapshot(30, [event(5)]), 7000)[1],
    [],
  );
  assert.deepEqual(
    freshEvents(cursor, "one", snapshot(10, [event(5)]), 1200)[1],
    [],
  );
  assert.ok(
    freshEvents(
      cursor,
      "one",
      snapshot(
        21,
        Array.from({ length: 150 }, (_, i) => event(i + 5)),
      ),
      1200,
    )[1].length <= 40,
  );
});
test("path interpolation rounds a corner and clamps malformed distances", () => {
  assert.deepEqual(pointOnPath(path, 125), { x: 100, y: 25 });
  assert.deepEqual(pointOnPath(path, -50), { x: 0, y: 0 });
  assert.deepEqual(pointOnPath(path, 500), { x: 100, y: 100 });
  assert.deepEqual(pointOnPath(path, NaN), { x: 0, y: 0 });
  assert.deepEqual(pointOnPath([], 5), { x: 0, y: 0 });
});
test("enemy motion follows authoritative progress, never guesses beyond the snapshot", () => {
  const previous = { id: 1, x: 75, y: 0, progress: 75 };
  const current = { id: 1, x: 100, y: 75, progress: 175 };
  assert.deepEqual(enemyPoint(path, previous, current, 0.5, false), {
    x: 100,
    y: 25,
  });
  assert.deepEqual(enemyPoint(path, previous, current, 2, false), {
    x: 100,
    y: 75,
  });
  assert.deepEqual(enemyPoint(path, previous, current, 0.1, true), {
    x: 100,
    y: 75,
  });
  assert.deepEqual(enemyPoint(path, undefined, current, 0.1, false), {
    x: 100,
    y: 75,
  });
  assert.deepEqual(
    enemyPoint(path, { ...previous, id: 2 }, current, 0.1, false),
    { x: 100, y: 75 },
  );
});
test("one cue per snapshot prioritizes victory and damage, with throttled fire", () => {
  assert.equal(pickSound([event(1), event(2, "base_hit")], 2000, 0), "leak");
  assert.equal(pickSound([event(1), event(2, "win")], 2000, 0), "victory");
  assert.equal(pickSound([event(1)], 100, 0), null);
  assert.equal(pickSound([event(1, "shot", "cannon")], 2000, 0), "cannon");
  assert.equal(pickSound([event(1, "pause")], 2000, 0), null);
});
test("asset URLs use one namespace and support bundled fallback", () => {
  assert.equal(
    assetURL("https://cdn.example/assets/", "atlas.webp"),
    "https://cdn.example/assets/starfort/v1/atlas.webp",
  );
  assert.equal(assetURL("", "map.webp"), "/starfort/v1/map.webp");
});

// Edge placement failures: an enlarged mobile tower clips above top pads;
// right-edge art is cut off; fixing art accidentally moves a gameplay anchor.
test("tower sprite placement stays within the field while preserving safe anchors", () => {
  const bounds = { width: 960, height: 640 };
  const original = { x: 140, y: 54 };
  const top = spritePlacement(original, 140, 164, bounds);
  assert.deepEqual(original, { x: 140, y: 54 });
  assert.deepEqual(top, { x: 140, y: 168 });
  const edge = spritePlacement({ x: 930, y: 660 }, 140, 164, bounds);
  assert.ok(edge.x - 70 >= 4 && edge.x + 70 <= 956);
  assert.ok(edge.y - 164 >= 4 && edge.y <= 636);
  assert.deepEqual(spritePlacement({ x: 400, y: 350 }, 140, 164, bounds), {
    x: 400,
    y: 350,
  });
});

// Camera failures: clamping top-edge art down overlaps a different tower;
// resizing leaves hit targets at old locations; hidden surfaces divide by zero.
test("camera padding keeps top towers at their true anchors on mobile and desktop", () => {
  const mobile = battlefieldTopInset(288, 960, 40);
  const desktop = battlefieldTopInset(1060, 960, 40);
  assert.equal(mobile, 114);
  assert.equal(desktop, 27);
  const anchor = { x: 140, y: 54 };
  const size = 42 / (288 / 960);
  const placed = spritePlacement(anchor, size, (size * 224) / 192, {
    width: 960,
    height: 640,
    top: -mobile,
  });
  assert.deepEqual(placed, anchor);
  assert.ok(placed.y - (size * 224) / 192 + mobile >= 4);
  assert.equal(battlefieldTopInset(0, 960, 40), 0);
});
