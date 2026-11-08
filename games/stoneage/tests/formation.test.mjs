// Before correcting facing: native right/lower party faces upper-left (3),
// native left/upper opponents face lower-right (7); preserve the classic slots.
import test from "node:test";
import assert from "node:assert/strict";
import { position, facing } from "../web/formation.ts";
import { readFileSync } from "node:fs";
test("both formations face each other on the upper-left/lower-right diagonal", () => {
  assert.equal(facing(0), 3);
  assert.equal(facing(1), 7);
  const right = position(0, 0, 0),
    left = position(1, 0, 0);
  assert.ok(right.x > left.x && right.y > left.y);
  assert.equal(right.x, (517 / 640) * 100);
  assert.equal(right.y, (384 / 480) * 100);
  assert.equal(left.x, (101 / 640) * 100);
  assert.equal(left.y, (150 / 480) * 100);
});
test("all ten actor slots per side are distinct and inside the original viewport", () => {
  for (const team of [0, 1]) {
    const positions = new Set();
    for (let row = 0; row < 5; row++)
      for (const pet of [0, 1]) {
        const p = position(team, row, pet);
        assert.ok(p.x > 0 && p.x < 100 && p.y > 0 && p.y < 100);
        positions.add(JSON.stringify(p));
      }
    assert.equal(positions.size, 10);
  }
});
test("every published actor has the two native battle directions and complete actions", () => {
  const art = JSON.parse(
    readFileSync(new URL("../web/art.json", import.meta.url), "utf8"),
  );
  for (const [id, actions] of Object.entries(art.sprites))
    for (const direction of [3, 7])
      for (const action of [0, 1, 2, 3, 4, 10])
        assert.ok(
          actions.some(
            (a) =>
              a.direction === direction &&
              a.action === action &&
              a.frames.length,
          ),
          `${id}/${direction}/${action}`,
        );
});
