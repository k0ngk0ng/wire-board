// Failure inventory BEFORE implementation: spectator sparks lead the cars by
// 110ms; autoplay audio uses another timeline; stale reconnect events replay;
// muting only the media player leaves a delayed game sound that plays on unmute.
import test from "node:test";
import assert from "node:assert/strict";
import {
  sceneTime,
  effectAge,
  soundDelay,
  RaceAudioQueue,
} from "../web/timeline.ts";
test("driver and spectator use one timestamp for vehicles, particles and sound", () => {
  const now = 1000,
    impact = 1000;
  assert.equal(sceneTime(now, true), 1000);
  assert.equal(effectAge(impact, sceneTime(now, true)), 0);
  assert.equal(soundDelay(impact, now, true), 0);
  assert.equal(sceneTime(now, false), 890);
  assert.equal(effectAge(impact, sceneTime(now, false)), null);
  assert.equal(soundDelay(impact, now, false), 110);
  assert.equal(effectAge(impact, sceneTime(1110, false)), 0);
  assert.equal(soundDelay(impact, 1110, false), 0);
});
test("long network gap never replays an old impact", () => {
  assert.equal(effectAge(1000, sceneTime(2000, false)), null);
  assert.equal(soundDelay(1000, 2000, false), null);
});
test("mute clears scheduled events even if unmuted before their visual time", () => {
  let id = 0,
    played = 0;
  const timers = new Map();
  const q = new RaceAudioQueue(
    (fn, ms) => {
      timers.set(++id, { fn, ms });
      return id;
    },
    (n) => timers.delete(n),
  );
  q.enqueue(() => played++, 110);
  assert.equal(timers.size, 1);
  q.clear();
  for (const t of timers.values()) t.fn();
  assert.equal(played, 0);
  q.enqueue(() => played++, 110);
  for (const t of [...timers.values()]) t.fn();
  assert.equal(played, 1);
  assert.equal(q.pending, 0);
});
