import { test } from "node:test";
import assert from "node:assert/strict";
import { catanColorIndex, catanSeatColor, catanPieceColors } from "../src/catan-player-colors.ts";

test("fixed setup keeps original piece colors for randomly assigned seats and neutral villages", () => {
  const game = { baseSetup: { colors: [2, 1, 4, 5, 3], neutralColor: 0 } };
  assert.deepEqual([0, 1, 2, 3, 4, -1].map(seat => catanPieceColors[catanColorIndex(game, seat)]),
    ["white", "red", "purple", "green", "orange", "blue"]);
  assert.equal(catanSeatColor(game, -1), "#3078be");
});

test("legacy and pirate seat colors remain stable", () => {
  assert.equal(catanColorIndex({}, 4), 4);
  const game = { seafarers: { pirateIslands: { colors: [0, 1, 3, 2, 5, 4] } } };
  assert.deepEqual([0, 1, 2, 3, 4, 5].map(seat => catanColorIndex(game, seat)), [0, 1, 3, 2, 5, 4]);
});
