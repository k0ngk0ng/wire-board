import { test } from "node:test";
import assert from "node:assert/strict";
import {
  catanEventsSupported,
  CATAN_EVENT_CATALOGUE,
} from "../src/catan-event-options.ts";
import { catanRuleContext } from "../src/catan-rule-context.ts";

test("events are optional and support only accepted public recipes", () => {
  for (const catanScenario of [
    "",
    "cities-knights",
    "rivers",
    "caravans",
    "barbarian-attack",
    "shores",
    "islands",
    "fog",
    "desert",
    "cloth",
    "wonders",
    "new_world",
  ]) {
    const room = { kind: "catan", capacity: 3, catanScenario };
    assert.equal(catanEventsSupported(room), true);
    assert.equal(catanRuleContext(room).events, "");
    for (const extra of [
      { catanFishing: true },
      { catanHarbors: { enabled: true } },
      { catanFriendlyRobber: { enabled: true } },
    ])
      assert.equal(catanEventsSupported({ ...room, ...extra }), false);
  }
  for (const catanScenario of [
    "transport",
    "fishing",
    "tribe",
    "pirate_islands",
    "land-ho",
    "explorers-and-pirates",
  ])
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
