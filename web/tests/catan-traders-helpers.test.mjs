import test from "node:test";
import assert from "node:assert/strict";
import { supportsCatanTradersHelpers } from "../src/catan-traders-helpers.ts";
import { supportsTwoCatanHelpers } from "../src/catan-two-helpers.ts";
test("traders helpers expose every existing standalone, cross and sea family", () => {
  for (const s of [
    "rivers",
    "caravans",
    "barbarian-attack",
    "transport",
    "rivers-caravans",
    "rivers-attack",
    "rivers-transport",
    "caravans-attack",
    "caravans-transport",
    "attack-transport",
    "rivers-shores",
    "rivers-fog",
    "rivers-desert",
    "rivers-desert-belt",
    "rivers-tribe",
    "rivers-new-world",
    "caravans-shores",
    "caravans-islands",
    "caravans-desert",
    "caravans-tribe",
    "caravans-new-world",
    "attack-shores",
    "attack-desert",
    "attack-tribe",
    "attack-wonders",
    "attack-pirates",
    "transport-shores",
    "transport-desert",
  ]) {
    assert.equal(supportsCatanTradersHelpers(s), true, s);
    assert.equal(supportsTwoCatanHelpers(s), true, s);
  }
  for (const s of ["", "unknown", "cities-knights", "land-ho", "shores"]) {
    assert.equal(supportsCatanTradersHelpers(s), false, s);
  }
});
