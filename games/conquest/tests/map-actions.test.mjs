// Before map-action implementation: half dispatches must use the remaining
// server-projected army, never the original army; odd/single units, debt,
// insufficient movement/sea transport, population and capacity must be safe.
import test from "node:test";
import assert from "node:assert/strict";
import { halfMarch, quickRecruit } from "../web/map-actions.ts";

test("repeated target taps dispatch half of the remaining army", () => {
  let remaining = 1000;
  const sent = [];
  for (let i = 0; i < 3; i++) {
    const n = halfMarch(remaining, 5000, 3.2, false, 2, 1);
    sent.push(n);
    remaining -= n;
  }
  assert.deepEqual(sent, [500, 250, 125]);
  assert.equal(remaining, 125);
});
test("one soldier, odd armies and invalid budgets stay bounded", () => {
  assert.equal(halfMarch(1, 0, 1, false, 2, 1), 1);
  assert.equal(halfMarch(501, 5000, 1, false, 2, 1), 251);
  assert.equal(halfMarch(500, 5000, 0.2, false, 2, 1), 0);
  assert.equal(halfMarch(500, -1, 1, true, 2, 1), 0);
  assert.equal(halfMarch(NaN, 5000, 3, true, 2, 1), 0);
  // The promised half must not silently shrink into an unsafe tiny landing.
  assert.equal(halfMarch(1000, 200, 1, true, 2, 1), 0);
  assert.equal(halfMarch(1000, 999, 1, true, 2, 1), 0);
  assert.equal(halfMarch(1000, 1000, 1, true, 2, 1), 500);
});
test("one-tap recruitment respects cash, population, capacity and action points", () => {
  assert.equal(quickRecruit(5000, 25000, 500, 3.2, 1, 1), 1000);
  assert.equal(quickRecruit(175, 25000, 500, 3.2, 1, 1), 175);
  assert.equal(quickRecruit(5000, 250, 500, 3.2, 1, 1), 125);
  assert.equal(quickRecruit(5000, 25000, 999900, 3.2, 1, 1), 99);
  assert.equal(quickRecruit(-20, 25000, 500, 3.2, 1, 1), 0);
  assert.equal(quickRecruit(5000, 25000, 500, 0.2, 1, 1), 0);
});
