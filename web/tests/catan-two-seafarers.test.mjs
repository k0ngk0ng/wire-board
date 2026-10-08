import test from "node:test";
import assert from "node:assert/strict";
import {
  twoCatanSeaScenarios,
  supportsTwoCatanSeafarers,
} from "../src/catan-two-seafarers.ts";
import { supportsTwoCatanHelpers } from "../src/catan-two-helpers.ts";
import { twoCatanVariantsAvailable } from "../src/catan-two-variants.ts";
import { catanRuleContext } from "../src/catan-rule-context.ts";
import {
  twoChoiceName,
  twoRetreatTargets,
  twoRetreatAction,
} from "../src/catan-two-state.ts";

test("eight two-player sea recipes expose their own target and saved rules", () => {
  const targets = {
    shores: 14,
    islands: 13,
    fog: 12,
    desert: 14,
    tribe: 13,
    cloth: 14,
    wonders: 10,
    new_world: 12,
  };
  assert.equal(twoCatanSeaScenarios.length, 8);
  for (const [scenario, target] of Object.entries(targets)) {
    const room = {
      kind: "catan",
      capacity: 2,
      catanTwoRules: "catan-for-two-2025",
      catanTwoScenario: scenario,
      catanSeafarers: { scenario },
      catanOptions: { helpers: true },
      catanHarbors: { enabled: true },
    };
    assert.equal(supportsTwoCatanSeafarers(scenario), true);
    assert.equal(supportsTwoCatanHelpers(scenario), true);
    assert.equal(twoCatanVariantsAvailable(room), true);
    assert.equal(catanRuleContext(room).target, target + 1);
    assert.equal(
      catanRuleContext(room).twoSeafarers,
      "wire-board-two-seafarers-v1",
    );
    room.game = { catan: { players: [{}, {}], two: {}, vertices: [] } };
    assert.equal(catanRuleContext(room).twoSeafarers, "");
    assert.equal(catanRuleContext(room).target, 10);
    room.game.catan.two.seafarers = "wire-board-two-seafarers-v1";
    room.game.catan.seafarers = { scenario, victoryPoints: target };
    assert.equal(catanRuleContext(room).target, target);
    assert.equal(
      catanRuleContext(room).twoSeafarers,
      "wire-board-two-seafarers-v1",
    );
  }
  for (const scenario of ["pirate_islands", "land-ho", "unknown", ""])
    assert.equal(supportsTwoCatanSeafarers(scenario), false);
});

test("neutral ship prompt follows actual server choice, including road fallback", () => {
  const g = { two: { pending: { kind: "ship" }, buildingShip: true } };
  assert.equal(twoChoiceName(g, { edge: 4, vertex: -1 }), "船只");
  g.two.buildingShip = false;
  assert.equal(twoChoiceName(g, { edge: 4, vertex: -1 }), "道路");
  assert.equal(twoChoiceName(g, { edge: -1, vertex: 4 }), "村庄");
});

test("no-desert sea retreat requires explicit server target and current authority", () => {
  const room = {
    status: "playing",
    you: 0,
    game: {
      turn: 0,
      catan: {
        players: [{}, {}],
        robber: 0,
        tiles: [{ id: 0, resource: 0 }],
        two: {
          seafarers: "wire-board-two-seafarers-v1",
          tokenWindow: true,
          tokens: [5, 5],
          cost: 2,
          retreatTiles: [-1],
        },
      },
    },
  };
  assert.deepEqual(twoRetreatTargets(room), [-1]);
  assert.deepEqual(twoRetreatAction(room, { tile: -1 }), {
    type: "catan_two_robber",
    tile: -1,
  });
  for (const mutate of [
    (r) => (r.game.catan.robber = -1),
    (r) => delete r.game.catan.two.retreatTiles,
    (r) => (r.game.catan.two.retreatTiles = [0]),
    (r) => (r.game.catan.two.seafarers = ""),
    (r) => (r.game.catan.two.spent = true),
    (r) => (r.game.catan.two.tokens[0] = 1),
    (r) => (r.seats = [{ autoPlay: true }, {}]),
    (r) => (r.spectating = true),
    (r) => (r.game.turn = 1),
    (r) => (r.game.finished = true),
    (r) => r.game.catan.tiles.push({ id: 1, resource: 5 }),
  ]) {
    const copy = structuredClone(room);
    mutate(copy);
    assert.deepEqual(twoRetreatTargets(copy), []);
    assert.equal(twoRetreatAction(copy, { tile: -1 }), null);
  }
  room.game.catan.tiles.push({ id: 1, resource: 5 });
  room.game.catan.two.retreatTiles = [1];
  assert.deepEqual(twoRetreatTargets(room), [1]);
});

test("two-player fishing sea rules follow saved state rather than stale waiting flags", () => {
  const room = {
    capacity: 2,
    catanTwoRules: "catan-for-two-2025",
    catanTwoScenario: "cloth",
    catanSeafarers: { scenario: "cloth" },
    catanFishing: true,
  };
  const marker = "wire-board-two-fishing-seafarers-v1";
  assert.equal(catanRuleContext(room).twoFishingSeafarers, marker);
  assert.equal(catanRuleContext(room).fishing, true);
  room.game = { catan: { players: [{}, {}], two: {}, vertices: [] } };
  assert.equal(catanRuleContext(room).twoFishingSeafarers, "");
  assert.equal(catanRuleContext(room).fishing, false);
  room.game.catan.fishing = { twoSea: marker };
  room.game.catan.two.seafarers = "wire-board-two-seafarers-v1";
  room.game.catan.seafarers = { scenario: "cloth", victoryPoints: 14 };
  assert.equal(catanRuleContext(room).twoFishingSeafarers, marker);
  assert.equal(catanRuleContext(room).target, 14);
});
