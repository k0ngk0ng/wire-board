// Failure inventory: the atlas/fallback mapping hides a gem kind, warning
// bands are off by one, or a missing asset base produces a malformed URL.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  ATLAS_CELL,
  atlasSources,
  gemLabel,
  gemTile,
  warningClass,
} from "../web/gems.ts";
import { characterMeta, portraitSources } from "../web/characters.ts";

test("atlas rows cover all four colors and every non-power kind", () => {
  for (const color of ["r", "y", "g", "b"]) {
    for (const kind of ["normal", "crash", "counter", "power"]) {
      const tile = gemTile(kind, color);
      assert.ok(tile, `${kind}:${color}`);
      assert.equal(tile.w, ATLAS_CELL);
      assert.equal(tile.h, ATLAS_CELL);
    }
  }
  assert.deepEqual(gemTile("diamond"), { x: 0, y: 4 * ATLAS_CELL, w: 16, h: 16 });
  assert.equal(gemTile("normal", undefined), null);
});

test("asset URLs prefer the configured base and always keep a local fallback", () => {
  assert.deepEqual(atlasSources("https://cdn.example/bga/v1"), [
    "https://cdn.example/bga/v1/spf2t/v1/gems.webp",
    "/spf2t/v1/gems.webp",
  ]);
  assert.deepEqual(atlasSources(""), ["/spf2t/v1/gems.webp"]);
  assert.deepEqual(portraitSources("https://cdn.example/bga/v1", "ryu"), [
    "https://cdn.example/bga/v1/spf2t/v1/portraits/ryu.webp",
    "/spf2t/v1/portraits/ryu.webp",
  ]);
  assert.equal(characterMeta("missing").fallbackName, "未知");
});

test("warning bands and labels are stable at the boundaries", () => {
  assert.equal(warningClass(0), "is-clear");
  assert.equal(warningClass(1), "is-caution");
  assert.equal(warningClass(10), "is-caution");
  assert.equal(warningClass(11), "is-warning");
  assert.equal(warningClass(30), "is-warning");
  assert.equal(warningClass(31), "is-danger");
  assert.match(gemLabel({ kind: "counter", color: "g", timer: 3 }), /反击宝石/);
  assert.match(gemLabel({ kind: "counter", color: "g", timer: 3 }), /3 回合/);
  assert.match(gemLabel({ kind: "diamond" }), /钻石/);
});
