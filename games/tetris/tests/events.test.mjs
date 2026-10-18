// Failure inventory: reconnect/reload replays old animation or audio; an
// opponent's clear uses the viewer's sound; a spectator hears private cues;
// reduced motion still animates.
import { test } from "node:test";
import assert from "node:assert/strict";
import { animationFor, nextEvents, soundFor } from "../web/events.ts";

const event = (id, type, seat, extra = {}) => ({
  id,
  type,
  at: id * 100,
  seat,
  ...extra,
});

test("a new scope initializes at the latest event and never replays history", () => {
  const events = [event(1, "start", -1), event(2, "lock", 0), event(3, "clear", 0)];
  const first = nextEvents(events, { scope: "old", id: 0 }, "room:epoch:0:0");
  assert.deepEqual(first.events, []);
  assert.equal(first.seen.id, 3);
  const same = nextEvents(events, first.seen, "room:epoch:0:0");
  assert.deepEqual(same.events, []);
});

test("only newer events are emitted and ids stay monotonic", () => {
  const seen = { scope: "room:epoch:0:0", id: 2 };
  const result = nextEvents(
    [event(1, "start", -1), event(3, "clear", 0), event(4, "level", 0)],
    seen,
    seen.scope,
  );
  assert.deepEqual(result.events.map((item) => item.id), [3, 4]);
  assert.equal(result.seen.id, 4);
});

test("sound is limited to the viewer's seat and global start/finish", () => {
  assert.equal(soundFor(event(1, "start", -1), 0, false), "start");
  assert.equal(soundFor(event(2, "lock", 1), 0, false), null);
  assert.equal(soundFor(event(3, "lock", 0), 0, false), "lock");
  assert.equal(soundFor(event(4, "clear", 0, { count: 4 }), 0, false), "tetris");
  assert.equal(soundFor(event(5, "garbage", 0, { applied: true }), 0, false), "garbage");
  assert.equal(soundFor(event(6, "garbage", 0), 0, false), null);
  assert.equal(soundFor(event(7, "clear", 0, { count: 2 }), -1, true), null);
  assert.equal(soundFor(event(8, "finish", 0), 0, false), "win");
  assert.equal(soundFor(event(9, "finish", -1), -1, true), "finish");
});

test("animation mapping covers clear, garbage, elimination and finish", () => {
  assert.equal(animationFor(event(1, "clear", 0)), "clear");
  assert.equal(animationFor(event(2, "garbage", 0)), null);
  assert.equal(animationFor(event(3, "garbage", 0, { applied: true })), "garbage");
  assert.equal(animationFor(event(4, "eliminate", 0)), "eliminate");
  assert.equal(animationFor(event(5, "finish", 0)), "finish");
  assert.equal(animationFor(event(6, "start", -1)), null);
});
