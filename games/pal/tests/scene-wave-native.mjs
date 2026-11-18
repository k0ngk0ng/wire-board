// E2E first: map waves disappear outside combat; actors/UI are waved with the
// map; progression or phase wrapping differs from original C; prediction is
// disabled or loses the wave; raster seeds/backups lose the prior wave; saves
// restart phase; observations/probes overwrite a suspended C stack. Fixtures
// configure rendering only after a genuine new-game arrival at the inn.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { gzipSync } from "node:zlib";
import { create, root, wasm, digest } from "./native-vm.mjs";

const reference = process.env.PAL_ORIGINAL_PIXELS === "1";
const out =
  root +
  ".local/pal/verification/scene-wave-" +
  (reference ? "reference" : "authority") +
  ".json";
const vm = await create();
const state = () =>
  Array.from(new Int32Array(vm.e.memory.buffer, vm.e.wb_view(), 32));
let event = vm.step("wb_run_campaign");
for (let i = 0; i < 4096; i++) {
  if (
    event.kind === 4 &&
    state()[1] === 2 &&
    state()[17] === 0 &&
    state()[15] === 72
  )
    break;
  event = vm.step("wb_run_campaign", 0, event.kind === 2 ? 1 : 0);
  assert.notEqual(i, 4095, "original new game did not reach the inn");
}
assert.equal(
  typeof vm.e.wb_probe_scene_wave,
  "function",
  "missing field-wave rendering fixture",
);
const inn = vm.snapshot();
const service = (v, name, ...args) => {
  const old = v.e.__stack_pointer.value;
  v.e.__stack_pointer.value = v.e.wb_service_stack.value + 262144;
  try {
    return v.e[name](...args);
  } finally {
    v.e.__stack_pointer.value = old;
  }
};
function operations(v) {
  const p = v.e.wb_scene_ops(),
    count = new Int32Array(v.e.memory.buffer, p, 1)[0];
  const data = new Int32Array(v.e.memory.buffer, p + 4, count * 8);
  return Array.from({ length: count }, (_, i) => {
    const [kind, a, b, c, d, x, baseY, sortY] = data.slice(i * 8, i * 8 + 8);
    return { kind, a, b, c, d, x, baseY, sortY };
  }).sort((a, b) => a.sortY - b.sortY);
}
function base(v, slot) {
  const p = v.e.wb_raster_seed_info(slot),
    h = Array.from(new Int32Array(v.e.memory.buffer, p, 8));
  assert.equal(h[0], 1);
  const data = new Int32Array(v.e.memory.buffer, p + 32, (h[6] + h[7]) * 8);
  return {
    sceneReady: !!h[1],
    clearColor: h[2],
    map: h[3],
    cameraX: h[4],
    cameraY: h[5],
    draws: Array.from({ length: h[6] }, (_, i) => {
      const [kind, a, b, c, d, x, baseY, sortY] = data.slice(i * 8, i * 8 + 8);
      return { kind, a, b, c, d, x, baseY, sortY };
    }).sort((a, b) => a.sortY - b.sortY),
    overlay: Array.from({ length: h[7] }, (_, i) => {
      const [kind, source, id, frame, x, y, color, small] = data.slice(
        (h[6] + i) * 8,
        (h[6] + i + 1) * 8,
      );
      return { kind, source, id, frame, x, y, color, small };
    }),
  };
}
function projection(v) {
  const s = Array.from(new Int32Array(v.e.memory.buffer, v.e.wb_view(), 32));
  const p = v.e.wb_raster_graph(),
    h = Array.from(new Int32Array(v.e.memory.buffer, p, 3));
  const graph = h[1]
    ? {
        version: h[0],
        root: h[2],
        nodes: Array.from({ length: h[1] }, (_, i) =>
          Array.from(new Int32Array(v.e.memory.buffer, p + 12 + i * 64, 16)),
        ),
      }
    : undefined;
  if (graph?.nodes.some((n) => n[0] === 9 && n[5] === 0))
    graph.base = base(v, 0);
  if (graph?.nodes.some((n) => n[0] === 9 && n[5] === 1))
    graph.backup = base(v, 1);
  const ready = v.e.wb_scene_state();
  const fp = v.e.wb_field_info(),
    fh = Array.from(new Int32Array(v.e.memory.buffer, fp, 16));
  const f = fh[0]
    ? Array.from(new Int32Array(v.e.memory.buffer, fp, 365 + fh[13] * 8))
    : null;
  const field = f
    ? {
        frame: f[1] >>> 0,
        anchor: f.slice(2, 4),
        camera: f.slice(4, 6),
        offset: f.slice(6, 8),
        layer: f[8],
        direction: f[9],
        step: f[10],
        party: f[11],
        followers: f[12],
        radius: f[14],
        trail: Array.from({ length: 5 }, (_, i) =>
          f.slice(16 + i * 3, 19 + i * 3),
        ),
        checks: Array.from({ length: 5 }, (_, i) =>
          f.slice(16 + i * 3, 18 + i * 3),
        ),
        cells: Buffer.from(f.slice(31, 365)).toString("base64"),
        actors: Array.from({ length: f[13] }, (_, i) =>
          f.slice(365 + i * 8, 373 + i * 8),
        ),
      }
    : undefined;
  return {
    field,
    sceneReady: !!(ready & 1),
    clearColor: (ready >>> 8) & 255,
    map: s[25],
    cameraX: s[23],
    cameraY: s[24],
    palette: s[26],
    night: !!s[27],
    draws: operations(v),
    overlay: v.ops().map(([kind, source, id, frame, x, y, color, small]) => ({
      kind,
      source,
      id,
      frame,
      x,
      y,
      color,
      small,
    })),
    raster: graph,
  };
}
const pages = [],
  rgbaPages = [];
