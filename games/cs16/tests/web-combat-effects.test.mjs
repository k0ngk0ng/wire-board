import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const out = path.join(root, '.local/cs16/verification/combat-effects');
mkdirSync(out, { recursive: true });
const require = createRequire(new URL('../../../web/package.json', import.meta.url));
const { build } = require('esbuild');
const built = await build({ entryPoints: [path.join(root, 'games/cs16/web/combat-effects.ts')], bundle: true, platform: 'node', format: 'cjs', write: false, nodePaths: [path.join(root, 'web/node_modules')] });
const file = path.join(out, 'combat-effects.cjs');
writeFileSync(file, built.outputFiles[0].contents);
const { CombatEffects, hitboxBloodPoint } = require(file);
const T = require('three');
const definitions = Object.fromEntries([['blood', 9, 16, 16], ['bloodspray', 10, 64, 64], ['fexplo', 30, 80, 96], ['eexplo', 25, 80, 80]].map(([name, count, width, height]) => [name, { atlas: { file: name + '.png', bytes: 1, sha256: 'a'.repeat(64) }, width: width * count, height, frames: Array.from({ length: count }, (_, frame) => ({ x: frame * width, y: 0, width, height, origin: [-width / 2, height / 2] })), textureFormat: name.startsWith('blood') ? 3 : 1, orientation: 2, blend: name.startsWith('blood') ? 'normal' : 'additive', ...(name.startsWith('blood') ? { tint: [103, 0, 0], paletteIndex: 247 } : {}), sourceSHA256: 'b'.repeat(64) }]));
const textures = new Map(Object.keys(definitions).map(name => [name, new T.Texture()]));
const target = { visible: true, life: 3, armor: 0, helmet: false };
const hit = { id: 1, frame: 100, type: 'hit', seat: 0, target: 1, weapon: 'ak47', value: 25, hitgroup: 2, fx: 1, life: 3, x: 256, y: 512, z: 768 };

test('only a server-confirmed current visible player hit produces native blood sprites', () => {
  const effects = new CombatEffects(definitions, textures);
  effects.event({ ...hit, type: 'shot' }, 100, target, new T.Vector3(1, 3, -2));
  assert.equal(effects.stats.blood, 0);
  effects.event(hit, 100, target, new T.Vector3(1, 3, -2));
  assert.equal(effects.stats.blood, 1);
  assert.ok(effects.root.children.some(mesh => mesh.visible && mesh.userData.effect === 'bloodspray'));
  assert.ok(effects.root.children.some(mesh => mesh.visible && mesh.userData.effect === 'blood'));
  const spray = effects.root.children.find(mesh => mesh.visible && mesh.userData.effect === 'bloodspray');
  assert.equal(spray.material.color.getHexString(T.SRGBColorSpace), '670000');
  assert.equal(spray.material.alphaTest, 0.5);
  effects.event(hit, 110, target, new T.Vector3());
  assert.equal(effects.stats.blood, 1);
  for (const invalid of [{ ...target, visible: false }, { ...target, life: 4 }]) effects.event({ ...hit, id: 2 }, 120, invalid, new T.Vector3());
  effects.event({ ...hit, id: 3, target: -1 }, 120, target, new T.Vector3());
  effects.event({ ...hit, id: 4, weapon: 'hegrenade' }, 120, target, new T.Vector3());
  assert.equal(effects.stats.blood, 1);
  effects.dispose();
});

test('the authoritative contact enum preserves armor suppression without guessing private opponent armor', () => {
  const effects = new CombatEffects(definitions, textures);
  for (const group of [2, 3, 4, 5]) effects.event({ ...hit, id: group, hitgroup: group, fx: 0 }, 100, { visible: true, life: 3 }, new T.Vector3());
  effects.event({ ...hit, id: 8, hitgroup: 1, fx: 2 }, 100, { visible: true, life: 3 }, new T.Vector3());
  assert.equal(effects.stats.blood, 0);
  assert.equal(effects.stats.sparks, 1);
  const spark = effects.root.children.find(object => object.isPoints && object.visible);
  assert.equal(spark.material.color.getHexString(T.SRGBColorSpace), '4b4b4b');
  effects.event({ ...hit, id: 11, fx: undefined }, 100, target, new T.Vector3());
  assert.equal(effects.stats.blood, 0);
  effects.event({ ...hit, id: 9, hitgroup: 1 }, 100, { ...target, armor: 100 }, new T.Vector3());
  effects.event({ ...hit, id: 10, hitgroup: 6 }, 100, { ...target, armor: 100 }, new T.Vector3());
  assert.equal(effects.stats.blood, 2);
  effects.event({ ...hit, id: 12, hitgroup: 1, fx: 2, value: 0 }, 100, { visible: true, life: 3 }, new T.Vector3());
  effects.event({ ...hit, id: 13, fx: 1, value: undefined }, 100, { visible: true, life: 3 }, new T.Vector3());
  assert.equal(effects.stats.sparks, 2);
  assert.equal(effects.stats.blood, 3);
  effects.dispose();
});

