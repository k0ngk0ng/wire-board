import test from 'node:test';
import assert from 'node:assert/strict';
import { assetURL, validateManifest, validateResource, selectModelKeys, selectSoundKeys, validateRulesIdentity } from '../web/asset-contract.ts';

const resource = (file = 'maps/de_dust2.json') => ({ file, bytes: 100, sha256: 'a'.repeat(64) });
const mapModes = [['de_dust2', 'bomb'], ['de_inferno', 'bomb'], ['de_aztec', 'bomb'], ['cs_assault', 'hostage'], ['fy_iceworld', 'elimination'], ['cs_bloodstrike', 'elimination']];
const manifest = () => ({ schema: 1, maps: Object.fromEntries(mapModes.map(([id, mode]) => [id, { name: id, mode, render: resource(), rules: resource(), preview: resource(), sky: [] }])), models: {}, sounds: {} });

test('original HUD atlas is validated and item confirmation sounds are loaded', () => {
  const value = { ...manifest(), hud: { schema: 1, atlas: resource('hud/atlas.png'), width: 128, height: 128, sprites: { 640: { c4: { x: 0, y: 0, width: 32, height: 32 } } } } };
  assert.doesNotThrow(() => validateManifest(value));
  value.hud.sprites[640].c4.x = 129;
  assert.throws(() => validateManifest(value));
  assert.deepEqual(selectSoundKeys({ 'items/9mmclip1.wav': {}, 'items/flashlight1.wav': {}, 'items/gunpickup2.wav': {}, 'weapons/ak47-1.wav': {}, 'hostage/h.wav': {}, 'radio/bombpl.wav': {}, 'unrelated.wav': {} }, 'bomb'), ['items/9mmclip1.wav', 'items/flashlight1.wav', 'items/gunpickup2.wav', 'weapons/ak47-1.wav', 'radio/bombpl.wav']);
});

test('night vision purchase and toggles load their existing original audio in every map mode', () => {
  for (const mode of ['bomb', 'hostage', 'elimination']) {
    const sounds = { 'items/equip_nvg.wav': {}, 'items/nvg_on.wav': {}, 'items/nvg_off.wav': {} };
    assert.deepEqual(selectSoundKeys(sounds, mode), Object.keys(sounds));
  }
});

test('rejects traversal, invalid byte counts and unknown media protocols', () => {
  for (const file of ['../outside', 'maps/../../outside', 'file:///tmp/test', 'javascript:alert(1)', 'maps\\outside']) assert.throws(() => validateResource(resource(file)));
  for (const bytes of [-1, 1.5, NaN]) assert.throws(() => validateResource({ ...resource(), bytes }));
  assert.throws(() => validateResource({ ...resource(), sha256: 'not-a-digest' }));
  assert.doesNotThrow(() => validateResource(resource()));
});

test('requires six classic maps with explicit modes and independently validates render/rules identities', () => {
  assert.doesNotThrow(() => validateManifest(manifest()));
  for (const [id] of mapModes) {
    const incomplete = manifest();
    delete incomplete.maps[id];
    assert.throws(() => validateManifest(incomplete));
  }
  for (const id of ['fy_iceworld', 'cs_bloodstrike']) {
    for (const mode of ['bomb', 'hostage']) {
      const wrongMode = manifest();
      wrongMode.maps[id].mode = mode;
      assert.throws(() => validateManifest(wrongMode));
    }
  }
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
  assert.equal(selectModelKeys(['urban'], 'bomb').includes('w-defuser'), true);
  assert.equal(selectModelKeys(['urban'], 'elimination').includes('w-defuser'), false);
  assert.equal(selectModelKeys(['urban'], 'elimination').includes('hostage'), false);
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
