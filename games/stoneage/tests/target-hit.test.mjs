import test from "node:test";
import assert from "node:assert/strict";
import { pickSpriteTarget } from "../web/target-hit.ts";
const body = {
  id: 2,
  depth: 10,
  rect: { left: 0, top: 0, width: 100, height: 100 },
  width: 200,
  height: 200,
  alpha: () => 255,
};
test("a transparent overlapping pet canvas cannot steal a visible character click", () => {
  const pet = { ...body, id: 3, depth: 20, alpha: () => 0 };
  assert.equal(pickSpriteTarget(45, 45, [body, pet]), 2);
  assert.equal(
    pickSpriteTarget(45, 45, [body, { ...pet, alpha: () => 255 }]),
    3,
  );
  assert.equal(pickSpriteTarget(145, 45, [body, pet]), null);
});
test("CSS pixels map to the actual 2x canvas and hidden pixels remain untargetable", () => {
  let sampled;
  const scaled = {
    ...body,
    rect: { left: 10, top: 20, width: 50, height: 50 },
    width: 100,
    height: 100,
    alpha: (x, y) => {
      sampled = [x, y];
      return x === 50 && y === 50 ? 255 : 0;
    },
  };
  assert.equal(pickSpriteTarget(35, 45, [scaled]), 2);
  assert.deepEqual(sampled, [50, 50]);
  assert.equal(pickSpriteTarget(11, 21, [scaled]), null);
});
