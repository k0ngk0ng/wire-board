import test from 'node:test';
import assert from 'node:assert/strict';
import { classicBuyEntries, classicBuyChoice } from '../web/classic-menu.ts';

test('pinned CS 1.6 team menus preserve every original number', () => {
  const expected = {
    t: { 1: ['glock18', 'usp', 'p228', 'deagle', 'elite'], 2: ['m3', 'xm1014'], 3: ['mac10', 'mp5navy', 'ump45', 'p90'], 4: ['galil', 'ak47', 'scout', 'sg552', 'awp', 'g3sg1'], 5: ['m249'] },
    ct: { 1: ['glock18', 'usp', 'p228', 'deagle', 'fiveseven'], 2: ['m3', 'xm1014'], 3: ['tmp', 'mp5navy', 'ump45', 'p90'], 4: ['famas', 'scout', 'm4a1', 'aug', 'sg550', 'awp'], 5: ['m249'] },
  };
  for (const [team, groups] of Object.entries(expected)) for (const [group, ids] of Object.entries(groups)) {
    assert.deepEqual(classicBuyEntries(team, Number(group), 'bomb').map(entry => entry.item), ids);
    for (let index = 0; index < ids.length; index++) assert.deepEqual(classicBuyChoice(team, Number(group), index + 1, 'bomb'), { item: ids[index], close: true });
  }
  assert.deepEqual(classicBuyChoice('ct', 0, 3, 'bomb'), { category: 3 });
  assert.deepEqual(classicBuyChoice('ct', 3, 1, 'bomb'), { item: 'tmp', close: true });
  assert.deepEqual(classicBuyChoice('ct', 4, 3, 'bomb'), { item: 'm4a1', close: true });
});

test('ammunition and equipment retain numeric paths and unavailable choices do nothing', () => {
  for (const team of ['t', 'ct']) {
    assert.deepEqual(classicBuyChoice(team, 0, 6, 'bomb'), { item: 'primammo', close: true });
    assert.deepEqual(classicBuyChoice(team, 0, 7, 'bomb'), { item: 'secammo', close: true });
    assert.deepEqual(classicBuyChoice(team, 0, 8, 'bomb'), { category: 8 });
    assert.deepEqual(classicBuyChoice(team, 8, 0, 'bomb'), { close: true });
    assert.deepEqual(classicBuyChoice(team, 5, 9, 'bomb'), {});
    assert.deepEqual(classicBuyEntries(team, 8, 'elimination').map(entry => entry.item), ['vest', 'vesthelm', 'flashbang', 'hegrenade', 'smokegrenade']);
  }
  assert.equal(classicBuyEntries('ct', 8, 'bomb').find(entry => entry.key === 7)?.item, 'defuser');
  assert.equal(classicBuyEntries('t', 8, 'bomb').some(entry => entry.item === 'defuser'), false);
});
