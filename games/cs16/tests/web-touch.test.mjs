import test from 'node:test';
import assert from 'node:assert/strict';
import { TouchInput } from '../web/touch-input.ts';

test('two thumbs preserve movement while dragging fire beyond the original button', () => {
  const input = new TouchInput();
  input.press(1, 'move', 50, 100);
  input.move(1, 50, 65);
  input.press(2, 'fire', 220, 100);
  assert.equal(input.sample().actions.has('fire'), true);
  const look = input.move(2, 270, 110);
  assert.deepEqual(look, { x: 50, y: 10 });
  assert.ok(input.sample().moveY > 0);
  assert.equal(input.sample().actions.has('fire'), true);
  input.release(1);
  assert.equal(input.sample().moveY, 0);
  assert.equal(input.sample().actions.has('fire'), true);
  input.press(1, 'move', 50, 100);
  input.move(1, 70, 100);
  input.release(2);
  assert.ok(input.sample().moveX > 0);
  assert.equal(input.sample().actions.has('fire'), false);
});

test('touch crouch toggles, cancellation clears it, and empty look drag does not fire', () => {
  const input = new TouchInput();
  input.press(1, 'crouch', 0, 0);
  input.release(1);
  assert.equal(input.sample().actions.has('crouch'), true);
  input.press(2, 'look', 10, 10);
  assert.deepEqual(input.move(2, 20, 25), { x: 10, y: 15 });
  assert.equal(input.sample().actions.has('fire'), false);
  input.clear();
  assert.equal(input.sample().actions.has('crouch'), false);
  assert.equal(input.sample().moveX, 0);
});

test('cancelled fire identifies its source while retaining the other moving thumb', () => {
  const input = new TouchInput();
  input.press(1, 'move', 0, 0);
  input.move(1, 0, -20);
  input.press(2, 'fire', 20, 0);
  assert.equal(input.release(2, true), 'fire');
  assert.ok(input.sample().moveY > 0);
  assert.equal(input.sample().actions.has('fire'), false);
  assert.equal(input.release(2, true), undefined);
});
