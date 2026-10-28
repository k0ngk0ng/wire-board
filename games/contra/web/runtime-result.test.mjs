// Failure cases before result projection: the common settlement overlay opens
// before the last buffered hit/death; delaying it strips authority records from
// Board; stale data from another epoch blocks a new game; stored results or new
// observers wait for an animation they never saw.
import test from 'node:test';import assert from 'node:assert/strict';
import {createRuntimeRoomPresentation} from './runtime-result.ts';
function fixture(){let visible={epoch:'e',finalized:false};const controller=createRuntimeRoomPresentation(()=>visible),data={epoch:'e',snapshot:{finalized:true}};const room={id:'r',status:'finished',game:{finished:true,data,winners:[0,1]},ratingEligible:true};const snapshot={room,clock:{serverNow:100,receivedAt:90}};return{controller,snapshot,set:v=>visible=v};}
test('settlement waits for the painted terminal state without hiding server data or changing scoring',()=>{
 const f=fixture(),first=f.controller.update(f.snapshot,200,false);assert.equal(first.room.status,'playing');assert.equal(first.room.game.finished,false);assert.equal(first.room.game.data,f.snapshot.room.game.data);assert.equal(first.room.ratingEligible,true);assert.equal(f.snapshot.room.game.finished,true);assert.equal(first.blocked,true);assert.ok(first.wakeAt>200);
 f.set({epoch:'e',finalized:true});const complete=f.controller.sample(220);assert.equal(complete.room,f.snapshot.room);assert.equal(complete.blocked,false);assert.equal(complete.wakeAt,undefined);
});
test('an absent or different-epoch presentation does not block historical results',()=>{
 for(const visible of [undefined,{epoch:'old',finalized:false}]){const f=fixture();f.set(visible);assert.equal(f.controller.update(f.snapshot,200,true).room,f.snapshot.room)}
});
