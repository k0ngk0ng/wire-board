import test from "node:test";
import assert from "node:assert/strict";
import { catanRuleContext } from "../src/catan-rule-context.ts";
import { catanEventsSupported } from "../src/catan-event-options.ts";

test("two sea knights preserve map targets, fishing rules and saved-state precedence", () => {
  for (const [scenario, target] of [
    ["shores", 16],
    ["islands", 15],
    ["fog", 14],
    ["desert", 16],
    ["tribe", 15],
    ["cloth", 16],
    ["wonders", 12],
    ["new_world", 14],
  ]) {
    const room = {
      kind: "catan",
      capacity: 2,
      catanTwoRules: "catan-for-two-2025",
      catanTwoScenario: scenario,
      catanSeafarers: { scenario, layout: "fixed" },
      catanCitiesKnights: {},
      catanFishing: true,
    };
    assert.equal(catanEventsSupported(room), true);
    let info = catanRuleContext(room);
    assert.equal(info.target, target, scenario);
    assert.equal(
      info.twoSeafarersKnights,
      "wire-board-two-seafarers-knights-v1",
    );
    assert.equal(info.twoFishingKnights, "wire-board-two-fishing-knights-v1");
    room.catanHarbors = { enabled: true };
    assert.equal(catanRuleContext(room).target, target + 1);
    room.game = { catan: { players: [{}, {}], vertices: [], two: {} } };
    info = catanRuleContext(room);
    assert.equal(info.twoSeafarersKnights, "");
    assert.equal(info.twoFishingKnights, "");
    assert.equal(info.citiesKnights, false);
  }
});
