import test from "node:test";
import assert from "node:assert/strict";
import catalog from "../data/classic-2002.json" with { type: "json" };
import { drawBoard, visibleBoxMotions } from "../web/canvas.ts";
import { loadCharacter } from "../web/character-art.ts";
import { loadAvatar } from "../web/avatar-art.ts";
import { classicSpritePaths } from "../web/classic-terrain.ts";
const p = () => ({
  seat: 0,
  team: 1,
  character: 5,
  avatar: 1,
  x: 3,
  y: 4,
  alive: true,
  trappedUntil: 0,
  invulnerableUntil: 0,
  applied: { move: "none" },
  dir: "down",
});
const view = () => ({
  rulesVersion: 3,
  mapId: "community10",
  phase: "playing",
  you: 0,
  cells: Array(195).fill(0),
  tiles: Array(195).fill(0),
  players: [p()],
  events: [],
  items: [],
  bombs: [],
  blasts: [],
});
function context() {
  const calls = [];
  return new Proxy(
    {
      canvas: { width: 384, height: 364 },
      calls,
      globalAlpha: 1,
      drawImage(...args) {
        calls.push(args);
      },
      fillText(...a) {
        calls.push(["text", ...a]);
      },
    },
    {
      get: (o, k) => (k in o ? o[k] : () => {}),
      set: (o, k, v) => ((o[k] = v), true),
    },
  );
}
function images() {
  globalThis.Image = class {
    complete = true;
    naturalWidth = 100;
    set src(v) {
      this.url = v;
      queueMicrotask(() => this.onload?.());
    }
  };
}
const urls = (c) => c.calls.filter((a) => a[0]?.url).map((a) => a[0].url);
test("push envelope interpolates source to destination only over authoritative time", () => {
  const v = {
    boxMotions: [
      { fromX: 5, fromY: 5, x: 6, y: 5, at: 1000, endAt: 1300, objectId: 66 },
    ],
  };
  assert.deepEqual(
    visibleBoxMotions(v, 1150).map((m) => [m.x, m.y, m.toX, m.toY, m.objectId]),
    [[5.5, 5, 6, 5, 66]],
  );
  assert.equal(visibleBoxMotions(v, 999)[0].x, 5);
  assert.deepEqual(visibleBoxMotions(v, 1300), []);
  assert.deepEqual(visibleBoxMotions(v, 2000), []);
});
test("classic live drawing uses authoritative object ids and does not duplicate moving box at destination", () => {
  const v = view();
  v.mapId = "classic2002-village07";
  v.players = [];
  v.nativeObjects = Array(195).fill(-1);
  v.nativeObjects[5 * 15 + 6] = 66;
  v.cells[5 * 15 + 6] = 4;
  const atlas = Object.fromEntries(
    classicSpritePaths(v.mapId).map((path) => [
      path,
      { complete: true, naturalWidth: 100, url: path },
    ]),
  );
  v.boxMotions = [
    { fromX: 5, fromY: 5, x: 6, y: 5, at: 1000, endAt: 1300, objectId: 66 },
  ];
  const c = context(),
    d = drawBoard(c, v, new Map(), 1150, false, "", atlas);
  const boxCalls = c.calls.filter((a) => a[0]?.url?.includes("BoxGray"));
  assert.equal(boxCalls.length, 1);
  assert.equal(boxCalls[0][5], (5.5 * 40 - 1) * 0.6);
  assert.equal(d.nativeObjectsDrawn, 1);
  assert.equal(d.movingBoxes.length, 1);
  assert.ok(c.calls.filter((a) => a[0]?.url).length >= 196);
  const dead = { ...v, nativeObjects: Array(195).fill(-1), boxMotions: [] };
  const c2 = context();
  drawBoard(c2, dead, new Map(), 1400, false, "", atlas);
  assert.equal(
    c2.calls.filter((a) => a[0]?.url?.includes("BoxGray")).length,
    0,
  );
});
test("v3 uses selected original blue character while saved v2 keeps its avatar", async () => {
  images();
  await loadCharacter("", 5, 1);
  await loadAvatar("", 1, 1);
  const c = context();
  drawBoard(c, view(), new Map(), 1000, false);
  assert.ok(
    urls(c).some(
      (u) => u.includes("/v4/characters/") && u.endsWith("-blue.png"),
    ),
  );
  assert.ok(!urls(c).some((u) => u.includes("/v3/avatars/")));
  const c2 = context();
  drawBoard(c2, { ...view(), rulesVersion: 2 }, new Map(), 1000, false);
  assert.ok(
    urls(c2).some((u) => u.includes("/v3/avatars/") && u.endsWith("-blue.png")),
  );
});
test("mount item and mounted rider are actually drawn, and dismount returns character art", async () => {
  images();
  const v = view();
  v.players[0].mount = "turtle";
  v.items = [{ id: 1, type: "turtle", x: 7, y: 7 }];
  drawBoard(context(), v, new Map(), 1000, false);
  await new Promise((resolve) => setImmediate(resolve));
  const c = context(),
    d = drawBoard(c, v, new Map(), 1020, false);
  assert.ok(urls(c).some((u) => u.includes("/v4/mounts/")));
  assert.equal(d.mountedNative, 1);
  assert.equal(d.mountItemsNative, 1);
  assert.ok(!urls(c).some((u) => u.includes("/v4/characters/")));
  const c2 = context();
  drawBoard(
    c2,
    { ...v, items: [], players: [{ ...v.players[0], mount: "" }] },
    new Map(),
    1040,
    false,
  );
  assert.ok(!urls(c2).some((u) => u.includes("/v4/mounts/")));
  assert.ok(urls(c2).some((u) => u.includes("/v4/characters/")));
});
test("fully unavailable sprites retain explicit terrain/character/mount fallbacks without identity numbers", () => {
  globalThis.Image = class {
    complete = false;
    naturalWidth = 0;
    set src(v) {
      queueMicrotask(() => this.onerror?.());
    }
  };
  const v = view();
  v.mapId = "classic2002-village01";
  v.nativeObjects = Array(195).fill(-1);
  v.nativeObjects[3 * 15 + 3] = 66;
  v.players[0].mount = "owl";
  const c = context(),
    d = drawBoard(c, v, new Map(), 1000, true, "https://failed.invalid", {});
  assert.equal(d.nativeObjectsFallback, 1);
  assert.equal(d.playersFallback, 1);
  assert.equal(d.mountedFallback, 1);
  assert.ok(
    !c.calls.some((a) => a[0] === "text" && /^\d+$/.test(String(a[1]))),
  );
});
test("destroyed authoritative destination suppresses a still-retained motion envelope", () => {
  const v = view();
  v.players = [];
  v.boxMotions = [
    { fromX: 5, fromY: 5, x: 6, y: 5, at: 1000, endAt: 1300, objectId: 66 },
  ];
  assert.deepEqual(visibleBoxMotions(v, 1150), []);
  const d = drawBoard(context(), v, new Map(), 1150, false);
  assert.deepEqual(d.movingBoxes, []);
});
