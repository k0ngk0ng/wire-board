// First list and exercise replay failures before implementing the cursor:
// initial/reconnected events replay; same-ID arrival missed or repeated; mute
// queues old clips; old events accumulate; hidden motion resumes on foreground.
import test from "node:test";
import assert from "node:assert/strict";
import { createCursor, consumeEvents } from "../web/events.ts";
const move = { id: 1, type: "move", at: 100, until: 300, captured: false };
const snapshot = (events = [], round = 1) => ({ events, round });
test("initial history never replays, including an in-flight event on arrival", () => {
  const c = createCursor(snapshot([move]), 0);
  const x = consumeEvents(
    c,
    snapshot([{ ...move, after: {}, captured: true }]),
    0,
    301,
    true,
    true,
  );
  assert.deepEqual(x.clips, []);
  assert.deepEqual(x.events, []);
});
test("new march and updated same-ID capture play once each", () => {
  const c = createCursor(snapshot(), 0);
  const a = consumeEvents(c, snapshot([move]), 0, 120, true, true);
  assert.deepEqual(a.clips, ["march"]);
  assert.equal(a.events.length, 1);
  const b = consumeEvents(
    c,
    snapshot([{ ...move, after: {}, captured: true }]),
    0,
    301,
    true,
    true,
  );
  assert.deepEqual(b.clips, ["capture"]);
  assert.deepEqual(
    consumeEvents(
      c,
      snapshot([{ ...move, after: {}, captured: true }]),
      0,
      302,
      true,
      true,
    ).clips,
    [],
  );
});
test("presentation epoch clears backlog and consumes missed rounds without sounds", () => {
  const c = createCursor(snapshot(), 0);
  const a = consumeEvents(
    c,
    snapshot([{ ...move, after: {}, captured: true }], 3),
    1,
    301,
    true,
    true,
  );
  assert.equal(a.reset, true);
  assert.deepEqual(a.clips, []);
  assert.deepEqual(a.events, []);
  assert.deepEqual(
    consumeEvents(
      c,
      snapshot([{ ...move, after: {}, captured: true }], 3),
      1,
      302,
      true,
      true,
    ).clips,
    [],
  );
});
test("muted and reduced-motion events are consumed without deferred playback", () => {
  const c = createCursor(snapshot(), 0);
  assert.deepEqual(
    consumeEvents(c, snapshot([move]), 0, 120, false, false).clips,
    [],
  );
  assert.deepEqual(
    consumeEvents(c, snapshot([move]), 0, 130, true, true).clips,
    [],
  );
  assert.deepEqual(
    consumeEvents(c, snapshot([move]), 0, 130, true, true).events,
    [],
  );
});
test("old arrivals do not produce late capture sounds, new round sounds once", () => {
  const c = createCursor(snapshot(), 0);
  assert.deepEqual(
    consumeEvents(
      c,
      snapshot([{ ...move, after: {}, captured: true }]),
      0,
      5000,
      true,
      true,
    ).clips,
    [],
  );
  assert.deepEqual(
    consumeEvents(c, snapshot([], 2), 0, 5100, true, true).clips,
    ["round"],
  );
  assert.deepEqual(
    consumeEvents(c, snapshot([], 2), 0, 5200, true, true).clips,
    [],
  );
});
test("arrival bookkeeping remains bounded by retained server events", () => {
  const c = createCursor(snapshot(), 0);
  for (let id = 1; id < 500; id++)
    consumeEvents(
      c,
      snapshot([{ ...move, id, after: {} }]),
      0,
      1000,
      true,
      true,
    );
  assert.ok(c.arrivals.size <= 1);
});
