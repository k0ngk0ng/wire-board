import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gunzipSync } from 'node:zlib';
import { carriedBoneMap, playerPose } from '../web/model-pose.ts';
import { smokeRadius } from '../web/presentation.ts';
import { weapons } from '../web/content.ts';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const assets = [path.join(root, '.local/cs16/assets'), path.join(root, 'games/cs16/assets/public/cs16')].find(directory => fs.existsSync(path.join(directory, 'manifest.json')));
if (!assets) throw Error('Missing verified CS16 model resources');
const manifest = JSON.parse(fs.readFileSync(path.join(assets, 'manifest.json')));
const documentFor = key => {
  const resource = manifest.models[key];
  const raw = fs.readFileSync(path.join(assets, resource.file));
  const bytes = resource.encoding === 'gzip' ? gunzipSync(raw) : raw;
  return JSON.parse(bytes.subarray(20, 20 + bytes.readUInt32LE(12)).toString());
};

test('all carried weapon bones map into the player skeleton without a second hand hierarchy transform', () => {
  const player = documentFor('player-urban');
  const targets = player.skins[0].joints.map(index => player.nodes[index].name);
  for (const weapon of weapons) {
    const source = documentFor(`p-${weapon.id === 'mp5navy' ? 'mp5' : weapon.id}`);
    const names = source.skins[0].joints.map(index => source.nodes[index].name);
    const mapping = carriedBoneMap(names, targets);
    assert.equal(mapping.length, names.length);
    assert.ok(mapping.every((index, sourceIndex) => index < 0 ? !names[sourceIndex].startsWith('Bip01') : targets[index] === names[sourceIndex]));
    assert.ok(mapping.some(index => index >= 0));
  }
});

test('stationary character animation uses the original weapon-specific center aim blend', () => {
  const clips = documentFor('player-urban').animations.map(animation => animation.name);
  for (const weapon of [...weapons.map(item => item.id), 'knife', 'hegrenade', 'c4']) {
    assert.ok(clips.includes(playerPose(weapon, false)), weapon);
    assert.ok(clips.includes(playerPose(weapon, true)), weapon);
  }
});

test('smoke growth and expiry match the integer authoritative collision radius', () => {
  const smoke = { kind: 'smokegrenade', active: true, expiresAt: 2000 };
  assert.equal(smokeRadius(smoke, 200), 0);
  assert.equal(smokeRadius(smoke, 201), 460);
  assert.equal(smokeRadius(smoke, 250), 90 * 256);
  assert.equal(smokeRadius(smoke, 300), 180 * 256);
  assert.equal(smokeRadius(smoke, 2000), 0);
});
