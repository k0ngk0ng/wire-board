// Failure inventory: recruitment beyond population, spending debt, exhausted
// movement, sea transport without funds, fractional or invalid user input.
import test from "node:test";
import assert from "node:assert/strict";
import { recruitLimit, moveLimit, wholeAmount } from "../web/action-limits.ts";
test("recruitment uses gold, remaining population and the required action point", () => {
  assert.equal(recruitLimit(5000, 1, 200, 3.2, 1), 100);
  assert.equal(recruitLimit(50, 1, 200, 3.2, 1), 50);
  assert.equal(recruitLimit(-1, 1, 200, 3.2, 1), 0);
  assert.equal(recruitLimit(5000, 1, 200, 0.2, 1), 0);
});
test("land and sea movement share points but only sea consumes transport funds", () => {
  assert.equal(moveLimit(500, 3.2, 1, false, 0, 2), 500);
  assert.equal(moveLimit(500, 3.2, 1, true, 100, 2), 50);
  assert.equal(moveLimit(500, 0.2, 1, false, 5000, 2), 0);
  assert.equal(moveLimit(500, 3.2, 1, true, -1, 2), 0);
});
test("user quantities remain finite nonnegative integers", () => {
  assert.equal(wholeAmount("3.9"), 3);
  assert.equal(wholeAmount("-20"), 0);
  assert.equal(wholeAmount("Infinity"), 0);
  assert.equal(wholeAmount(""), 0);
});
