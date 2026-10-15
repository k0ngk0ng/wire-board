// Before implementation: captures disappear early, castling rook omitted,
// promotion revealed in flight, rapid updates reordered, reconnect replays,
// reduced motion leaves controls blocked, or authoritative state is mutated.
import test from 'node:test';
import assert from 'node:assert/strict';
import {createPlayback,moveDuration} from '../web/playback.ts';
const snap=(id,extra={})=>({room:{id:'r',version:id+1,status:extra.phase==='finished'?'finished':'playing',you:0,seats:[],game:{finished:extra.phase==='finished',data:{round:'a',ply:id,phase:'playing',board:Array(64).fill(''),last:{id,from:12,to:28,piece:'wP',capture:'',captureAt:-1,rookFrom:-1,rookTo:-1,promotion:'',check:false,at:1},...extra}}},clock:{epoch:0,serverNow:100,receivedAt:0}});
for(const [name,event] of Object.entries({capture:{from:36,to:43,piece:'wP',capture:'bP',captureAt:35},castle:{from:4,to:6,piece:'wK',rookFrom:7,rookTo:5},promotion:{from:48,to:56,piece:'wP',promotion:'q'}}))test(name+' preserves old board until motion completes',()=>{
 const p=createPlayback(()=>false),first=snap(0);first.room.game.data.board[event.from]=event.piece;
 if(event.capture)first.room.game.data.board[event.captureAt]=event.capture;
 if(event.rookFrom>=0)first.room.game.data.board[event.rookFrom]='wR';
 const next=snap(1,{phase:'finished'});Object.assign(next.room.game.data.last,event);const raw=JSON.stringify([first,next]);
 p.update(first,0,true);const frame=p.update(next,10,false);
 assert.equal(frame.blocked,true);assert.equal(frame.room.status,'playing');assert.equal(frame.room.game.finished,false);
 assert.deepEqual(frame.room.game.data.board,first.room.game.data.board);
 assert.equal(frame.room.game.data.motion.event.from,event.from);
 assert.equal(frame.room.game.data.motion.event.captureAt,next.room.game.data.last.captureAt);
 assert.equal(p.sample(10+moveDuration-1).blocked,true);
 assert.deepEqual(p.sample(10+moveDuration).room,next.room);
 assert.equal(JSON.stringify([first,next]),raw);
});
test('rapid snapshots play once in order and use previous authoritative board',()=>{
 const p=createPlayback(()=>false),a=snap(0),b=snap(1),c=snap(2);
 b.room.game.data.board[28]='wP';c.room.game.data.last.from=52;c.room.game.data.last.to=36;
 p.update(a,0,true);p.update(b,10,false);p.update(c,20,false);p.update(c,30,false);
 assert.equal(p.sample(30).room.game.data.motion.event.id,1);
 const second=p.sample(10+moveDuration);assert.equal(second.room.game.data.motion.event.id,2);assert.equal(second.room.game.data.board[28],'wP');
 assert.equal(p.sample(10+2*moveDuration).blocked,false);
});
test('refresh, reconnect, changed round, gap, reduced motion never replay old events',()=>{
 for(const mode of ['reset','round','gap','reduced']){
  let reduced=false;const p=createPlayback(()=>reduced);p.update(snap(4),0,true);
  const next=snap(mode==='gap'?8:5);if(mode==='round')next.room.game.data.round='b';if(mode==='reduced')reduced=true;
  const frame=p.update(next,10,mode==='reset');assert.equal(frame.blocked,false);assert.equal(frame.room.game.data.motion,undefined);
 }
});
