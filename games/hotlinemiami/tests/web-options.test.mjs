import test from 'node:test';
import assert from 'node:assert/strict';
import { resizeOptions } from '../web/options.ts';

test('room capacity resizes only seat arrays and preserves existing choices', () => {
  const original = { map: 'overdose', roundSeconds: 300, characters: ['jacket', 'biker', 'tony', 'richter'], weapons: ['pistol', 'shotgun', 'knife', 'bat'] };
  const before = structuredClone(original);
  const two = resizeOptions(original, 2);
  assert.deepEqual(two, { ...original, characters: original.characters.slice(0, 2), weapons: original.weapons.slice(0, 2) });
  const four = resizeOptions(two, 4);
  assert.equal(four.characters.length, 4);
  assert.deepEqual(four.characters.slice(0, 2), original.characters.slice(0, 2));
  assert.deepEqual(four.weapons, ['pistol', 'shotgun', 'pistol', 'pistol']);
  assert.equal(four.map, original.map);
  assert.equal(four.roundSeconds, original.roundSeconds);
  assert.deepEqual(original, before);
});
