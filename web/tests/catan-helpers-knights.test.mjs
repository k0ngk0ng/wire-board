import test from "node:test";
import assert from "node:assert/strict";
import { catanRuleContext } from "../src/catan-rule-context.ts";

test("knight helpers use the saved adaptation, independent of draft options", () => {
  const marker = "wire-board-helpers-knights-v1";
  const draft = {
    capacity: 3,
    catanScenario: "cities-knights",
    catanCitiesKnights: {},
    catanOptions: { helpers: true },
  };
  assert.equal(catanRuleContext(draft).helpersKnights, marker);
  for (const fields of [{ capacity: 1 }, { catanScenario: "land-ho" }])
    assert.equal(catanRuleContext({ ...draft, ...fields }).helpersKnights, "");
  const game = {
    catan: {
      players: [{}, {}, {}],
      vertices: [],
      citiesKnights: {},
      options: {},
    },
  };
  assert.equal(catanRuleContext({ ...draft, game }).helpersKnights, "");
  game.catan.citiesKnights.helpers = { rules: marker };
  game.catan.options.helpers = true;
  assert.equal(
    catanRuleContext({ ...draft, catanOptions: {}, game }).helpersKnights,
    marker,
  );
  assert.equal(
    catanRuleContext({ ...draft, catanOptions: {}, game }).helpers,
    true,
  );
});

test("fishing and knight helper markers coexist and retain saved precedence", () => {
  const draft = {
    capacity: 6,
    catanScenario: "shores",
    catanFishing: true,
    catanCitiesKnights: {},
    catanOptions: { helpers: true, fiveSix: true },
  };
  const info = catanRuleContext(draft);
  assert.equal(info.helpersKnights, "wire-board-helpers-knights-v1");
  assert.equal(info.fishingHelpers, "wire-board-fishing-helpers-v1");
  const saved = {
    ...draft,
    game: {
      catan: {
        players: [{}, {}, {}],
        vertices: [],
        options: { helpers: true },
        citiesKnights: { helpers: { rules: "wire-board-helpers-knights-v1" } },
        fishing: { helpers: "wire-board-fishing-helpers-v1" },
      },
    },
  };
  assert.equal(catanRuleContext(saved).helpersKnights, info.helpersKnights);
  assert.equal(catanRuleContext(saved).fishingHelpers, info.fishingHelpers);
  delete saved.game.catan.fishing.helpers;
  assert.equal(catanRuleContext(saved).fishingHelpers, "");
});
