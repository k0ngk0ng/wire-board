import { test } from "node:test";
import assert from "node:assert/strict";
import { catanRuleContext } from "../src/catan-rule-context.ts";

test("cities and knights waiting rules use thirteen points and hide stale base layout", () => {
  const room = {
    capacity: 6,
    catanOptions: { fiveSix: true },
    catanCitiesKnights: { layout: "variable", rules: "catan-knights-5-6-2025" },
    catanBaseConfiguration: { layout: "fixed" },
  };
  const rules = catanRuleContext(room);
  assert.equal(rules.citiesKnights, true);
  assert.equal(rules.target, 13);
  assert.equal(rules.layout, "variable");
  assert.equal(rules.fixedBase, false);
  assert.equal(rules.neutral, false);
  assert.equal(rules.fiveSix, true);
  assert.equal(rules.helpers, false);
  assert.equal(rules.waiting, true);
});

test("cities and knights saved games resolve actual players and legacy layout", () => {
  for (const count of [3, 6]) {
    const room = {
      capacity: 4,
      catanOptions: { helpers: true },
      catanSeafarers: { scenario: "shores", layout: "fixed" },
      game: {
        catan: {
          players: Array(count).fill({}),
          citiesKnights: { rules: "catan-knights-2025" },
          paired: count > 4 ? { primary: 0 } : undefined,
        },
      },
    };
    const rules = catanRuleContext(room);
    assert.equal(rules.citiesKnights, true);
    assert.equal(rules.target, 13);
    assert.equal(rules.layout, "variable");
    assert.equal(rules.players, count);
    assert.equal(rules.fiveSix, count > 4);
    assert.equal(rules.helpers, false);
    assert.equal(rules.scenario, "");
    assert.equal(rules.waiting, false);
  }
});

test("a cities and knights waiting draft cannot change saved base or seafarers rules", () => {
  const room = {
    capacity: 6,
    catanOptions: { fiveSix: true },
    catanCitiesKnights: { layout: "variable" },
    game: { catan: { players: Array(3).fill({}) } },
  };
  assert.equal(catanRuleContext(room).citiesKnights, false);
  assert.equal(catanRuleContext(room).target, 10);
  assert.equal(catanRuleContext(room).fiveSix, false);
  room.game.catan.seafarers = {
    scenario: "shores",
    layout: "fixed",
    victoryPoints: 14,
  };
  assert.equal(catanRuleContext(room).citiesKnights, false);
  assert.equal(catanRuleContext(room).target, 14);
  assert.equal(catanRuleContext(room).layout, "fixed");
});

test("waiting rules follow scenario selection, player extension and Helpers", () => {
  const room = {
    capacity: 4,
    catanOptions: {},
    catanSeafarers: { scenario: "shores", layout: "variable" },
  };
  assert.equal(catanRuleContext(room).target, 14);
  room.catanSeafarers.scenario = "tribe";
  assert.equal(catanRuleContext(room).target, 13);
  room.catanSeafarers.scenario = "islands";
  room.capacity = 6;
  room.catanSeafarers.layout = "fixed";
  room.catanOptions = { fiveSix: true, helpers: true, allHelpers: true };
  const rules = catanRuleContext(room);
  assert.equal(rules.scenario, "six_islands");
  assert.equal(rules.layout, "fixed");
  assert.equal(rules.target, 13);
  assert.equal(rules.waiting, true);
  assert.equal(rules.fiveSix, true);
  assert.equal(rules.helpers, true);
  assert.equal(rules.allHelpers, true);
});

