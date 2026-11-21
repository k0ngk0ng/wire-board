import test from 'node:test';
import assert from 'node:assert/strict';
import { motionColliders } from '../web/motion-colliders.ts';

function player(extra = {}) {
  return { seat: 0, alive: true, x: 0, y: 256, z: -512, crouch: false, team: 'ct', character: 'gign', weapon: 'm4a1', health: 100, armor: 100, inventory: [{ id: 'm4a1', ammo: 30, reserve: 90 }], ...extra };
}

test('motion colliders retain exactly the six fields read by player collision', () => {
  assert.deepEqual(motionColliders([player(), player({ seat: 1, alive: false, crouch: true })]), [
    { seat: 0, alive: true, x: 0, y: 256, z: -512, crouch: false },
    { seat: 1, alive: false, x: 0, y: 256, z: -512, crouch: true },
  ]);
});

test('hidden and invalid positions retain the existing collision filtering semantics', () => {
  const values = [player(), player({ visible: false }), player({ x: NaN }), player({ y: Infinity }), player({ z: -Infinity })];
  assert.deepEqual(motionColliders(values), [{ seat: 0, alive: true, x: 0, y: 256, z: -512, crouch: false }]);
});

test('collision snapshots do not mutate or retain the original player object', () => {
  const value = Object.freeze(player());
  const result = motionColliders([value]);
  result[0].x = 999;
  assert.equal(value.x, 0);
  assert.equal(value.inventory[0].ammo, 30);
});

test('large unrelated player state cannot increase the movement collision payload', () => {
  const small = player();
  const large = player({ inventory: Array.from({ length: 100 }, () => ({ id: 'm4a1', ammo: 30, reserve: 90 })), pending: Array.from({ length: 100 }, () => ({ seq: 1, moveX: 1000, fire: true })), privateHistory: 'x'.repeat(10000) });
  assert.equal(JSON.stringify(motionColliders([small])), JSON.stringify(motionColliders([large])));
  assert.ok(JSON.stringify(large).length > JSON.stringify(motionColliders([large])).length * 10);
});
