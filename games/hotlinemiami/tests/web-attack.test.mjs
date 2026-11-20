import test from "node:test";
import assert from "node:assert/strict";
import { AttackPresentation } from "../web/attack.ts";

const weapons = [{ id: "pistol", cooldown: 15, melee: false }, { id: "shotgun", cooldown: 44, melee: false }, { id: "knife", cooldown: 15, melee: true }];
const player = (extra = {}) => ({ life: 1, weapon: "pistol", ammo: 2, alive: true, nextAttackFrame: 0, downUntil: 0, busyUntil: 0, appliedSeq: 0, ...extra });

test("first visible attack is immediate but a pistol cannot flash before shared cooldown", () => {
  const p = new AttackPresentation(weapons);
  p.confirm(player());
  assert.equal(p.fire(1, 10), true);
  assert.equal(p.fire(2, 24), false);
  assert.equal(p.fire(3, 25), true);
});

test("unacknowledged last round cannot create a second shot", () => {
  const p = new AttackPresentation(weapons);
  p.confirm(player({ ammo: 1 }));
  assert.equal(p.fire(1, 0), true);
  assert.equal(p.fire(2, 15), false);
  p.confirm(player({ ammo: 0, appliedSeq: 1, nextAttackFrame: 15 }));
  assert.equal(p.fire(3, 16), false);
});

test("authority gates downed, busy, dead and weapon-specific fire intervals", () => {
  const p = new AttackPresentation(weapons);
  p.confirm(player({ weapon: "shotgun", downUntil: 10, busyUntil: 20, nextAttackFrame: 25 }));
  assert.equal(p.fire(1, 9), false);
  assert.equal(p.fire(2, 19), false);
  assert.equal(p.fire(3, 24), false);
  assert.equal(p.fire(4, 25), true);
  assert.equal(p.fire(5, 68), false);
  assert.equal(p.fire(6, 69), true);
  p.confirm(player({ alive: false }));
  assert.equal(p.fire(7, 90), false);
});

test("melee has no ammo cost and new life discards previous speculative fire", () => {
  const p = new AttackPresentation(weapons);
  p.confirm(player({ weapon: "knife", ammo: 0 }));
  assert.equal(p.fire(1, 0), true);
  assert.equal(p.fire(2, 15), true);
  p.confirm(player({ life: 2, weapon: "pistol", ammo: 1 }));
  assert.equal(p.fire(3, 16), true);
});

test("unarmed attack immediately drives the next draw without consuming ammunition or adding a selectable weapon", () => {
  const before = structuredClone(weapons);
  const p = new AttackPresentation(weapons, { id: "", cooldown: 22, melee: true });
  p.confirm(player({ weapon: "", ammo: 0 }));
  assert.equal(p.fire(1, 0), true);
  assert.equal(p.fire(2, 21), false);
  assert.equal(p.fire(3, 22), true);
  assert.deepEqual(weapons, before);
  assert.equal(weapons.some(weapon => weapon.id === ""), false);
});
