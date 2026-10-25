import test from "node:test";
import assert from "node:assert/strict";
import { reorderVisible } from "../src/admin/game-order.ts";
test("filtered sorting preserves the slots of all hidden-by-filter games", () => {
  const all = ["a", "x", "b", "y", "c"];
  assert.deepEqual(reorderVisible(all, ["a", "b", "c"], "c", "a"), [
    "c",
    "x",
    "a",
    "y",
    "b",
  ]);
  assert.deepEqual(reorderVisible(all, ["a", "b", "c"], "a", "b"), [
    "b",
    "x",
    "a",
    "y",
    "c",
  ]);
  assert.deepEqual(all, ["a", "x", "b", "y", "c"]);
  assert.deepEqual(reorderVisible(all, ["a", "b"], "unknown", "b"), all);
});
