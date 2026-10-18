// Failure inventory: batches overlap, stale sequence retries duplicate input,
// a stuck in-flight request grows forever, DAS repeats double-fire, and a
// released touch keeps moving the piece.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  BATCH_MAX,
  InputQueue,
  PENDING_MAX,
  SENT_MAX,
  repeatTimes,
} from "../web/input.ts";

test("input batches preserve order, sequence and a bounded in-flight set", () => {
  const queue = new InputQueue(10);
  for (let i = 0; i < BATCH_MAX + 3; i++) assert.equal(queue.push("left"), true);
  const first = queue.take();
  assert.equal(first.seq, 11);
  assert.equal(first.events.length, BATCH_MAX);
  assert.equal(first.events[0].kind, "left");
  const second = queue.take();
  assert.equal(second.seq, 12);
  assert.equal(second.events.length, 3);
  assert.equal(queue.take(), null);
  assert.equal(queue.sentCount, SENT_MAX);
  queue.acknowledge(11);
  assert.equal(queue.sentCount, 1);
  assert.deepEqual(
    queue.unconfirmed().map((event) => event.kind),
    ["left", "left", "left"],
  );
  queue.reset(12);
  assert.equal(queue.take(), null);
  assert.equal(queue.pendingCount, 0);
});

test("pending input is capped instead of silently dropping old moves", () => {
  const queue = new InputQueue(0);
  for (let i = 0; i < PENDING_MAX; i++) assert.equal(queue.push("right"), true);
  assert.equal(queue.push("right"), false);
  assert.equal(queue.pendingCount, PENDING_MAX);
  queue.clearPending();
  assert.equal(queue.pendingCount, 0);
});

test("DAS timing includes the delay before the first repeat", () => {
  assert.equal(repeatTimes(0, 149, 150, 50), 0);
  assert.equal(repeatTimes(0, 150, 150, 50), 1);
  assert.equal(repeatTimes(0, 199, 150, 50), 1);
  assert.equal(repeatTimes(0, 200, 150, 50), 2);
  assert.equal(repeatTimes(0, 0, 50, 50), 0);
  assert.equal(repeatTimes(0, 50, 50, 50), 1);
});
