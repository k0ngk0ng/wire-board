import test from 'node:test';
import assert from 'node:assert/strict';
import { catanRiversSeaScenarios, catanRiversSeaAvailable } from '../src/catan-rivers-sea-options.ts';
test('river sea choices only admit implemented player counts',()=>{
 assert.equal(catanRiversSeaScenarios.length,6);
 for(const {id} of catanRiversSeaScenarios){
  assert.equal(catanRiversSeaAvailable(id,2),true);
  assert.equal(catanRiversSeaAvailable(id,4),true);
  assert.equal(catanRiversSeaAvailable(id,6),['rivers-shores','rivers-fog','rivers-desert','rivers-desert-belt','rivers-new-world','rivers-tribe'].includes(id));
  assert.equal(catanRiversSeaAvailable(id,7),false);
 }
 assert.equal(catanRiversSeaAvailable('shores',3),false);
});

// Waiting-room targets must match the game factory, even though the room uses
// a combined scenario ID rather than the ordinary Seafarers configuration.
import { catanRuleContext } from '../src/catan-rule-context.ts';
test('river sea waiting room describes actual victory and two-player rules',()=>{
 for(const [id,target] of [['rivers-shores',14],['rivers-fog',12],['rivers-desert',14],['rivers-desert-belt',14],['rivers-tribe',13],['rivers-new-world',12]]){
  const context=catanRuleContext({kind:'catan',capacity:2,catanScenario:id});
  assert.equal(context.target,target,id);assert.equal(context.rivers,true,id);assert.equal(context.two,true,id);
 }
});

test("extended river sea waiting rooms announce paired turns",()=>{assert.equal(catanRuleContext({kind:"catan",capacity:6,catanScenario:"rivers-fog"}).fiveSix,true);});

import { supportsPublicCatanKnightsCombination } from '../src/catan-knights-combination.ts';
test('river sea rooms offer the knight nesting and raise the target by two',()=>{
 for(const id of ['rivers-shores','rivers-fog','rivers-desert','rivers-desert-belt','rivers-tribe','rivers-new-world'])
  assert.equal(supportsPublicCatanKnightsCombination(id),true,id);
 for(const [id,target] of [['rivers-shores',16],['rivers-fog',14],['rivers-desert',16],['rivers-desert-belt',16],['rivers-tribe',15],['rivers-new-world',14]]){
  const room={kind:'catan',capacity:3,catanScenario:id,catanCitiesKnights:{layout:'variable'}};
  const context=catanRuleContext(room);
  assert.equal(context.citiesKnights,true,id);
  assert.equal(context.target,target,id);
 }
 // The printed targets stay unchanged without the knight option.
 assert.equal(catanRuleContext({kind:'catan',capacity:3,catanScenario:'rivers-shores'}).target,14);
 assert.equal(supportsPublicCatanKnightsCombination('fishing'),true);
});
