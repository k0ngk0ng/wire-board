import test from 'node:test';
import assert from 'node:assert/strict';
import { nativeLifeInterrupted } from './native-life.mjs';

const identity = { epoch: 'room-epoch', life: 1, seat: 1 };
function authority(extra = {}) { return { epoch: 'room-epoch', life: 1, partial: false, player: { seat: 1, alive: true }, ...extra }; }

test('only a complete own-seat death or life replacement interrupts a native group', () => {
  assert.equal(nativeLifeInterrupted(authority(), identity), false);
  assert.equal(nativeLifeInterrupted(authority({ player: { seat: 1, alive: false } }), identity), true);
  assert.equal(nativeLifeInterrupted(authority({ life: 2 }), identity), true);
});

test('unknown, partial, foreign and invalid identities never turn a release failure into a retry', () => {
  const dead = { player: { seat: 1, alive: false } };
  for (const value of [undefined, {}, authority({ ...dead, partial: true }), authority({ ...dead, epoch: 'another' }), authority({ player: { seat: 2, alive: false } }), authority({ ...dead, life: undefined }), authority({ ...dead, life: 1.5 })]) assert.equal(nativeLifeInterrupted(value, identity), false);
  for (const value of [undefined, {}, { ...identity, epoch: '' }, { ...identity, life: 0 }, { ...identity, seat: undefined }]) assert.equal(nativeLifeInterrupted(authority(dead), value), false);
});
