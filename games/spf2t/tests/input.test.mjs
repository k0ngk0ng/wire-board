// Failure inventory: batches overlap, stale sequence retries duplicate input,
// a stuck in-flight request grows forever, DAS repeats double-fire.
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
  queue.acknowledge(11);
  const third = queue.take();
  assert.equal(third, null);
  queue.reset(20);
  assert.equal(queue.take(), null);
  queue.push("hardDrop");
  const afterReset = queue.take();
  assert.equal(afterReset.seq, 21);
  assert.equal(afterReset.events[0].kind, "hardDrop");
  assert.equal(SENT_MAX, 2);
});

test("pending input is capped instead of silently dropping old moves", () => {
  const queue = new InputQueue(0);
  for (let i = 0; i < PENDING_MAX; i++) assert.equal(queue.push("right"), true);
  assert.equal(queue.push("right"), false);
  queue.clearPending();
  assert.equal(queue.take(), null);
});

test("DAS timing includes the delay before the first repeat", () => {
  assert.equal(repeatTimes(0, 149, 150, 50), 0);
  assert.equal(repeatTimes(0, 150, 150, 50), 1);
  assert.equal(repeatTimes(0, 199, 150, 50), 1);
  assert.equal(repeatTimes(0, 200, 150, 50), 2);
  assert.equal(repeatTimes(0, 0, 50, 50), 0);
  assert.equal(repeatTimes(0, 50, 50, 50), 1);
});
