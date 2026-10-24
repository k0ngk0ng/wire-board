// Preimplementation: collision boundary interpolation, mirrored seats, frozen
// clocks, refresh sound replay, and delayed/duplicate authoritative events.
import test from "node:test";
import assert from "node:assert/strict";
import {
  sampleFlight,
  localPoint,
  gameNow,
  EventTimeline,
} from "../web/timeline.ts";
const flight = {
  segments: [
    {
      at: 1000,
      until: 1500,
      p: { x: 0, y: 1, z: 1 },
      v: { x: 1, y: -1, z: -2 },
      a: { x: 0, y: 0, z: 0 },
    },
    {
      at: 1500,
      until: 2000,
      p: { x: 0.5, y: 0.5, z: 0 },
      v: { x: 1, y: 1, z: -2 },
      a: { x: 0, y: 0, z: 0 },
    },
  ],
  startAt: 1000,
  endAt: 2000,
};
test("samples the correct side of a bounce without interpolating through the table", () => {
  assert.deepEqual(sampleFlight(flight, 1500), { x: 0.5, y: 0.5, z: 0 });
  assert.deepEqual(sampleFlight(flight, 1750), { x: 0.75, y: 0.75, z: -0.5 });
  assert.deepEqual(sampleFlight(flight, 3000), { x: 1, y: 1, z: -1 });
});
test("far seat rotates view coordinates without changing ball height", () => {
  const p = { x: 0.3, y: 1.1, z: -0.8 };
  assert.deepEqual(localPoint(p, 1), { x: -0.3, y: 1.1, z: 0.8 });
  assert.deepEqual(localPoint(p, 0), p);
});
test("clock freezes during loading/pause", () => {
  assert.equal(
    gameNow({ serverNow: 1000, receivedAt: 200, paused: true }, 900),
    1000,
  );
  assert.equal(gameNow({ serverNow: 1000, receivedAt: 200 }, 900), 1700);
});
test("refresh skips old events; new events wait for their actual impact time", () => {
  const q = new EventTimeline();
  q.reset([{ id: 5, at: 900, type: "hit" }], 1000);
  assert.deepEqual(
    q.take(
      [
        { id: 5, at: 900, type: "hit" },
        { id: 6, at: 1100, type: "bounce" },
      ],
      1050,
    ),
    [],
  );
  assert.deepEqual(
    q.take([{ id: 6, at: 1100, type: "bounce" }], 1100).map((e) => e.id),
    [6],
  );
  assert.deepEqual(q.take([{ id: 6, at: 1100, type: "bounce" }], 1200), []);
});
test("stale effects are discarded without hiding a current impact", () => {
  const q = new EventTimeline();
  q.reset([], 0);
  assert.deepEqual(
    q
      .take(
        [
          { id: 1, at: 100, type: "hit" },
          { id: 2, at: 900, type: "bounce" },
        ],
        1000,
      )
      .map((e) => e.id),
    [2],
  );
});

test("late delivery of pre-resume sounds stays silent after a visibility reset", () => {
  const q = new EventTimeline();
  q.reset([{ id: 5, at: 900, type: "hit" }], 1000);
  assert.deepEqual(
    q
      .take(
        [
          { id: 6, at: 990, type: "bounce" },
          { id: 7, at: 1040, type: "hit" },
        ],
        1050,
      )
      .map((e) => e.id),
    [7],
  );
});
