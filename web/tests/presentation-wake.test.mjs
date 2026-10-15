// Failure cases before implementation: fractional timers fire before deadline
// and never wake again; overdue deadlines wait unnecessarily; cancelled or
// superseded timers still wake; cleanup misses a timer rearmed by early fire;
// duplicate callbacks unlock twice. Browser integration also measures real
// animation-to-unlock time without relying on the 15-second state poll.
import test from 'node:test';
import assert from 'node:assert/strict';
import {schedulePresentationWake} from '../src/game-api/presentation-wake.ts';
function timers(t){let now=100,next=0;const pending=new Map(),all=new Map();t.mock.method(performance,'now',()=>now);t.mock.method(globalThis,'setTimeout',(fn,delay)=>{const id=++next;pending.set(id,{fn,delay});all.set(id,fn);return id});t.mock.method(globalThis,'clearTimeout',id=>pending.delete(id));return {pending,setNow:v=>{now=v},fire(id){const cb=pending.get(id);assert.ok(cb);pending.delete(id);cb.fn()},stale:id=>all.get(id)()}}
test('fractional deadline survives early timer delivery and wakes only once',t=>{const clock=timers(t);let wakes=0;schedulePresentationWake(320.75,()=>wakes++);assert.equal(clock.pending.get(1).delay,221);clock.setNow(320);clock.fire(1);assert.equal(wakes,0);assert.equal(clock.pending.get(2).delay,1);clock.setNow(320.5);clock.fire(2);assert.equal(wakes,0);assert.equal(clock.pending.get(3).delay,1);clock.setNow(321);clock.fire(3);assert.equal(wakes,1);clock.stale(3);assert.equal(wakes,1);assert.equal(clock.pending.size,0)});
test('cancellation cancels the rearmed timer and rejects late callbacks',t=>{const clock=timers(t);let wakes=0;const stop=schedulePresentationWake(200.5,()=>wakes++);clock.setNow(200);clock.fire(1);assert.equal(clock.pending.size,1);stop();assert.equal(clock.pending.size,0);clock.setNow(201);clock.stale(2);assert.equal(wakes,0)});
test('overdue wake runs immediately and replacement cannot wake old work',t=>{const clock=timers(t);const seen=[];const stop=schedulePresentationWake(150,()=>seen.push('old'));stop();schedulePresentationWake(90,()=>seen.push('new'));assert.equal(clock.pending.get(2).delay,0);clock.fire(2);clock.setNow(160);clock.stale(1);assert.deepEqual(seen,['new'])});
