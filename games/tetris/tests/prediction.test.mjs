// Failure inventory: prediction mutates the authoritative board, predicts line
// clears or results, ignores wall kicks, keeps applying inputs after a hold or
// hard drop, or lets a wall/floor move leave the board.
import { test } from "node:test";
import assert from "node:assert/strict";
import { predict, ghostY } from "../web/prediction.ts";
import { dropY, fullBoard, shapeCells } from "../web/shapes.ts";

const empty = Array.from({ length: 20 }, () => "0".repeat(10));
const T = { id: 7, kind: 6, rot: 0, x: 3, y: 1 };

test("prediction applies movement and rotation without touching the board", () => {
  const board = [...empty];
  const piece = predict(board, T, [
    { kind: "left" },
    { kind: "right" },
    { kind: "rotateCW" },
  ]);
  assert.equal(piece.x, 3);
  assert.equal(piece.rot, 1);
  assert.deepEqual(board, empty);
});

test("prediction stops after hold and hard drop", () => {
  const held = predict(empty, T, [
    { kind: "hold" },
    { kind: "left" },
    { kind: "left" },
  ]);
  assert.equal(held.x, T.x);
  const dropped = predict(empty, T, [
    { kind: "hardDrop" },
    { kind: "right" },
  ]);
  assert.equal(dropped.y, dropY(fullBoard(empty), T.kind, T.rot, T.x, T.y));
  assert.equal(dropped.x, T.x);
});

test("prediction uses the same wall kicks and stays inside the board", () => {
  const piece = predict(empty, { ...T, x: 0 }, [{ kind: "rotateCCW" }]);
  for (const cell of shapeCells(piece.kind, piece.rot)) {
    const x = piece.x + cell.x;
    assert.ok(x >= 0 && x < 10, `cell x=${x}`);
  }
});

test("prediction respects the stack and never clears a line", () => {
  const board = [...empty];
  board[19] = "1111111111";
  const piece = predict(board, { id: 1, kind: 4, rot: 0, x: 4, y: 1 }, [
    { kind: "hardDrop" },
  ]);
  assert.equal(piece.y, 19);
  assert.equal(board[19], "1111111111");
  assert.equal(ghostY(board, piece), 19);
});

test("prediction ignores an unknown kind of private result data", () => {
  const piece = predict(empty, T, [{ kind: "softDrop" }]);
  assert.ok(!("score" in piece) && !("result" in piece));
});
