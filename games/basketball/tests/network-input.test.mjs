// Failure cases, specified before the transport: RTT collapses turbo edges;
// stale responses undo cancellation; account/seat changes reuse old callbacks;
// a lost request leaves held input or replays an old gesture after reconnect;
// a stalled HTTP response blocks newer authoritative acknowledgements.
import test from 'node:test';
import assert from 'node:assert/strict';
import { BasketballNetworkInput } from '../web/network-input.ts';

const binding = (extra={}) => ({roomID:'room',userID:'alice',seat:0,epoch:'epoch',serverAt:1000,sequence:0,canInput:true,...extra});
function setup() {
  let time=0;
  const requests=[],resets=[];
  const input=new BasketballNetworkInput({now:()=>time,send:action=>new Promise((resolve,reject)=>requests.push({action,resolve,reject})),onReset:reason=>resets.push(reason)});
  input.update(binding());
  return {input,requests,resets,at:t=>{time=t;}};
}
const settle=async()=>{for(let n=0;n<8;n++)await Promise.resolve();};

test('short and turbo edges survive a pending HTTP request',async()=>{
  const {input,requests,at}=setup();
  input.sample(64);at(20);input.sample(0);at(70);input.sample(64);at(80);input.sample(0);
  assert.equal(requests.length,1);
  requests[0].resolve();await settle();
  assert.equal(requests.length,2);
  assert.deepEqual(requests[1].action.events,[
    {sequence:2,buttons:0,offsetMS:0,release:false},
    {sequence:3,buttons:64,offsetMS:50,release:false},
    {sequence:4,buttons:0,offsetMS:60,release:false},
  ]);
  assert.equal(requests[1].action.seat,0);
  assert.equal(requests[1].action.basedOn,1000);
});

test('priority cancellation and new gesture survive reversed responses',async()=>{
  const {input,requests,at}=setup();
  input.sample(64);at(20);input.cancel();
  assert.equal(requests.length,2);
  assert.equal(requests[1].action.events[0].release,true);
  at(30);input.sample(1);
  requests[0].resolve();await settle();
  assert.equal(requests.length,3);
  assert.deepEqual(requests[2].action.events,[
    {sequence:2,buttons:0,offsetMS:0,release:true},
    {sequence:3,buttons:1,offsetMS:10,release:false},
  ]);
  requests[2].resolve();await settle();requests[1].reject(Error('late cancelled request'));await settle();
  assert.equal(input.pendingCount,0);
  at(300);input.sample(1);
  assert.equal(requests.at(-1).action.events[0].buttons,1);
});

test('authoritative acknowledgement releases a stalled HTTP flight',async()=>{
  const {input,requests,at}=setup();
  input.sample(64);at(20);input.sample(0);
  input.update(binding({serverAt:1100,sequence:1}));
  assert.equal(requests.length,2);
  assert.equal(requests[1].action.sequence,2);
  requests[0].reject(Error('response lost after commit'));await settle();
  assert.equal(input.pendingCount,1);
});

test('identity, room and seat changes discard old callbacks and pending controls',async()=>{
  for(const changed of [{userID:'bob'},{seat:1},{epoch:'new'},{roomID:'other'}]) {
    const {input,requests,at}=setup();
    input.sample(64);at(20);input.sample(0);
    input.update(binding({...changed,sequence:10}));
    input.sample(1);
    requests[0].reject(Error('old binding failed'));await settle();
    assert.equal(requests.length,2);
    assert.equal(requests[1].action.sequence,11);
    assert.equal(input.pendingCount,1);
  }
});

test('unacknowledged input expires and only cancellation survives recovery',async()=>{
  const {input,requests,resets,at}=setup();
  input.sample(64);at(501);input.sample(64);
  assert.ok(resets.includes('timeout'));
  assert.equal(requests.length,2);
  assert.deepEqual(requests[1].action.events.map(e=>[e.buttons,e.release]),[[0,true]]);
  requests[0].resolve();await settle();
  input.update(binding({serverAt:1600,sequence:2}));
  at(600);input.sample(0);
  assert.equal(input.pendingCount,0);
  assert.equal(requests.length,2);
});

test('held input heartbeats without creating idle zero heartbeats',async()=>{
  const {input,requests,at}=setup();
  input.sample(0);assert.equal(requests.length,0);
  input.sample(1);requests[0].resolve();await settle();
  at(249);input.sample(1);assert.equal(requests.length,1);
  at(250);input.sample(1);assert.equal(requests.length,2);
  requests[1].resolve();await settle();
  at(260);input.sample(0);requests[2].resolve();await settle();
  input.update(binding({serverAt:1800,sequence:3}));
  at(900);input.sample(0);assert.equal(requests.length,3);
});

test('stale snapshots cannot refresh network freshness or restore writable state',async()=>{
  const {input,requests,at}=setup();
  input.sample(1);requests[0].resolve();await settle();
  at(800);input.update(binding({serverAt:999}));input.sample(1);
  assert.equal(requests.length,1);
  assert.equal(input.connected,false);
  input.update(binding({serverAt:1800,sequence:1,canInput:false}));
  input.update(binding({serverAt:1700,sequence:1,canInput:true}));
  input.sample(1);assert.equal(requests.length,1);
});

test('send failure cancels once; disposal prevents queued or late callbacks',async()=>{
  const {input,requests,resets,at}=setup();
  input.sample(64);requests[0].reject(Error('offline'));await settle();
  assert.ok(resets.includes('network'));
  assert.equal(requests.length,2);
  assert.equal(requests[1].action.events[0].release,true);
  requests[1].reject(Error('offline'));await settle();assert.equal(requests.length,2);
  input.dispose();at(1000);input.sample(1);input.update(binding({serverAt:2000}));
  assert.equal(requests.length,2);
});

test('a long-unacknowledged cancellation cannot make the next gesture exceed the server timeline',async()=>{
  const {input,requests,at}=setup();
  input.sample(64);input.cancel();requests[0].resolve();await settle();
  at(600);input.update(binding({serverAt:1600,sequence:1}));input.sample(1);
  assert.equal(requests.length,3);
  assert.deepEqual(requests[2].action.events,[
    {sequence:2,buttons:0,offsetMS:0,release:true},
    {sequence:3,buttons:1,offsetMS:0,release:false},
  ]);
});

test('partial device cancellation preserves the other held source and queued edges',async()=>{
  const {input,requests,at}=setup();
  input.sample(1);requests[0].resolve();await settle();
  at(20);input.sample(65);at(30);input.cancel(64);
  assert.equal(requests.length,3);
  assert.deepEqual(requests[2].action.events,[
    {sequence:2,buttons:65,offsetMS:0,release:false},
    {sequence:3,buttons:0,offsetMS:10,release:true,cancelMask:64},
  ]);
  requests[2].resolve();await settle();
  at(40);input.sample(1);assert.equal(requests.length,3);
  at(300);input.sample(1);assert.equal(requests.at(-1).action.events[0].buttons,1);
});
