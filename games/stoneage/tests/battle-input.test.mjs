// Before implementation: inspection steals attacks, repeat taps submit twice,
// invalid targets commit, cancel loses the player's choice, no-pet rounds
// stall, and a new round reuses stale commands.
import test from 'node:test';
import assert from 'node:assert/strict';
import { initialInput, inspectActor, chooseCommand, confirmTarget, cancelInput, readyPlan } from '../web/battle-input.ts';
test('idle clicks toggle details without changing combat decisions',()=>{
 const start=initialInput(3);const open=inspectActor(start,2);
 assert.equal(open.details,2);assert.equal(inspectActor(open,2).details,null);
 assert.equal(inspectActor(open,5).details,5);assert.equal(open.player,null);assert.equal(open.pet,null);
});
test('attack selection requires a legal target, then hands off to pet skills',()=>{
 const input=chooseCommand(initialInput(4),'player',{kind:'attack',id:0,target:-1});
 assert.equal(input.details,null);assert.equal(readyPlan(input,true,1),null);
 assert.equal(confirmTarget(input,0,false),input);
 const next=confirmTarget(input,2,true);
 assert.deepEqual(next.player,{kind:'attack',id:0,target:2});assert.equal(next.owner,'pet');assert.equal(next.popup,'skills');
 const pet=confirmTarget(chooseCommand(next,'pet',{kind:'skill',id:3,target:-1}),3,true);
 assert.deepEqual(readyPlan(pet,true,1),{player:{kind:'attack',id:0,target:2},pet:{kind:'skill',id:3,target:3}});
});
test('cancellation preserves the selected player action and dead pets never block',()=>{
 const player=confirmTarget(chooseCommand(initialInput(5),'player',{kind:'guard',id:0,target:0}),0,true);
 const pending=chooseCommand(player,'pet',{kind:'skill',id:10,target:-1});
 const canceled=cancelInput(pending);
 assert.equal(canceled.pending,null);assert.equal(canceled.popup,null);assert.deepEqual(canceled.player,player.player);
 assert.deepEqual(readyPlan(canceled,false,1),{player:player.player,pet:{kind:'wait',id:0,target:1}});
 assert.equal(readyPlan(initialInput(6),true,1),null);
});
