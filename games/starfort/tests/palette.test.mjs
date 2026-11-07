// Failure scenarios specified before implementation:
// - Opaque terrain tiles hide the new colors, leaving grass, roads and water muted.
// - A road or biome boundary disappears against one of the six textured biomes.
// - Tower/enemy silhouettes disappear on grass, roads or water; asset fallbacks
//   lose their outline or gain an unrelated player-number badge.
// - Healthy/critical health fills blend into their track at a small viewport.
// - Canvas labels lose contrast when the terrain becomes more saturated.
// - A whole-canvas color filter changes labels, effects and semantic colors.
// Browser acceptance must still inspect the rendered image at desktop/320px,
// including assets, an occupied tower, enemies, river and full-screen resize.
import { test } from "node:test";
import assert from "node:assert/strict";
import { battlefieldPalette as palette } from "../web/palette.ts";

const rgb = (color) =>
  color
    .slice(1)
    .match(/../g)
    .map((value) => parseInt(value, 16) / 255);
const luminance = (color) =>
  rgb(color)
    .map((value) =>
      value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4,
    )
    .reduce(
      (sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index],
      0,
    );
const contrast = (a, b) => {
  const values = [luminance(a), luminance(b)].sort((x, y) => x - y);
  return (values[1] + 0.05) / (values[0] + 0.05);
};
const readable = (foreground, background, minimum, message) => {
  const ratio = contrast(foreground, background);
  assert.ok(ratio >= minimum, `${message}: ${ratio.toFixed(2)} < ${minimum}`);
};

test("terrain, paths and water have distinct green, brown and blue identities", () => {
  const [grassR, grassG, grassB] = rgb(palette.terrain.forest);
  assert.ok(grassG > grassR * 1.2 && grassG > grassB * 1.4);
  const [pathR, pathG, pathB] = rgb(palette.road.fill);
  assert.ok(pathR > pathG * 1.15 && pathG > pathB * 1.25);
  const [waterR, waterG, waterB] = rgb(palette.water.fill);
  assert.ok(waterB > waterR * 1.5 && waterG > waterR * 1.4);
  assert.ok(palette.textureOpacity > 0 && palette.textureOpacity <= 0.4);
});

test("terrain and path edges separate every biome and playable surface", () => {
  for (const [biome, color] of Object.entries(palette.terrain)) {
    readable(palette.terrainEdge, color, 3, `${biome} boundary`);
    readable(palette.unitOutline, color, 3, `${biome} unit silhouette`);
    readable(palette.road.edge, color, 3, `${biome} road edge`);
  }
  for (const color of [
    palette.road.fill,
    palette.road.center,
    palette.water.fill,
  ])
    readable(palette.unitOutline, color, 3, "unit silhouette on path/water");
  readable(palette.road.edge, palette.road.fill, 3, "road interior boundary");
  readable(palette.water.edge, palette.water.fill, 3, "water bank");
  readable(palette.pad.edge, palette.pad.fill, 3, "empty construction pad");
});

test("health states remain legible against their track and labels retain a halo", () => {
  readable(palette.health.healthy, palette.health.track, 4.5, "healthy health");
  readable(
    palette.health.critical,
    palette.health.track,
    4.5,
    "critical health",
  );
  readable(palette.label.ink, palette.label.halo, 4.5, "map label");
  readable(palette.label.danger, palette.label.halo, 4.5, "warning label");
});
