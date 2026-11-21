import test from 'node:test';
import assert from 'node:assert/strict';
import { assetURL, validateManifest, validateResource, selectModelKeys, validateRulesIdentity } from '../web/asset-contract.ts';

const resource = (file = 'maps/de_dust2.json') => ({ file, bytes: 100, sha256: 'a'.repeat(64) });
const manifest = () => ({ schema: 1, maps: Object.fromEntries(['de_dust2', 'de_inferno', 'de_aztec', 'cs_assault'].map(id => [id, { name: id, mode: id === 'cs_assault' ? 'hostage' : 'bomb', render: resource(), rules: resource(), preview: resource(), sky: [] }])), models: {}, sounds: {} });

test('rejects traversal, invalid byte counts and unknown media protocols', () => {
  for (const file of ['../outside', 'maps/../../outside', 'file:///tmp/test', 'javascript:alert(1)', 'maps\\outside']) assert.throws(() => validateResource(resource(file)));
  for (const bytes of [-1, 1.5, NaN]) assert.throws(() => validateResource({ ...resource(), bytes }));
  assert.throws(() => validateResource({ ...resource(), sha256: 'not-a-digest' }));
  assert.doesNotThrow(() => validateResource(resource()));
});

test('requires four original maps and independently validates render/rules identities', () => {
  assert.doesNotThrow(() => validateManifest(manifest()));
  const incomplete = manifest();
  delete incomplete.maps.cs_assault;
  assert.throws(() => validateManifest(incomplete));
  const invalid = manifest();
  invalid.maps.de_dust2.rules.sha256 = 'bad';
  assert.throws(() => validateManifest(invalid));
});

test('selects all firearm view models and participating player models without unrelated maps', () => {
  const keys = selectModelKeys(['urban', 'terror']);
  for (const key of ['player-urban', 'player-terror', 'v-ak47', 'v-m4a1', 'v-mp5', 'v-knife', 'v-c4', 'v-hegrenade']) assert.ok(keys.includes(key));
  assert.equal(keys.includes('player-sas'), false);
  assert.equal(new Set(keys).size, keys.length);
  assert.equal(selectModelKeys(['urban'], 'bomb').includes('hostage'), false);
  assert.equal(selectModelKeys(['urban'], 'hostage').includes('hostage'), true);
});

test('asset URL preserves CDN object URLs and prefixes game resources once', () => {
  assert.equal(assetURL('', 'maps/test.json'), '/cs16/maps/test.json');
  assert.equal(assetURL('https://cdn.example/assets', 'models/v-ak47.glb'), 'https://cdn.example/assets/cs16/models/v-ak47.glb');
  assert.equal(assetURL('', 'https://cdn.example/object'), 'https://cdn.example/object');
});

test('authoritative rules identity matches decompressed CDN rules rather than transport gzip bytes', () => {
  const rules = { ...resource('rules.json.gz'), encoding: 'gzip', decodedSHA256: 'b'.repeat(64) };
  assert.doesNotThrow(() => validateRulesIdentity(rules, 'b'.repeat(64)));
  assert.throws(() => validateRulesIdentity(rules, 'a'.repeat(64)));
  assert.throws(() => validateRulesIdentity(rules, undefined));
  assert.doesNotThrow(() => validateRulesIdentity(resource(), 'a'.repeat(64)));
});
