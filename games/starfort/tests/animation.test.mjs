import test from 'node:test';
import assert from 'node:assert/strict';
import { shotPose, workerPose, craterDepth } from '../web/animation.ts';

test('recoil is bounded, recovers, and reduced motion stays still', () => {
  for (const kind of ['arrow', 'cannon', 'frost', 'arcane']) {
    assert.equal(shotPose(kind, -1, false).recoil, 0);
    assert.ok(shotPose(kind, 35, false).recoil > 0);
    assert.equal(shotPose(kind, 300, false).recoil, 0);
    assert.equal(shotPose(kind, 35, true).recoil, 0);
  }
  assert.ok(shotPose('cannon', 35, false).recoil > shotPose('arrow', 35, false).recoil);
});
test('workers have stable distinct positions and animated tools without moving gameplay coordinates', () => {
  const a = workerPose(0, 4, 1000, false), b = workerPose(1, 4, 1000, false);
  assert.notDeepEqual([a.x,a.y], [b.x,b.y]);
  assert.notEqual(workerPose(0,4,1100,false).swing, a.swing);
  assert.deepEqual(workerPose(0,4,1100,true), workerPose(0,4,2000,true));
});
test('pit depth only follows authoritative remaining work', () => {
  assert.equal(craterDepth(6,6),1);
  assert.equal(craterDepth(3,6),0.5);
  assert.equal(craterDepth(0,6),0);
  assert.equal(craterDepth(-1,6),0);
  assert.equal(craterDepth(9,6),1);
});
test('recoil ends within projectile lifetime', () => {
  assert.equal(shotPose('cannon',180,false).recoil,0);
});
