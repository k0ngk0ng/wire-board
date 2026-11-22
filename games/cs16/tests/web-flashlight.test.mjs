import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const out = path.join(root, '.local/cs16/verification/classic-render');
mkdirSync(out, { recursive: true });
const require = createRequire(new URL('../../../web/package.json', import.meta.url));
const { build } = require('esbuild');
const result = await build({ entryPoints: [path.join(root, 'games/cs16/web/flashlight.ts')], bundle: true, platform: 'node', format: 'cjs', write: false, nodePaths: [path.join(root, 'web/node_modules')] });
const file = path.join(out, 'flashlight.cjs');
writeFileSync(file, result.outputFiles[0].contents);
const { MapFlashlight } = require(file);
const T = require('three');

test('flashlight updates share one local beam and preserve baked map lighting settings', () => {
  const torch = new MapFlashlight();
  const material = new T.MeshBasicMaterial({ color: 0x887766, lightMapIntensity: 1 });
  torch.apply(material);
  const shader = { vertexShader: '#include <common>\n#include <project_vertex>', fragmentShader: '#include <common>\n#include <opaque_fragment>', uniforms: {} };
  material.onBeforeCompile(shader, undefined);
  const position = new T.Vector3(1, 2, 3);
  torch.update(position, new T.Vector3(0, 0, -1), true);
  assert.deepEqual(shader.uniforms.csFlashOrigin.value.toArray(), [1, 2, 3]);
  assert.equal(shader.uniforms.csFlashPower.value, 1);
  assert.equal(material.lightMapIntensity, 1);
  assert.equal(material.color.getHex(), 0x887766);
  torch.update(position, new T.Vector3(0, 0, -1), false);
  assert.equal(shader.uniforms.csFlashPower.value, 0);
  material.dispose();
});

test('night vision uses a separate local light state and restores unchanged baked lighting on exit', () => {
  const light = new MapFlashlight();
  const material = new T.MeshBasicMaterial({ lightMapIntensity: 1 });
  light.apply(material);
  const shader = { vertexShader: '#include <common>\n#include <project_vertex>', fragmentShader: '#include <common>\n#include <opaque_fragment>', uniforms: {} };
  material.onBeforeCompile(shader, undefined);
  assert.equal(shader.uniforms.csNightVisionPower.value, 0);
  light.update(new T.Vector3(10, 20, 30), new T.Vector3(1, 0, 0), false, true);
  assert.equal(shader.uniforms.csNightVisionPower.value, 1);
  assert.equal(shader.uniforms.csFlashPower.value, 0);
  assert.equal(material.lightMapIntensity, 1);
  light.update(new T.Vector3(), new T.Vector3(0, 0, -1), true, false);
  assert.equal(shader.uniforms.csNightVisionPower.value, 0);
  assert.equal(shader.uniforms.csFlashPower.value, 1);
  material.dispose();
});
