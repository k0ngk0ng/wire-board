import test from "node:test";
import assert from "node:assert/strict";
import {
  catanRuleContext,
  catanVictoryTarget,
  catanSavedVictoryTarget,
} from "../src/catan-rule-context.ts";

test("transport sea module goals match draft, active and legacy saves", () => {
  for (const scene of ["transport-shores", "transport-desert"]) {
    for (const [knights, fish, base] of [
      [false, false, 17],
      [false, true, 16],
      [true, false, 19],
      [true, true, 19],
    ]) {
      const room = {
        kind: "catan",
        capacity: 2,
        catanScenario: scene,
        catanFishing: fish,
        catanCitiesKnights: knights ? { layout: "variable" } : undefined,
        catanHarbors: { enabled: true },
      };
      assert.equal(catanVictoryTarget(scene, knights, fish), base);
      assert.equal(catanRuleContext(room).target, base + 1);
      const game = {
        transport: { map: { sea: "catan-transport-seafarers-2025" } },
        seafarers: { scenario: "shores", victoryPoints: base },
        citiesKnights: knights ? {} : undefined,
        fishing: fish ? {} : undefined,
        harbors: {},
      };
      assert.equal(catanSavedVictoryTarget(game), base + 1);
      game.victoryTarget = base + 2;
      assert.equal(catanSavedVictoryTarget(game), base + 2);
    }
  }
});
