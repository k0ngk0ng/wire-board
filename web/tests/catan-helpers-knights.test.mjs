import test from "node:test";
import assert from "node:assert/strict";
import { catanRuleContext } from "../src/catan-rule-context.ts";

test("knight helpers use the saved adaptation, independent of draft options", () => {
 const marker = "wire-board-helpers-knights-v1";
 const draft = {capacity:3,catanScenario:"cities-knights",catanCitiesKnights:{},catanOptions:{helpers:true}};
 assert.equal(catanRuleContext(draft).helpersKnights,marker);
 for (const fields of [{capacity:2},{catanFishing:true},{catanScenario:"land-ho"},{catanScenario:"fishing"}])
  assert.equal(catanRuleContext({...draft,...fields}).helpersKnights,"");
 const game={catan:{players:[{},{},{}],vertices:[],citiesKnights:{},options:{}}};
 assert.equal(catanRuleContext({...draft,game}).helpersKnights,"");
 game.catan.citiesKnights.helpers={rules:marker};game.catan.options.helpers=true;
 assert.equal(catanRuleContext({...draft,catanOptions:{},game}).helpersKnights,marker);
 assert.equal(catanRuleContext({...draft,catanOptions:{},game}).helpers,true);
});
