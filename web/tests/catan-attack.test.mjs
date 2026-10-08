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

test("partial treason requires the full available count and uses only feasible sources", () => {
  const room = fixture(),
    a = room.game.catan.attack;
  a.treasonCount = 1;
  a.fromBoard = 1;
  a.sources = [2, 3];
  a.destinations = [1];
  a.treasonPlans = [
    { sources: [2], destinations: [1] },
    { sources: [3], destinations: [1] },
  ];
  assert.deepEqual(attackChoices(room, emptyAttackSelection()).tiles, [2, 3]);
  const pick = { ...emptyAttackSelection(), sources: [2], destinations: [1] };
  assert.deepEqual(attackChoices(room, pick).tiles, [1]);
  assert.deepEqual(attackSelectedAction(room, pick), {
    type: "catan_attack_card",
    prompt: 7,
    choice: "treason",
    give: [2],
    take: [1],
  });
  for (const changes of [
    { sources: [1] },
    { sources: [] },
    { destinations: [] },
    { destinations: [1, 3] },
  ])
    assert.equal(attackSelectedAction(room, { ...pick, ...changes }), null);
  a.fromBoard = 0;
  a.sources = [];
  a.treasonPlans = [{ sources: [-1], destinations: [1] }];
  assert.deepEqual(
    attackSelectedAction(room, { ...pick, sources: [] }).give,
    [-1],
  );
  assert.equal(attackChoices(room, emptyAttackSelection()).sources, false);
});

test("treason source pairs cannot mix individually legal but incompatible choices", () => {
  const room = fixture(),
    a = room.game.catan.attack;
  a.treasonCount = 2;
  a.sources = [1, 2, 3, 4];
  a.destinations = [5, 6, 7];
  a.treasonPlans = [
    { sources: [1, 2], destinations: [5, 6] },
    { sources: [3, 4], destinations: [6, 7] },
  ];
  assert.deepEqual(
    attackChoices(room, { ...emptyAttackSelection(), sources: [1] }).tiles,
    [2],
  );
  assert.deepEqual(
    attackChoices(room, { ...emptyAttackSelection(), sources: [3] }).tiles,
    [4],
  );
  assert.equal(
    attackSelectedAction(room, {
      ...emptyAttackSelection(),
      sources: [1, 4],
      destinations: [5, 6],
    }),
    null,
  );
  assert.equal(
    attackSelectedAction(room, {
      ...emptyAttackSelection(),
      sources: [1, 2],
      destinations: [6, 7],
    }),
    null,
  );
  assert.ok(
    attackSelectedAction(room, {
      ...emptyAttackSelection(),
      sources: [2, 1],
      destinations: [6, 5],
    }),
  );
  for (const view of [
    { ...room, spectating: true },
    { ...room, seats: [{ autoPlay: true }] },
  ]) {
    assert.deepEqual(attackChoices(view, emptyAttackSelection()).tiles, []);
    assert.equal(
      attackSelectedAction(view, {
        ...emptyAttackSelection(),
        sources: [1, 2],
        destinations: [5, 6],
      }),
      null,
    );
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

test("Attack city target is thirteen and ordinary attack stays twelve", () => {
  const waiting = {
    capacity: 6,
    catanScenario: "barbarian-attack",
    catanCitiesKnights: { layout: "variable" },
  };
  assert.equal(catanRuleContext(waiting).target, 13);
  const g = {
    attack: { city: { rules: "catan-attack-knights-2025" } },
    citiesKnights: {},
    players: [{}, {}, {}],
  };
  assert.equal(catanSavedVictoryTarget(g), 13);
  delete g.attack.city;
  delete g.citiesKnights;
  assert.equal(catanSavedVictoryTarget(g), 12);
});

test("fish movement requires selected payment and authoritative five-step choices", () => {
  const room=fixture(),a=room.game.catan.attack;
  delete a.pending;
  a.endPlan={id:12,player:0};
  a.moveChoices=[{from:8,normal:[2],wheat:[2,5],fish:[2,5]}];
  a.fishCost=2;a.fishTokens=[{id:3,fish:1},{id:13,fish:2}];
  const pick={...emptyAttackSelection(),from:8,target:5,fish:true,tokens:[3]};
  assert.equal(attackSelectedAction(room,pick),null);
  assert.deepEqual(attackSelectedAction(room,{...pick,tokens:[13]}),{type:"catan_attack_move",prompt:12,choice:"fish",edge:8,target:5,tokens:[13]});
  assert.equal(attackSelectedAction(room,{...pick,tokens:[99]}),null);
  assert.equal(attackSelectedAction(room,{...pick,target:9,tokens:[13]}),null);
});
