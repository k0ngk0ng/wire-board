import test from 'node:test';
import assert from 'node:assert/strict';
import { defaults, randomCharacter, resizeOptions } from '../web/labels.ts';

test('all supported room sizes default every character to random', () => {
  for (const capacity of [2, 3, 4]) assert.deepEqual(defaults(capacity), Array(capacity).fill(randomCharacter));
  for (const capacity of [0, 1, 5]) assert.throws(() => defaults(capacity));
});


test('room capacity preserves chosen players and resets unavailable team modes', () => {
  const original = { arena: 14, teamBattle: true, oneVsThree: true, characters: [3, 4, 8, 10] };
  const before = structuredClone(original);
  const two = resizeOptions(original, 2);
  assert.deepEqual(two, { arena: 14, teamBattle: false, oneVsThree: false, characters: [3, 4] });
  assert.deepEqual(resizeOptions(two, 4), { arena: 14, teamBattle: false, oneVsThree: false, characters: [3, 4, randomCharacter, randomCharacter] });
  assert.deepEqual(resizeOptions(original, 4), original);
  assert.deepEqual(original, before);
});
