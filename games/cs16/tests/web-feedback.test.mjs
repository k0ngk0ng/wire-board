import test from 'node:test';
import assert from 'node:assert/strict';
import { localFeedback, eventFeedback, announcementSound } from '../web/feedback.ts';
import { emptyInput } from '../web/input.ts';

test('knife primary and secondary animate with zero ammunition and never produce gunfire', () => {
  assert.equal(localFeedback('knife', { ...emptyInput(), fire: true }, emptyInput(), 0), 'slash');
  assert.equal(localFeedback('knife', { ...emptyInput(), secondary: true }, emptyInput(), 0), 'stab');
  assert.equal(localFeedback('knife', { ...emptyInput(), secondary: true }, { ...emptyInput(), secondary: true }, 0), undefined);
});

test('grenades pull pin and throw on release while C4 uses a plant animation', () => {
  for (const weapon of ['hegrenade', 'flashbang', 'smokegrenade']) {
    assert.equal(localFeedback(weapon, { ...emptyInput(), fire: true }, emptyInput(), 1), 'pin');
    assert.equal(localFeedback(weapon, emptyInput(), { ...emptyInput(), fire: true }, 1), 'throw');
  }
  assert.equal(localFeedback('c4', { ...emptyInput(), fire: true }, emptyInput(), 1), 'plant');
  assert.equal(localFeedback('ak47', { ...emptyInput(), fire: true }, emptyInput(), 0), undefined);
  assert.equal(localFeedback('ak47', { ...emptyInput(), fire: true }, emptyInput(), 30), 'shot');
});

test('server mode and utility event names select corresponding original animation classes', () => {
  assert.equal(eventFeedback({ type: 'mode', weapon: 'm4a1' }), 'mode');
  assert.equal(eventFeedback({ type: 'shot', weapon: 'knife', value: 1 }), 'stab');
  assert.equal(eventFeedback({ type: 'knife_air', weapon: 'knife', value: 0 }), undefined);
  assert.equal(eventFeedback({ type: 'grenade_pin', weapon: 'hegrenade' }), 'pin');
  assert.equal(eventFeedback({ type: 'throw', weapon: 'hegrenade' }), 'throw');
  assert.equal(eventFeedback({ type: 'plant_start', weapon: 'c4' }), 'plant');
});

test('reload begins once per press and fire release does not repeat a mode animation', () => {
  assert.equal(localFeedback('ak47', { ...emptyInput(), reload: true }, emptyInput(), 10), 'reload');
  assert.equal(localFeedback('ak47', { ...emptyInput(), reload: true }, { ...emptyInput(), reload: true }, 10), undefined);
  assert.equal(localFeedback('usp', { ...emptyInput(), secondary: true }, emptyInput(), 12), 'mode');
  assert.equal(localFeedback('usp', emptyInput(), { ...emptyInput(), secondary: true }, 12), undefined);
});

test('location-free global objectives use radio announcements without positional sound assumptions', () => {
  assert.equal(announcementSound({ type: 'bomb_planted', global: true }), 'radio/bombpl.wav');
  assert.equal(announcementSound({ type: 'bomb_defused', global: true }), 'radio/bombdef.wav');
  assert.equal(announcementSound({ type: 'round_end', global: true, value: 1 }), 'radio/terwin.wav');
  assert.equal(announcementSound({ type: 'round_end', global: true, value: 2 }), 'radio/ctwin.wav');
  assert.equal(announcementSound({ type: 'kill', global: true }), undefined);
  assert.equal(announcementSound({ type: 'shot', global: false }), undefined);
});

test('touch utilities retain flashlight and dropping without adding right-thumb buttons', async () => {
  const { readFile } = await import('node:fs/promises');
  const source = await readFile(new URL('../web/TouchControls.tsx', import.meta.url), 'utf8');
  const more = source.slice(source.indexOf('{more &&'), source.indexOf('className="cs16-touch-utility"'));
  assert.ok(more.includes('data-cs16-action="flashlight"'));
  assert.ok(more.includes('data-cs16-action="drop"'));
});
