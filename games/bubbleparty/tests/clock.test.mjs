// Failure cases: finished state freezes serverAt, so a later snapshot/reload
// must not restart death/rescue animations; zero receivedAt is a valid origin;
// a backwards client performance sample must never rewind before server time.
import { test } from "node:test";
import assert from "node:assert/strict";
import { animationNow } from "../web/clock.ts";
test("finished snapshots use the platform server clock, not frozen simulation time", () => {
  assert.equal(
    animationNow(1000, { serverNow: 5000, receivedAt: 100 }, 150),
    5050,
  );
  assert.equal(
    animationNow(1000, { serverNow: 9000, receivedAt: 0 }, 20),
    9020,
  );
});
test("clock interpolation honors zero, missing server clock and backwards samples", () => {
  assert.equal(animationNow(1000, { serverNow: 0, receivedAt: 0 }, 80), 1080);
  assert.equal(
    animationNow(1000, { serverNow: 900, receivedAt: 100 }, 50),
    1000,
  );
});
