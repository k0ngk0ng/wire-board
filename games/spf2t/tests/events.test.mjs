// Failure inventory: reconnect/reload replays old animation or audio; an
// opponent's clear uses the viewer's sound; a spectator hears private cues.
import { test } from "node:test";
import assert from "node:assert/strict";
import { fxFor, nextEvents, soundFor } from "../web/events.ts";

const event = (id, type, seat, extra = {}) => ({
  id,
  type,
  at: id * 100,
  seat,
  ...extra,
});

test("a new scope initializes at the latest event and never replays history", () => {
  const events = [event(1, "start", -1), event(2, "lock", 0), event(3, "clear", 0)];
  const first = nextEvents(events, { scope: "old", id: 0 }, "room:epoch:0");
  assert.deepEqual(first.events, []);
  assert.equal(first.seen.id, 3);
  const same = nextEvents(events, first.seen, "room:epoch:0");
  assert.deepEqual(same.events, []);
});

test("only newer events are emitted and ids stay monotonic", () => {
  const seen = { scope: "room:epoch:0", id: 2 };
  const result = nextEvents(
    [event(1, "start", -1), event(3, "clear", 0), event(4, "finish", -1)],
    seen,
    seen.scope,
  );
  assert.deepEqual(result.events.map((item) => item.id), [3, 4]);
  assert.equal(result.seen.id, 4);
});

test("sound is limited to the viewer's seat, so an opponent's clear is silent", () => {
  assert.equal(soundFor(event(1, "countdown", -1), 0, false, false), "countdown");
  assert.equal(soundFor(event(2, "lock", 1), 0, false, false), null);
  assert.equal(soundFor(event(3, "lock", 0), 0, false, false), "lock");
  assert.equal(soundFor(event(4, "clear", 0, { chain: 2 }), 0, false, false), "chain");
  assert.equal(soundFor(event(5, "clear", 0, { chain: 1 }), 0, false, false), "crash");
  assert.equal(soundFor(event(6, "garbage", 0), 0, false, false), "garbage");
  assert.equal(soundFor(event(7, "clear", 0, { chain: 2 }), -1, true, false), null);
  assert.equal(soundFor(event(8, "finish", -1), 0, false, true), "win");
  assert.equal(soundFor(event(9, "finish", -1), 0, false, false), "lose");
});

test("visual effects carry the cleared cells and ignore non-spatial events", () => {
  const effect = fxFor(event(1, "clear", 0, { cells: [{ x: 1, y: 2 }] }));
  assert.deepEqual(effect, { type: "clear", cells: [{ x: 1, y: 2 }] });
  assert.equal(fxFor(event(2, "attack", 0)), null);
  assert.equal(fxFor(event(3, "garbage", 1))?.type, "garbage");
});
