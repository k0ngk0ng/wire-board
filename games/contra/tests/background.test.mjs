// Failure scenarios before implementation: scroll sampled a frame early;
// horizontal/240-pixel vertical wraps select the wrong screen; background
// palette quadrants/zero/CHR pages differ; disabled/left-clipped layers remain;
// behind-background priority is lost; renderer mutates authoritative buffers.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { gunzipSync } from "node:zlib";
import { createHash } from "node:crypto";
import { decodeChrPage, rasterizeSpriteOAM } from "../web/sprites.ts";
import { rasterizeBackground, composeScene } from "../web/background.ts";

const raw = readFileSync(
  new URL("./testdata/background-original.json.gz", import.meta.url),
);
const source = JSON.parse(
  readFileSync(
    new URL("./testdata/background-original-source.json", import.meta.url),
    "utf8",
  ),
);
assert.equal(
  createHash("sha256").update(raw).digest("hex"),
  source.fixtureSHA256,
);
assert.equal(source.source_is_original_execution, true);
const fixture = JSON.parse(gunzipSync(raw));
const bytes = (value) => Uint8Array.from(Buffer.from(value, "hex"));
const pages = new Map(
  Object.entries(fixture.pages).map(([id, value]) => [
    id,
    decodeChrPage(bytes(value)),
  ]),
);

test("13 original complete frames match background, sprite priority and streamed scrolling", () => {
  assert.equal(fixture.samples.length, 13);
  let checked = 0;
  for (const sample of fixture.samples) {
    assert.equal(sample.scrollWrites, 2);
    assert.equal(sample.dmaFrame, sample.emulatorFrame);
    const maps = sample.screens.map((screen) => ({
      tiles: bytes(screen.tiles),
      attributes: bytes(screen.attributes),
    }));
    const options = {
      pages,
      mapping: sample.mapping,
      palette: bytes(sample.palette),
      rgb: bytes(fixture.rgb),
      control: sample.control,
      mask: sample.mask,
    };
    const background = rasterizeBackground({
      ...options,
      screens: [maps[0], maps[1], maps[0], maps[1]],
      scrollX: sample.scrollX,
      scrollY: sample.scrollY,
    });
    const sprites = rasterizeSpriteOAM(bytes(sample.oam), options);
    const image = composeScene(background, sprites);
    const expected = Buffer.from(sample.expectedRGB, "base64");
    let differences = 0;
    const examples = [];
    for (let pixel = 0; pixel < 256 * 240; pixel++) {
      checked++;
      const actual = Array.from(image.subarray(pixel * 4, pixel * 4 + 3));
      const want = Array.from(expected.subarray(pixel * 3, pixel * 3 + 3));
      if (actual.some((v, i) => v !== want[i])) {
        differences++;
        if (examples.length < 4)
          examples.push({
            x: pixel % 256,
            y: Math.floor(pixel / 256),
            actual,
            want,
          });
      }
      assert.equal(image[pixel * 4 + 3], 255);
    }
    assert.equal(
      differences,
      0,
      `original frame ${sample.frame}: ${JSON.stringify(examples)}`,
    );
  }
  assert.equal(checked, 798720);
});

function syntheticOptions() {
  const page = new Uint8Array(4096);
  for (let tile = 0; tile < 64; tile++)
    for (let row = 0; row < 8; row++)
      for (let col = 0; col < 8; col++)
        page[(Math.floor(tile / 8) * 8 + row) * 64 + (tile % 8) * 8 + col] =
          tile % 4;
  const rgb = new Uint8Array(192);
  for (let i = 0; i < 64; i++) rgb[i * 3] = i;
  return {
    pages: new Map([["test", page]]),
    mapping: Array(8).fill("test"),
    rgb,
    palette: Uint8Array.from({ length: 32 }, (_, i) => i),
    control: 0,
    mask: 10,
    scrollX: 0,
    scrollY: 0,
    screens: [0, 1, 2, 3].map((n) => ({
      tiles: new Uint8Array(960).fill(n),
      attributes: new Uint8Array(64),
    })),
  };
}

test("horizontal and vertical screen crossings keep exact pixel anchors", () => {
  const options = syntheticOptions();
  let result = rasterizeBackground({ ...options, scrollX: 255 });
  assert.equal(result.rgba[0], 0);
  assert.equal(result.rgba[4], 1);
  result = rasterizeBackground({ ...options, scrollY: 239 });
  assert.equal(result.rgba[0], 0);
  assert.equal(result.rgba[256 * 4], 2);
  result = rasterizeBackground({
    ...options,
    control: 3,
    scrollX: 255,
    scrollY: 239,
  });
  assert.equal(result.rgba[0], 3);
  assert.equal(result.rgba[4], 2);
  assert.equal(result.rgba[256 * 4], 1);
  assert.equal(result.rgba[(256 + 1) * 4], 0);
});

test("palette quadrants, universal zero and clipping preserve sprite occlusion", () => {
  const options = syntheticOptions();
  options.screens[0].tiles.fill(1);
  options.screens[0].attributes[0] = 0xe4;
  const before = Buffer.from(options.screens[0].tiles);
  let result = rasterizeBackground(options);
  assert.equal(result.rgba[0], 1);
  assert.equal(result.rgba[16 * 4], 5);
  assert.equal(result.rgba[16 * 256 * 4], 9);
  assert.equal(result.rgba[(16 * 256 + 16) * 4], 13);
  assert.equal(result.opaque[0], 1);
  assert.deepEqual(Buffer.from(options.screens[0].tiles), before);
  result = rasterizeBackground({ ...options, mask: 8 });
  assert.equal(result.rgba[0], 0);
  assert.equal(result.opaque[0], 0);
  assert.equal(result.rgba[8 * 4], 1);
  result = rasterizeBackground({ ...options, mask: 0 });
  assert.equal(result.rgba[8 * 4], 0);
  assert.equal(result.opaque[8], 0);
});

test("composition honors behind-background pixels and does not mutate either layer", () => {
  const background = {
    rgba: new Uint8ClampedArray(256 * 240 * 4),
    opaque: new Uint8Array(256 * 240),
  };
  const sprites = {
    rgba: new Uint8ClampedArray(256 * 240 * 4),
    foreground: new Uint8Array(256 * 240),
  };
  for (let n = 0; n < 3; n++) {
    background.rgba.set([1, 2, 3, 255], n * 4);
    sprites.rgba.set([9, 8, 7, 255], n * 4);
  }
  background.opaque[0] = background.opaque[1] = 1;
  sprites.foreground[0] = 1;
  const out = composeScene(background, sprites);
  assert.deepEqual(
    Array.from(out.slice(0, 12)),
    [9, 8, 7, 255, 1, 2, 3, 255, 9, 8, 7, 255],
  );
  assert.deepEqual(Array.from(background.rgba.slice(0, 4)), [1, 2, 3, 255]);
  assert.deepEqual(Array.from(sprites.rgba.slice(4, 8)), [9, 8, 7, 255]);
  assert.throws(() =>
    composeScene({ ...background, opaque: new Uint8Array(1) }, sprites),
  );
  assert.throws(() =>
    rasterizeBackground({ ...syntheticOptions(), screens: [] }),
  );
});
