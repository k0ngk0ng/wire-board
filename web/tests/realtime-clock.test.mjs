// Failure inventory before implementation: packet jitter rewinds game time;
// one delayed sample slows later frames; OS wall-clock changes affect time;
// new match/server epoch retains old offsets; loading freeze/release continues;
// stale receipts replace a newer clock; caller-owned snapshots get mutated.
import test from 'node:test';
import assert from 'node:assert/strict';
import {RealtimeClock} from '../src/game-api/realtime-clock.ts';
const at=(c,t)=>c.serverNow+(c.paused?0:Math.max(0,t-c.receivedAt));
test('late server samples cannot rewind an active realtime clock',()=>{
 const c=new RealtimeClock();let a=c.update('match1',10000,100,false);assert.equal(at(a,200),10100);
 a=c.update('match1',10050,200,false);assert.equal(at(a,200),10100);assert.equal(at(a,250),10150);
 a=c.update('match1',10110,300,false);assert.equal(at(a,300),10200);
 a=c.update('match1',10300,350,false);assert.equal(at(a,350),10300);
});
test('paused or replaced timelines are explicitly rebased',()=>{
 const c=new RealtimeClock();c.update('match1',10000,100,false);
 let a=c.update('loading',10050,500,true);assert.equal(at(a,1000),10050);
 a=c.update('released',10050,1500,false);assert.equal(at(a,1520),10070);
 a=c.update('new-server',100,1600,false);assert.equal(at(a,1650),150);
});
test('old receipt timestamps never overwrite the current anchor',()=>{
 const c=new RealtimeClock();const a=c.update('s',1000,100,false);const before=structuredClone(a);
 c.update('s',1050,150,false);const current=c.update('s',1040,140,false);
 assert.equal(at(current,200),1100);assert.deepEqual(a,before);
});
