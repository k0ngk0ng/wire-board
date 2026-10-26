// Failure scenarios written before implementation: world growth resets scale or
// relocates old towers; drag/zoom transforms disagree with HTML hit targets;
// anchored pinch drifts; resize jumps center; pan escapes world; minimap cannot
// locate offscreen entries; hidden zero-size views cause NaN; tiny enemy sprites
// disappear when fitting a large world; drag release selects a construction pad.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  worldBounds,
  fitCamera,
  fillCamera,
  clampCamera,
  worldToScreen,
  screenToWorld,
  panCamera,
  zoomCamera,
  focusCamera,
  cameraWindow,
  dragMoved,
  isWorldOverview,
  detailCameraAt,
} from "../web/camera.ts";
const viewport = { width: 300, height: 380 };
const world = worldBounds({ width: 960, height: 640 });
const near = (a, b) => assert.ok(Math.abs(a - b) < 1e-8, `${a} != ${b}`);
test("world transforms roundtrip the same tower anchor at every zoom", () => {
  for (const zoom of [0.13, 0.3, 0.8, 1.8]) {
    const camera = { x: 480, y: 240, zoom };
    const point = { x: 140, y: 40 };
    const result = screenToWorld(
      worldToScreen(point, camera, viewport),
      camera,
      viewport,
    );
    near(result.x, point.x);
    near(result.y, point.y);
  }
});
test("fit includes sprite headroom, mobile fit stays finite even before layout", () => {
  const camera = fitCamera(world, viewport);
  const rect = cameraWindow(camera, viewport);
  assert.ok(rect.x <= world.x && rect.y <= world.y);
  assert.ok(rect.x + rect.width >= world.x + world.width);
  assert.ok(rect.y + rect.height >= world.y + world.height);
  const zero = fitCamera(world, { width: 0, height: 0 });
  assert.ok(Number.isFinite(zero.zoom) && zero.zoom > 0);
});
test("growth preserves scale and previous focus; resize only clamps boundaries", () => {
  const larger = worldBounds({ width: 1800, height: 1000 });
  const camera = { x: 600, y: 400, zoom: 0.8 };
  assert.deepEqual(clampCamera(camera, larger, viewport), camera);
  assert.deepEqual(
    clampCamera(camera, larger, { width: 380, height: 500 }),
    camera,
  );
});
test("drag is measured in pixels and never escapes the current world bounds", () => {
  const camera = { x: 600, y: 400, zoom: 0.8 };
  const pan = panCamera(camera, { x: 32, y: -16 }, world, viewport);
  near(pan.x, 560);
  near(pan.y, 420);
  for (const dx of [-1e6, 1e6]) {
    const moved = panCamera(camera, { x: dx, y: dx }, world, viewport);
    const rect = cameraWindow(moved, viewport);
    assert.ok(rect.x >= world.x && rect.y >= world.y);
    assert.ok(rect.x + rect.width <= world.x + world.width + 1e-8);
    assert.ok(rect.y + rect.height <= world.y + world.height + 1e-8);
  }
});
test("zoom keeps the pointer world anchor and clamps extreme zoom", () => {
  const larger = worldBounds({ width: 2500, height: 1800 });
  const camera = { x: 1200, y: 800, zoom: 0.6 };
  const pointer = { x: 225, y: 240 };
  const before = screenToWorld(pointer, camera, viewport);
  const after = zoomCamera(camera, 1.5, pointer, larger, viewport);
  const anchor = screenToWorld(pointer, after, viewport);
  near(before.x, anchor.x);
  near(before.y, anchor.y);
  assert.ok(zoomCamera(camera, 10000, pointer, larger, viewport).zoom <= 2);
  assert.ok(zoomCamera(camera, 0, pointer, larger, viewport).zoom > 0);
});
test("base and minimap focus use world coordinates, drag threshold suppresses accidental build selection", () => {
  const larger = worldBounds({ width: 1800, height: 1000 });
  const camera = focusCamera(
    { x: 480, y: 240, zoom: 0.8 },
    { x: 1500, y: 800 },
    larger,
    viewport,
  );
  near(camera.x, 1500);
  near(camera.y, 800);
  assert.equal(dragMoved({ x: 1, y: 1 }, { x: 3, y: 4 }), false);
  assert.equal(dragMoved({ x: 1, y: 1 }, { x: 10, y: 1 }), true);
});

