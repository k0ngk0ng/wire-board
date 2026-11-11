// Before H integration: held horn must use the same sequenced drive stream;
// releasing it has priority while throttle is still held, and scope loss drops it.
import test from 'node:test';
import assert from 'node:assert/strict';
import {LatestDrive,neutral} from '../web/input.ts';
test('horn changes enter the bounded drive stream and release without a throttle gap',()=>{
 const q=new LatestDrive();q.sync({scope:'room',epoch:'epoch',seq:0,enabled:true});
 q.set({...neutral,throttle:1,horn:true});
 const held=q.next(0,8);assert.equal(held.horn,true);assert.equal(held.throttle,1);
 q.set({...neutral,throttle:1,horn:false});
 const released=q.next(20,8);assert(released,'horn release waits behind normal coalescing');assert.equal(!!released.horn,false);assert.equal(released.throttle,1);assert.equal(released.seq,held.seq+1);
 q.sync({scope:'new-room',epoch:'new-epoch',seq:0,enabled:true});assert.equal(!!q.value.horn,false);
});