function capture(label) {
  const view = projection(vm),
    p = vm.e.wb_field_info();
  assert.equal(
    new Int32Array(vm.e.memory.buffer, p, 1)[0],
    1,
    "original field prediction was disabled by a visual wave",
  );
  const page = { label, view };
  if (reference) {
    const indexed = new Uint8Array(vm.e.memory.buffer, vm.e.wb_pixels(), 64000),
      colors = new Uint8Array(vm.e.memory.buffer, vm.e.wb_colors(), 1024),
      rgba = Buffer.alloc(256000);
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
  return view;
}
for (const [level, progression, frames] of [
  [0, 0, 1],
  [8, 0, 35],
  [1, 2, 5],
  [8, -2, 5],
  [254, 2, 2],
  [1, -32768, 2],
]) {
  vm.restore(inn);
  service(vm, "wb_probe_scene_wave", level, progression);
  let expected = (level + progression) & 65535;
  if (expected === 0 || expected >= 256) expected = 0;
  let previous;
  for (let i = 0; i < frames; i++) {
    const frame = capture(`${level}/${progression}/${i}`),
      wave = frame.draws.filter((d) => d.kind === 2);
    if (expected) {
      assert.equal(wave.length, 1, "original map wave is absent or duplicated");
      assert.equal(wave[0].a, expected);
      assert.ok(wave[0].b >= 0 && wave[0].b < 32);
      assert.equal(wave[0].c, progression);
      assert.equal(
        frame.draws[0].kind,
        2,
        "wave must precede actors and covering tiles",
      );
    } else assert.equal(wave.length, 0, "stopped wave left a public effect");
    if (previous) {
      const predicted = {
        label: `predicted ${level}/${progression}/${i}`,
        view: previous,
        predicted: frame.field,
      };
      if (reference) {
        predicted.rgbaSHA256 = pages.at(-1).rgbaSHA256;
        rgbaPages.push(rgbaPages.at(-1));
      }
      pages.push(predicted);
      if (level === 8 && progression === 0 && i === 2) {
        const stale = {
          label: "stale prediction uses current authority",
          view: frame,
          predicted: previous.field,
        };
        if (reference) {
          stale.rgbaSHA256 = predicted.rgbaSHA256;
          rgbaPages.push(rgbaPages.at(-1));
        }
        pages.push(stale);
        const future = {
          label: "prediction beyond three-frame horizon uses current authority",
          view: frame,
          predicted: { ...frame.field, frame: frame.field.frame + 4 },
        };
        if (reference) {
          future.rgbaSHA256 = predicted.rgbaSHA256;
          rgbaPages.push(rgbaPages.at(-1));
        }
        pages.push(future);
      }
      if (level === 8 && progression === 0 && i === 3) {
        const wrapped = {
          label: "field frame uint32 wrap preserves next wave phase",
          view: {
            ...previous,
            field: { ...previous.field, frame: 4294967295 },
          },
          predicted: { ...frame.field, frame: 0 },
        };
        if (reference) {
          wrapped.rgbaSHA256 = predicted.rgbaSHA256;
          rgbaPages.push(rgbaPages.at(-1));
        }
        pages.push(wrapped);
      }
    }
    previous = frame;
    if (i + 1 < frames) {
      service(vm, "wb_probe_field_move", i % 2 ? 0 : 2);
      expected = expected ? (expected + progression) & 65535 : 0;
      if (expected >= 256) expected = 0;
    }
  }
}
// Real original text stays above the waved map. Raster capture then retains
// that complete scene as an immutable seed, including its map-only wave.
vm.restore(inn);
service(vm, "wb_probe_scene_wave", 8, 0);
const text = Buffer.alloc(5 * 4);
[..."得到物品"].forEach((c, i) => text.writeUInt32LE(c.codePointAt(0), i * 4));
const ptr = vm.e.wb_allocate(text.length);
new Uint8Array(vm.e.memory.buffer).set(text, ptr);
service(vm, "PAL_DrawText", ptr, 16 | (24 << 16), 15, 1, 0, 0);
capture("text above waved map");
service(vm, "wb_raster_enable");
const seeded = capture("immutable waved scene seed");
assert.equal(seeded.raster.base.draws[0].kind, 2);
// A fresh native instance must continue both the script and static wave index.
const saved = vm.snapshot(),
  fresh = await create();
fresh.restore(saved);
fresh.e.wb_release();
vm.e.wb_release();
assert.deepEqual(projection(vm), projection(fresh));
for (let i = 0; i < 12; i++) {
  const a = vm.step("wb_run_campaign"),
    b = fresh.step("wb_run_campaign");
  assert.deepEqual(a, b);
  assert.deepEqual(
    projection(vm),
    projection(fresh),
    "fresh save lost original wave phase or UI",
  );
}
await fs.writeFile(
  out,
  JSON.stringify(
    {
      passed: true,
      scope:
        "genuine new-game inn, isolated original map-wave fixtures; original phase/progression, enabled public motor, map-before-actors ordering, text and immutable raster seed, fresh-instance continuation; not blood-pool campaign completion",
      authoritySHA256: digest(wasm),
      pages,
      savedContinuationFrames: 12,
    },
    null,
    2,
  ) + "\n",
);
if (reference)
  await fs.writeFile(
    root + ".local/pal/verification/original-scene-wave-pages.rgba.gz",
    gzipSync(Buffer.concat(rgbaPages)),
  );
if (!reference) {
  const oracle = JSON.parse(
    await fs.readFile(
      root + ".local/pal/verification/scene-wave-reference.json",
    ),
  );
  const normalize = (rows) =>
    JSON.parse(
      JSON.stringify(
        rows.map(({ label, view, predicted }) => ({ label, view, predicted })),
      ),
    );
  assert.deepEqual(
    normalize(pages),
    normalize(oracle.pages),
    "stripped authority wave observations differ from original pixel C",
  );
}
console.log(
  `PASS ${pages.length} original scene-wave frames, ${reference ? "pixel reference" : "stripped authority"}, fresh saved continuation`,
);