test("large-world fit reserves headroom for minimum-size tower art without shifting its pad", () => {
  const bounds = worldBounds({ width: 2880, height: 1120 });
  const fit = fitCamera(bounds, viewport);
  const towerTop = 54 - ((42 / fit.zoom) * 224) / 192;
  assert.ok(towerTop >= bounds.y, `${towerTop} clipped by ${bounds.y}`);
});

// Overview failures specified before code: a fitted 3x mobile map crowds forty
// numbered hit targets and 42px tower sprites; tapping a tiny map accidentally
// selects a pad; zooming back does not restore accessible detail; starter maps
// lose the existing visible construction controls.
test("only expanded maps at small scale switch to compact overview", () => {
  const starter = { width: 960, height: 640 };
  const expanded = { width: 1920, height: 960 };
  const fit = fitCamera(worldBounds(expanded), viewport);
  assert.equal(isWorldOverview(starter, viewport, fit), false);
  assert.equal(isWorldOverview(expanded, viewport, fit), true);
  assert.equal(
    isWorldOverview(
      expanded,
      viewport,
      fitCamera(worldBounds(starter), viewport),
    ),
    false,
  );
  assert.equal(isWorldOverview(expanded, { width: 0, height: 0 }, fit), false);
});
test("overview focus restores playable detail while retaining the tapped world region", () => {
  const expanded = { width: 1920, height: 960 };
  const fit = fitCamera(worldBounds(expanded), viewport);
  const worldPoint = { x: 1500, y: 400 };
  const click = worldToScreen(worldPoint, fit, viewport);
  const next = detailCameraAt(
    fit,
    screenToWorld(click, fit, viewport),
    expanded,
    viewport,
  );
  assert.equal(isWorldOverview(expanded, viewport, next), false);
  assert.ok(next.zoom > fit.zoom);
  near(next.x, worldPoint.x);
  near(next.y, worldPoint.y);
  const alreadyDetailed = detailCameraAt(
    { x: 1500, y: 620, zoom: 1 },
    worldPoint,
    expanded,
    viewport,
  );
  assert.equal(alreadyDetailed.zoom, 1);
});

// A compact desktop battlefield is wide and short: height-only fit can leave
// the entire map visible horizontally, so a region tap cannot pan to its target.
test("wide compact desktop overview opens the selected region at a pannable scale", () => {
  const expanded = { width: 1920, height: 960 };
  const desktop = { width: 1392, height: 376 };
  const fit = fitCamera(worldBounds(expanded), desktop);
  const point = { x: 1228, y: 364 };
  const detail = detailCameraAt(fit, point, expanded, desktop);
  near(detail.x, point.x);
  near(detail.y, point.y);
  assert.ok(cameraWindow(detail, desktop).width <= 1088);
});

// Fill-view failures specified before implementation: a larger/fullscreen
// viewport still letterboxes the initial battlefield, portrait becomes a tiny
// strip, extreme/hidden sizes create invalid zoom, and fit-overview is lost.
test("fill view covers wide and portrait viewports while fit remains an overview", () => {
  for (const viewport of [{width:1904,height:1012},{width:304,height:688}]) {
    const filled=fillCamera(world,viewport),overview=fitCamera(world,viewport);
    assert.ok(filled.zoom>=overview.zoom);
    assert.ok(world.width*filled.zoom>=viewport.width-1e-8);
    assert.ok(world.height*filled.zoom>=viewport.height-1e-8);
    near(filled.x,overview.x);near(filled.y,overview.y);
  }
  for (const viewport of [{width:0,height:0},{width:99999,height:99999}]) {
    const filled=fillCamera(world,viewport);assert.ok(filled.zoom>0&&filled.zoom<=2&&Number.isFinite(filled.zoom));
  }
});
