import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  catanRiverImages,
  catanCoinReason,
} from "../src/catan-rivers-layout.ts";
import {
  catanProductionNumbers,
  catanTileProducing,
} from "../src/catan-production.ts";
const maps = JSON.parse(
  readFileSync(new URL("./fixtures/catan-rivers-maps.json", import.meta.url)),
);
const project = (m, x, y) => [
  m[0] * x + m[2] * y + m[4],
  m[1] * x + m[3] * y + m[5],
];

test("original river images align to all three-to-six-player hex centers without stretching or mirroring", () => {
  for (const g of maps) {
    const before = structuredClone(g),
      images = catanRiverImages(g);
    assert.equal(images.length, g.tiles.length === 19 ? 2 : 3);
    for (const image of images) {
      const m = image.matrix;
      assert.ok(m[0] * m[3] - m[1] * m[2] > 0);
      assert.equal(m[0], m[3]);
      assert.equal(m[1], -m[2]);
      image.tiles.forEach((id, index) => {
        const t = index / (image.tiles.length - 1);
        const point = project(
          m,
          image.first[0] * (1 - t) + image.last[0] * t,
          image.first[1] * (1 - t) + image.last[1] * t,
        );
        assert.ok(
          Math.hypot(point[0] - g.tiles[id].x, point[1] - g.tiles[id].y) < 0.02,
          `${image.kind} tile ${id}`,
        );
      });
    }
    const bridgeEdges = g.rivers.map.bridges.map((id) => g.edges[id]);
    assert.equal(bridgeEdges.length, g.tiles.length === 19 ? 7 : 10);
    for (const [index, channel] of g.rivers.map.channels.entries()) {
      const last = channel.tiles.at(-1),
        outlet = g.edges[channel.outlet];
      // Midpoints of the actual printed outlet sides, measured in native art.
      // This catches wrong river rotations and reflected coastal mouths.
      const image = images[index];
      const nativeMouth = {
        long: [66.385, 1164.41],
        short: [325.345, 843.74],
        extended: [284.63, 742.61],
      }[image.kind];
      const mouth = project(image.matrix, ...nativeMouth),
        a = g.vertices[outlet.a],
        b = g.vertices[outlet.b];
      assert.ok(
        Math.hypot(mouth[0] - (a.x + b.x) / 2, mouth[1] - (a.y + b.y) / 2) < 1,
        `${image.kind} outlet mismatch`,
      );
      assert.deepEqual(outlet.tiles, [last]);
      assert.ok(bridgeEdges.some((e) => e.id === outlet.id));
      for (let i = 1; i < channel.tiles.length; i++)
        assert.ok(
          bridgeEdges.some(
            (e) =>
              e.tiles.includes(channel.tiles[i - 1]) &&
              e.tiles.includes(channel.tiles[i]),
          ),
        );
    }
    assert.deepEqual(g, before);
  }
  assert.deepEqual(catanRiverImages({}), []);
});

test("Rivers combines 2/12 only on the public designated hex, with both blocked by robber", () => {
  for (const map of maps) {
    const g = structuredClone(map);
    g.robber = -1;
    for (const t of g.tiles) {
      const expected =
        t.id === g.rivers.map.doubleNumberTile
          ? [12, 2]
          : t.number
            ? [t.number]
            : [];
      assert.deepEqual(catanProductionNumbers(g, t.id), expected);
      for (const roll of [2, 12])
        assert.equal(
          catanTileProducing(g, t.id, roll),
          expected.includes(roll),
        );
    }
    const dual = g.rivers.map.doubleNumberTile;
    if (dual >= 0) {
      g.robber = dual;
      assert.equal(catanTileProducing(g, dual, 2), false);
      assert.equal(catanTileProducing(g, dual, 12), false);
    }
  }
});

test("coin controls respect private holdings, public stock, purchase cap and actual port rates", () => {
  const g = {
    rivers: { gold: [2], bank: 4, bought: 0 },
    players: [{ resources: [4, 3, 2, 1, 0], rates: [4, 3, 2, 4, 4] }],
    bank: [1, 0, 1, 1, 1],
  };
  assert.equal(catanCoinReason(g, 0, 0, true), "");
  assert.match(catanCoinReason(g, 0, 1, true), /没有/);
  for (const c of [0, 1, 2]) assert.equal(catanCoinReason(g, 0, c, false), "");
  assert.match(catanCoinReason(g, 0, 3, false), /需要4/);
  g.rivers.bought = 2;
  assert.match(catanCoinReason(g, 0, 0, true), /两张/);
  assert.equal(catanCoinReason(g, 0, 0, false), "");
  g.rivers.bought = 0;
  g.rivers.gold[0] = 1;
  assert.match(catanCoinReason(g, 0, 0, true), /2金币/);
  g.rivers.bank = 0;
  assert.match(catanCoinReason(g, 0, 0, false), /供给/);
  g.rivers.goldRule = "ledger";
  assert.equal(catanCoinReason(g, 0, 0, false), "");
  assert.match(catanCoinReason(g, 0, 3, false), /需要4/);
  assert.match(catanCoinReason(g, 0, 1, true), /2金币/);
  assert.notEqual(catanCoinReason(g, -1, 0, true), "");
  delete g.players[0].resources;
  assert.notEqual(catanCoinReason(g, 0, 0, true), "");
});
