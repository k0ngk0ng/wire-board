import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { AttackFeedbackQueue, attackFeedbackBlocked, attackEventInCurrentLife } from '../web/feedback.ts';
import { soundCues } from '../web/classic-hud.ts';

test('local first-shot feedback is immediate while held shots follow real authority cadence and deployment remains gated', () => {
  const ready = { kind: 'shot', heldBefore: false, nextAttack: 100, frame: 100, reloadUntil: 0 };
  assert.equal(attackFeedbackBlocked(ready), false);
  assert.equal(attackFeedbackBlocked({ ...ready, heldBefore: true }), true);
  assert.equal(attackFeedbackBlocked({ ...ready, nextAttack: 101 }), true);
  assert.equal(attackFeedbackBlocked({ ...ready, kind: 'slash', heldBefore: true }), true);
  assert.equal(attackFeedbackBlocked({ ...ready, kind: 'slash', heldBefore: false }), false);
  assert.equal(attackFeedbackBlocked({ ...ready, reloadUntil: 101 }), true);
  assert.equal(attackFeedbackBlocked({ ...ready, reloadUntil: 100 }), false);
  assert.equal(attackFeedbackBlocked({ ...ready, kind: 'mode', heldBefore: true, nextAttack: 300 }), true);
});

test('every automatic shot consumes exactly its own prediction and does not swallow the next unpredicted shot', () => {
  const feedback = new AttackFeedbackQueue();
  const serials = [];
  for (let index = 0; index < 12; index++) {
    serials.push(feedback.predict('shot', 'ak47', 1, index * 100));
    assert.equal(feedback.confirm('shot', 'ak47', 1, index * 100 + 40), true);
    assert.equal(feedback.confirm('shot', 'ak47', 1, index * 100 + 80), false);
  }
  assert.equal(new Set(serials).size, 12);
});

test('batch confirmations preserve weapon and life identity and never replay an old prediction', () => {
  const feedback = new AttackFeedbackQueue();
  for (const at of [1000, 1070, 1140]) feedback.predict('shot', 'p90', 2, at);
  assert.equal(feedback.confirm('shot', 'p90', 3, 1200), false);
  assert.equal(feedback.confirm('shot', 'ak47', 2, 1200), false);
  assert.equal(feedback.confirm('slash', 'p90', 2, 1200), false);
  for (let index = 0; index < 3; index++) assert.equal(feedback.confirm('shot', 'p90', 2, 1200), true);
  assert.equal(feedback.confirm('shot', 'p90', 2, 1200), false);
  feedback.predict('shot', 'ak47', 2, 1300);
  assert.equal(feedback.confirm('shot', 'ak47', 2, 1800), true);
  feedback.predict('shot', 'ak47', 2, 2000);
  assert.equal(feedback.confirm('shot', 'ak47', 2, 2501), false);
  feedback.predict('slash', 'knife', 2, 3000);
  feedback.reset();
  assert.equal(feedback.confirm('slash', 'knife', 2, 3100), false);
});

test('equipment shortcut is a configurable O binding with classic armour and grenade digits', () => {
  const controls = JSON.parse(readFileSync(new URL('../controls.json', import.meta.url), 'utf8'));
  assert.deepEqual(controls.actions.find(action => action.id === 'buyEquipment')?.keys, ['KeyO']);
  assert.deepEqual(controls.actions.find(action => action.id === 'nightVision')?.keys, ['KeyN']);
  assert.deepEqual(soundCues({ type: 'buy', weapon: 'nvgs' }).files, ['items/equip_nvg.wav']);
  assert.deepEqual(soundCues({ type: 'nightvision', value: 1 }), { files: ['items/nvg_on.wav'], range: 0, local: true });
  assert.deepEqual(soundCues({ type: 'nightvision', value: 0 }), { files: ['items/nvg_off.wav'], range: 0, local: true });
});


test('rejected input removes only its prediction and cannot swallow a later real shot', () => {
  const feedback = new AttackFeedbackQueue();
  feedback.predict('shot', 'ak47', 1, 100, 10);
  feedback.predict('shot', 'ak47', 1, 300, 20);
  feedback.reject(10, 1);
  assert.equal(feedback.confirm('shot', 'ak47', 1, 340), true);
  assert.equal(feedback.confirm('shot', 'ak47', 1, 440), false);
  feedback.predict('shot', 'ak47', 1, 500, 30);
  feedback.reject(29, 1);
  assert.equal(feedback.confirm('shot', 'ak47', 1, 540), true);
  assert.equal(feedback.confirm('shot', 'ak47', 1, 640), false);
});


test('late rejects and old-life shots do not affect a newly spawned player', () => {
  const feedback = new AttackFeedbackQueue();
  feedback.predict('shot', 'ak47', 2, 100, 10);
  feedback.reject(10, 1);
  assert.equal(feedback.confirm('shot', 'ak47', 2, 140), true);
  assert.equal(attackEventInCurrentLife({ seat: 0, life: 1 }, 0, 2), false);
  assert.equal(attackEventInCurrentLife({ seat: 0, life: 2 }, 0, 2), true);
  assert.equal(attackEventInCurrentLife({ seat: 1, life: 1 }, 0, 2), true);
  assert.equal(attackEventInCurrentLife({ seat: 0 }, 0, 2), true);
});
