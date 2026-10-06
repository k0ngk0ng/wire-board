import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  orientArtCell,
  orientAtlasStyle,
  orientBackStyle,
} from "../src/splendor-orient-art.ts";

const read = (path) => JSON.parse(readFileSync(new URL(path, import.meta.url)));
const catalog = read("../../internal/game/splendor_modern_catalog.json");
const proof = read("../../docs/research/splendor-bga-components.json");

test("all 30 Orient illustrations match independent BGA component image positions", () => {
  const ids = [
    209, 208, 207, 206, 210, 201, 203, 204, 202, 205, 211, 212, 213, 214, 215,
    220, 217, 219, 216, 218, 226, 227, 228, 229, 230, 224, 225, 222, 223, 221,
  ];
  assert.equal(catalog.orient.length, 30);
  for (const [index, card] of catalog.orient.entries()) {
    const cell = proof.material.orient[ids[index]].img - 1;
    assert.equal(orientArtCell(card), cell, `card ${card.id}`);
    const style = orientAtlasStyle("/assets", orientArtCell(card));
    assert.equal(
      style.backgroundPosition,
      `${(cell % 5) * 25}% ${Math.floor(cell / 5) * 25}%`,
    );
  }
});

test("paired copy cards retain their original illustration for every acquired bonus color", () => {
  for (const card of catalog.orient.filter((c) =>
    c.orient.startsWith("copy"),
  )) {
    for (let color = 0; color < 5; color++) {
      assert.equal(
        orientArtCell({ ...card, color, bonusCount: 2 }),
        orientArtCell(card),
      );
    }
  }
});

test("all three deck and hidden-flight backs share the original atlas; no-art remains supported", () => {
  for (let tier = 1; tier <= 3; tier++) {
    const style = orientBackStyle("/assets", tier);
    assert.equal(style.backgroundPosition, `${(tier - 1) * 25}% 100%`);
    assert.equal(style.backgroundSize, "500% 500%");
    assert.equal(
      style.backgroundImage,
      orientAtlasStyle("/assets", 0).backgroundImage,
    );
    assert.equal(orientBackStyle("", tier), undefined);
  }
  assert.equal(orientBackStyle("/assets", 0), undefined);
  assert.equal(orientBackStyle("/assets", 4), undefined);
  assert.equal(orientArtCell({ color: 0 }), undefined);
});
