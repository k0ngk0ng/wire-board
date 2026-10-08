import test from "node:test";
import assert from "node:assert/strict";
import { catanRuleContext } from "../src/catan-rule-context.ts";
import { catanEventsSupported } from "../src/catan-event-options.ts";

test("two-player fishing knights use their own saved rules and expose event cards", () => {
  const room = {
    kind: "catan",
    capacity: 2,
    catanTwoRules: "catan-for-two-2025",
    catanTwoScenario: "cities-knights",
    catanFishing: true,
  };
  const marker = "wire-board-two-fishing-knights-v1";
  assert.equal(catanEventsSupported(room), true);
  assert.equal(catanRuleContext(room).twoFishingKnights, marker);
  assert.equal(catanRuleContext(room).fishingSeaKnights, "");
  assert.equal(catanRuleContext(room).target, 13);
  room.game = { catan: { players: [{}, {}], vertices: [] } };
  assert.equal(catanRuleContext(room).twoFishingKnights, "");
  assert.equal(catanRuleContext(room).fishing, false);
  assert.equal(catanRuleContext(room).citiesKnights, false);
  room.game.catan = {
    players: [{}, {}],
    vertices: [],
    two: { knights: "catan-for-two-knights-2025" },
    fishing: { twoKnights: marker },
    citiesKnights: {},
  };
  assert.equal(catanRuleContext(room).twoFishingKnights, marker);
  assert.equal(catanRuleContext(room).fishingSeaKnights, "");
  assert.equal(catanRuleContext(room).citiesKnights, true);
  assert.equal(catanRuleContext(room).target, 13);
  room.game.catan.harbors = {};
  assert.equal(catanRuleContext(room).target, 14);
});
