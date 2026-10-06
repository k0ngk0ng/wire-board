import test from "node:test";
import assert from "node:assert/strict";
import {
  attackCanRespond,
  attackChoices,
  attackSelectedAction,
  emptyAttackSelection,
} from "../src/catan-attack-state.ts";
import { catanTileProducing } from "../src/catan-production.ts";
import {
  catanSavedVictoryTarget,
  catanRuleContext,
} from "../src/catan-rule-context.ts";
const fixture = () => ({
  status: "playing",
  you: 0,
  seats: [{}],
  game: {
    finished: false,
    catan: {
      players: [{}],
      attack: {
        canAct: true,
        pending: { id: 7, player: 0, card: "treason" },
        sources: [1, 2, 3],
        destinations: [1, 2, 3, 4, 5],
        fromBoard: 2,
      },
    },
  },
});
test("treason excludes selected sources, requires different destinations and pads supply sources", () => {
  const room = fixture(),
    pick = { ...emptyAttackSelection(), sources: [1, 2], destinations: [3, 4] };
  assert.deepEqual(attackChoices(room, pick).tiles, [3, 4, 5]);
  assert.deepEqual(attackSelectedAction(room, pick), {
    type: "catan_attack_card",
    prompt: 7,
    choice: "treason",
    give: [1, 2],
    take: [3, 4],
  });
  for (const bad of [
    { sources: [1, 1] },
    { destinations: [3, 3] },
    { destinations: [1, 4] },
    { sources: [1] },
    { sources: [1, 8] },
  ])
    assert.equal(attackSelectedAction(room, { ...pick, ...bad }), null);
  room.game.catan.attack.fromBoard = 1;
  assert.deepEqual(
    attackSelectedAction(room, { ...pick, sources: [2] }).give,
    [2, -1],
  );
  room.game.catan.attack.fromBoard = 0;
  assert.deepEqual(
    attackSelectedAction(room, { ...pick, sources: [] }).give,
    [-1, -1],
  );
});
test("knight preview uses authoritative normal/paid legal destinations and original origin", () => {
  const room = fixture(),
    a = room.game.catan.attack;
  delete a.pending;
  a.endPlan = { id: 12, player: 0 };
  a.moveChoices = [{ from: 8, normal: [2, 3], wheat: [2, 3, 5] }];
  const pick = { ...emptyAttackSelection(), from: 8, target: 5 };
  assert.equal(attackSelectedAction(room, pick), null);
  assert.deepEqual(attackSelectedAction(room, { ...pick, wheat: true }), {
    type: "catan_attack_move",
    prompt: 12,
    choice: "wheat",
    edge: 8,
    target: 5,
  });
  assert.equal(
    attackSelectedAction(room, { ...pick, from: 9, wheat: true }),
    null,
  );
  for (const changes of [
    { spectating: true },
    { you: -1 },
    { status: "finished" },
    { seats: [{ autoPlay: true }] },
  ]) {
    const view = { ...room, ...changes };
    assert.equal(attackCanRespond(view), false);
    assert.equal(attackSelectedAction(view, { ...pick, wheat: true }), null);
    assert.deepEqual(attackChoices(view, pick).edges, []);
  }
});
test("conquest disables production and saved scenario supplies its actual target", () => {
  const g = { tiles: [{ number: 6 }], robber: -1, attack: { conquered: [0] } };
  assert.equal(catanTileProducing(g, 0, 6), false);
  g.attack.conquered = [];
  assert.equal(catanTileProducing(g, 0, 6), true);
  assert.equal(catanSavedVictoryTarget(g), 12);
  const room = fixture();
  assert.equal(catanRuleContext(room).attack, true);
  assert.equal(catanRuleContext(room).target, 12);
});
