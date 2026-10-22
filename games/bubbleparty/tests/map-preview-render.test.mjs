// Failure cases recorded in MAP_PREVIEW_SCENARIOS.md before implementation.
import test from "node:test";
import assert from "node:assert/strict";
import { drawPreview } from "../web/canvas.ts";
import { loadSprites } from "../web/sprites.ts";

test("current-map preview uses game sprites and preserves exact colored tile identities", async () => {
  globalThis.Image = class {
    complete = true;
    naturalWidth = 100;
    set src(value) {
      this.url = value;
      queueMicrotask(() => this.onload?.());
    }
  };
  await loadSprites("https://preview.test");
  const calls = [];
  const ctx = new Proxy(
    {
      canvas: { width: 600, height: 520 },
      drawImage(...args) {
        calls.push(args);
      },
    },
    {
      get: (target, key) => (key in target ? target[key] : () => {}),
      set: (target, key, value) => {
        target[key] = value;
        return true;
      },
    },
  );
  const cells = Array(195).fill(0),
    tiles = Array(195).fill(0);
  for (const [index, code] of [4, 5, 6, 7, 8, 9].entries()) {
    cells[index] = code < 7 ? 1 : 2;
    tiles[index] = code;
  }
  drawPreview(ctx, cells, "community10", tiles);
  const urls = calls.map((call) => call[0]?.url);
  for (const name of [
    "TownHouseRed",
    "TownHouseBlue",
    "TownHouseYellow",
    "TownBlockRed",
    "TownBlockYellow",
    "TownBox",
  ]) {
    assert.ok(
      urls.some((url) => url.endsWith("/" + name + ".png")),
      name,
    );
  }
  assert.equal(
    urls.filter((url) => url.endsWith("/TownGround.png")).length,
    195,
  );
});
