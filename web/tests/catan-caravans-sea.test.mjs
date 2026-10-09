import test from 'node:test';
import assert from 'node:assert/strict';
import {catanRuleContext} from '../src/catan-rule-context.ts';
import {catanProductionNumbers} from '../src/catan-production.ts';
test('caravan desert waiting target and transferred token agree with rules',()=>{
 const r=catanRuleContext({kind:'catan',capacity:3,catanScenario:'caravans-desert'});
 assert.equal(r.target,16);assert.equal(r.caravans,true);
 assert.deepEqual(catanProductionNumbers({tiles:[{number:12}],caravans:{extraNumbers:[{tile:0,number:2}]}},0),[12,2]);
});

import {catanEventsSupported} from '../src/catan-event-options.ts';
test('public caravan sea room offers event cards without admitting fishing',()=>{
 assert.equal(catanEventsSupported({kind:'catan',capacity:3,catanScenario:'caravans-desert'}),true);
 assert.equal(catanEventsSupported({kind:'catan',capacity:3,catanScenario:'caravans-desert',catanFishing:true}),false);
});
test('caravan tribe target and events are explicit',()=>{
 const room={kind:'catan',capacity:4,catanScenario:'caravans-tribe'};
 assert.equal(catanRuleContext(room).target,15);assert.equal(catanRuleContext(room).caravans,true);assert.equal(catanEventsSupported(room),true);
});
test('two-player caravan sea waiting context uses neutral and scenario target',()=>{
 for(const [scenario,target] of [['caravans-desert',16],['caravans-tribe',15],['caravans-islands',15]]){
  const c=catanRuleContext({kind:'catan',capacity:2,catanScenario:scenario});
  assert.equal(c.two,true);assert.equal(c.target,target);
 }
});
test('extended merchant New Shores reports sixteen points and paired turns',()=>{
 const room={kind:'catan',capacity:6,catanScenario:'caravans-shores'};
 const context=catanRuleContext(room);assert.equal(context.target,16);assert.equal(context.caravans,true);assert.equal(context.fiveSix,true);assert.equal(catanEventsSupported(room),true);
});

test('merchant Four Islands waiting rules show fifteen points and events',()=>{
 for(const capacity of [3,4]) {
  const room={kind:'catan',capacity,catanScenario:'caravans-islands'};
  const c=catanRuleContext(room);
  assert.equal(c.target,15);assert.equal(c.caravans,true);assert.equal(c.two,false);
  assert.equal(catanEventsSupported(room),true);
 }
});

test('merchant New World shows fourteen points for all public counts',()=>{
 for(const capacity of [2,3,4,5,6]) {
 const room={kind:'catan',capacity,catanScenario:'caravans-new-world'};
 const c=catanRuleContext(room);assert.equal(c.target,14);assert.equal(c.caravans,true);assert.equal(c.two,capacity===2);assert.equal(catanEventsSupported(room),true);
 }
});

import {supportsCaravanSeaHelpers,supportsTwoCatanHelpers} from '../src/catan-two-helpers.ts';
test('merchant sea helpers admitted without enabling standalone merchant helpers',()=>{
 for(const scenario of ['caravans-shores','caravans-islands','caravans-desert','caravans-tribe','caravans-new-world']) {assert.equal(supportsCaravanSeaHelpers(scenario),true);assert.equal(supportsTwoCatanHelpers(scenario),true)}
 assert.equal(supportsCaravanSeaHelpers('caravans'),false);assert.equal(supportsTwoCatanHelpers('caravans'),false);
});
