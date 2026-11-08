// Isolated failure cases are listed in open_roster_FAILURE_SCENARIOS.md.
// These lightweight helpers guard selection/load planning, not rule outcomes.
import test from "node:test";
import assert from "node:assert/strict";
import { hasOpenRoster, rackKinds, selectionUnits } from "../web/cards.ts";
import { neededUnits } from "../web/entry-assets.ts";

const view = () => ({
  rulesVersion: 4,
  tick: 0,
  players: [
    {
      side: "plants",
      deck: ["sunflower", "peashooter"],
      imitaterKind: "squash",
    },
  ],
  plants: [],
  zombies: [],
  events: [],
  catalog: {
    plants: ["sunflower", "peashooter", "starfruit", "imitater", "squash"].map(
      (kind) => ({ kind }),
    ),
    zombies: [{ kind: "zomboss", unlockAt: 2400 }],
  },
});
test("an outside quickbar selection is visible without changing favorites or board height", () => {
  const v = view(),
    before = structuredClone(v.players[0]);
  assert.deepEqual(rackKinds(v, 0, "starfruit"), [
    "starfruit",
    "sunflower",
    "peashooter",
  ]);
  assert.deepEqual(v.players[0], before);
  v.players[0].deck = Array.from({ length: 10 }, (_, i) => "seed" + i);
  assert.equal(rackKinds(v, 0, "starfruit").length, 10);
  assert.equal(rackKinds(v, 0, "starfruit")[0], "starfruit");
});
test("legacy and unknown selections retain their saved quickbar", () => {
  const v = view();
  assert.ok(hasOpenRoster(v));
  v.rulesVersion = 3;
  assert.ok(!hasOpenRoster(v));
  assert.deepEqual(rackKinds(v, 0, "starfruit"), v.players[0].deck);
  v.rulesVersion = 4;
  assert.deepEqual(rackKinds(v, 0, "unknown"), v.players[0].deck);
  assert.deepEqual(rackKinds(v, -1, "starfruit"), []);
});
test("selected non-quickbar assets and imitater target load without the whole roster", () => {
  const v = view();
  assert.deepEqual(selectionUnits(v, 0, "imitater"), ["imitater", "squash"]);
  assert.ok(neededUnits(v, "starfruit", 0).includes("starfruit"));
  assert.ok(!neededUnits(v, "starfruit", 0).includes("zomboss"));
  const kinds = neededUnits(v, "imitater", 0);
  assert.ok(kinds.includes("imitater") && kinds.includes("squash"));
  v.players.push({ side: "zombies", deck: [], imitaterKind: "peashooter" });
  assert.deepEqual(selectionUnits(v, 1, "zomboss"), []);
  assert.deepEqual(selectionUnits(v, 0, "__ability"), []);
});
