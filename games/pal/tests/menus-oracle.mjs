// Exact C raster references for real in-game menus. Rules use original inputs;
// no inventory, status, position or result is injected by the fixture.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createHash } from "node:crypto";
import { create, root, wasm } from "./native-vm.mjs";
assert.equal(process.env.PAL_REFERENCE_FONT, "1");
assert.equal(process.env.PAL_ORIGINAL_PIXELS, "1");
const vm = await create();
let event = vm.step("wb_run_campaign");
function state() {
  const e = vm.e,
    v = Array.from(new Uint32Array(e.memory.buffer, e.wb_view(), 32)),
    p = e.wb_scene_ops(),
    count = new Uint32Array(e.memory.buffer, p, 1)[0],
    ops = new Int32Array(e.memory.buffer, p + 4, count * 8);
  const draws = Array.from({ length: count }, (_, i) => {
    const p = i * 8;
    return {
      kind: ops[p],
      a: ops[p + 1],
      b: ops[p + 2],
      c: ops[p + 3],
      d: ops[p + 4],
      x: ops[p + 5],
      baseY: ops[p + 6],
      sortY: ops[p + 7],
    };
  }).sort((a, b) => a.sortY - b.sortY);
  const overlay = vm
    .ops()
    .map((op) => ({
      kind: op[0],
      source: op[1],
      id: op[2],
      frame: op[3],
      x: op[4],
      y: op[5],
      color: op[6],
      small: op[7],
    }));
  const menu = Array.from(new Int32Array(e.memory.buffer, e.wb_menu_info(), 6));
  return {
    scene: v[1],
    x: v[2],
    y: v[3],
    messages: v[15],
    mode: v[17],
    cameraX: v[23] | 0,
    cameraY: v[24] | 0,
    map: v[25],
    palette: v[26],
    night: !!v[27],
    draws,
    overlay,
    menuKind: menu[1],
    selected: menu[3],
  };
}
for (let i = 0; i < 4096; i++) {
  const v = state();
  if (event.kind === 4 && v.scene === 2 && v.mode === 0 && v.messages === 72)
    break;
  event = vm.step("wb_run_campaign", 0, event.kind === 2 ? 1 : 0);
  if (i === 4095) throw Error("original inn unavailable");
}
const pages = [],
  buffers = [];
function capture(label) {
  const v = state(),
    e = vm.e,
    indices = new Uint8Array(e.memory.buffer, e.wb_pixels(), 64000),
    palette = new Uint8Array(e.memory.buffer, e.wb_colors(), 1024),
    rgba = Buffer.alloc(256000);
  for (let i = 0; i < 64000; i++) {
    rgba[i * 4] = palette[indices[i] * 4];
    rgba[i * 4 + 1] = palette[indices[i] * 4 + 1];
    rgba[i * 4 + 2] = palette[indices[i] * 4 + 2];
    rgba[i * 4 + 3] = 255;
  }
  pages.push({
    label,
    view: v,
    rgbaSHA256: createHash("sha256").update(rgba).digest("hex"),
  });
  buffers.push(rgba);
}
const input = (key) => {
  event = vm.step("wb_run_campaign", 0, 0, key);
};
function until(kind) {
  for (let i = 0; i < 32; i++) {
    if (state().menuKind === kind) return;
    input(0);
  }
  throw Error("menu did not reach kind " + kind);
}
capture("field");
input(1);
until(1);
capture("main");
input(2);
until(2);
capture("status");
input(1);
until(0);
capture("after-status");
input(1);
until(1);
input(4);
input(4);
input(2);
until(4);
capture("inventory");
input(1);
until(0);
capture("after-inventory");
input(1);
until(1); // remembered selection is inventory; move back to magic.
input(16);
input(2);
until(10);
capture("magic");
input(1);
until(0);
capture("after-magic");
const out = root + ".local/pal/verification/";
await fs.writeFile(out + "original-menu-pages.rgba", Buffer.concat(buffers));
const report = {
  scope:
    "actual original GameMain, field/status/inventory/magic menus, default layout, original glyphs and source bitmaps; no platform completion claim",
  runtimeSHA256: createHash("sha256").update(wasm).digest("hex"),
  pages,
  view: pages[0].view,
  rgbaSHA256: pages[0].rgbaSHA256,
};
await fs.writeFile(
  out + "original-menus-reference.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log(
  "recorded original field/menu rasters",
  pages.map((p) => p.label).join(", "),
);
