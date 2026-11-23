import test from 'node:test';
import assert from 'node:assert/strict';
import * as cues from '../web/classic-hud.ts';
import { matchClock } from '../web/presentation.ts';
import { announcementSound } from '../web/feedback.ts';

test('planting hides the clock instead of displaying a C4 deadline while ordinary round clocks remain', () => {
  for (const mode of ['bomb', 'hostage', 'elimination']) for (const phase of ['live', 'freeze', 'round_end']) {
    assert.equal(matchClock(mode, phase, 4100, true), '');
    assert.equal(matchClock(mode, phase, 4100, false), mode === 'elimination' && phase === 'live' ? '' : '0:41');
  }
});

test('a compact public bomb cue exposes only a validated serial and source sample stage', () => {
  assert.deepEqual(cues.decodeBombCue(9), { serial: 1, stage: 1 });
  assert.deepEqual(cues.decodeBombCue(261), { serial: 32, stage: 5 });
  for (const code of [undefined, 0, -1, 0.5, NaN, Infinity, 1, 8, 14, 15, Number.MAX_SAFE_INTEGER + 1]) assert.equal(cues.decodeBombCue(code), undefined);
});

test('new audio cues play once and stage changes or out-of-order serials do not restart them', () => {
  const cursor = new cues.BombAudioCursor();
  assert.equal(cursor.consume('round1', 0, true), undefined);
  assert.equal(cursor.consume('round1', 9, true), 1);
  assert.equal(cursor.consume('round1', 9, true), undefined);
  assert.equal(cursor.consume('round1', 10, true), undefined);
  assert.equal(cursor.consume('round1', 17, true), 1);
  assert.equal(cursor.consume('round1', 9, true), undefined);
  assert.equal(cursor.consume('round1', 25, true), 1);
});

test('initial connection, mute, explicit suspension, recovery and skipped cues prime without catch-up', () => {
  const cursor = new cues.BombAudioCursor();
  assert.equal(cursor.consume('round1', 9, true), undefined);
  assert.equal(cursor.consume('round1', 17, true), 1);
  assert.equal(cursor.consume('round1', 25, false), undefined);
  assert.equal(cursor.consume('round1', 33, true), undefined);
  assert.equal(cursor.consume('round1', 41, true), 1);
  cursor.suspend();
  assert.equal(cursor.consume('round1', 49, true), undefined);
  assert.equal(cursor.consume('round1', 58, true), 2);
  assert.equal(cursor.consume('round1', 90, true, true), undefined);
  assert.equal(cursor.consume('round1', 98, true), undefined);
  assert.equal(cursor.consume('round1', 106, true), 2);
  assert.equal(cursor.consume('round1', 138, true), undefined);
  assert.equal(cursor.consume('round1', 146, true), 2);
  assert.equal(cursor.consume('round2', 9, true), undefined);
  assert.equal(cursor.consume('round2', 17, true), 1);
});

test('global C4 cues choose the five original samples without needing a source position', () => {
  for (let stage = 1; stage <= 5; stage++) assert.equal(announcementSound({ type: 'bomb_beep', global: true, value: stage }), `weapons/c4_beep${stage}.wav`);
  for (const value of [0, 6, 1.5, undefined]) assert.equal(announcementSound({ type: 'bomb_beep', global: true, value }), undefined);
  assert.equal(announcementSound({ type: 'bomb_beep', global: false, value: 1 }), undefined);
});
