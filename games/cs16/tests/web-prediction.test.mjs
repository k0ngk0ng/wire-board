import test from 'node:test';
import assert from 'node:assert/strict';
import { EventCursor, MotionPrediction } from '../web/prediction.ts';
import { emptyInput } from '../web/input.ts';

const player = (changes = {}) => ({ seat: 0, x: 0, y: 0, z: 0, vx: 0, vy: 0, vz: 0, yaw: 0, pitch: 0, alive: true, life: 1, appliedSeq: 0, receivedSeq: 0, crouch: false, ...changes });
const action = (seq, changes = {}) => ({ ...emptyInput(), seq, life: 1, viewFrame: 100, type: 'input', epoch: 'epoch', ...changes });
const motion = (state, input, frames) => ({ ...state, x: state.x + input.moveY * frames, yaw: input.yaw, pitch: input.pitch });

test('responds before authoritative acknowledgement and caps prediction at 150 ms', () => {
  const predictor = new MotionPrediction(motion);
  predictor.confirm(player(), 100, 1000, 'room');
  predictor.input(action(1, { moveY: 1000, yaw: 4000 }), 1001);
  const immediate = predictor.sample(1001);
  assert.equal(immediate.x, 1000);
  assert.equal(immediate.yaw, 4000);
  predictor.sample(100000);
  assert.equal(predictor.predictedFrame, 115);
});

test('replays only unacknowledged input and retains immediate mouse facing', () => {
  const predictor = new MotionPrediction(motion);
  predictor.confirm(player(), 100, 1000, 'room');
  predictor.input(action(1, { moveY: 1000, yaw: 5000 }), 1001);
  predictor.sample(1001);
  predictor.input(action(2, { moveY: 1000, yaw: 6000 }), 1002);
  predictor.confirm(player({ x: 1000, appliedSeq: 1, yaw: 5000 }), 101, 1010, 'room');
  assert.equal(predictor.pendingCount, 1);
  assert.equal(predictor.sample(1011).yaw, 6000);
  predictor.confirm(player({ x: 2000, appliedSeq: 2, yaw: 6000 }), 102, 1020, 'room');
  assert.equal(predictor.pendingCount, 0);
});

test('death, generation and suspend clear prediction without applying old commands', () => {
  const predictor = new MotionPrediction(motion);
  predictor.confirm(player(), 100, 1000, 'room');
  predictor.input(action(1, { moveY: 1000 }), 1001);
  predictor.confirm(player({ alive: false }), 101, 1010, 'room');
  assert.equal(predictor.sample(1100).x, 0);
  predictor.confirm(player({ life: 2, x: 500 }), 200, 2000, 'new-room');
  assert.equal(predictor.pendingCount, 0);
  predictor.input(action(2, { moveY: 1000 }), 2001);
  predictor.suspend();
  assert.equal(predictor.pendingCount, 0);
  assert.equal(predictor.sample(2100).x, 500);
});

test('event cursor ignores historical reconnect events and deduplicates live events', () => {
  const cursor = new EventCursor();
  assert.deepEqual(cursor.consume('epoch', 5, [{ id: 4 }, { id: 5 }]), []);
  assert.deepEqual(cursor.consume('epoch', 7, [{ id: 5 }, { id: 6 }, { id: 7 }]).map(event => event.id), [6, 7]);
  assert.deepEqual(cursor.consume('epoch', 7, [{ id: 6 }, { id: 7 }]), []);
  assert.deepEqual(cursor.consume('new', 2, [{ id: 1 }, { id: 2 }]), []);
});

test('same-display-frame jump press and release execute distinct prediction steps', () => {
  const inputs = [];
  const predictor = new MotionPrediction((state, input, frames) => { for (let index = 0; index < frames; index++) inputs.push(input.jump); return { ...state, z: state.z + (input.jump ? 10 : 0) }; });
  predictor.confirm(player(), 100, 1000, 'room');
  predictor.input(action(1, { jump: true }), 1001);
  predictor.input(action(2, { jump: false }), 1001);
  const result = predictor.sample(1001);
  assert.deepEqual(inputs, [true, false]);
  assert.equal(result.z, 10);
});
