import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { gzipSync } from "node:zlib";
import { create, root, wasm, digest } from "./native-vm.mjs";

const reference = process.env.PAL_ORIGINAL_PIXELS === "1";
const vm = await create();
assert.equal(typeof vm.e.wb_game_over_info, "function", "native failure continuation is missing");
assert.equal(typeof vm.e.wb_palette_red_step, "function", "native original red palette observation is missing");
function base(v, slot) {
  const p = v.e.wb_raster_seed_info(slot), h = Array.from(new Int32Array(v.e.memory.buffer, p, 8));
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
function projection(v) {
  const s = Array.from(new Int32Array(v.e.memory.buffer, v.e.wb_view(), 32));
  const gp = v.e.wb_game_over_info(), g = Array.from(new Int32Array(v.e.memory.buffer, gp, 3));
  assert.equal(g[0], 1);
  assert.ok(g[1] === 1 || g[1] === 2);
  const pp = v.e.wb_palette_state(), palette = Array.from(new Int32Array(v.e.memory.buffer, pp, 12));
  const rp = v.e.wb_raster_graph(), h = Array.from(new Int32Array(v.e.memory.buffer, rp, 3));
  const raster = h[1] ? { version: h[0], root: h[2], nodes: Array.from({ length: h[1] }, (_, i) => Array.from(new Int32Array(v.e.memory.buffer, rp + 12 + i * 64, 16))) } : undefined;
  if (raster?.nodes.some(n => n[0] === 9 && n[5] === 0)) raster.base = base(v, 0);
  if (raster?.nodes.some(n => n[0] === 9 && n[5] === 1)) raster.backup = base(v, 1);
  const scene = v.e.wb_scene_ops(), count = new Int32Array(v.e.memory.buffer, scene, 1)[0], ready = v.e.wb_scene_state();
  return {
    gameOver: { stage: g[1], slot: g[2] },
    paletteEffect: { palette: palette[0], night: !!palette[1], factors: palette.slice(3, 3 + palette[2]), redStep: v.e.wb_palette_red_step() },
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
let event = vm.step("wb_run_campaign");
const initial = Array.from(new Uint32Array(vm.e.memory.buffer, vm.e.wb_view(), 32));
const pages = [], rgbaPages = [], saved = new Set(), redSteps = new Set(), messages = new Set();
let dialogueSeen = false, faded = false;
for (let i = 0; i < 512; i++) {
  const view = projection(vm);
  redSteps.add(view.paletteEffect.redStep);
  for (const message of vm.info().slice(6, 6 + vm.info()[4])) messages.add(message);
  if (event.kind === 2) dialogueSeen = true;
  if (dialogueSeen && event.kind === 1 && view.paletteEffect.factors.length) faded = true;
  const page = { label: `native-${i}`, event: { kind: event.kind, value: event.value }, view };
  if (reference) {
    const indexed = new Uint8Array(vm.e.memory.buffer, vm.e.wb_pixels(), 64000), colors = new Uint8Array(vm.e.memory.buffer, vm.e.wb_colors(), 1024), rgba = Buffer.alloc(256000);
    for (let p = 0; p < 64000; p++) {
      rgba[p * 4] = colors[indexed[p] * 4];
      rgba[p * 4 + 1] = colors[indexed[p] * 4 + 1];
      rgba[p * 4 + 2] = colors[indexed[p] * 4 + 2];
      rgba[p * 4 + 3] = 255;
    }
    page.rgbaSHA256 = digest(rgba);
    rgbaPages.push(rgba);
  }
  pages.push(page);
  const label = event.kind === 7 ? "reload-request" : event.kind === 2 ? "original-dialogue" : faded ? "fade-out" : [1, 16, 32].includes(view.paletteEffect.redStep) ? `red-${view.paletteEffect.redStep}` : "";
  if (label && !saved.has(label)) {
    const snapshot = vm.snapshot(), fresh = await create();
    fresh.restore(snapshot);
    assert.deepEqual(projection(fresh), view, "fresh failure save lost its exact original display position");
    const response = event.kind === 2 ? 1 : 0;
    const expected = vm.step("wb_run_campaign", 0, response), actual = fresh.step("wb_run_campaign", 0, response);
    assert.deepEqual(actual, expected, "fresh failure save lost native continuation");
    assert.deepEqual(projection(fresh), projection(vm));
    vm.restore(snapshot);
    saved.add(label);
  }
  assert.notEqual(event.kind, 3, "original failure became a native victory/shutdown");
  if (event.kind === 7) {
    assert.equal(view.gameOver.stage, 2);
    assert.equal(event.value, 0);
    const final = Array.from(new Uint32Array(vm.e.memory.buffer, vm.e.wb_view(), 32));
    assert.equal(final[11], initial[11], "failure changed the original wallet");
    assert.equal(final[12], initial[12], "failure changed the original random sequence");
    break;
  }
  event = vm.step("wb_run_campaign", 0, event.kind === 2 ? 1 : 0);
  assert.notEqual(i, 511, "original failure never reached its retained reload request");
}
for (let step = 1; step <= 32; step++) assert.ok(redSteps.has(step));
for (const id of [13135, 13136, 13137, 13138]) assert.ok(messages.has(id), `original failure message ${id} missing`);
assert.ok(dialogueSeen && faded && saved.size === 6);
const report = { passed: true, scope: "independent original game-over script fixture, all32 original red steps, actual original message page and fade-out, native reload request held without resetting; six full-memory fresh continuations; no actual battle loss, platform save or story acceptance", runtimeSHA256: digest(wasm), saves: [...saved], pages };
if (!reference) {
  const oracle = JSON.parse(await fs.readFile(root + ".local/pal/verification/game-over-reference.json"));
  assert.deepEqual(JSON.parse(JSON.stringify(pages)), oracle.pages.map(({ rgbaSHA256, ...p }) => p));
}
await fs.writeFile(root + `.local/pal/verification/game-over-${reference ? "reference" : "authority"}.json`, JSON.stringify(report, null, 2) + "\n");
if (reference) await fs.writeFile(root + ".local/pal/verification/game-over-reference.rgba.gz", gzipSync(Buffer.concat(rgbaPages)));
console.log(JSON.stringify({ passed: true, reference, positions: pages.length, saves: saved.size }));
