// Failure: rejected old input resets queue before a newer server snapshot;
// acknowledge does not advance nextSeq and the queue reuses accepted numbers.
import test from 'node:test';
import assert from 'node:assert/strict';
import { InputQueue } from '../web/input.ts';
test('a later authoritative sequence advances queued input after rejection recovery',()=>{
 const q=new InputQueue(2);q.reset(2);q.acknowledge(8);q.push('right');assert.equal(q.take().seq,9);
 q.acknowledge(4);q.push('left');assert.equal(q.take().seq,10);
});