test("running saved rules outrank stale room setup, capacity and options", () => {
  const room = {
    capacity: 6,
    catanOptions: { fiveSix: true, helpers: true },
    catanSeafarers: { scenario: "pirate_islands", layout: "fixed" },
    catanNewWorldMap: { hexes: [] },
    game: {
      catan: {
        players: [{}, {}, {}],
        options: {},
        seafarers: { scenario: "cloth", layout: "variable", victoryPoints: 14 },
      },
    },
  };
  const rules = catanRuleContext(room);
  assert.equal(rules.scenario, "cloth");
  assert.equal(rules.players, 3);
  assert.equal(rules.waiting, false);
  assert.equal(rules.layout, "variable");
  assert.equal(rules.helpers, false);
  assert.equal(rules.fiveSix, false);
  assert.equal(rules.target, 14);
  delete room.game.catan.seafarers;
  delete room.game.catan.options;
  assert.equal(catanRuleContext(room).scenario, "");
  assert.equal(catanRuleContext(room).target, 10);
  assert.equal(catanRuleContext(room).helpers, false);
});

test("legacy scenario snapshots and prepared waiting maps keep their rules", () => {
  const room = { capacity: 4, catanNewWorldMap: { hexes: [] } };
  assert.equal(catanRuleContext(room).scenario, "new_world");
  assert.equal(catanRuleContext(room).layout, "prepared");
  room.game = {
    catan: {
      players: Array(5).fill({}),
      paired: { primary: 1 },
      seafarers: {
        scenario: "six_islands",
        victoryPoints: 13,
        variable: false,
      },
    },
  };
  assert.equal(catanRuleContext(room).scenario, "six_islands");
  assert.equal(catanRuleContext(room).layout, "fixed");
  assert.equal(catanRuleContext(room).fiveSix, true);
  room.game.catan.seafarers = {
    scenario: "wonders",
    variable: true,
    victoryPoints: 10,
  };
  assert.equal(catanRuleContext(room).layout, "variable");
  room.game.catan.seafarers = {
    scenario: "new_world",
    newWorld: { index: 2 },
    variable: true,
    victoryPoints: 12,
  };
  assert.equal(catanRuleContext(room).layout, "prepared");
  assert.equal(catanRuleContext(room).target, 12);
});

test("fixed base snapshots preserve neutral setup rules independently of capacity", () => {
  const room = {
    capacity: 6,
    game: {
      catan: {
        players: Array(5).fill({}),
        baseSetup: {
          layout: "fixed",
          rules: "catan-base-5-6-2025",
          colors: [2, 1, 4, 5, 3],
          neutralColor: 0,
        },
        paired: { primary: 0 },
      },
    },
  };
  assert.equal(catanRuleContext(room).fixedBase, true);
  assert.equal(catanRuleContext(room).neutral, true);
  assert.equal(catanRuleContext(room).players, 5);
  assert.equal(catanRuleContext(room).scenario, "");
  room.game.catan.baseSetup.neutralColor = -1;
  assert.equal(catanRuleContext(room).neutral, false);
  delete room.game.catan.baseSetup;
  assert.equal(catanRuleContext(room).fixedBase, false);
  assert.equal(catanRuleContext(room).neutral, false);
});

test("waiting base layout changes the quick reference but cannot override a running game", () => {
  const room = {
    capacity: 6,
    catanOptions: { fiveSix: true },
    catanBaseConfiguration: { layout: "fixed" },
  };
  assert.equal(catanRuleContext(room).fixedBase, true);
  assert.equal(catanRuleContext(room).neutral, true); // Explain five-player case before actual player count is known.
  room.game = {
    catan: {
      players: Array(6).fill({}),
      baseSetup: { layout: "variable", neutralColor: -1 },
      options: { fiveSix: true },
    },
  };
  assert.equal(catanRuleContext(room).fixedBase, false);
  assert.equal(catanRuleContext(room).neutral, false);
  delete room.game;
  room.catanSeafarers = { scenario: "fog", layout: "fixed" };
  assert.equal(catanRuleContext(room).fixedBase, false);
});

