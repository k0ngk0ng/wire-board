import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { gunzipSync } from "node:zlib";
import { create, root, wasm, digest } from "./native-vm.mjs";

assert.equal(process.env.PAL_ORIGINAL_PIXELS, "1");
const pan = process.env.PAL_GROUND_PAN === "1";
const out = path.join(
  root,
  ".local/pal/verification/",
  pan ? "ground-pan" : "ground-tiles",
);
await fs.mkdir(out, { recursive: true });
const inputs = [
  [
    "water",
    ".local/pal/candidate-scene-wave/solo-past-sacrifice-browser/phone-solo-queen-frozen.json",
  ],
  [
    "yuhang",
    ".local/pal/candidate-scene-wave/solo-past-sacrifice-browser/frozen-view.json",
  ],
  [
    "palace",
    ".local/pal/candidate-scene-wave/solo-past-queen-browser/initial-view.json",
  ],
];
const vm = await create();
const e = vm.e;
const put = (bytes) => {
  const p = e.wb_allocate(bytes.length);
  new Uint8Array(e.memory.buffer, p, bytes.length).set(bytes);
  return p;
};
const mapFile = e.UTIL_OpenRequiredFile(put(Buffer.from("map.mkf\0")));
const gopFile = e.UTIL_OpenRequiredFile(put(Buffer.from("gop.mkf\0")));
const mgoFile = e.UTIL_OpenRequiredFile(put(Buffer.from("mgo.mkf\0")));
const screen = new Uint32Array(e.memory.buffer, e.gpScreen.value, 1)[0];
const rect = e.wb_allocate(16);
const pages = [];
const rgbaPages = [];
const sprites = new Map();
function capture(label, file, bytes, frame) {
  const pixels = new Uint8Array(e.memory.buffer, e.wb_pixels(), 64000);
  const colors = new Uint8Array(e.memory.buffer, e.wb_colors(), 1024);
  const rgba = Buffer.alloc(256000);
  for (let i = 0; i < pixels.length; i++) {
    for (let color = 0; color < 3; color++) {
      let value = colors[pixels[i] * 4 + color];
      for (const factor of frame.paletteEffect?.factors ?? [])
        value = (value * factor) >> 6;
      rgba[i * 4 + color] = value;
    }
    rgba[i * 4 + 3] = 255;
  }
  pages.push({
    label,
    input: file,
    inputSHA256: digest(bytes),
    view: frame,
    rgbaSHA256: digest(rgba),
  });
  rgbaPages.push(rgba);
  return rgba;
}
for (const [label, file] of inputs) {
  const bytes = await fs.readFile(path.join(root, file));
  const view = JSON.parse(bytes);
  const cameras = pan
    ? [
        ...[
          [0, 0],
          [-16, -8],
          [-1, -1],
          [1, 1],
          [15, 7],
          [16, 8],
          [31, 15],
          [32, 16],
          [-32, -16],
        ].map(([x, y]) => [view.cameraX + x, view.cameraY + y]),
        [0, 0],
        [-31, -15],
        [2032, 2032],
        [2048, 2048],
      ]
    : [[view.cameraX, view.cameraY]];
  for (let camera = 0; camera < cameras.length; camera++) {
    const pageLabel = pan ? label + "-camera-" + camera : label;
    const frame = {
      sceneReady: true,
      clearColor: view.clearColor ?? 0,
      map: view.map,
      cameraX: cameras[camera][0],
      cameraY: cameras[camera][1],
      palette: view.palette,
      night: view.night,
      paletteEffect: view.paletteEffect,
      draws: [],
      overlay: [],
    };
    const map = e.PAL_LoadMap(frame.map, mapFile, gopFile);
    assert.ok(map, label);
    e.PAL_SetPalette(frame.palette, frame.night ? 1 : 0);
    new Int32Array(e.memory.buffer, rect, 4).set([
      frame.cameraX,
      frame.cameraY,
      320,
      200,
    ]);
    e.SDL_FillRect(screen, 0, frame.clearColor);
    e.PAL_MapBlitToSurface(map, screen, rect, 0);
    e.PAL_MapBlitToSurface(map, screen, rect, 1);
    const rgba = capture(pageLabel, file, bytes, frame);
    await fs.writeFile(path.join(out, pageLabel + "-original.rgba"), rgba);
    if (pan) {
      e.PAL_FreeMap(map);
      continue;
    }
    assert.ok(
      !view.draws.some((op) => op.kind === 2),
      "waved source needs an independent native phase oracle",
    );
    for (const op of view.draws) {
      let bitmap;
      if (op.kind === 0) {
        if (!sprites.has(op.a)) {
          const size = e.PAL_MKFGetDecompressedSize(op.a, mgoFile);
          assert.ok(size > 0);
          const pointer = e.wb_allocate(size);
          assert.ok(e.PAL_MKFDecompressChunk(pointer, size, op.a, mgoFile) > 0);
          sprites.set(op.a, pointer);
        }
        bitmap = e.PAL_SpriteGetFrame(sprites.get(op.a), op.b);
      } else if (op.kind === 1) {
        bitmap = e.PAL_MapGetTileBitmap(op.a, op.b, op.c, op.d, map);
      } else {
        assert.fail("unsupported original draw operation");
      }
      assert.ok(bitmap);
      const y = op.baseY - e.PAL_RLEGetHeight(bitmap);
      e.PAL_RLEBlitToSurface(
        bitmap,
        screen,
        ((y & 65535) << 16) | (op.x & 65535),
      );
    }
    const sceneFrame = { ...frame, draws: view.draws, field: view.field };
    const scene = capture(label + "-scene", file, bytes, sceneFrame);
    await fs.writeFile(path.join(out, label + "-scene-original.rgba"), scene);
    if (view.field) {
      capture(label + "-predicted-same", file, bytes, sceneFrame);
      pages[pages.length - 1].predicted = view.field;
    }
    e.PAL_FreeMap(map);
  }
}
if (pan) {
  const wavePath = path.join(
    root,
    ".local/pal/verification/scene-wave-reference.json",
  );
  const waveBytes = await fs.readFile(wavePath);
  const wave = JSON.parse(waveBytes);
  const rgba = gunzipSync(
    await fs.readFile(
      path.join(
        root,
        ".local/pal/verification/original-scene-wave-pages.rgba.gz",
      ),
    ),
  );
  assert.equal(wave.authoritySHA256, digest(wasm));
  assert.ok(wave.passed && wave.pages.length >= 90);
  assert.equal(rgba.length, wave.pages.length * 256000);
  for (let i = 0; i < wave.pages.length; i++) {
    const data = rgba.subarray(i * 256000, (i + 1) * 256000);
    assert.equal(digest(data), wave.pages[i].rgbaSHA256);
    pages.push({
      ...wave.pages[i],
      label: "wave-" + i,
      input: path.relative(root, wavePath),
      inputSHA256: digest(waveBytes),
    });
    rgbaPages.push(data);
  }
}
e.wb_fclose(mapFile);
e.wb_fclose(gopFile);
e.wb_fclose(mgoFile);
const reference = {
  scope: pan
    ? "39 map-only camera and boundary fixtures on three genuine story maps, plus unchanged original C inn wave fixtures; no claim of whole-story draw projection or user issue localization"
    : "independent pinned original C map layers and RLE actor/cover composition from genuine public draw metadata; no wave or independent draw-projection claim and no saved-game mutation",
  runtimeSHA256: digest(wasm),
  pages,
};
await fs.writeFile(
  path.join(out, "reference.json"),
  JSON.stringify(reference, null, 2) + "\n",
);
await fs.writeFile(
  path.join(out, "original-pages.rgba"),
  Buffer.concat(rgbaPages),
);
console.log(
  JSON.stringify({
    originalReferenceGenerated: true,
    pages: pages.map(({ label, view, rgbaSHA256 }) => ({
      label,
      map: view.map,
      rgbaSHA256,
    })),
  }),
);
