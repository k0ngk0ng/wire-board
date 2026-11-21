import test from 'node:test';
import assert from 'node:assert/strict';
import { InputChannel, emptyInput } from '../web/input.ts';

const binding = (changes = {}) => ({ identity: 'room:0', epoch: 'epoch', life: 1, receivedSeq: 0, enabled: true, window: 1, frame: 120, ...changes });
const turn = async () => { await new Promise(resolve => setImmediate(resolve)); };

function fakeClock() {
  let at = 0;
  let serial = 0;
  const timers = new Map();
  return { now: () => at, set: (run, delay) => { const id = ++serial; timers.set(id, { run, at: at + delay }); return id; }, clear: id => timers.delete(id), advance: value => { at = value; for (const [id, timer] of [...timers]) if (timer.at <= at) { timers.delete(id); timer.run(); } }, get timers() { return timers.size; } };
}

test('1000 continuous updates retain immediate local preparation and send at most 100 Hz', async () => {
  const clock = fakeClock();
  const sent = [];
  let predicted = 0;
  const channel = new InputChannel(action => { sent.push({ action, at: clock.now() }); return Promise.resolve(); }, undefined, clock);
  channel.update(binding({ window: 8 }));
  for (let index = 0; index < 1000; index++) {
    clock.advance(index / 10);
    channel.push({ ...emptyInput(index, 0), moveY: index }, () => predicted++);
    await Promise.resolve();
  }
  clock.advance(110);
  await turn();
  assert.equal(predicted, 1000);
  assert.ok(sent.length <= 12);
  for (let index = 1; index < sent.length; index++) assert.ok(sent[index].at - sent[index - 1].at >= 10);
  assert.equal(sent.at(-1).action.yaw, 999);
});

test('fire edges and cancellation bypass continuous pacing while lifecycle clears its timer', async () => {
  const clock = fakeClock();
  const sent = [];
  const channel = new InputChannel(action => { sent.push(action); return Promise.resolve(); }, undefined, clock);
  channel.update(binding({ window: 8 }));
  channel.push(emptyInput());
  await Promise.resolve();
  clock.advance(1);
  channel.push(emptyInput(100, 0));
  channel.push({ ...emptyInput(200, 0), fire: true });
  channel.push(emptyInput(300, 0));
  channel.release(300, 0);
  assert.deepEqual(sent.map(action => [action.fire, !!action.cancel]), [[false, false], [true, false], [false, false], [false, true]]);
  channel.push(emptyInput(400, 0));
  assert.ok(clock.timers > 0);
  channel.update(binding({ enabled: false, window: 0 }));
  assert.equal(clock.timers, 0);
  clock.advance(1000);
  assert.equal(sent.length, 4);
});

test('preserves short fire and secondary edges while an earlier command is in flight', async () => {
  const sent = [];
  const releases = [];
  const channel = new InputChannel(action => { sent.push(action); return new Promise(resolve => releases.push(resolve)); });
  channel.update(binding());
  channel.push(emptyInput(2000, -300));
  channel.push({ ...emptyInput(2100, -300), fire: true });
  channel.push(emptyInput(2200, -300));
  channel.push({ ...emptyInput(2200, -300), secondary: true });
  channel.push(emptyInput(2200, -300));
  for (let index = 0; index < 5; index++) { releases[index]?.(); await turn(); }
  assert.deepEqual(sent.map(action => [action.fire, action.secondary]), [[false, false], [true, false], [false, false], [false, true], [false, false]]);
  assert.deepEqual(sent.map(action => action.seq), [1, 2, 3, 4, 5]);
  assert.ok(sent.every(action => action.viewFrame === 120));
});

test('coalesces only continuous input and leaves release facing unchanged', async () => {
  const sent = [];
  let release;
  const channel = new InputChannel(action => { sent.push(action); return sent.length === 1 ? new Promise(resolve => { release = resolve; }) : Promise.resolve(); });
  channel.update(binding());
  channel.push(emptyInput(9000, 200));
  for (let index = 0; index < 20; index++) channel.push({ ...emptyInput(9000 + index, 200), moveY: index * 20 });
  assert.equal(channel.queued, 1);
  channel.release(9019, 200);
  release();
  await turn();
  assert.equal(sent.at(-1).yaw, 9019);
  assert.equal(sent.at(-1).pitch, 200);
  assert.equal(sent.at(-1).moveY, 0);
});

test('new life and disabled transport discard queued actions without replay', async () => {
  const sent = [];
  let release;
  const channel = new InputChannel(action => { sent.push(action); return new Promise(resolve => { release = resolve; }); });
  channel.update(binding());
  channel.push({ ...emptyInput(), fire: true });
  channel.push({ ...emptyInput(), reload: true });
  channel.update(binding({ life: 2, receivedSeq: 50, window: 0, enabled: false }));
  release();
  await turn();
  assert.equal(sent.length, 1);
  assert.equal(channel.queued, 0);
  assert.equal(channel.push(emptyInput()), undefined);
  channel.update(binding({ life: 2, receivedSeq: 50 }));
  const action = channel.push(emptyInput());
  assert.equal(action.seq, 51);
  assert.equal(action.life, 2);
});

test('bounds discrete queue and refuses excess without reporting a predicted command', () => {
  const errors = [];
  const channel = new InputChannel(() => new Promise(() => {}), message => errors.push(message));
  channel.update(binding());
  channel.push(emptyInput());
  for (let index = 0; index < 64; index++) assert.ok(channel.push({ ...emptyInput(), select: index % 2 ? 'usp' : 'knife' }));
  assert.equal(channel.push({ ...emptyInput(), reload: true }), undefined);
  assert.equal(channel.queued, 64);
  assert.equal(errors.length, 1);
});
