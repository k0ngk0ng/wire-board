import test from 'node:test';
import assert from 'node:assert/strict';
import { defaults, randomCharacter } from '../web/labels.ts';

test('all supported room sizes default every character to random', () => {
  for (const capacity of [2, 3, 4]) assert.deepEqual(defaults(capacity), Array(capacity).fill(randomCharacter));
  for (const capacity of [0, 1, 5]) assert.throws(() => defaults(capacity));
});
