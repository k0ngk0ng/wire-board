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
