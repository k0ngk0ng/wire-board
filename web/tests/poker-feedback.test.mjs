// Before implementation: delayed reconnect snapshots replay audio/animations;
// client wall-clock skew makes a live event stale; older snapshots lower the
// consumed cursor; previews without new events replay the preceding action.
import test from 'node:test';
import assert from 'node:assert/strict';
import {nextFeedbackEvents} from '../src/poker/events.ts';
const event=(id,at)=>({id,at,type:'play',seat:0});
test('feedback uses the server clock and consumes delayed reconnect events silently',()=>{
 const r=nextFeedbackEvents([event(10,1000),event(11,18000),event(12,19900)],9,{serverNow:20000,receivedAt:500},600);
 assert.deepEqual(r.events.map(e=>e.id),[11,12]);assert.equal(r.id,12);
 const stale=nextFeedbackEvents([event(13,12000)],12,{serverNow:20000,receivedAt:500},600);
 assert.equal(stale.events.length,0);assert.equal(stale.id,13);
});
test('snapshot reorder and private previews cannot reset the cursor or replay',()=>{
 assert.deepEqual(nextFeedbackEvents([event(4,19900)],8,{serverNow:20000,receivedAt:500},500),{id:8,events:[]});
 assert.deepEqual(nextFeedbackEvents([event(8,19900)],8,{serverNow:20000,receivedAt:500},500),{id:8,events:[]});
});
test('elapsed rendering time counts toward staleness and impossible future events are dropped',()=>{
 assert.equal(nextFeedbackEvents([event(1,19900)],0,{serverNow:20000,receivedAt:500},5000).events.length,0);
 assert.equal(nextFeedbackEvents([event(1,25000)],0,{serverNow:20000,receivedAt:500},500).events.length,0);
});
