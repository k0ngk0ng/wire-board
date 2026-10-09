import test from "node:test";
import assert from "node:assert/strict";
import {
  catanRuleContext,
  catanSavedVictoryTarget,
} from "../src/catan-rule-context.ts";
import { supportsTwoCatanVariants } from "../src/catan-two-variants.ts";
test("traders harbor targets add exactly one in drafts and legacy saves", () => {
  for (const [scene, base] of [
    ["transport", 13],
    ["barbarian-attack", 12],
    ["attack-transport", 14],
    ["caravans-transport", 15],
    ["attack-wonders", 12],
    ["attack-pirates", 12],
    ["attack-shores", 14],
    ["attack-tribe", 13],
    ["transport-shores", 17],
    ["rivers-desert", 14],
  ]) {
    const draft = {
      kind: "catan",
      capacity: 2,
      catanScenario: scene,
      catanHarbors: { enabled: true },
      catanFriendlyRobber: { enabled: true },
    };
    assert.equal(catanRuleContext(draft).target, base + 1, scene);
    assert.equal(
      catanRuleContext(draft).twoVariants,
      "wire-board-two-variants-v1",
      scene,
    );
    assert.equal(supportsTwoCatanVariants(scene), true, scene);
  }
  assert.equal(catanSavedVictoryTarget({ transport: {}, harbors: {} }), 14);
  assert.equal(
    catanSavedVictoryTarget({
      transport: { attack: true },
      attack: {},
      harbors: {},
    }),
    15,
  );
  assert.equal(
    catanSavedVictoryTarget({
      transport: { map: { sea: "catan-transport-seafarers-2025" } },
      seafarers: { victoryPoints: 17 },
      harbors: {},
    }),
    18,
  );
  assert.equal(
    catanSavedVictoryTarget({
      attack: {},
      seafarers: { scenario: "wonders", victoryPoints: 12 },
      harbors: {},
    }),
    13,
  );
  assert.equal(
    catanSavedVictoryTarget({ victoryTarget: 20, transport: {}, harbors: {} }),
    20,
  );
});