test("combined maps retain sea layout and add two points only to waiting recipes", () => {
  for (const [scenario, target] of [
    ["shores", 16],
    ["islands", 15],
    ["fog", 14],
    ["desert", 16],
    ["new_world", 14],
    ["cloth", 16],
  ]) {
    const room = {
      capacity: 6,
      catanCitiesKnights: { layout: "variable" },
      catanSeafarers: {
        scenario,
        layout: scenario === "new_world" ? "prepared" : "fixed",
      },
    };
    const waiting = catanRuleContext(room);
    assert.equal(waiting.target, target);
    assert.equal(waiting.layout, room.catanSeafarers.layout);
    assert.equal(
      waiting.scenario,
      scenario === "islands" ? "six_islands" : scenario,
    );
    room.game = {
      catan: {
        players: Array(3).fill({}),
        citiesKnights: { layout: "variable" },
        seafarers: { scenario, layout: "fixed", victoryPoints: target },
      },
    };
    room.catanSeafarers = { scenario: "shores", layout: "variable" };
    const playing = catanRuleContext(room);
    assert.equal(playing.target, target); // Never add the two-point adjustment twice.
    assert.equal(playing.layout, "fixed");
    assert.equal(playing.scenario, scenario);
  }
});

test("combined wonders use twelve-point threshold and preserve variable sea layout", () => {
  const room = {
    capacity: 4,
    catanCitiesKnights: { layout: "variable" },
    catanSeafarers: { scenario: "wonders", layout: "fixed" },
  };
  assert.equal(catanRuleContext(room).target, 12);
  assert.equal(catanRuleContext(room).layout, "fixed");
  room.game = {
    catan: {
      players: Array(3).fill({}),
      citiesKnights: {},
      seafarers: { scenario: "wonders", layout: "variable", victoryPoints: 12 },
    },
  };
  assert.equal(catanRuleContext(room).target, 12);
  assert.equal(catanRuleContext(room).layout, "variable");
});

test("harbors target is added once and comes from the running game instead of its room draft", () => {
 for (const [scenario, ck, expected] of [["",false,11],["",true,14],["wonders",true,13],["cloth",false,15],["pirate_islands",false,11],["shores",true,17]]) {
  const room = { capacity: 3, catanHarbors: {enabled:true}, ...(ck ? {catanCitiesKnights:{}} : {}), ...(scenario ? {catanSeafarers:{scenario}} : {}) };
  assert.equal(catanRuleContext(room).target,expected);
  room.game = {catan:{players:[{},{},{}], harbors:{owner:-1,points:[0,0,0]}, ...(ck ? {citiesKnights:{}} : {}), ...(scenario ? {seafarers:{scenario,victoryPoints:expected-1}} : {})}};
  room.catanHarbors.enabled=false;
  assert.equal(catanRuleContext(room).harbors,true);
  assert.equal(catanRuleContext(room).target,expected);
  room.game.catan.victoryTarget=expected;
  assert.equal(catanRuleContext(room).target,expected);
  delete room.game.catan.harbors;
  room.catanHarbors.enabled=true;
  room.game.catan.victoryTarget=expected-1;
  assert.equal(catanRuleContext(room).harbors,false);
  assert.equal(catanRuleContext(room).target,expected-1);
 }
});

test("friendly robber follows the actual game rather than a stale waiting draft and does not change target", () => {
  const room = { capacity: 3, catanFriendlyRobber: { enabled: true } };
  assert.equal(catanRuleContext(room).friendlyRobber, true);
  assert.equal(catanRuleContext(room).target, 10);
  room.catanHarbors = { enabled: true };
  assert.equal(catanRuleContext(room).target, 11);
  room.game = { catan: { players: [{}, {}, {}] } };
  assert.equal(catanRuleContext(room).friendlyRobber, false);
  assert.equal(catanRuleContext(room).target, 10);
  room.game.catan.friendlyRobber = { rules: "catan-friendly-robber-2025", protectedPlayers: [0, 1] };
  room.catanFriendlyRobber.enabled = false;
  assert.equal(catanRuleContext(room).friendlyRobber, true);
  assert.equal(catanRuleContext(room).target, 10);
});

