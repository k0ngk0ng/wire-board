// Failure inventory: a released direction waits for the previous network ACK;
// pipelining loses an edge, exceeds its window, or old ACK releases a new epoch.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { LatestInput } from '../web/input.ts';

test('stream window sends release before the earlier ACK and remains bounded',()=>{
 const q=new LatestInput();q.sync({scope:'one',epoch:'one',seq:0,enabled:true});
 q.set({move:'right',bomb:true});const first=q.next(0, true, 2);assert.ok(first);
 q.set({move:'none',bomb:false});const release=q.next(20, true, 2);assert.ok(release,'release must not wait for earlier ACK');
 assert.equal(release.seq,first.seq+1);
 q.set({move:'right',bomb:true});assert.equal(q.next(200, true, 2),null,'bounded window');
 q.complete(first.token);const third=q.next(210, true, 2);assert.ok(third);
 q.complete(release.token);q.complete(third.token);
});

test('legacy HTTP window remains serial and stale ACK cannot open new epoch window',()=>{
 const q=new LatestInput();q.sync({scope:'one',epoch:'one',seq:0,enabled:true});
 q.set({move:'right',bomb:true});const first=q.next(0, true, 1);assert.ok(first);
 q.set({move:'none',bomb:false});assert.equal(q.next(30, true, 1),null);
 q.sync({scope:'two',epoch:'two',seq:0,enabled:true});q.set({move:'right',bomb:true});const newer=q.next(200, true, 1);
 q.complete(first.token);q.set({move:'none',bomb:false});assert.equal(q.next(250, true, 1),null);
 q.complete(newer.token);assert.ok(q.next(260, true, 1));
});

test('pipelined bomb edge is never repeated by release or heartbeat',()=>{
 const q=new LatestInput();q.sync({scope:'one',epoch:'one',seq:0,enabled:true});
 q.set({move:'right',bomb:true});const shot=q.next(0,true,4);
 q.set({move:'none',bomb:false});const stop=q.next(20,true,4);
 assert.equal(shot.bomb,true);assert.equal(stop.bomb,false);
 q.complete(shot.token);q.complete(stop.token);assert.equal(q.next(500,false,4),null);
});
