// Before UI implementation: captured armies disappear before arrival, swap
// reveals new faces early, route loses a bend, rapid events reorder, refresh
// replays hidden info/audio, reduced motion keeps controls blocked.
import test from 'node:test';import assert from 'node:assert/strict';
import {createPlayback,durationOf} from '../web/playback.ts';
const snapshot=(id,extra={})=>({room:{id:'r',kind:'junqi',version:id+1,status:extra.phase==='finished'?'finished':'playing',you:0,seats:[],game:{finished:extra.phase==='finished',eliminated:extra.phase==='finished'?[1]:[],data:{round:'a',phase:'playing',ply:id,pieces:[{id:'own',seat:0,kind:'engineer',pos:0},{id:'enemy',seat:1,kind:'unknown',pos:5}],players:[{alive:true},{alive:true}],last:{id,type:'move',seat:0,moves:[{unit:'own',from:0,to:5,path:[0,1,2,5],turned:true}],removed:['enemy']},...extra}}},clock:{serverNow:100,receivedAt:0,epoch:0}});
test('route, casualties, elimination and final result share one arrival',()=>{
 const p=createPlayback(()=>false),a=snapshot(0),b=snapshot(1,{phase:'finished',pieces:[{id:'own',pos:5,kind:'engineer',seat:0}],players:[{alive:true},{alive:false}]});const original=JSON.stringify([a,b]);p.update(a,0,true);const f=p.update(b,10,false);
 assert.equal(f.blocked,true);assert.equal(f.room.game.finished,false);assert.deepEqual(f.room.game.eliminated,[]);assert.equal(f.room.game.data.players[1].alive,true);assert.equal(f.room.game.data.pieces[1].kind,'unknown');assert.deepEqual(f.room.game.data.motion.event.moves[0].path,[0,1,2,5]);
 assert.equal(p.sample(10+durationOf(b.room.game.data.last)-1).blocked,true);assert.deepEqual(p.sample(10+durationOf(b.room.game.data.last)).room,b.room);assert.equal(JSON.stringify([a,b]),original);
});
test('two-piece swap remains private, both pieces use the same animation',()=>{
 const p=createPlayback(()=>false),a=snapshot(0,{phase:'deploy'}),b=snapshot(1,{phase:'deploy'});b.room.game.data.last.type='swap';b.room.game.data.last.moves.push({unit:'enemy',from:5,to:0,path:[5,0],turned:false});p.update(a,0,true);const f=p.update(b,1,false);assert.equal(f.room.game.data.motion.event.moves.length,2);assert.equal(f.room.game.data.pieces[1].kind,'unknown');assert.equal(f.room.game.data.phase,'deploy');
});
test('rapid events queue, reconnect/round/gaps reset and reduced motion settles',()=>{
 let reduced=false;const p=createPlayback(()=>reduced),a=snapshot(0),b=snapshot(1),c=snapshot(2);p.update(a,0,true);p.update(b,1,false);p.update(c,2,false);assert.equal(p.sample(2).room.game.data.motion.event.id,1);const at=1+durationOf(b.room.game.data.last);assert.equal(p.sample(at).room.game.data.motion.event.id,2);reduced=true;assert.deepEqual(p.sample(at+1).room,c.room);assert.equal(p.update(snapshot(7),at+2,true).blocked,false);reduced=false;assert.equal(p.update(snapshot(10),at+3,false).blocked,false);
});
