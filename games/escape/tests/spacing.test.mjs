// Before implementation: overlap hides a teammate; camera/input is displaced;
// another room affects this room; order flickers; authority is mutated; or a
// separated actor is incorrectly moved outside the playable floor.
import { test } from "node:test";
import assert from "node:assert/strict";
import { spreadCharacters } from "../web/spacing.ts";
const players = Array.from({ length: 4 }, (_, seat) => ({ seat, room: "r4" }));
test("four coincident characters remain separately readable without moving the viewer", () => {
  const points = new Map(players.map((p) => [p.seat, { x: 500, y: 420 }]));
  const before = JSON.stringify([...points]);
  const display = spreadCharacters(players, points, 0);
  assert.deepEqual(display.get(0), { x: 500, y: 420 });
  assert.equal(
    new Set([...display.values()].map((p) => `${p.x},${p.y}`)).size,
    4,
  );
  assert.equal(JSON.stringify([...points]), before);
  assert.deepEqual(
    spreadCharacters([...players].reverse(), points, 0),
    display,
  );
});
test("a single actor and actors in separate rooms retain their actual locations", () => {
  const points = new Map([
    [0, { x: 400, y: 400 }],
    [1, { x: 400, y: 400 }],
  ]);
  const pair = [
    { seat: 0, room: "r0" },
    { seat: 1, room: "r1" },
  ];
  assert.deepEqual(spreadCharacters(pair, points, 0), points);
});
test("crowd presentation stays inside floor bounds", () => {
  const points = new Map(players.map((p) => [p.seat, { x: 145, y: 270 }]));
  const display = spreadCharacters(players, points, -1);
  for (const p of display.values()) {
    assert.ok(p.x >= 145 && p.x <= 855 && p.y >= 270 && p.y <= 565);
  }
});
