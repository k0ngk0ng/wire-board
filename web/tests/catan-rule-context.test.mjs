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
