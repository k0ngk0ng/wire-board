import { test } from "node:test";
import assert from "node:assert/strict";
import {
  catanEventsSupported,
  CATAN_EVENT_CATALOGUE,
} from "../src/catan-event-options.ts";
import { catanRuleContext } from "../src/catan-rule-context.ts";

test("fleet event site rules follow the actual saved catalogue", () => {
  const room = {
    kind: "catan",
    capacity: 4,
    catanScenario: "pirate_islands",
    catanSeafarers: { scenario: "pirate_islands" },
    catanEvents: CATAN_EVENT_CATALOGUE,
  };
  assert.equal(catanRuleContext(room).eventFleet, "wire-board-events-fleet-v1");
  room.game = {
    catan: {
      players: Array(4).fill({}),
      seafarers: { scenario: "pirate_islands" },
    },
  };
  assert.equal(catanRuleContext(room).eventFleet, "");
  room.game.catan.eventDeck = {
    catalogue: CATAN_EVENT_CATALOGUE,
    fleetRules: "wire-board-events-fleet-v1",
  };
  room.catanEvents = "";
  assert.equal(catanRuleContext(room).eventFleet, "wire-board-events-fleet-v1");
});

test("events are optional and support only accepted public recipes", () => {
  for (const catanScenario of [
    "",
    "cities-knights",
    "fishing",
    "rivers",
    "caravans",
    "barbarian-attack",
    "shores",
    "islands",
    "fog",
    "desert",
    "tribe",
    "pirate_islands",
    "cloth",
    "wonders",
    "new_world",
  ]) {
    const room = { kind: "catan", capacity: 3, catanScenario };
    assert.equal(catanEventsSupported(room), true);
    assert.equal(catanRuleContext(room).events, "");
    assert.equal(
      catanEventsSupported({ ...room, catanFishing: true }),
      [
        "islands",
        "fog",
        "desert",
        "tribe",
        "cloth",
        "wonders",
        "new_world",
      ].includes(catanScenario),
    );
  }
  for (const catanScenario of ["transport", "land-ho", "explorers-and-pirates"])
    assert.equal(catanEventsSupported({ kind: "catan", catanScenario }), false);
  assert.equal(catanEventsSupported({ kind: "splendor" }), false);
  assert.equal(
    catanEventsSupported({
      kind: "catan",
      catanHarbors: { enabled: false },
      catanFriendlyRobber: { enabled: false },
    }),
    true,
  );
  assert.equal(
    catanEventsSupported({
      kind: "catan",
      catanSeafarers: { scenario: "cloth" },
    }),
    true,
  );
});

test("saved event catalogue overrides waiting draft and disabling removes it", () => {
  const room = {
    kind: "catan",
    capacity: 3,
    catanEvents: CATAN_EVENT_CATALOGUE,
  };
  assert.equal(catanRuleContext(room).events, CATAN_EVENT_CATALOGUE);
  room.game = { catan: { players: Array(3).fill({}) } };
  assert.equal(catanRuleContext(room).events, "");
  room.game.catan.eventDeck = { catalogue: CATAN_EVENT_CATALOGUE };
  room.catanEvents = "";
  assert.equal(catanRuleContext(room).events, CATAN_EVENT_CATALOGUE);
});

test("event knights rules use the actual save and never inherit stale waiting options", () => {
  const room = {
    kind: "catan",
    capacity: 6,
    catanScenario: "cities-knights",
    catanCitiesKnights: {},
    catanEvents: CATAN_EVENT_CATALOGUE,
  };
  assert.equal(catanEventsSupported(room), true);
  assert.equal(
    catanRuleContext(room).eventKnights,
    "wire-board-events-knights-v1",
  );
  for (const scenario of ["shores", "islands", "fog", "desert"]) {
    assert.equal(
      catanEventsSupported({ ...room, catanSeafarers: { scenario } }),
      true,
    );
  }
  room.game = { catan: { players: Array(6).fill({}), citiesKnights: {} } };
  assert.equal(catanRuleContext(room).eventKnights, "");
  room.game.catan.eventDeck = {
    catalogue: CATAN_EVENT_CATALOGUE,
    knights: "wire-board-events-knights-v1",
  };
  room.catanEvents = "";
  assert.equal(
    catanRuleContext(room).eventKnights,
    "wire-board-events-knights-v1",
  );
});

test("event new-world selection accepts its own map only", () => {
  assert.equal(
    catanEventsSupported({
      kind: "catan",
      catanScenario: "new_world",
      catanNewWorldMap: {},
    }),
    true,
  );
  assert.equal(
    catanEventsSupported({
      kind: "catan",
      catanScenario: "cloth",
      catanNewWorldMap: {},
    }),
    false,
  );
});

test("cloth event fallback follows the saved rule, never a stale room draft", () => {
  const room = {
    kind: "catan",
    capacity: 3,
    catanScenario: "cloth",
    catanSeafarers: { scenario: "cloth", layout: "fixed" },
    catanEvents: CATAN_EVENT_CATALOGUE,
  };
  assert.equal(
    catanRuleContext(room).eventClothFallback,
    "wire-board-events-cloth-fallback-v1",
  );
  room.game = {
    catan: { players: Array(3).fill({}), seafarers: { scenario: "cloth" } },
  };
  assert.equal(catanRuleContext(room).eventClothFallback, "");
  room.game.catan.eventDeck = {
    catalogue: CATAN_EVENT_CATALOGUE,
    clothFallback: "wire-board-events-cloth-fallback-v1",
  };
  room.catanEvents = "";
  assert.equal(
    catanRuleContext(room).eventClothFallback,
    "wire-board-events-cloth-fallback-v1",
  );
});

test("events remain selectable with friendly and harbor variants; running rules ignore drafts", () => {
  for (const scenario of [
    "",
    "cities-knights",
    "shores",
    "islands",
    "fog",
    "desert",
    "cloth",
    "wonders",
    "new_world",
  ]) {
    const room = {
      kind: "catan",
      capacity: 6,
      catanScenario: scenario,
      catanEvents: CATAN_EVENT_CATALOGUE,
      catanFriendlyRobber: { enabled: true },
      catanHarbors: { enabled: true },
    };
    assert.equal(catanEventsSupported(room), true);
    assert.equal(catanRuleContext(room).friendlyRobber, true);
    assert.equal(catanRuleContext(room).harbors, true);
    room.game = {
      catan: {
        players: Array(6).fill({}),
        eventDeck: { catalogue: CATAN_EVENT_CATALOGUE },
      },
    };
    assert.equal(catanRuleContext(room).friendlyRobber, false);
    assert.equal(catanRuleContext(room).harbors, false);
    room.game.catan.friendlyRobber = { rules: "catan-friendly-robber-2025" };
    room.game.catan.harbors = { rules: "catan-harbors-2025", owner: -1 };
    room.catanFriendlyRobber.enabled = false;
    room.catanHarbors.enabled = false;
    assert.equal(catanRuleContext(room).friendlyRobber, true);
    assert.equal(catanRuleContext(room).harbors, true);
  }
});
