import test from "node:test";
import assert from "node:assert/strict";
import { supportsTwoCatanHelpers } from "../src/catan-two-helpers.ts";
import { twoCatanVariantsAvailable } from "../src/catan-two-variants.ts";
import { catanRuleContext } from "../src/catan-rule-context.ts";

test("two-player helpers retain compatible variants and authoritative saved rules", () => {
  const draft = {
    kind: "catan",
    capacity: 2,
    catanTwoRules: "catan-for-two-2025",
    catanOptions: { helpers: true },
    catanHarbors: { enabled: true },
  };
  for (const catanTwoScenario of ["", "fishing"]) {
    const room = { ...draft, catanTwoScenario };
    assert.equal(supportsTwoCatanHelpers(catanTwoScenario), true);
    assert.equal(twoCatanVariantsAvailable(room), true);
    const info = catanRuleContext(room);
    assert.equal(info.twoHelpers, "wire-board-two-helpers-v1");
    assert.equal(!!info.fishingHelpers, catanTwoScenario === "fishing");
    assert.equal(info.target, 11);
  }
  for (const scenario of [
    "rivers",
    "caravans",
    "cities-knights",
    "transport",
    "shores",
  ])
    assert.equal(supportsTwoCatanHelpers(scenario), false);
  const game = {
    catan: { players: [{}, {}], vertices: [], two: {}, options: {} },
  };
  assert.equal(catanRuleContext({ ...draft, game }).twoHelpers, "");
  assert.equal(catanRuleContext({ ...draft, game }).helpers, false);
  game.catan.two.helpers = "wire-board-two-helpers-v1";
  game.catan.options.helpers = true;
  assert.equal(
    catanRuleContext({ ...draft, catanOptions: {}, game }).twoHelpers,
    "wire-board-two-helpers-v1",
  );
});
