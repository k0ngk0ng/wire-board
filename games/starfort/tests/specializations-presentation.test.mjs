// Before UI implementation: area control appears on packet receipt, grouped
// statuses miss a target, later hit strips control, knockback precedes impact,
// or reconnect replays old spells. Exercise the actual combat playback clock.
import { test } from "node:test";
import assert from "node:assert/strict";
import { CombatPlayback } from "../web/combat-timeline.ts";
const enemy = (id) => ({
  id,
  kind: "boss",
  x: 100,
  y: 0,
  progress: 100,
  hp: 1000,
  maxHP: 1000,
  shield: 0,
  speed: 10,
  slowUntil: 0,
});
const snap = (ms, enemies) => ({
  elapsedMS: ms,
  tick: ms / 200,
  phase: "battle",
  baseHP: 30,
  enemies,
});
for (const [type, key, until] of [
  ["freeze", "frozenUntil", 11],
  ["meteor", "silencedUntil", 16],
]) {
  test(`${type} reaches every area target on impact and survives later compact damage`, () => {
    const initial = snap(0, [enemy(1), enemy(2)]),
      p = new CombatPlayback(initial, 0);
    const events = [
      {
        id: 1,
        type,
        skill: type,
        tick: 1,
        atMS: 200,
        x: 0,
        y: 0,
        statusTargets: [1, 2],
        [key]: until,
      },
      {
        id: 2,
        type: "hit",
        tick: 1,
        atMS: 200,
        x: 0,
        y: 0,
        targetId: 1,
        vitals: { hp: 900, shield: 0, slowUntil: 0 },
      },
    ];
    const end = snap(1000, [
      { ...enemy(1), hp: 900, [key]: until },
      { ...enemy(2), [key]: until },
    ]);
    const effects = p.push(end, events, 1000),
      at = effects[0].born;
    assert.equal(p.read(at - 1).enemies[0][key], undefined);
    assert.deepEqual(
      p.read(at).enemies.map((e) => e[key]),
      [until, until],
    );
    assert.equal(p.read(at).enemies[0].hp, 900);
    p.reset(end, 3000);
    assert.equal(p.read(3000).enemies[1][key], until);
  });
}
test("knockback uses its impact time and saved authoritative road position", () => {
  const p = new CombatPlayback(snap(0, [enemy(1)]), 0),
    target = { ...enemy(1), x: 40, progress: 40 };
  const event = {
    id: 1,
    type: "knockback",
    skill: "knockback",
    tick: 1,
    atMS: 200,
    x: 40,
    y: 0,
    targetId: 1,
    target,
  };
  const effect = p.push(snap(1000, [target]), [event], 1000)[0];
  assert.equal(p.read(effect.born - 1).enemies[0].progress, 100);
  assert.equal(p.read(effect.born).enemies[0].progress, 40);
});
