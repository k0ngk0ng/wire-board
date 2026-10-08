import test from "node:test";
import assert from "node:assert/strict";
import {
  supportsTwoCatanVariants,
  twoCatanVariantsAvailable,
} from "../src/catan-two-variants.ts";
import { catanRuleContext } from "../src/catan-rule-context.ts";

test("two-player variant entrypoints follow the verified recipes", () => {
  const room = {
    kind: "catan",
    capacity: 2,
    catanTwoRules: "catan-for-two-2025",
  };
  for (const catanTwoScenario of ["", "fishing", "cities-knights"]) {
    assert.equal(supportsTwoCatanVariants(catanTwoScenario), true);
    assert.equal(
      twoCatanVariantsAvailable({ ...room, catanTwoScenario }),
      true,
    );
  }
  for (const scenario of [
    "rivers",
    "caravans",
    "transport",
    "land-ho",
    "unknown",
  ]) {
    assert.equal(supportsTwoCatanVariants(scenario), false);
    assert.equal(
      twoCatanVariantsAvailable({ ...room, catanTwoScenario: scenario }),
      false,
    );
  }
  for (const extra of [
    { capacity: 3 },
    { catanTwoRules: "unknown" },
    { catanScenario: "transport" },
    { catanTwoScenario: "cities-knights", catanOptions: { helpers: true } },
    { catanOptions: { fiveSix: true } },
  ]) {
    assert.equal(twoCatanVariantsAvailable({ ...room, ...extra }), false);
  }
});

test("two-player harbor goals and fishing fallback follow the running save", () => {
  const room = {
    kind: "catan",
    capacity: 2,
    catanTwoRules: "catan-for-two-2025",
    catanHarbors: { enabled: true },
    catanFriendlyRobber: { enabled: true },
  };
  for (const [catanTwoScenario, target] of [
    ["", 11],
    ["fishing", 11],
    ["cities-knights", 14],
  ]) {
    room.catanTwoScenario = catanTwoScenario;
    const info = catanRuleContext(room);
    assert.equal(info.target, target);
    assert.equal(info.friendlyFishingFallback, catanTwoScenario === "fishing");
    assert.equal(!!info.friendlyKnights, catanTwoScenario === "cities-knights");
    assert.equal(info.twoVariants, "wire-board-two-variants-v1");
  }
  room.game = { catan: { players: [{}, {}], two: {}, vertices: [] } };
  const saved = catanRuleContext(room);
  assert.equal(saved.target, 10);
  assert.equal(saved.twoVariants, "");
  assert.equal(saved.harbors, false);
  assert.equal(saved.friendlyRobber, false);
  assert.equal(saved.friendlyFishingFallback, false);
  room.game.catan.two.variants = "wire-board-two-variants-v1";
  room.game.catan.harbors = { rules: "catan-harbors-2025" };
  assert.equal(catanRuleContext(room).target, 11);
  assert.equal(
    catanRuleContext(room).twoVariants,
    "wire-board-two-variants-v1",
  );
});
