import test from 'node:test';
import assert from 'node:assert/strict';
import { actionProgress, soundCues, bombBeep, plantClick, CueCursor } from '../web/classic-hud.ts';

test('plant and defuse progress clamp authoritative frames and disappear on cancellation', () => {
  const bomb = { planted: false };
  const player = { seat: 0, alive: true, action: 'plant', actionSince: 100 };
  assert.deepEqual(actionProgress(player, bomb, 100), { kind: 'plant', value: 0 });
  assert.deepEqual(actionProgress(player, bomb, 250), { kind: 'plant', value: 500 });
  assert.deepEqual(actionProgress(player, bomb, 400), { kind: 'plant', value: 1000 });
  assert.equal(actionProgress({ ...player, action: '' }, bomb, 250), undefined);
  assert.equal(actionProgress({ ...player, alive: false }, bomb, 250), undefined);
  for (const [defuser, duration] of [[false, 1000], [true, 500]]) {
    assert.deepEqual(actionProgress({ ...player, defuser, action: 'defuse' }, { planted: true, defuser: 0, defuseAt: 100 + duration }, 100 + duration / 2), { kind: 'defuse', value: 500 });
  }
});

test('purchase confirmation and knife contact choose original distinct samples and ranges', () => {
  assert.deepEqual(soundCues({ type: 'buy', weapon: 'primammo' }).files, ['items/9mmclip1.wav']);
  assert.deepEqual(soundCues({ type: 'knife_air', value: 1 }).files, ['weapons/knife_slash1.wav', 'weapons/knife_slash2.wav']);
  assert.deepEqual(soundCues({ type: 'knife_hit', value: 1 }).files, ['weapons/knife_stab.wav']);
  assert.deepEqual(soundCues({ type: 'knife_wall', value: 1 }).files, ['weapons/knife_hitwall1.wav']);
  assert.ok(soundCues({ type: 'knife_air' }).range < soundCues({ type: 'shot', weapon: 'ak47' }).range);
  assert.ok(soundCues({ type: 'step' }).range < soundCues({ type: 'shot', weapon: 'ak47' }).range);
  assert.equal(soundCues({ type: 'knife_air' }).range, 1250);
  assert.equal(soundCues({ type: 'shot', weapon: 'usp', value: 1 }).range, 500);
  assert.deepEqual(soundCues({ type: 'buy', weapon: 'vest' }).files, ['items/kevlar.wav']);
  assert.deepEqual(soundCues({ type: 'hit', hitgroup: 1 }).files, ['player/headshot1.wav']);
  assert.deepEqual(soundCues({ type: 'hit', hitgroup: 2 }).files, ['player/bhit_flesh-1.wav']);
});

test('bomb timer schedules bounded local cues without network events or historical catch-up', () => {
  const early = bombBeep(151, 4600);
  const late = bombBeep(4401, 4600);
  assert.ok(early && late);
  assert.ok(late.stage > early.stage);
  const cursor = new CueCursor();
  assert.equal(cursor.consume('epoch:1', 'beep:1'), false);
  assert.equal(cursor.consume('epoch:1', 'beep:1'), false);
  assert.equal(cursor.consume('epoch:1', 'beep:2'), true);
  assert.equal(cursor.consume('epoch:2', 'beep:2'), false);
  assert.equal(cursor.consume('epoch:2', ''), false);
  assert.equal(cursor.consume('epoch:2', 'beep:3'), true);
  assert.equal(bombBeep(4100, 4100), undefined);
});

test('C4 pressbutton sounds follow original MDL event frames', () => {
  assert.equal(plantClick(100, 100), undefined);
  assert.equal(plantClick(218, 100), undefined);
  assert.equal(plantClick(219, 100), 0);
  assert.equal(plantClick(243, 100), 1);
  assert.equal(plantClick(355, 100), 6);
});
