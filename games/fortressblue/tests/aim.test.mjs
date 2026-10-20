import { test } from "node:test";
import assert from "node:assert/strict";
import {
  clamp,
  mapById,
  maps,
  sourceList,
  tankById,
  tanks,
  worldHeight,
} from "../web/types.ts";
import {
  aimFromPoint,
  carveCraters,
  decodeMask,
  pointerWorld,
  previewPath,
  stepPower,
} from "../web/aim.ts";
import serverCatalog from "../maps/catalog.json" with { type: "json" };
import clientCatalog from "../maps/client.json" with { type: "json" };

test("aim and movement controls stay inside server ranges", () => {
  assert.equal(clamp(3, 0, 90), 3);
  assert.equal(clamp(91, 0, 90), 90);
  assert.equal(clamp(20, 20, 100), 20);
  assert.equal(clamp(101, 20, 100), 100);
  assert.equal(stepPower(65, -1), 67);
  assert.equal(stepPower(99, -1), 100);
  assert.equal(stepPower(21, 1), 20);
});

test("mouse aim follows the pointer on both sides of the tank", () => {
  const tank = { x: 500, y: 700 };
  const right = aimFromPoint(tank, { x: 800, y: 500 }, 70);
  assert.equal(right.facing, 1);
  assert.ok(Math.abs(right.angle - 30.4) < 0.5, right);
  assert.deepEqual(aimFromPoint(tank, { x: 200, y: 500 }, 70), {
    angle: right.angle,
    facing: -1,
  });
  assert.deepEqual(aimFromPoint(tank, { x: 500, y: 900 }, 70), {
    angle: 0,
    facing: 1,
  });
  assert.equal(aimFromPoint(tank, { x: 500, y: 0 }, 70).angle, 70);
});

test("pointer mapping uses the rendered camera, not raw canvas pixels", () => {
  const world = pointerWorld(
    500,
    300,
    { left: 0, top: 0, width: 1000, height: 600 },
    { x: 200, y: 300, worldViewW: 1000, worldViewH: 600 },
  );
  assert.deepEqual(world, { x: 700, y: 600 });
});

test("trajectory preview stops on destructible terrain and honors wind", () => {
  const cell = 4;
  const cols = 128;
  const rows = 128;
  const mask = new Uint8Array((cols * rows + 7) / 8);
  for (let cy = 50; cy < rows; cy++) {
    for (let cx = 0; cx < cols; cx++) {
      const index = cy * cols + cx;
      mask[index >> 3] |= 1 << (7 - (index & 7));
    }
  }
  const path = previewPath({
    width: 512,
    height: 512,
    mask,
    maskCols: cols,
    maskRows: rows,
    cell,
    startX: 50,
    startY: 100,
    angle: 45,
    power: 20,
    facing: 1,
    rangeScale: 1,
    wind: 0,
  });
  assert.ok(path.length > 8);
  const impact = path[path.length - 1];
  assert.ok(impact.y >= 196 && impact.y <= 204, impact);
  assert.ok(path.slice(0, -1).every((point) => point.y < impact.y));

  const windy = previewPath({
    width: 512,
    height: 512,
    mask: new Uint8Array(mask.length),
    maskCols: cols,
    maskRows: rows,
    cell,
    startX: 50,
    startY: 100,
    angle: 45,
    power: 60,
    facing: 1,
    rangeScale: 1,
    wind: 10,
  });
  const last = windy[windy.length - 1];
  assert.ok(last.x > 300, last);
  assert.equal(decodeMask(btoa("\x80\x00")).length, 2);
});

test("craters carve the same cells the preview uses", () => {
  const bits = (mask) =>
    mask.reduce((total, value) => {
      let count = 0;
      for (let bit = 0; bit < 8; bit++) count += (value >> bit) & 1;
      return total + count;
    }, 0);
  const mask = new Uint8Array(32).fill(0xff);
  const carved = carveCraters(mask, 16, 16, 4, [{ x: 30, y: 30, r: 10 }]);
  assert.ok(bits(carved) < bits(mask), { before: bits(mask), after: bits(carved) });
});

test("asset URLs prefer CDN and always retain a same-origin fallback", () => {
  assert.deepEqual(sourceList("https://cdn.example.test/bga/v1", "/fortressblue/v2/audio/fire.mp3"), [
    "https://cdn.example.test/bga/v1/fortressblue/v2/audio/fire.mp3",
    "/fortressblue/v2/audio/fire.mp3",
  ]);
  assert.deepEqual(sourceList("", "/fortressblue/v2/audio/fire.mp3"), [
    "/fortressblue/v2/audio/fire.mp3",
  ]);
});

test("original catalog exposes all 17 maps and 14 tanks", () => {
  assert.equal(worldHeight, 960);
  assert.equal(maps.length, 17);
  assert.equal(new Set(maps.map((m) => m.id)).size, 17);
  assert.deepEqual(
    maps.map((map) => map.stage),
    Array.from({ length: 17 }, (_, index) => index + 1),
  );
  assert.equal(maps[0].name, "风车森林");
  assert.equal(tanks.length, 14);
  assert.equal(new Set(tanks.map((t) => t.id)).size, 14);
  for (const map of maps) {
    assert.ok(map.width >= 1200);
    assert.ok(map.height === worldHeight);
    assert.ok(map.thumb.startsWith("/fortressblue/v2/maps/"));
    assert.ok(map.fg.startsWith("/fortressblue/v2/maps/"));
    assert.ok(map.mask.length > 100);
    assert.ok(map.maskCols > 0 && map.maskRows > 0);
    assert.ok(mapById.get(map.id));
  }
  for (const tank of tanks) {
    assert.ok(tank.hp > 0 && tank.primary > 0 && tank.angle > 0);
    assert.ok(tank.sprites.idle && tank.bullet);
    assert.ok(tankById.get(tank.id));
  }
});

test("client collision masks mirror the server catalog", () => {
  assert.equal(clientCatalog.cell, serverCatalog.cell);
  for (const map of maps) {
    const server = serverCatalog.maps.find((item) => item.id === map.id);
    assert.ok(server, map.id);
    assert.equal(map.mask, server.mask, map.id);
    assert.equal(map.maskCols, server.maskCols, map.id);
    assert.equal(map.maskRows, server.maskRows, map.id);
  }
});
