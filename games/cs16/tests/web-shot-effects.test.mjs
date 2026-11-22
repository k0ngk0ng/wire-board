import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const out = path.join(root, '.local/cs16/verification/playability/effects');
mkdirSync(out, { recursive: true });
const require = createRequire(new URL('../../../web/package.json', import.meta.url));
const { build } = require('esbuild');
const result = await build({ entryPoints: [path.join(root, 'games/cs16/web/shot-effects.ts')], bundle: true, platform: 'node', format: 'cjs', write: false, nodePaths: [path.join(root, 'web/node_modules')] });
const file = path.join(out, 'effects.cjs');
writeFileSync(file, result.outputFiles[0].contents);
const { mapTextureColorSpace, createMuzzleTexture, muzzleAttachments, ShotEffects } = require(file);
const T = require('three');

test('muzzle positions follow original animated attachment nodes and mirrored hands', () => {
  const root = new T.Group();
  const bone = new T.Group();
  const attachment = new T.Object3D();
  attachment.userData.attachment = true;
  attachment.position.set(2, 5, -30);
  root.add(bone);
  bone.add(attachment);
  bone.position.set(0, 3, 0);
  assert.deepEqual(muzzleAttachments(root), [attachment]);
  const point = new T.Vector3();
  root.updateMatrixWorld(true);
  attachment.getWorldPosition(point);
  assert.deepEqual(point.toArray(), [2, 8, -30]);
  root.scale.y = -1;
  root.updateMatrixWorld(true);
  attachment.getWorldPosition(point);
  assert.deepEqual(point.toArray(), [2, -8, -30]);
});

test('raw BSP lighting remains linear and color textures retain sRGB decoding', () => {
  assert.equal(mapTextureColorSpace('lightmap'), T.LinearSRGBColorSpace);
  assert.equal(mapTextureColorSpace('texture:wall'), T.SRGBColorSpace);
  assert.equal(mapTextureColorSpace('sky:0'), T.SRGBColorSpace);
});

test('muzzle flame has transparent borders and a warm irregular core', () => {
  const texture = createMuzzleTexture();
  const { data, width, height } = texture.image;
  let visible = 0;
  for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
    const alpha = data[(y * width + x) * 4 + 3];
    if (x === 0 || y === 0 || x === width - 1 || y === height - 1) assert.equal(alpha, 0);
    if (alpha > 0) visible++;
  }
  const center = ((height / 2 | 0) * width + (width / 2 | 0)) * 4;
  assert.ok(data[center] >= data[center + 1] && data[center + 1] > data[center + 2]);
  assert.ok(data[center + 3] > 200);
  assert.ok(visible > width * height / 8 && visible < width * height * 3 / 4);
  texture.dispose();
});

test('continuous fire reuses a bounded pool, shows wall effects and expires them', () => {
  const effects = new ShotEffects();
  const camera = new T.PerspectiveCamera(75, 1, 1, 2000);
  camera.updateMatrixWorld();
  const wall = new T.Mesh(new T.PlaneGeometry(500, 500), new T.MeshBasicMaterial());
  wall.position.z = -100;
  wall.updateMatrixWorld();
  const targets = new T.Group();
  targets.add(wall);
  targets.updateMatrixWorld(true);
  const allocated = effects.root.children.length;
  for (let shot = 0; shot < 100; shot++) effects.shoot(camera, targets, shot * 10);
  assert.equal(effects.root.children.length, allocated);
  assert.ok(effects.root.children.some(object => object.visible));
  assert.ok(effects.stats.tracers >= 100 && effects.stats.impacts >= 100);
  effects.update(10000);
  assert.ok(effects.root.children.every(object => !object.visible));
  effects.dispose();
  wall.geometry.dispose();
  wall.material.dispose();
});

test('the tracer starts at the projected weapon muzzle instead of a fixed screen location', () => {
  const effects = new ShotEffects();
  const camera = new T.PerspectiveCamera(75, 1, 1, 2000);
  camera.updateMatrixWorld();
  effects.shoot(camera, new T.Group(), 10, new T.Vector3(7, 8, -12));
  assert.deepEqual([...effects.root.children[0].geometry.getAttribute('position').array].slice(0, 3), [7, 8, -12]);
  effects.dispose();
});
