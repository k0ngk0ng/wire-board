import test from 'node:test';
import assert from 'node:assert/strict';
import { weapons, maps, characters, defaultOptions, normalizeOptions, inventorySlots } from '../web/content.ts';

test('first version declares all 24 unique original firearms and four classic maps', () => {
  assert.equal(weapons.length, 24);
  assert.equal(new Set(weapons.map(weapon => weapon.id)).size, 24);
  for (const id of ['ak47', 'm4a1', 'awp', 'mp5navy', 'm249', 'elite', 'glock18', 'usp']) assert.ok(weapons.find(weapon => weapon.id === id));
  assert.deepEqual(maps.map(map => map.id), ['de_dust2', 'de_inferno', 'de_aztec', 'cs_assault']);
  assert.equal(maps.find(map => map.id === 'cs_assault').mode, 'hostage');
});

test('phone inventory keeps every equipment slot reachable without a clipped seven-button row', () => {
  const inventory = ['ak47', 'deagle', 'knife', 'hegrenade', 'flashbang', 'smokegrenade', 'c4'].map(id => ({ id, ammo: 1, reserve: 0 }));
  const slots = inventorySlots(inventory, 'flashbang');
  assert.equal(slots.length, 5);
  const grenades = slots.find(slot => slot.label === '投掷物');
  assert.equal(grenades.id, 'flashbang');
  assert.equal(grenades.next, 'smokegrenade');
  assert.ok(slots.find(slot => slot.id === 'c4'));
});

test('ten-seat defaults and capacity resizing preserve existing valid choices', () => {
  const options = normalizeOptions(defaultOptions, 10);
  assert.equal(options.teams.length, 10);
  assert.equal(options.teams.filter(team => team === 't').length, 5);
  assert.equal(options.teams.filter(team => team === 'ct').length, 5);
  assert.ok(options.characters.every((id, seat) => characters.find(character => character.id === id)?.team === options.teams[seat]));
  const resized = normalizeOptions({ ...options, map: 'de_aztec' }, 6);
  assert.equal(resized.map, 'de_aztec');
  assert.equal(resized.teams.length, 6);
});

test('friendly fire is disabled by default and remains a room option when capacity changes', () => {
  assert.equal(defaultOptions.friendlyFire, false);
  assert.equal(normalizeOptions(defaultOptions, 10).friendlyFire, false);
  assert.equal(normalizeOptions({ ...defaultOptions, friendlyFire: true }, 10).friendlyFire, true);
  assert.equal(normalizeOptions({ ...defaultOptions, friendlyFire: true }, 4).friendlyFire, true);
  assert.equal(normalizeOptions({ ...defaultOptions, friendlyFire: undefined }, 4).friendlyFire, false);
});
