// Pre-implementation failures: per-frame CHR rescans/ImageData allocations;
// stale cached pixels after in-place map/palette/CHR/scroll/mask changes;
// split/raster/scanline/priority output differs; resource reload keeps old RGB;
// a trusted renderer path weakens standalone raster validation.
import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const out = path.join(root, ".local/contra-runtime-perf-review/cache-test");
mkdirSync(out, { recursive: true });
const built = await build({
  stdin: {
    contents:
      'export {RuntimeRenderer} from "./games/contra/web/runtime-renderer";export {rasterizeSpriteOAM} from "./games/contra/web/sprites";',
    resolveDir: root,
    loader: "ts",
  },
  bundle: true,
  write: false,
  platform: "node",
  format: "cjs",
});
const compiled = path.join(out, "renderer.cjs");
writeFileSync(compiled, built.outputFiles[0].contents);
const { RuntimeRenderer, rasterizeSpriteOAM } = createRequire(import.meta.url)(
  compiled,
);
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");

async function scene() {
  let allocations = 0,
    drawn;
  const context = {
    imageSmoothingEnabled: false,
    createImageData: () => {
      allocations++;
      return { data: new Uint8ClampedArray(256 * 240 * 4) };
    },
    putImageData: (image) => {
      drawn = image.data;
    },
  };
  const renderer = new RuntimeRenderer({
    width: 0,
    height: 0,
    getContext: () => context,
  });
  async function load(variant = 0, invalidPage = false, invalidRGB = false) {
    const files = new Map(),
      pages = {};
    const asset = (url, bytes) => {
      files.set(url, bytes);
      return { url, bytes: bytes.length, sha256: hash(bytes) };
    };
    for (let page = 0; page < 8; page++) {
      const pixels = new Uint8Array(4096);
      for (let i = 0; i < pixels.length; i++)
        pixels[i] = (Math.floor(i / 64) + i + page + variant) % 4;
      if (invalidPage && page === 0) pixels[42] = 4;
      pages[page] = asset("/page" + page, pixels);
    }
    const rgb = Uint8Array.from(
      { length: invalidRGB ? 191 : 192 },
      (_, i) => (i * 37 + variant * 11) % 256,
    );
    const manifest = { pages, rgb: asset("/rgb", rgb) };
    const original = globalThis.fetch;
    globalThis.fetch = async (url) => new Response(files.get(url));
    try {
      await renderer.load(manifest, () => {});
    } finally {
      globalThis.fetch = original;
    }
  }
  await load();
  const background = Array.from({ length: 2048 }, (_, i) =>
    i < 960 || (i >= 1024 && i < 1984)
      ? (i + (i >= 1024 ? 67 : 0)) % 256
      : 0xe4,
  );
  const oam = new Array(256).fill(255);
  for (let i = 0; i < 10; i++)
    oam.splice(i * 4, 4, 35, i * 7 + 2, i % 2 ? 0x20 : 0x40, 8 + i * 12);
  const view = {
    Background: background,
    OAM: oam,
    Patterns: [0, 1, 2, 3, 4, 5, 6, 7],
    Palette: Array.from({ length: 32 }, (_, i) => i),
    Scroll: { X: 0, Y: 0, NameTable: 0x20 },
    Display: {
      AppliedMask: 0x1e,
      SplitActive: false,
      SplitScroll: 0,
      RasterActive: false,
      RasterMask: 0x0e,
      RasterControl: 0x21,
    },
  };
  return {
    renderer,
    view,
    load,
    allocations: () => allocations,
    pixels: () => drawn,
  };
}

