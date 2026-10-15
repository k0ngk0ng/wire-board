// Before realtime snapshots: a continuous notification stream must not discard
// every slower response; only one state request may run; notifications coalesce,
// a caller after a command awaits a request started after that caller, and a
// failed request must not poison later refreshes or create an unhandled loop.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {refreshQueue} from '../src/refresh-queue.ts';
const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no});return {promise,resolve,reject}};
const tick=()=>new Promise(resolve=>setImmediate(resolve));
test('continuous notifications coalesce with no overlap and keep applying completed snapshots',async()=>{
 const work=[];const applied=[];let active=0,max=0;
 const refresh=refreshQueue(async()=>{const d=deferred();work.push(d);active++;max=Math.max(max,active);await d.promise;applied.push(work.length);active--});
 const first=refresh();const pending=Array.from({length:20},()=>refresh());
 assert.equal(work.length,1);work[0].resolve();await first;await tick();
 assert.equal(applied.length,1);assert.equal(work.length,2);assert.equal(max,1);
 work[1].resolve();await Promise.all(pending);assert.equal(applied.length,2);assert.equal(work.length,2);
});
test('a command refresh during an existing request waits for the next request',async()=>{
 const work=[];const refresh=refreshQueue(()=>{const d=deferred();work.push(d);return d.promise});
 const before=refresh();let afterDone=false;const after=refresh().then(()=>{afterDone=true});
 work[0].resolve();await before;await tick();assert.equal(afterDone,false);
 work[1].resolve();await after;assert.equal(afterDone,true);
});
test('failure rejects only its batch and queued refresh can recover',async()=>{
 const work=[];const refresh=refreshQueue(()=>{const d=deferred();work.push(d);return d.promise});
 const first=refresh();const rejected=assert.rejects(first,/network/);const second=refresh();
 work[0].reject(Error('network'));await rejected;await tick();assert.equal(work.length,2);work[1].resolve();await second;
 const third=refresh();assert.equal(work.length,3);work[2].resolve();await third;
});
test('synchronous failure also leaves the queue usable',async()=>{
 let n=0;const refresh=refreshQueue(()=>{if(++n===1)throw Error('sync');return Promise.resolve()});
 await assert.rejects(refresh(),/sync/);await refresh();assert.equal(n,2);
});
