import test from "node:test";
import assert from "node:assert/strict";
import { catanFleetSteps, catanNewBattle } from "../src/catan-pirate-motion.ts";

const path = [41, 40, 31, 22, 13, 12, 11, 2, 3, 14, 23, 24, 33, 42];
const view = (rollId, pirate, dice = [2, 5], battle) => ({
  rollId,
  dice,
  seafarers: { pirate, pirateIslands: { fleetPath: path, battle } },
});

test("fleet follows printed arrows across the end of the loop using the smaller die", () => {
  assert.deepEqual(
    catanFleetSteps(view(7, 42), view(8, 40, [5, 2])),
    [42, 41, 40],
  );
  assert.deepEqual(
    catanFleetSteps(view(7, 40), view(8, 2, [6, 6])),
    [40, 31, 22, 13, 12, 11, 2],
  );
  assert.deepEqual(catanFleetSteps(view(7, 24), view(8, 33, [1, 4])), [24, 33]);
});
test("initial views, repeated snapshots, skipped rolls, rematches and removed fleets do not replay", () => {
  const before = view(7, 42);
  for (const after of [
    view(7, 42),
    view(7, 40),
    view(9, 40),
    view(0, 40),
    view(8, -1),
  ]) {
    assert.deepEqual(catanFleetSteps(before, after), []);
  }
  assert.deepEqual(catanFleetSteps(view(7, -1), view(8, 40)), []);
});
test("invalid destinations and dice cannot invent a movement path", () => {
  const before = view(7, 42);
  for (const dice of [[], [2], [2, 0], [2, 7], [2, 2.5], [2, NaN]]) {
    assert.deepEqual(catanFleetSteps(before, view(8, 40, dice)), []);
  }
  assert.deepEqual(catanFleetSteps(before, view(8, 24)), []);
  const after = view(8, 40);
  after.seafarers.pirateIslands.fleetPath = [...path].reverse();
  assert.deepEqual(catanFleetSteps(before, after), []);
});
test("only one newly observed battle in the same production turn can animate", () => {
  const before = view(7, 42, [2, 5], { id: 3 });
  const battle = {
    id: 4,
    player: 1,
    removed: [10, 11],
    warships: 2,
    die: 5,
    remaining: 3,
  };
  assert.equal(catanNewBattle(before, view(7, 42, [2, 5], battle)), battle);
  for (const after of [
    view(7, 42, [2, 5], { id: 3 }),
    view(7, 42, [2, 5], { id: 5 }),
    view(8, 42, [2, 5], battle),
    view(0, 42, [2, 5], { id: 1 }),
  ]) {
    assert.equal(catanNewBattle(before, after), undefined);
  }
  assert.equal(
    catanNewBattle(view(7, 42), view(7, 42, [2, 5], { id: 1 })).id,
    1,
  );
});