test('blood message size drives a quarter-second spray and longer-lived static-frame drops', () => {
  const effects = new CombatEffects(definitions, textures);
  effects.event(hit, 100, target, new T.Vector3());
  const spray = effects.root.children.find(object => object.visible && object.userData.effect === 'bloodspray');
  assert.ok(Math.round(spray.scale.x * 1000000) >= Math.round(5 / 35 * 1000000));
  assert.ok(Math.round(spray.scale.x * 1000000) <= Math.round(5 / 25 * 1000000));
  const drops = effects.root.children.filter(object => object.visible && object.userData.effect === 'blood');
  assert.ok(drops.length >= 7 && drops.length <= 21);
  const first = drops[0].userData.frame;
  effects.update(350, new T.PerspectiveCamera());
  assert.equal(spray.visible, false);
  assert.ok(drops.every(object => object.visible));
  assert.equal(drops[0].userData.frame, first);
  effects.update(3101, new T.PerspectiveCamera());
  assert.ok(effects.root.children.every(object => !object.visible));
  effects.dispose();
});

test('blood anchors follow the original animated hitbox bone without assuming a trace endpoint', () => {
  const model = new T.Group(), bone = new T.Bone();
  bone.position.set(10, 20, 30);
  model.add(bone);
  model.userData.hitboxes = [{ bone: 0, group: 1, min: [-2, -4, -6], max: [2, 4, 6] }];
  const mesh = new T.SkinnedMesh(new T.BufferGeometry(), new T.MeshBasicMaterial());
  mesh.bind(new T.Skeleton([bone]));
  model.add(mesh);
  const carried = new T.Group();
  carried.userData.hitboxes = [];
  model.add(carried);
  model.updateMatrixWorld(true);
  assert.deepEqual(hitboxBloodPoint(model, 1).toArray(), [10, 20, 30]);
  bone.position.z = 45;
  model.updateMatrixWorld(true);
  assert.equal(hitboxBloodPoint(model, 1).z, 45);
  assert.equal(hitboxBloodPoint(model, 0).z, 45);
  assert.deepEqual(hitboxBloodPoint(model, 1, new T.Vector3(0, 20, 45)).toArray(), [8, 20, 45]);
  assert.equal(hitboxBloodPoint(model, 7), undefined);
  mesh.geometry.dispose(); mesh.material.dispose();
});

test('HE uses the two native TE_EXPLOSION scales, offsets, animation rate and depth-tested blend', () => {
  const effects = new CombatEffects(definitions, textures);
  const event = { id: 20, frame: 100, type: 'explode', seat: 0, weapon: 'hegrenade', x: 2560, y: 5120, z: 7680 };
  effects.event(event, 100);
  assert.equal(effects.stats.explosions, 1);
  const fire = effects.root.children.filter(mesh => mesh.visible);
  assert.deepEqual(fire.map(mesh => mesh.userData.effect).sort(), ['eexplo', 'fexplo']);
  const first = fire.find(mesh => mesh.userData.effect === 'fexplo');
  assert.equal(first.scale.x, 2.5);
  assert.deepEqual(first.position.toArray(), [10, 50, -20]);
  for (const mesh of fire) { assert.equal(mesh.material.blending, T.AdditiveBlending); assert.equal(mesh.material.depthTest, true); assert.equal(mesh.material.depthWrite, false); }
  const camera = new T.PerspectiveCamera();
  effects.update(134, camera);
  assert.ok(fire.every(mesh => mesh.userData.frame === 1));
  effects.event({ ...event, id: 21, weapon: 'flashbang' }, 200);
  effects.event({ ...event, id: 22, weapon: 'smokegrenade' }, 200);
  assert.equal(effects.stats.explosions, 1);
  effects.dispose();
});

test('effect allocation is bounded and reset, expiration and disposal release all world effects', () => {
  const effects = new CombatEffects(definitions, textures);
  const allocated = effects.root.children.length;
  assert.ok(allocated > 0 && allocated <= 64);
  for (let id = 1; id <= 1000; id++) effects.event({ ...hit, id }, id, target, new T.Vector3());
  assert.equal(effects.root.children.length, allocated);
  effects.update(10000, new T.PerspectiveCamera());
  assert.ok(effects.root.children.every(mesh => !mesh.visible));
  effects.reset();
  effects.event(hit, 10001, target, new T.Vector3());
  assert.equal(effects.stats.activeBlood > 0, true);
  effects.dispose();
  assert.equal(effects.root.children.length, 0);
});
