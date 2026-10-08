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
    "rivers",
    "caravans",
    "barbarian-attack",
    "shores",
    "islands",
    "fog",
    "desert",
  ]) {
    const room = { kind: "catan", capacity: 3, catanScenario };
    assert.equal(catanEventsSupported(room), true);
    assert.equal(catanRuleContext(room).events, "");
    for (const extra of [
      { catanFishing: true },
      { catanCitiesKnights: {} },
      { catanHarbors: { enabled: true } },
      { catanFriendlyRobber: { enabled: true } },
      { catanNewWorldMap: {} },
    ])
      assert.equal(catanEventsSupported({ ...room, ...extra }), false);
  }
  for (const catanScenario of [
    "transport",
    "fishing",
    "cities-knights",
    "cloth",
    "tribe",
    "wonders",
    "new_world",
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
    false,
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
