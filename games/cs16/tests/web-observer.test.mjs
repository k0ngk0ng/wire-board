import test from 'node:test';
import assert from 'node:assert/strict';
import { observerCandidates, eyeOffset, viewModelVerticalFov, viewModelHand, formatDuration, matchClock, visibleVelocity, viewAngles } from '../web/presentation.ts';

test('dead participants can follow living teammates while independent spectators can follow both teams', () => {
  const players = [{ seat: 0, team: 't', alive: false, visible: true }, { seat: 1, team: 'ct', alive: true, visible: true }, { seat: 2, team: 't', alive: true, visible: true }, { seat: 3, team: 't', alive: true, visible: false }];
  assert.deepEqual(observerCandidates(players, players[0], false).map(player => player.seat), [2]);
  assert.deepEqual(observerCandidates(players, undefined, true).map(player => player.seat), [1, 2]);
  assert.deepEqual(observerCandidates(players.filter(player => player.seat !== 2), players[0], false), []);
});

test('CS standing and crouching eyes use original view offsets', () => {
  assert.equal(eyeOffset(false), 17);
  assert.equal(eyeOffset(true), 12);
});

test('view model preserves classic four-to-three vertical view on widescreen and at least horizontal 90 in portrait', () => {
  const classicVertical = 2 * Math.atan(3 / 4);
  for (const aspect of [4 / 3, 16 / 9, 320 / 420]) {
    const vertical = viewModelVerticalFov(aspect) * Math.PI / 180;
    if (aspect >= 4 / 3) assert.ok(Math.abs(vertical - classicVertical) < 0.000001);
    else assert.ok(Math.abs(2 * Math.atan(Math.tan(vertical / 2) * aspect) - Math.PI / 2) < 0.000001);
  }
  assert.equal(viewModelHand('ak47'), -1);
  assert.equal(viewModelHand('knife'), 1);
});

test('countdown rounds total seconds before splitting minutes so minute boundaries never jump backwards', () => {
  for (const [frames, label] of [[12000, '2:00'], [11999, '2:00'], [11900, '1:59'], [6000, '1:00'], [5999, '1:00'], [5900, '0:59'], [1, '0:01'], [0, '0:00'], [-1, '0:00']]) assert.equal(formatDuration(frames), label);
});

test('live elimination shows its objective while freeze and round-end keep authoritative countdowns', () => {
  for (const frames of [0, -1, -12000, 12000]) assert.equal(matchClock('elimination', 'live', frames), '歼灭敌队');
  for (const phase of ['freeze', 'round_end']) {
    for (const [frames, label] of [[300, '0:03'], [1, '0:01'], [0, '0:00']]) assert.equal(matchClock('elimination', phase, frames), label);
  }
  for (const mode of ['bomb', 'hostage']) {
    for (const phase of ['freeze', 'live', 'round_end']) {
      for (const [frames, label] of [[12000, '2:00'], [5999, '1:00'], [4000, '0:40'], [1, '0:01'], [0, '0:00']]) assert.equal(matchClock(mode, phase, frames), label);
    }
  }
});

test('automated camera follows authority while human mouse facing remains immediate', () => {
  const authority = { yaw: 4000, pitch: 500 };
  const mouse = { yaw: 1000, pitch: 100 };
  assert.deepEqual(viewAngles(authority, mouse, true), authority);
  assert.deepEqual(viewAngles(authority, mouse, false), mouse);
});

test('visible remote motion drives running animation without private velocity fields', () => {
  assert.deepEqual(visibleVelocity({ x: 0, y: 0 }, { x: 512, y: -256 }, 2), { vx: 25600, vy: -12800 });
  assert.deepEqual(visibleVelocity({ x: 0, y: 0 }, { x: 0, y: 0 }, 2), { vx: 0, vy: 0 });
  assert.deepEqual(visibleVelocity({ x: 0, y: 0 }, { x: 512, y: 0, vx: 1, vy: 2 }, 2), { vx: 1, vy: 2 });
});
