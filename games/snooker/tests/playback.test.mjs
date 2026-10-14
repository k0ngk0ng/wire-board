// Before local playback implementation: late packets skip the cue; settlement
// leaks scores/winner into common UI before visual impact; duplicate packets
// restart a shot; consecutive shots reorder; remount/reconnect/background replay
// history; queued display mutates authoritative snapshots or accepts input.
import test from 'node:test';
import assert from 'node:assert/strict';
import {createPlayback} from '../web/playback.ts';
const balls=[{id:0,pos:{x:.5,y:.5},down:false},{id:21,pos:{x:1,y:.5},down:false}];
function snapshot(id=0,resolved=false,version=id*2+1){
 const flight=id?{id,start:1000+id*100,end:1800+id*100,seat:0,resolved,strike:{angle:0,power:.3,top:0,side:0},context:{balls,scores:[0,0],target:'clear',clear:7,team:0,inHand:false,freeBall:false},motion:{duration:.8,frames:[{time:0,balls},{time:.8,balls:[balls[0],{...balls[1],down:true}]}],balls:[balls[0],{...balls[1],down:true}],events:[{kind:'pot',time:.7,ball:21}],first:[21],potted:[21],off:[]}}:undefined;
 const data={phase:id?(resolved?'finished':'rolling'):'aim',window:version,frame:1,turn:0,nextSeat:1,teams:[0,1],order:[0,1],scores:resolved?[7,0]:[0,0],framesWon:resolved?[1,0]:[0,0],results:resolved?[{frame:1,winner:0,scores:[7,0],reason:'black'}]:[],balls:resolved?flight.motion.balls:balls,flight,break:0,bestBreak:[0,0],target:'clear',clear:7,inHand:false,freeBall:false,remaining:resolved?0:7,controls:{canAct:!id,canShoot:!id},endExplanation:resolved?'黑球终局':''};
 return {room:{id:'table',kind:'snooker',status:resolved?'finished':'playing',version,you:0,seats:[{id:'a',name:'A'},{id:'b',name:'B'}],game:{data,finished:resolved,winners:resolved?[0]:[],log:resolved?['结束']:[],actors:resolved?[]:[0],attention:resolved?[]:[0]},result:resolved?{rated:true}:undefined},clock:{serverNow:resolved?2000:1500,receivedAt:0}};
}
test('new late shot plays from cue preparation; whole room waits for local finish',()=>{
 const p=createPlayback();p.update(snapshot(),0,true);
 const incoming=snapshot(1);const original=structuredClone(incoming);
 const first=p.update(incoming,100,false);
 assert.ok(first.clock.serverNow<incoming.room.game.data.flight.start);
 assert.equal(first.room.game.data.phase,'rolling');assert.equal(first.blocked,true);
 const end=first.wakeAt;assert.ok(end>900);
 const settled=p.update(snapshot(1,true,4),200,false);
 assert.deepEqual(settled.room.game.data.scores,[0,0]);assert.deepEqual(settled.room.game.data.framesWon,[0,0]);
 assert.equal(settled.room.game.finished,false);assert.equal(settled.room.result,undefined);
 assert.deepEqual(incoming,original,'authoritative packet mutated');
 const final=p.sample(end+1);assert.equal(final.room.game.finished,true);assert.deepEqual(final.room.game.data.scores,[7,0]);assert.equal(final.blocked,false);
});
test('duplicates and settlement do not reset local playback anchor',()=>{
 const p=createPlayback();p.update(snapshot(),0,true);
 const first=p.update(snapshot(1),100,false);
 const duplicate=p.update(snapshot(1,false,4),300,false);
 assert.deepEqual(duplicate.clock,first.clock);assert.equal(duplicate.wakeAt,first.wakeAt);
});
test('a short shot first observed settled still plays before publishing its result',()=>{
 const p=createPlayback();p.update(snapshot(),0,true);
 const late=p.update(snapshot(1,true),100,false);
 assert.equal(late.room.status,'playing');assert.equal(late.room.game.data.phase,'rolling');assert.deepEqual(late.room.game.data.scores,[0,0]);
 assert.deepEqual(late.room.game.data.balls,balls);assert.equal(late.room.game.data.endExplanation,'');
 assert.equal(p.sample(late.wakeAt+1).room.status,'finished');
});
test('initial mount and explicit resync never replay an already running stroke',()=>{
 const p=createPlayback();const source=snapshot(1);
 const initial=p.update(source,300,true);assert.equal(initial.blocked,false);assert.deepEqual(initial.clock,source.clock);
 p.update(snapshot(2),500,false);const current=snapshot(2,true,9);
 const sync=p.update(current,600,true);assert.equal(sync.blocked,false);assert.equal(sync.room.status,'finished');assert.equal(sync.wakeAt,undefined);
});
test('two received strokes play in order and keep gameplay disabled between them',()=>{
 const p=createPlayback();p.update(snapshot(),0,true);const a=p.update(snapshot(1),100,false);
 const b=snapshot(2,false,5);b.room.game.data.flight.context.scores=[7,0];
 p.update(b,200,false);
 const next=p.sample(a.wakeAt+1);assert.equal(next.room.game.data.flight.id,2);assert.deepEqual(next.room.game.data.scores,[7,0]);assert.equal(next.blocked,true);
 assert.ok(next.clock.serverNow<b.room.game.data.flight.start);
});
test('closing a room discards playback immediately',()=>{
 const p=createPlayback();p.update(snapshot(),0,true);p.update(snapshot(1),100,false);
 const closed=snapshot(1,false,4);closed.room.status='closed';
 const result=p.update(closed,150,false);assert.equal(result.blocked,false);assert.equal(result.room.status,'closed');
});
test('chat and room metadata remain live while gameplay waits for the movie',()=>{
 const p=createPlayback(),initial=snapshot();initial.room.name='Before';initial.room.chat=[];p.update(initial,0,true);
 const shot=snapshot(1);shot.room.name='Before';shot.room.chat=[];p.update(shot,100,false);
 const latest=snapshot(1,true,4);latest.room.name='After';latest.room.chat=[{text:'hello'}];latest.room.seats[0].autoPlay=true;
 const held=p.update(latest,200,false);
 assert.equal(held.room.name,'After');assert.deepEqual(held.room.chat,[{text:'hello'}]);assert.equal(held.room.seats[0].autoPlay,true);
 assert.deepEqual(held.room.game.data.scores,[0,0]);
});
