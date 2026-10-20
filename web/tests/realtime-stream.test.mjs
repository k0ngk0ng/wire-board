import test from 'node:test';
import assert from 'node:assert/strict';
import { mergeStreamState, mergeStreamGame, StreamInputs } from '../src/realtime-stream.ts';

// Failure inventory: stale game overwrites rematch; old bootstrap re-enters a
// left room; game updates drop chat/config; a closed transport leaves promises
// hanging; duplicate/out-of-order acknowledgements complete another action;
// congestion creates unbounded pending inputs; rejection is treated as success.
const state = (version=8, serverNow=100) => ({
 user:{id:'u'}, serverNow,receivedAt:10,rooms:[{id:'other'}],availableGames:['bubbleparty'],
 room:{id:'one',setupVersion:3,version,status:'playing',you:0,seats:[{id:'u'}],chat:[{text:'keep'}],game:{data:{x:1}}}
});
const packet = (version=9) => ({type:'game',roomId:'one',setupVersion:3,version,status:'playing',turnDeadline:0,serverNow:120,game:{data:{x:2}}});
test('game packets replace only current game fields and retain lobby/chat/seat identity',()=>{
 const initial=state(),next=mergeStreamGame(initial,packet(),20);
 assert.equal(next.room.game.data.x,2);assert.equal(next.room.version,9);
 assert.equal(next.rooms,initial.rooms);assert.equal(next.room.chat,initial.room.chat);assert.equal(next.room.seats,initial.room.seats);
 assert.equal(next.serverNow,120);assert.equal(next.receivedAt,20);
});
test('old room, old match, duplicate and reordered frames cannot overwrite a newer board',()=>{
 const current=state();
 for (const p of [packet(7),packet(8),{...packet(),roomId:'two'},{...packet(),setupVersion:2}]) assert.equal(mergeStreamGame(current,p,20),current);
 assert.equal(mergeStreamGame(undefined,packet(),20),undefined);
 assert.equal(mergeStreamGame({...current,room:undefined},packet(),20).room,undefined);
});
test('late full HTTP snapshot cannot roll game back or re-enter a room already left',()=>{
 const current=mergeStreamGame(state(),packet(12),20);
 const incoming=state(10,110);incoming.availableGames=['bubbleparty','tetris'];
 const next=mergeStreamState(current,incoming,30);
 assert.equal(next.room.version,12);assert.deepEqual(next.availableGames,incoming.availableGames);
 const left=mergeStreamState(next,{...incoming,serverNow:140,room:undefined},40);
 assert.equal(left.room,undefined);
 assert.equal(mergeStreamState(left,state(13,130),50),left);
});
test('a current full snapshot can change rooms and rematch metadata',()=>{
 const incoming=state(20,150);incoming.room={...incoming.room,id:'two',setupVersion:20};
 const next=mergeStreamState(state(),incoming,30);assert.equal(next.room.id,'two');assert.equal(next.room.setupVersion,20);
});
function transport(){const sent=[];const socket={readyState:1,bufferedAmount:0,send(raw){sent.push(JSON.parse(raw))}};return {socket,sent,inputs:new StreamInputs()}}
test('concurrent inputs resolve only their own durable ack; duplicate acks are harmless',async()=>{
 const {inputs,socket,sent}=transport();inputs.attach(socket);
 const a=inputs.send({roomId:'one',nonce:'a',version:1,action:{seq:1}});
 const b=inputs.send({roomId:'one',nonce:'b',version:1,action:{seq:2}});
 let aDone=false;a.then(()=>aDone=true);
 assert.equal(sent.length,2);
 inputs.ack({nonce:'b',status:200,version:3});await b;assert.equal(aDone,false);
 inputs.ack({nonce:'b',status:200,version:3});
 inputs.ack({nonce:'a',status:200,version:2});await a;assert.equal(aDone,true);
 inputs.close();
});
test('disconnect rejects in-flight requests and never replays old controls on attach',async()=>{
 const {inputs,socket,sent}=transport();inputs.attach(socket);
 const promise=inputs.send({roomId:'one',nonce:'a',version:1,action:{move:'right'}});
 const rejected=assert.rejects(promise,/连接/);inputs.close();await rejected;
 inputs.attach(socket);assert.equal(sent.length,1);inputs.close();
});
test('server rejection is explicit; congestion and pending limits remain bounded',async()=>{
 const {inputs,socket}=transport();inputs.attach(socket);
 const p=inputs.send({roomId:'one',nonce:'a',version:1,action:{seq:1}});
 const rejected=assert.rejects(p,/不能操作/);inputs.ack({nonce:'a',status:403,error:'不能操作'});await rejected;
 socket.bufferedAmount=1<<20;await assert.rejects(inputs.send({roomId:'one',nonce:'b',version:1,action:{}}),/拥塞/);
 socket.bufferedAmount=0;const pending=[];
 for(let i=0;i<32;i++)pending.push(assert.rejects(inputs.send({roomId:'one',nonce:'q'+i,version:1,action:{}}),/连接/));
 await assert.rejects(inputs.send({roomId:'one',nonce:'overflow',version:1,action:{}}),/拥塞/);
 inputs.close();await Promise.all(pending);
});
test('new server instance accepts lower checkpoint version; old-instance game packets cannot overwrite it',()=>{
 const old={...state(100,1000),serverInstance:'old'};
 const restored={...state(80,1100),serverInstance:'new'};
 const next=mergeStreamState(old,restored,30);
 assert.equal(next.room.version,80);
 assert.equal(next.serverInstance,'new');
 assert.equal(mergeStreamGame(next,{...packet(101),serverInstance:'old'},40),next);
 assert.equal(mergeStreamGame(next,{...packet(81),serverInstance:'new'},40).room.version,81);
});
test('volatile acknowledgement completes accepted input without claiming durability',async()=>{
 const {inputs,socket,sent}=transport();inputs.attach(socket);
 const p=inputs.send({roomId:'one',nonce:'volatile',version:1,serverInstance:'current',action:{seq:1}});
 assert.equal(sent[0].serverInstance,'current');
 inputs.ack({nonce:'volatile',status:200,version:2,durable:false});await p;inputs.close();
});