test("saved Explorer target overrides stale lobby drafts and survives legacy views", () => {
  for (const target of [8, 12, 15, 17]) {
    const room = {
      capacity: 6,
      catanSeafarers: {scenario: "shores"},
      catanCitiesKnights: {layout: "variable"},
      game: {catan: {players: [{}, {}, {}], explorer: {board: {target}}}},
    };
    assert.equal(catanRuleContext(room).target, target);
    room.game.catan.citiesKnights = {};
    assert.equal(catanRuleContext(room).target, target, "saved target already includes combination points");
    room.game.catan.victoryTarget = target + 1;
    assert.equal(catanRuleContext(room).target, target + 1, "explicit server target wins");
  }
});


test("public Rivers and Caravans drafts use the chosen recipe, saved games win", () => {
  for (const [scenario, target] of [["rivers", 10], ["caravans", 12]]) {
    const room = { capacity: 4, catanScenario: scenario, catanOptions: {} };
    const waiting = catanRuleContext(room);
    assert.equal(waiting[scenario], true);
    assert.equal(waiting.target, target);
    assert.equal(waiting.two, false);
    assert.equal(waiting.fiveSix, false);
    room.game = { catan: { players: [{}, {}, {}], options: {} } };
    const playing = catanRuleContext(room);
    assert.equal(playing.rivers, false);
    assert.equal(playing.caravans, false);
    assert.equal(playing.target, 10);
    room.game.catan[scenario] = {};
    assert.equal(catanRuleContext(room)[scenario], true);
    assert.equal(catanRuleContext(room).target, target);
  }
});

test("public Land Ho waiting rules support two players without trade tokens and saved rules prevail", () => {
  const room = { capacity: 2, catanScenario: "land-ho", catanOptions: {} };
  let info = catanRuleContext(room);
  assert.equal(info.explorer, true);
  assert.equal(info.two, false);
  assert.equal(info.target, 8);
  assert.equal(info.scenario, "land-ho");
  room.game = {
    catan: {
      players: [{}, {}, {}],
      explorer: { board: { scenario: "land-ho", target: 8 } },
    },
  };
  room.catanScenario = "shores";
  room.catanTwoRules = "catan-two-2025";
  info = catanRuleContext(room);
  assert.equal(info.explorer, true);
  assert.equal(info.players, 3);
  assert.equal(info.two, false);
  assert.equal(info.target, 8);
  assert.equal(info.scenario, "land-ho");
  delete room.game.catan.explorer;
  info = catanRuleContext(room);
  assert.equal(info.explorer, false);
  assert.equal(info.target, 10);
  assert.equal(info.scenario, "");
});

test("Spices waiting rules choose the task target and paired turns by player count", () => {
  for (const players of [2, 3, 4, 5, 6]) {
    const room = {capacity: players, catanScenario: "spices-for-catan", catanOptions: {}};
    const info = catanRuleContext(room);
    assert.equal(info.explorer, true);
    assert.equal(info.two, false);
    assert.equal(info.target, 15);
    assert.equal(info.layout, "variable");
    assert.equal(info.fiveSix, players > 4);
    room.game = {catan: {players: [{}, {}], explorer: {board: {scenario: "spices-for-catan", target: 15, layout: "variable"}}}};
    const saved = catanRuleContext(room);
    assert.equal(saved.players, 2);
    assert.equal(saved.target, 15);
    assert.equal(saved.fiveSix, false);
    assert.equal(saved.layout, "variable");
  }
});

test("public standalone cities-knights selection keeps its thirteen-point rules", () => {
  const room = {
    capacity: 4,
    catanScenario: "cities-knights",
    catanCitiesKnights: { layout: "variable", rules: "catan-knights-2025" },
    catanOptions: {},
  };
  const info = catanRuleContext(room);
  assert.equal(info.citiesKnights, true);
  assert.equal(info.explorer, false);
  assert.equal(info.scenario, "");
  assert.equal(info.target, 13);
  assert.equal(info.layout, "variable");
  assert.equal(info.fiveSix, false);
  room.game = { catan: { players: [{}, {}, {}], citiesKnights: { layout: "variable" }, options: {} } };
  assert.equal(catanRuleContext(room).players, 3);
  assert.equal(catanRuleContext(room).target, 13);
});

