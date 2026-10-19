// The browser imports this exact catalog; catch a drifted or asymmetric map
// before the Go embed and the UI disagree.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const catalog = JSON.parse(
  readFileSync(
    fileURLToPath(new URL("../maps/catalog.json", import.meta.url)),
    "utf8",
  ),
);

test("catalog has 12 symmetric 13x13 maps with fixed base rings", () => {
  assert.equal(catalog.version, 1);
  assert.equal(catalog.maps.length, 12);
  const ids = new Set();
  for (const map of catalog.maps) {
    assert.ok(map.id && !ids.has(map.id), map.id);
    ids.add(map.id);
    assert.equal(map.cells.length, 13, map.id);
    for (const row of map.cells) assert.equal(row.length, 13, map.id);
    for (let y = 0; y < 13; y++) {
      for (let x = 0; x < 13; x++) {
        assert.equal(
          map.cells[y][x],
          map.cells[12 - y][12 - x],
          `${map.id} asymmetry ${x},${y}`,
        );
      }
    }
    assert.equal(map.cells[0][6], "0", `${map.id} top base`);
    assert.equal(map.cells[12][6], "0", `${map.id} bottom base`);
    for (const [x, y] of [
      [5, 0],
      [7, 0],
      [5, 1],
      [6, 1],
      [7, 1],
      [5, 12],
      [7, 12],
      [5, 11],
      [6, 11],
      [7, 11],
    ]) {
      assert.equal(map.cells[y][x], "1", `${map.id} ring ${x},${y}`);
    }
  }
});
