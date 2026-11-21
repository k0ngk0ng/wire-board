import test from 'node:test';
import assert from 'node:assert/strict';
import { WireSelfObserver } from './wire-self-observer.mjs';

test('late CDP attachment observes only actual self patch fields and marks missing baseline partial', () => {
  const observer = new WireSelfObserver();
  observer.bind({ seat: 2, epoch: 'epoch', life: 3 });
  const value = observer.observe({ schema: 2, epoch: 'epoch', frame: 10, collections: { players: { upsert: [{ id: 2, patch: { values: { appliedSeq: 9 }, objects: { input: { values: { crouch: true } } } } }] } } });
  assert.equal(value.partial, true);
  assert.equal(value.player.appliedSeq, 9);
  assert.equal(value.player.input.crouch, true);
  assert.equal(value.player.weapon, undefined);
  assert.equal(value.life, 3);
  assert.equal(value.appliedObserved, true);
  assert.equal(observer.observe({ schema: 2, epoch: 'epoch', frame: 11 }).appliedObserved, false);
});

test('wire full replaces state and later patches preserve arrays and removed properties precisely', () => {
  const observer = new WireSelfObserver();
  observer.bind({ seat: 0, epoch: 'one', life: 1 });
  const first = observer.observe({ schema: 1, epoch: 'one', frame: 1, players: [{ seat: 0, life: 1, weapon: 'usp', ammo: 12, recoilYaw: 10 }] });
  assert.equal(first.partial, false);
  const second = observer.observe({ schema: 2, epoch: 'one', frame: 2, collections: { players: { upsert: [{ id: 0, patch: { remove: ['recoilYaw'], values: { inventory: [{ id: 'knife' }] } } }] } } });
  assert.equal(second.player.weapon, 'usp');
  assert.equal('recoilYaw' in second.player, false);
  assert.deepEqual(second.player.inventory, [{ id: 'knife' }]);
  assert.equal(observer.observe({ schema: 2, epoch: 'two', frame: 0 }).player, undefined);
});
