import test from "node:test";
import assert from "node:assert/strict";
import { adjustElementPoints } from "../web/element-input.ts";
test("native creation uses ten points and transfers a spent point to an adjacent element", () => {
  const pure = [10, 0, 0, 0];
  assert.deepEqual(adjustElementPoints(pure, 1, 1), [9, 1, 0, 0]);
  assert.deepEqual(pure, [10, 0, 0, 0]);
  assert.deepEqual(adjustElementPoints(pure, 2, 1), pure);
  assert.deepEqual(adjustElementPoints([0, 10, 0, 0], 3, 1), [0, 10, 0, 0]);
  assert.deepEqual(adjustElementPoints([7, 0, 0, 3], 0, -1), [6, 0, 0, 3]);
  assert.deepEqual(adjustElementPoints([6, 0, 0, 3], 3, 1), [6, 0, 0, 4]);
  assert.deepEqual(adjustElementPoints([0, 0, 0, 10], 3, 1), [0, 0, 0, 10]);
  assert.deepEqual(adjustElementPoints([0, 0, 0, 10], 0, -1), [0, 0, 0, 10]);
});
