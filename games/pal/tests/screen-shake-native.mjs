import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { gzipSync } from "node:zlib";
import { create, root, wasm, digest } from "./native-vm.mjs";

const reference = process.env.PAL_ORIGINAL_PIXELS === "1";
const vm = await create();
const state = () => Array.from(new Int32Array(vm.e.memory.buffer, vm.e.wb_view(), 32));
let event = vm.step("wb_run_campaign");
for (let i = 0; i < 4096; i++) {
  if (event.kind === 4 && state()[1] === 2 && state()[17] === 0 && state()[15] === 72) break;
  event = vm.step("wb_run_campaign", 0, event.kind === 2 ? 1 : 0);
  assert.notEqual(i, 4095, "genuine new game did not reach the inn");
}
assert.equal(typeof vm.e.wb_presentation_pixels, "function", "original physical screen presentation is missing");
function service(v, name, ...args) {
  const old = v.e.__stack_pointer.value;
  v.e.__stack_pointer.value = v.e.wb_service_stack.value + 262144;
  try {
    return v.e[name](...args);
  } finally {
    v.e.__stack_pointer.value = old;
  }
}
function base(v, slot) {
  const p = v.e.wb_raster_seed_info(slot);
  const h = Array.from(new Int32Array(v.e.memory.buffer, p, 8));
  const data = new Int32Array(v.e.memory.buffer, p + 32, (h[6] + h[7]) * 8);
  return {
    sceneReady: !!h[1], clearColor: h[2], map: h[3], cameraX: h[4], cameraY: h[5],
    draws: Array.from({ length: h[6] }, (_, i) => {
      const [kind, a, b, c, d, x, baseY, sortY] = data.slice(i * 8, i * 8 + 8);
      return { kind, a, b, c, d, x, baseY, sortY };
    }).sort((a, b) => a.sortY - b.sortY),
    overlay: Array.from({ length: h[7] }, (_, i) => {
      const [kind, source, id, frame, x, y, color, small] = data.slice((h[6] + i) * 8, (h[6] + i + 1) * 8);
      return { kind, source, id, frame, x, y, color, small };
    }),
  };
}
function view(v) {
  const s = Array.from(new Int32Array(v.e.memory.buffer, v.e.wb_view(), 32));
  const p = v.e.wb_raster_graph(), h = Array.from(new Int32Array(v.e.memory.buffer, p, 3));
  const raster = h[1] ? { version: h[0], root: h[2], nodes: Array.from({ length: h[1] }, (_, i) => Array.from(new Int32Array(v.e.memory.buffer, p + 12 + i * 64, 16))) } : undefined;
  if (raster?.nodes.some(n => n[0] === 9 && n[5] === 0)) raster.base = base(v, 0);
  if (raster?.nodes.some(n => n[0] === 9 && n[5] === 1)) raster.backup = base(v, 1);
  const ready = v.e.wb_scene_state();
  const scene = v.e.wb_scene_ops(), count = new Int32Array(v.e.memory.buffer, scene, 1)[0];
  return {
    sceneReady: !!(ready & 1), clearColor: (ready >>> 8) & 255,
    map: s[25], cameraX: s[23], cameraY: s[24], palette: s[26], night: !!s[27],
    draws: Array.from({ length: count }, (_, i) => {
      const [kind, a, b, c, d, x, baseY, sortY] = new Int32Array(v.e.memory.buffer, scene + 4 + i * 32, 8);
      return { kind, a, b, c, d, x, baseY, sortY };
    }).sort((a, b) => a.sortY - b.sortY),
    overlay: v.ops().map(([kind, source, id, frame, x, y, color, small]) => ({ kind, source, id, frame, x, y, color, small })),
    raster,
  };
}
const logical = v => Buffer.from(new Uint8Array(v.e.memory.buffer, v.e.wb_pixels(), 64000));
const physical = v => Buffer.from(new Uint8Array(v.e.memory.buffer, v.e.wb_presentation_pixels(), 64000));
const remaining = v => new Uint16Array(v.e.memory.buffer, v.e.g_wShakeTime.value, 1)[0];
const pages = [], rgbaPages = [];
function capture(label, active = vm) {
  const page = { label, remaining: remaining(active), view: view(active) };
  if (reference) {
    const indexed = physical(active), colors = new Uint8Array(active.e.memory.buffer, active.e.wb_colors(), 1024), rgba = Buffer.alloc(256000);
    for (let i = 0; i < 64000; i++) {
      rgba[i * 4] = colors[indexed[i] * 4];
      rgba[i * 4 + 1] = colors[indexed[i] * 4 + 1];
      rgba[i * 4 + 2] = colors[indexed[i] * 4 + 2];
      rgba[i * 4 + 3] = 255;
    }
    page.rgbaSHA256 = digest(rgba);
    rgbaPages.push(rgba);
  }
  pages.push(page);
  return page;
}
const inn = vm.snapshot();
let saves = 0;
for (const [time, level, wave] of [[4, 3, 0], [3, 9, 8], [2, 0, 0], [0, 7, 0], [2, 199, 0]]) {
  vm.restore(inn);
  service(vm, "wb_probe_scene_wave", wave, 0);
  service(vm, "VIDEO_ShakeScreen", time, level);
  const source = logical(vm);
  for (let i = 0; i < Math.max(time, 1); i++) {
    const before = remaining(vm);
    service(vm, "VIDEO_UpdateScreen", 0);
    assert.equal(remaining(vm), Math.max(0, before - 1));
    assert.deepEqual(logical(vm), source, "presentation changed the logical scene");
    if (reference) {
      const expected = Buffer.alloc(64000);
      if (!before || !level) source.copy(expected);
      else if (before & 1) source.copy(expected, 0, level * 320);
      else source.copy(expected, level * 320, 0, (200 - level) * 320);
      assert.deepEqual(physical(vm), expected, "original shake parity, edge fill or last frame changed");
    }
    capture(`time-${time}-level-${level}-wave-${wave}-${i}`);
    if (i === 0 && time > 1) {
      const snapshot = vm.snapshot(), fresh = await create();
      fresh.restore(snapshot);
      assert.deepEqual(view(fresh), view(vm), "fresh native save lost the presented graph");
      assert.equal(remaining(fresh), remaining(vm));
      service(fresh, "wb_release");
      service(fresh, "VIDEO_UpdateScreen", 0);
      service(vm, "VIDEO_UpdateScreen", 0);
      assert.deepEqual(view(fresh), view(vm), "fresh continuation lost the next presented frame");
      if (reference) assert.deepEqual(physical(fresh), physical(vm));
      vm.restore(snapshot);
      saves++;
    }
  }
  service(vm, "VIDEO_UpdateScreen", 0);
  if (reference) assert.deepEqual(physical(vm), source, "stopped original shake did not return to logical pixels");
  capture(`stopped-${time}-${level}`);
}
vm.restore(inn);
service(vm, "wb_probe_scene_wave", 0, 0);
service(vm, "VIDEO_ShakeScreen", 4, 5);
service(vm, "VIDEO_UpdateScreen", 0);
const beforePartial = physical(vm), beforeTime = remaining(vm), source = logical(vm);
const rect = service(vm, "wb_allocate", 16);
new Int32Array(vm.e.memory.buffer, rect, 4).set([8, 40, 80, 64]);
service(vm, "VIDEO_UpdateScreen", rect);
assert.equal(remaining(vm), beforeTime, "partial original update consumed a shake count");
if (reference) {
  const expected = Buffer.from(beforePartial);
  for (let y = 40; y < 104; y++) source.copy(expected, y * 320 + 8, y * 320 + 8, y * 320 + 88);
  assert.deepEqual(physical(vm), expected, "partial update lost the surrounding presented frame");
}
capture("partial-update");
const partial = vm.snapshot(), fresh = await create();
fresh.restore(partial);
assert.deepEqual(view(fresh), view(vm));
if (reference) assert.deepEqual(physical(fresh), physical(vm));
saves++;
for (let i = 0; i < 6000; i++) {
  service(vm, "VIDEO_ShakeScreen", 2, 5);
  service(vm, "VIDEO_UpdateScreen", 0);
  assert.ok(view(vm).raster.nodes.length < 32, "fully covered display history accumulated");
}
capture("bounded-display-history");
const fading = await create();
assert.equal(typeof fading.e.wb_probe_presentation_fade, "function", "original physical fade fixture is missing");
let fadeEvent = fading.step("wb_probe_presentation_fade");
const fadeLogical = logical(fading);
let fadeFrames = 0;
while (fadeEvent.kind !== 3) {
  assert.ok(fadeFrames < 128, "original fade never completed");
  assert.deepEqual(logical(fading), fadeLogical, "physical fade changed its source logical screen");
  capture(`original-fade-${fadeFrames}`, fading);
  if (fadeFrames === 0 || fadeFrames === 10) {
    const saved = fading.snapshot(), resumed = await create();
    resumed.restore(saved);
    assert.deepEqual(view(resumed), view(fading));
    if (reference) assert.deepEqual(physical(resumed), physical(fading));
    const expected = fading.step("wb_probe_presentation_fade"), actual = resumed.step("wb_probe_presentation_fade");
    assert.deepEqual(actual, expected);
    assert.deepEqual(view(resumed), view(fading));
    if (reference) assert.deepEqual(physical(resumed), physical(fading));
    fading.restore(saved);
    saves++;
  }
  fadeEvent = fading.step("wb_probe_presentation_fade");
  fadeFrames++;
}
assert.ok(fadeFrames >= 64);
if (reference) assert.deepEqual(physical(fading), fadeLogical, "original fade final presentation differs from its target");
capture("original-fade-finished", fading);
const report = { passed: true, scope: "isolated original VIDEO_UpdateScreen presentation after genuine new-game inn arrival; whole-screen parity/fill/stop, wave order, logical pixels unchanged, partial-update preservation, fresh saves and bounded history; not real-story shake or campaign acceptance", runtimeSHA256: digest(wasm), sourceVideoSHA256: digest(await fs.readFile(root + "games/pal/native/upstream/video.c")), saves, pages };
if (!reference) {
  const oracle = JSON.parse(await fs.readFile(root + ".local/pal/verification/screen-shake-reference.json"));
  assert.deepEqual(JSON.parse(JSON.stringify(pages)), oracle.pages.map(({ rgbaSHA256, ...page }) => page), "authority presentation graph differs from original C reference");
}
await fs.writeFile(root + `.local/pal/verification/screen-shake-${reference ? "reference" : "authority"}.json`, JSON.stringify(report, null, 2) + "\n");
if (reference) await fs.writeFile(root + ".local/pal/verification/screen-shake-reference.rgba.gz", gzipSync(Buffer.concat(rgbaPages)));
console.log(JSON.stringify({ passed: true, reference, pages: pages.length, saves }));
