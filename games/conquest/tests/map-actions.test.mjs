// Failure inventory: AMOUNT_CONTROL_FAILURE_SCENARIOS.md, written before implementation.
import test from "node:test";
import assert from "node:assert/strict";
import {
  marchAmount,
  recruitCapacity,
  troopAmount,
} from "../web/map-actions.ts";

test("default dispatches follow remaining projected army; explicit choices remain exact", () => {
  let remaining = 1000;
  const sent = [];
  for (let i = 0; i < 3; i++) {
    const n = marchAmount(remaining, 5000, 3.2, false, 2, 1);
    sent.push(n);
    remaining -= n;
  }
  assert.deepEqual(sent, [500, 250, 125]);
  assert.equal(marchAmount(1000, 5000, 3.2, false, 2, 1, 123), 123);
  assert.equal(marchAmount(1000, 5000, 3.2, false, 2, 1, 1000), 1000);
  assert.equal(marchAmount(70, 5000, 3.2, false, 2, 1, 123), 70);
});
test("half rounds up, precise input clamps to live limits and invalid edits cannot submit", () => {
  for (const [max, expected] of [
    [0, 0],
    [1, 1],
    [501, 251],
    [5000, 2500],
  ])
    assert.equal(troopAmount(max), expected);
  assert.equal(troopAmount(500, 73), 73);
  assert.equal(troopAmount(500, 73.8), 73);
  assert.equal(troopAmount(500, 700), 500);
  for (const draft of [0, -1, NaN, Infinity])
    assert.equal(troopAmount(500, draft), 0);
  for (const max of [-10, NaN, Infinity]) assert.equal(troopAmount(max), 0);
});
test("insufficient sea fees reject the chosen amount; reducing it can enable transport", () => {
  assert.equal(marchAmount(1000, 200, 1, true, 2, 1), 0);
  assert.equal(marchAmount(1000, 999, 1, true, 2, 1), 0);
  assert.equal(marchAmount(1000, 1000, 1, true, 2, 1), 500);
  assert.equal(marchAmount(1000, 200, 1, true, 2, 1, 100), 100);
  assert.equal(marchAmount(1000, 200, 1, true, 2, 1, 101), 0);
  assert.equal(marchAmount(500, 5000, 0.2, false, 2, 1, 1), 0);
  assert.equal(marchAmount(500, -1, 1, true, 2, 1, 1), 0);
  assert.equal(marchAmount(NaN, 5000, 3, true, 2, 1), 0);
});
test("recruitment allows the full legal range and defaults to half of that range", () => {
  for (const [args, capacity, half] of [
    [[5000, 25000, 500, 3.2, 1, 1], 5000, 2500],
    [[175, 25000, 500, 3.2, 1, 1], 175, 88],
    [[5000, 250, 500, 3.2, 1, 1], 125, 63],
    [[5000, 25000, 999900, 3.2, 1, 1], 99, 50],
    [[-20, 25000, 500, 3.2, 1, 1], 0, 0],
    [[5000, 25000, 500, 0.2, 1, 1], 0, 0],
  ]) {
    assert.equal(recruitCapacity(...args), capacity);
    assert.equal(troopAmount(recruitCapacity(...args)), half);
    assert.equal(troopAmount(capacity, capacity), capacity);
  }
});

test("recruitment identifies each resource shortage and simultaneous shortages", async () => {
  const { recruitShortages } = await import("../web/map-actions.ts");
  assert.equal(typeof recruitShortages, "function");
  assert.deepEqual(recruitShortages(5000, 25000, 500, 3.2, 1, 1), []);
  assert.deepEqual(recruitShortages(0, 25000, 500, 3.2, 1, 1), ["gold"]);
  assert.deepEqual(recruitShortages(5000, 1, 500, 3.2, 1, 1), ["population"]);
  assert.deepEqual(recruitShortages(5000, 25000, 999999, 3.2, 1, 1), [
    "capacity",
  ]);
  assert.deepEqual(recruitShortages(5000, 25000, 500, 0.2, 1, 1), ["movement"]);
  assert.deepEqual(recruitShortages(0, 1, 999999, 0, 1, 1), [
    "movement",
    "gold",
    "population",
    "capacity",
  ]);
});

test("display counts complete marches without changing the underlying fractional budget", async () => {
  const { completeMarches } = await import("../web/map-actions.ts");
  assert.equal(typeof completeMarches,"function");
  for(const [points,count] of [[3.2,3],[2.7,2],[.2,0],[10.1,10],[3.3,3],[4.8,4]]) assert.equal(completeMarches(points,1),count);
  assert.equal(completeMarches(3.2,2),1);
  for(const points of [-1,NaN,Infinity])assert.equal(completeMarches(points,1),0);
  assert.equal(completeMarches(3.2,0),0);
});