test("fishing waiting rules do not masquerade as Seafarers or override saved games", () => {
  const room = { capacity: 4, catanScenario: "fishing" };
  assert.equal(catanRuleContext(room).fishing, true);
  assert.equal(catanRuleContext(room).scenario, "");
  assert.equal(catanRuleContext(room).target, 10);
  room.game = { catan: { players: Array(3).fill({}) } };
  assert.equal(catanRuleContext(room).fishing, false);
  room.game.catan.fishing = {};
  assert.equal(catanRuleContext(room).fishing, true);
  assert.equal(catanRuleContext(room).players, 3);
});


test("public Barbarian Attack uses twelve points and actual player pairing", () => {
 const room = {capacity:6,catanScenario:"barbarian-attack"};
 assert.equal(catanRuleContext(room).attack,true);
 assert.equal(catanRuleContext(room).target,12);
 assert.equal(catanRuleContext(room).fiveSix,true);
 assert.equal(catanRuleContext(room).scenario,"");
 room.game={catan:{players:Array(3).fill({}),attack:{}}};
 assert.equal(catanRuleContext(room).players,3);
 assert.equal(catanRuleContext(room).fiveSix,false);
 assert.equal(catanRuleContext(room).target,12);
 delete room.game.catan.attack;
 assert.equal(catanRuleContext(room).attack,false);
 assert.equal(catanRuleContext(room).target,10);
});


test("transport waiting rules and actual players retain thirteen points and neutral rules", () => {
 const room = {capacity:2,catanScenario:"transport"};
 let info=catanRuleContext(room);
 assert.equal(info.transport,true); assert.equal(info.two,true);
 assert.equal(info.explorer,false); assert.equal(info.target,13);
 room.capacity=4;
 assert.equal(catanRuleContext(room).two,false);
 room.game={catan:{players:[{},{}],transport:{},two:{}}};
 info=catanRuleContext(room);
 assert.equal(info.players,2); assert.equal(info.two,true); assert.equal(info.target,13);
 delete room.game.catan.transport; delete room.game.catan.two;
 info=catanRuleContext(room);
 assert.equal(info.transport,false); assert.equal(info.two,false); assert.equal(info.target,10);
});


test("Fishing with Cities and Knights uses actual combination and thirteen points", () => {
 const room={capacity:4,catanScenario:"fishing",catanCitiesKnights:{layout:"variable"}};
 let info=catanRuleContext(room);
 assert.equal(info.fishing,true);assert.equal(info.citiesKnights,true);assert.equal(info.target,13);
 assert.equal(info.scenario,"");assert.equal(info.layout,"variable");
 room.game={catan:{players:[{},{},{}],fishing:{},citiesKnights:{layout:"variable"}}};
 info=catanRuleContext(room);assert.equal(info.players,3);assert.equal(info.target,13);
 delete room.game.catan.citiesKnights;
 info=catanRuleContext(room);assert.equal(info.citiesKnights,false);assert.equal(info.target,10);
});


test("public sea Fishing keeps the sea target and saved recipe authoritative", () => {
 for (const [scenario,target] of [["islands",13],["fog",12],["desert",14],["tribe",13],["cloth",14],["wonders",10],["new_world",12]]) {
  const room={capacity:4,catanScenario:scenario,catanFishing:true,catanSeafarers:{scenario,layout:"fixed"}};
  const info=catanRuleContext(room);assert.equal(info.fishing,true);assert.equal(info.scenario,scenario);assert.equal(info.target,target);assert.equal(info.citiesKnights,false);
  room.game={catan:{players:[{},{},{}],seafarers:{scenario,layout:"fixed"}}};assert.equal(catanRuleContext(room).fishing,false);
  room.game.catan.fishing={};assert.equal(catanRuleContext(room).fishing,true);
 }
});
