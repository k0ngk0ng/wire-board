import { test } from "node:test";
import assert from "node:assert/strict";
import {
  catanProductionNumbers,
  catanTileProducing,
} from "../src/catan-production.ts";

test("relocated desert disc produces on either number with robber blocking both", () => {
  for (const [original, moved] of [
    [12, 2],
    [2, 11],
  ]) {
    const g = {
      robber: -1,
      tiles: [{ number: original }, { number: 0 }],
      fishing: {
        map: {
          lakes: [{ tile: 1, numbers: [2, 3, 11, 12] }],
          extraNumbers: [{ tile: 0, number: moved }],
        },
      },
    };
    const before = structuredClone(g);
    assert.deepEqual(catanProductionNumbers(g, 0), [original, moved]);
    assert.deepEqual(catanProductionNumbers(g, 1), [2, 3, 11, 12]);
    for (let roll = 2; roll <= 12; roll++) {
      assert.equal(
        catanTileProducing(g, 0, roll),
        [original, moved].includes(roll),
      );
      assert.equal(
        catanTileProducing(g, 1, roll),
        [2, 3, 11, 12].includes(roll),
      );
    }
    assert.deepEqual(g, before);
    g.robber = 0;
    assert.equal(catanTileProducing(g, 0, original), false);
    assert.equal(catanTileProducing(g, 0, moved), false);
    assert.equal(catanTileProducing(g, 1, 2), true);
  }
});

test("ordinary, sea and old fishing maps need no extra number metadata", () => {
  const g = { robber: -1, tiles: [{ number: 6 }, { number: 0 }] };
  assert.deepEqual(catanProductionNumbers(g, 0), [6]);
  assert.equal(catanTileProducing(g, 0, 6), true);
  assert.deepEqual(catanProductionNumbers(g, 1), []);
  assert.equal(catanTileProducing(g, 1, 0), false);
  assert.deepEqual(catanProductionNumbers(g, 99), []);
});

test("no-lake combinations tolerate old null lake arrays", () => {
  const g = {
    robber: -1,
    tiles: [{ number: 6 }],
    fishing: { map: { lakes: null } },
  };
  assert.deepEqual(catanProductionNumbers(g, 0), [6]);
  assert.equal(catanTileProducing(g, 0, 6), true);
});

test("rivers caravans keeps each additional disc with its tile after invention", () => {
  const g = {
    robber: -1,
    tiles: [{ number: 3 }, { number: 3 }],
    rivers: { map: { doubleNumberTile: -1 } },
    caravans: {
      extraNumbers: [
        { tile: 0, number: 2 },
        { tile: 1, number: 12 },
      ],
    },
  };
  assert.deepEqual(catanProductionNumbers(g, 0), [3, 2]);
  assert.deepEqual(catanProductionNumbers(g, 1), [3, 12]);
  g.tiles[0].number = 5;
  assert.deepEqual(catanProductionNumbers(g, 0), [5, 2]);
  g.robber = 0;
  assert.equal(catanTileProducing(g, 0, 2), false);
  assert.equal(catanTileProducing(g, 1, 12), true);
});
