// Failure scenarios: see interaction-acceptance.md (written before implementation).
import { test } from "node:test";
import assert from "node:assert/strict";
import { CombatTimeline, FLIGHT_MS } from "../web/combat-timeline.ts";
const frame = (tick, hp, shield = 0) => ({ tick, enemies: hp > 0 ? [{ id: 1, hp, shield }] : [] });
test("health, shield and death wait until projectile arrival", () => {
  const a = frame(1, 100, 20), b = frame(2, 70), c = frame(3, 0);
  const t = new CombatTimeline(a);
  t.push(b, 1000);
  assert.deepEqual(t.read(1000 + FLIGHT_MS - 1), a);
  assert.deepEqual(t.read(1000 + FLIGHT_MS), b);
  t.push(c, 1200);
  assert.deepEqual(t.read(1200 + FLIGHT_MS - 1), b);
  assert.deepEqual(t.read(1200 + FLIGHT_MS), c);
});
test("rapid snapshots keep damage, healing and death in order", () => {
  const t = new CombatTimeline(frame(0, 100));
  t.push(frame(1, 60), 10); t.push(frame(2, 80), 30); t.push(frame(3, 0), 40);
  assert.equal(t.read(10 + FLIGHT_MS).enemies[0].hp, 60);
  assert.equal(t.read(30 + FLIGHT_MS).enemies[0].hp, 80);
  assert.equal(t.read(40 + FLIGHT_MS).enemies.length, 0);
});
test("reset on reconnect, pause or reduced motion discards buffered history", () => {
  const t = new CombatTimeline(frame(2, 100));
  t.push(frame(3, 80), 100); t.reset(frame(8, 40));
  assert.equal(t.read(99999).enemies[0].hp, 40);
});
test("a stalled animation frame catches up to the latest authoritative snapshot", () => {
  const t = new CombatTimeline(frame(0, 100));
  for(let i=1;i<=50;i++) t.push(frame(i, 100-i), i*200);
  assert.equal(t.read(20000).tick, 50);
});
