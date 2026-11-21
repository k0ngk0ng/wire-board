import test from 'node:test';
import assert from 'node:assert/strict';
import { BrushCache } from '../web/brush-cache.ts';
import { InputChannel, emptyInput } from '../web/input.ts';

test('brush deltas retain omitted state, clear explicit arrays and reset across identity', () => {
  const cache = new BrushCache();
  const doors = [{ id: 4, progress: 300, open: false }];
  assert.deepEqual(cache.update('epoch', 'de_dust2', doors), doors);
  assert.deepEqual(cache.update('epoch', 'de_dust2', undefined), doors);
  assert.deepEqual(cache.update('epoch', 'de_dust2', []), []);
  cache.update('epoch', 'de_dust2', doors);
  assert.deepEqual(cache.update('new', 'de_dust2', undefined), []);
  cache.update('new', 'de_dust2', doors);
  assert.deepEqual(cache.update('new', 'cs_assault', undefined), []);
});

test('disposing an old room cancels its generation without sending release or reporting late failures', async () => {
  const sent = [];
  const errors = [];
  let reject;
  const channel = new InputChannel(action => { sent.push(action); return new Promise((resolve, fail) => { reject = fail; }); }, error => errors.push(error));
  channel.update({ identity: 'old', epoch: 'old', life: 1, receivedSeq: 0, enabled: true, window: 1, frame: 0 });
  channel.push({ ...emptyInput(), fire: true });
  channel.push({ ...emptyInput(), jump: true });
  channel.dispose();
  channel.release();
  reject(Error('old room already closed'));
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(sent.length, 1);
  assert.equal(channel.queued, 0);
  assert.deepEqual(errors, []);
});

test('input interruption sends cancellation but intentional fire release remains ordinary input', async () => {
  const sent = [];
  const channel = new InputChannel(action => { sent.push(action); return Promise.resolve(); });
  channel.update({ identity: 'room', epoch: 'epoch', life: 7, receivedSeq: 0, enabled: true, window: 8, frame: 50 });
  channel.push({ ...emptyInput(), fire: true });
  channel.push(emptyInput());
  channel.release(100, -200);
  assert.equal(sent[1].cancel, undefined);
  assert.equal(sent[2].cancel, true);
  assert.equal(sent[2].yaw, 100);
  assert.equal(sent[2].life, 7);
  assert.equal(sent[2].fire, false);
});

test('transport expiry drops stale queued edges until a newer authoritative frame arrives', async () => {
  const sent = [];
  let reject;
  const channel = new InputChannel(action => { sent.push(action); return sent.length === 1 ? new Promise((resolve, fail) => { reject = fail; }) : Promise.resolve(); });
  const binding = { identity: 'room', epoch: 'epoch', life: 1, receivedSeq: 0, enabled: true, window: 1, frame: 10 };
  channel.update(binding);
  channel.push(emptyInput());
  channel.push({ ...emptyInput(), fire: true });
  channel.push(emptyInput());
  reject(Object.assign(Error('传输中已过期，请重新同步'), { status: 409 }));
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(channel.recovering, true);
  assert.equal(channel.queued, 0);
  assert.equal(sent.length, 1);
  channel.update(binding);
  assert.equal(channel.push({ ...emptyInput(), fire: true }), undefined);
  channel.update({ ...binding, frame: 11 });
  assert.equal(channel.recovering, false);
  assert.ok(channel.push({ ...emptyInput(), jump: true }));
  assert.equal(sent.length, 2);
  assert.equal(sent.at(-1).fire, false);
});
