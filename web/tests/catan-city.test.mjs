import { test } from "node:test";
import assert from "node:assert/strict";
import {
  catanCardImage,
  catanCardSupply,
  catanCardNames,
} from "../src/catan-cards.ts";
import {
  cityDefense,
  cityDiscardLimit,
  cityImprovementReason,
  cityMetropolisSites,
  cityWallSites,
} from "../src/catan-city-state.ts";

test("commodity art cannot be confused with desert, sea or gold terrain IDs", () => {
  assert.equal(catanCardNames[5], "纸张");
  assert.equal(
    catanCardImage("cdn", 5, true),
    "cdn/catan/cities-knights/commodity-paper-v1.webp",
  );
  assert.equal(
    catanCardImage("cdn", 6),
    "cdn/catan/cities-knights/commodity-cloth-v1.webp",
  );
  assert.equal(
    catanCardImage("cdn", 7),
    "cdn/catan/cities-knights/commodity-coin-v1.webp",
  );
  assert.equal(catanCardImage("cdn", 0, true), "cdn/catan/icon-wood-v1.webp");
  assert.equal(catanCardImage("cdn", 4), "cdn/catan/resource-ore-v1.webp");
  assert.equal(catanCardImage("cdn", 8), undefined);
  assert.deepEqual(catanCardSupply(8, false), [19, 19, 19, 19, 19, 12, 12, 12]);
  assert.deepEqual(catanCardSupply(8, true), [24, 24, 24, 24, 24, 18, 18, 18]);
  assert.deepEqual(catanCardSupply(5, false), [19, 19, 19, 19, 19]);
});
function fixture() {
  return {
    vertices: [
      { id: 0, owner: 0, level: 2 },
      { id: 1, owner: 0, level: 2 },
      { id: 2, owner: 1, level: 2 },
      { id: 3, owner: 0, level: 1 },
    ],
    players: [{ resources: [0, 0, 0, 0, 0, 5, 5, 5] }, { resources: [] }],
    citiesKnights: {
      players: [{ improvements: [3, 2, 0] }, { improvements: [4, 0, 0] }],
      metropolises: [2, -1, -1],
      walls: [0, 2],
      knights: [],
    },
  };
}
test("city building choices respect walls, metropolis occupancy and required city", () => {
  const g = fixture();
  assert.deepEqual(cityWallSites(g, 0), [1]);
  assert.equal(cityDiscardLimit(g, 0), 9);
  assert.deepEqual(cityMetropolisSites(g, 0), [0, 1]);
  assert.equal(cityImprovementReason(g, 0, 0), ""); // Same level may be built without taking the opponent's metropolis.
  g.citiesKnights.metropolises = [2, 0, 1];
  assert.match(cityImprovementReason(g, 0, 0), /没有大都会/);
  g.citiesKnights.players[0].improvements[0] = 2;
  assert.equal(cityImprovementReason(g, 0, 0), "");
  g.players[0].resources[5] = 2;
  assert.match(cityImprovementReason(g, 0, 0), /3张纸张/);
  g.vertices[0].level = g.vertices[1].level = 1;
  assert.match(cityImprovementReason(g, 0, 0), /至少一座城市/);
  assert.equal(cityDiscardLimit(g, 0), 7);
});
test("only active on-board knights of remaining players contribute defense", () => {
  const g = fixture();
  g.citiesKnights.knights = [
    { owner: 0, strength: 3, active: true },
    { owner: 0, strength: 2, active: false },
    { owner: 1, strength: 2, active: true },
  ];
  assert.equal(cityDefense(g), 5);
  assert.equal(cityDefense(g, 0), 3);
  g.players[1].eliminated = true;
  assert.equal(cityDefense(g), 3);
});

test("commodity bank trades use current port rates and actual separate stock", async () => {
  const { catanBankReason } = await import("../src/catan-cards.ts");
  const hand = [0, 0, 0, 0, 0, 5, 0, 0],
    bank = [19, 19, 19, 19, 19, 7, 12, 12],
    rates = Array(8).fill(3);
  const give = [0, 0, 0, 0, 0, 3, 0, 0],
    take = [0, 0, 0, 0, 0, 0, 0, 1];
  assert.equal(catanBankReason(give, take, hand, bank, rates), "");
  give[5] = 4;
  assert.match(catanBankReason(give, take, hand, bank, rates), /3:1/);
  give[5] = 3;
  bank[7] = 0;
  assert.match(catanBankReason(give, take, hand, bank, rates), /银行/);
  bank[7] = 12;
  rates[5] = 2;
  give[5] = 2;
  assert.equal(catanBankReason(give, take, hand, bank, rates), "");
  give[5] = 4;
  assert.match(catanBankReason(give, take, hand, bank, rates), /换取2张/);
});
