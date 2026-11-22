import test from 'node:test';
import assert from 'node:assert/strict';
import { scopeFov, weaponAnimation, deathAnimation, deathView, corpseAge, corpseTravel, CorpseInterpolation, ViewImpulse } from '../web/view-effects.ts';

test('original four-to-three horizontal FOV maps to a stable vertical FOV across wider viewports', () => {
  for (const [value, vertical] of [[10, 7508333], [15, 11278142], [40, 30536943], [55, 42653862]]) assert.equal(Math.round(scopeFov(value) * 1000000), vertical);
  assert.equal(Math.round(scopeFov(0) * 1000000), 73739795);
  assert.equal(Math.round(scopeFov(undefined) * 1000000), 73739795);
});

test('knife motion uses original full swing and miss clips while gun variants match their own names', () => {
  const clips = ['idle', 'slash1', 'slash2', 'draw', 'stab', 'stab_miss', 'midslash1', 'midslash2'];
  assert.equal(weaponAnimation(clips, 'knife', 'slash', 0), 'midslash1');
  assert.equal(weaponAnimation(clips, 'knife', 'slash', 1), 'midslash2');
  assert.equal(weaponAnimation(clips, 'knife', 'stab', 0), 'stab_miss');
  assert.equal(weaponAnimation(clips, 'knife', 'stab', 0, false, 'hit'), 'stab');
  assert.equal(weaponAnimation(['idle1', 'shoot', 'shoot2', 'shoot3'], 'glock18', 'shot', 0), 'shoot');
  assert.equal(weaponAnimation(['idle', 'shoot_left1', 'shoot_right1'], 'elite', 'shot', 0), 'shoot_left1');
  assert.equal(weaponAnimation(['idle', 'shoot_left1', 'shoot_right1'], 'elite', 'shot', 1), 'shoot_right1');
  assert.equal(weaponAnimation(['shoot1', 'shoot1_unsil', 'shoot2_unsil'], 'm4a1', 'shot', 0, false), 'shoot1_unsil');
  assert.equal(weaponAnimation(['shoot1', 'shoot1_unsil'], 'm4a1', 'shot', 0, true), 'shoot1');
});

test('death selection follows model hit regions and crouching overrides standing death activities', () => {
  const clips = ['death1', 'death2', 'death3', 'head', 'gutshot', 'left', 'right', 'crouch_die'];
  for (const [group, name] of [[1, 'head'], [2, 'death1'], [3, 'gutshot'], [4, 'left'], [5, 'right'], [6, 'death2']]) assert.equal(deathAnimation(clips, group, false), name);
  assert.equal(deathAnimation(clips, 1, true), 'crouch_die');
  assert.equal(deathAnimation(['death1'], 1, false), 'death1');
});

test('view impulses are immediate, bounded and expire without changing authority aim', () => {
  const impulse = new ViewImpulse();
  impulse.shot(1000);
  const first = impulse.sample(1000);
  assert.ok(first.weaponBack > 0);
  for (let i = 0; i < 100; i++) impulse.shot(1000);
  assert.deepEqual(impulse.sample(1000), first);
  assert.equal(impulse.sample(2000).weaponBack, 0);
  impulse.land(349, 2000);
  assert.equal(impulse.sample(2000).roll, 0);
  impulse.land(350, 2000);
  assert.ok(impulse.sample(2000).roll > 0);
  impulse.reset();
  assert.deepEqual(impulse.sample(2000), { weaponBack: 0, weaponUp: 0, roll: 0 });
});

test('death view falls once and uses authority elapsed time without replaying an old death', () => {
  assert.equal(deathView(0), undefined);
  const start = deathView(1);
  const fallen = deathView(600);
  assert.ok(fallen.eye < start.eye && fallen.roll > start.roll);
  assert.equal(deathView(2000), undefined);
  assert.equal(deathView(10000), undefined);
  assert.deepEqual(deathView(10000, true), deathView(1999));
});

test('legacy corpses without a death timestamp start at their final pose instead of replaying death', () => {
  assert.equal(corpseAge(400, undefined), 3000);
  assert.equal(corpseAge(400, 0), 3000);
  assert.equal(corpseAge(400, 390), 100);
  assert.equal(corpseAge(400, 410), 0);
});

test('corpse travel uses fixed velocity units, gravity and a finite horizontal envelope', () => {
  assert.deepEqual(corpseTravel({ x: 25600, y: 0, z: 0 }, 0), { x: 0, y: 0, z: 0 });
  const falling = corpseTravel({ x: 25600, y: 0, z: 0 }, 500);
  assert.ok(falling.x > 0 && falling.x < 128 && falling.z < 0);
  const fast = corpseTravel({ x: 2560000, y: 2560000, z: 0 }, 2000);
  assert.ok(Math.round(Math.hypot(fast.x, fast.y) * 256) <= 1024 * 256);
  assert.ok(corpseTravel({ x: 300 * 256, y: 0, z: 0 }, 750).x > 64);
  assert.ok(corpseTravel({ x: 0, y: 0, z: 300 * 256 }, 375).z > 32);
  assert.deepEqual(corpseTravel({ x: 25600, y: 0, z: 0 }, 10000), corpseTravel({ x: 25600, y: 0, z: 0 }, 3000));
  assert.ok(corpseTravel({ x: 575 * 256, y: 0, z: 836 * 256 }, 3000).x > 800);
  assert.ok(corpseTravel({ x: 0, y: 0, z: 836 * 256 }, 1000).z > 400);
  assert.ok(corpseTravel({ x: 0, y: 0, z: 836 * 256 }, 3000).z <= 0);
});

test('corpse map traces follow the original tenth-second cadence and reuse interpolated positions', () => {
  const motion = new CorpseInterpolation();
  const traces = [];
  const evaluate = at => { traces.push(at); return { x: at, y: 0, z: 0 }; };
  assert.equal(motion.sample(0, evaluate).x, 0);
  assert.equal(motion.sample(50, evaluate).x, 50);
  assert.equal(motion.sample(99, evaluate).x, 99);
  assert.deepEqual(traces, [0, 100]);
  assert.equal(motion.sample(100, evaluate).x, 100);
  assert.deepEqual(traces, [0, 100, 200]);
  assert.equal(motion.sample(3000, evaluate).x, 3000);
  const count = traces.length;
  assert.equal(motion.sample(10000, evaluate).x, 3000);
  assert.equal(traces.length, count);
  motion.reset();
  assert.equal(motion.sample(0, evaluate).x, 0);
  assert.ok(traces.length > count);
});
