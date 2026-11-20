import assert from "node:assert/strict";
import test from "node:test";
import { personalView } from "../web/personal-view.ts";

test("each interface uses its trainer while retaining the complete battlefield", () => {
  const team = [{ id: 1, owner: 0 }, { id: 2, owner: 1 }];
  const first = { actor: 0, gold: 10, team: [team[0]], box: [], items: [], progression: { eggPrice: 10, settings: { language: 8 } } };
  const second = { actor: 1, gold: 40, team: [team[1]], box: [{ id: 3, owner: 1 }], items: [{ id: "leftovers" }], progression: { eggPrice: 47, settings: { language: 0 } } };
  const source = { gold: 10, team, box: [], trainers: [first, second], towers: [{ pokemonID: 1 }, { pokemonID: 2 }], health: 14, wave: 1 };
  const one = personalView(source, 0);
  const two = personalView(source, 1);
  assert.equal(one.gold, 10);
  assert.equal(two.gold, 40);
  assert.deepEqual(one.team, [team[0]]);
  assert.deepEqual(two.team, [team[1]]);
  assert.deepEqual(two.box, second.box);
  assert.deepEqual(two.items, second.items);
  assert.equal(two.progression.settings.language, 0);
  assert.equal(one.progression.eggPrice, 10);
  assert.equal(two.progression.eggPrice, 47);
  assert.deepEqual(two.battleTeam, team);
  assert.deepEqual(two.towers, source.towers);
  assert.equal(source.team.length, 2);
  assert.equal(source.gold, 10);
  assert.deepEqual(personalView(two, 0).battleTeam, team);
});

test("spectators retain the common battlefield without an invented personal roster", () => {
  const source = { team: [{ id: 1 }], trainers: [{ actor: 0, gold: 50, team: [{ id: 1 }] }] };
  assert.equal(personalView(source, -1), source);
  assert.equal(personalView(source, 9), source);
});
