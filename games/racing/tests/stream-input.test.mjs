// Failure inventory: a released direction waits for the previous network ACK;
// pipelining loses an edge, exceeds its window, or old ACK releases a new epoch.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { LatestDrive } from '../web/input.ts';

test('stream window sends release before the earlier ACK and remains bounded',()=>{
 const q=new LatestDrive();q.sync({scope:'one',epoch:'one',seq:0,enabled:true});
 q.set({steer:1,throttle:1,brake:false,drift:false});const first=q.next(0, 2);assert.ok(first);
 q.set({steer:0,throttle:0,brake:false,drift:false});const release=q.next(20, 2);assert.ok(release,'release must not wait for earlier ACK');
 assert.equal(release.seq,first.seq+1);
 q.set({steer:1,throttle:1,brake:false,drift:false});assert.equal(q.next(200, 2),null,'bounded window');
 q.complete(first.token);const third=q.next(210, 2);assert.ok(third);
 q.complete(release.token);q.complete(third.token);
});

test('legacy HTTP window remains serial and stale ACK cannot open new epoch window',()=>{
 const q=new LatestDrive();q.sync({scope:'one',epoch:'one',seq:0,enabled:true});
 q.set({steer:1,throttle:1,brake:false,drift:false});const first=q.next(0, 1);assert.ok(first);
 q.set({steer:0,throttle:0,brake:false,drift:false});assert.equal(q.next(30, 1),null);
 q.sync({scope:'two',epoch:'two',seq:0,enabled:true});q.set({steer:1,throttle:1,brake:false,drift:false});const newer=q.next(200, 1);
 q.complete(first.token);q.set({steer:0,throttle:0,brake:false,drift:false});assert.equal(q.next(250, 1),null);
 q.complete(newer.token);assert.ok(q.next(260, 1));
});