test("rendered bytes survive map, palette, CHR, scroll, split, raster and resource changes", async () => {
  const f = await scene(),
    v = f.view,
    observed = [];
  const draw = () => {
    f.renderer.draw(v);
    observed.push(hash(f.pixels()));
  };
  draw();
  draw();
  v.Scroll.X = 233;
  draw();
  v.Scroll.Y = 236;
  draw();
  v.Scroll.X = v.Scroll.Y = 0;
  v.Palette[1] = 17;
  draw();
  v.Patterns[0] = 7;
  draw();
  v.Background[37] = 64;
  draw();
  v.OAM[3] = 30;
  draw();
  v.Scroll.NameTable = 0x01;
  draw();
  v.Scroll.NameTable = 0x30;
  draw();
  v.Scroll.NameTable = 0x20;
  draw();
  v.Display.AppliedMask = 0x18;
  draw();
  v.Display.SplitActive = true;
  v.Display.SplitScroll = 132;
  draw();
  v.Display.SplitScroll = 12;
  draw();
  v.Display.RasterActive = true;
  draw();
  v.Display.RasterControl ^= 1;
  v.OAM[0] = 45;
  draw();
  v.Display.RasterActive = false;
  v.Display.SplitActive = false;
  draw();
  await f.load(1);
  draw();
  writeFileSync(
    path.join(out, "observed-pixels.json"),
    JSON.stringify(observed, null, 2) + "\n",
  );
  const expected = [
    "5e98fe837e8465d84eb40857044bd61cf1b59f05a904b3c868954a30636aed16",
    "5e98fe837e8465d84eb40857044bd61cf1b59f05a904b3c868954a30636aed16",
    "2570402a9b047b01674775068e05cde1a429d3fd0bd345be01aa0f6e61915fa9",
    "3ce51f9739692357e2a0933b4f7501d5c8f260d8820988c98d1929d7439bdb87",
    "312f44970e7287099ca80e56bc4c569af75d33a48a73c7c5e838c7ff405a74f4",
    "b0750d24c12a671f2f5b7034a6421701c8341924119594a357393148f00690a1",
    "c1bac59e3630368aa01a1cc48bb1bef80eda467f94d4eef8ff79ef6ce5f062b7",
    "636a9c7ef75d30e55cb448d406e15343bd1c4657b48b76dd410b1ae17c52c20c",
    "43c1090cc63eeb3eaa50a7e766fe833c39063a9f367b5319d75446186b246d2f",
    "407d447f1e4596d93b2ec613765dc7bcaf51b730c6e34e4b61cb62dd9eb51322",
    "636a9c7ef75d30e55cb448d406e15343bd1c4657b48b76dd410b1ae17c52c20c",
    "9c507f0d15adc0f1bd09d39cc00836482e5237cf07c83752d7bf92db933d9f66",
    "847de0b5394fe8b0bf6e6fa66b7cf7b3d4e273f6e5234cd97a7e56a1ff8a4e8c",
    "9475459c117c98b7ef55e282528f4b48cf75f5c2ec54a0e6c689168eb942baa2",
    "9002a5ffa867e4e20280cae6f674b9885da62028173ef16e00a765c8e32b1fb5",
    "6991c7671fcbe14594f2c1e88ed251008c581d018b6b1c9faf786b38d1845901",
    "3ac36a9d4f4a5e391f1acf7743010093252a2522eb1ad2258de5b696ee79baaa",
    "a467e3850015179aead7d5b391d621b283cb42c50adba07f67e4f327c9a92099",
  ];
  assert.deepEqual(observed, expected);
});

test("validated renderer resources and ImageData are reused across frames", async () => {
  const f = await scene();
  let scans = 0;
  const original = Uint8Array.prototype.some;
  Uint8Array.prototype.some = function (...args) {
    if (this.length === 4096) scans++;
    return original.apply(this, args);
  };
  try {
    for (let i = 0; i < 10; i++) {
      f.view.OAM[3] = i + 8;
      f.renderer.draw(f.view);
    }
  } finally {
    Uint8Array.prototype.some = original;
  }
  assert.equal(
    f.allocations(),
    1,
    "ImageData must be allocated once per renderer",
  );
  assert.equal(scans, 0, "page pixels were already checked when loading");
});

test("invalid resources fail during load and standalone sprite callers still validate", async () => {
  const f = await scene();
  await assert.rejects(f.load(0, true), /图形|page|format/i);
  await assert.rejects(f.load(0, false, true), /色|RGB|palette|format/i);
  const page = new Uint8Array(4096);
  page[42] = 4;
  assert.throws(
    () =>
      rasterizeSpriteOAM(new Uint8Array(256), {
        pages: new Map([["0", page]]),
        mapping: Array(8).fill("0"),
        palette: new Uint8Array(32),
        rgb: new Uint8Array(192),
        control: 0,
        mask: 0x1e,
      }),
    /invalid|CHR/,
  );
});
