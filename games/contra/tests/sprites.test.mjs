// Failure scenarios (written before sprites.ts): source identity/shape drift;
// shared-parts recoil reset lost; compact FE bullets treated as regular sprites;
// palette/flip/anchor errors; partial writes on X clipping or OAM cursor wrap;
// 8x16 low tile bit/vertical flip selects the wrong CHR page or tile row;
// transparent pixels, background priority, scanline limit or left clipping lost;
// caller-owned buffers mutated; a DOM/network/ROM dependency added to rendering.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { gunzipSync } from "node:zlib";
import { createHash } from "node:crypto";
import {
  parseSpriteCatalog,
  appendSpriteOAM,
  decodeChrPage,
  resolveSpriteRow,
  rasterizeSpriteOAM,
} from "../web/sprites.ts";

const raw = readFileSync(
  new URL("./testdata/sprites-original.json.gz", import.meta.url),
);
const source = JSON.parse(
  readFileSync(
    new URL("./testdata/sprites-original-source.json", import.meta.url),
    "utf8",
  ),
);
assert.equal(
  createHash("sha256").update(raw).digest("hex"),
  source.fixtureSHA256,
);
const fixture = JSON.parse(gunzipSync(raw));
const bytes = (hex) => Uint8Array.from(Buffer.from(hex, "hex"));
const catalog = parseSpriteCatalog(fixture.catalog, fixture.source.romSHA256);
const pages = new Map(
  Object.entries(fixture.pages).map(([id, hex]) => [
    id,
    decodeChrPage(bytes(hex)),
  ]),
);

test("607 original builder calls retain full OAM, shared-part recoil, flips and clipping", () => {
  const counts = { regular: 0, compact: 0 };
  const observed = new Set();
  assert.equal(fixture.calls.length, 607);
  for (const call of fixture.calls) {
    const frame = catalog.frames.get(call.code);
    const before = bytes(call.before),
      saved = before.slice();
    const result = appendSpriteOAM(frame, call, before);
    assert.deepEqual(
      result.oam,
      bytes(call.after),
      `original call ${call.index}, sprite ${call.code}`,
    );
    assert.equal(result.offset, call.afterOffset);
    assert.equal(result.remaining, call.afterRemaining);
    assert.deepEqual(before, saved, "input OAM is immutable");
    counts[frame.kind]++;
    observed.add(call.code);
  }
  assert.deepEqual(counts, { regular: 480, compact: 127 });
  assert.equal(observed.size, 14);
});

test("caller-owned typed-array views and Node buffers remain unchanged", () => {
  const call = fixture.calls[0];
  const original = Buffer.from(call.before, "hex");
  const expected = Buffer.from(original);
  const result = appendSpriteOAM(catalog.frames.get(call.code), call, original);
  assert.deepEqual(original, expected);
  result.oam.fill(0);
  assert.deepEqual(
    original,
    expected,
    "returned storage is independent of its caller",
  );
});

test("15 actual DMA frames reproduce all 13,409 original opaque foreground pixels", () => {
  let verified = 0;
  assert.equal(fixture.samples.length, 15);
  for (const sample of fixture.samples) {
    const result = rasterizeSpriteOAM(bytes(sample.oam), {
      pages,
      mapping: sample.mapping,
      palette: bytes(sample.palette),
      rgb: bytes(fixture.rgb),
      control: sample.control,
      mask: sample.mask,
    });
    const expected = Buffer.from(sample.expectedRGB, "base64");
    assert.equal(result.rgba.length, 256 * 240 * 4);
    for (let pixel = 0; pixel < 256 * 240; pixel++) {
      if (!result.foreground[pixel]) continue;
      verified++;
      assert.deepEqual(
        Array.from(result.rgba.subarray(pixel * 4, pixel * 4 + 3)),
        Array.from(expected.subarray(pixel * 3, pixel * 3 + 3)),
        `frame ${sample.frame}, pixel ${pixel}`,
      );
      assert.equal(result.rgba[pixel * 4 + 3], 255);
    }
  }
  assert.equal(verified, 13409);
});

test("catalog has stable original anchors and rejects mismatched or invalid descriptors", () => {
  assert.equal(catalog.anchor, "original-sprite-position");
  assert.equal(catalog.frames.size, 255);
  assert.equal(
    [...catalog.frames.values()].reduce(
      (n, frame) => n + frame.parts.length,
      0,
    ),
    1145,
  );
  assert.equal(catalog.frames.get(30).kind, "compact");
  assert.deepEqual(catalog.frames.get(30).parts[0], {
    dy: -8,
    tile: 14,
    attr: 2,
    dx: -4,
    recoil: false,
  });
  for (const corrupt of [
    { ...fixture.catalog, format: "unknown" },
    { ...fixture.catalog, sourceRomSHA256: "0".repeat(64) },
    { ...fixture.catalog, anchor: "bottom-left" },
    {
      ...fixture.catalog,
      frames: [...fixture.catalog.frames, fixture.catalog.frames[0]],
    },
    {
      ...fixture.catalog,
      frames: [
        {
          code: 2,
          address: 0x95d9,
          kind: "regular",
          parts: [{ dy: 0, dx: 0, tile: 256, attr: 0, recoil: true }],
        },
      ],
    },
  ])
    assert.throws(() => parseSpriteCatalog(corrupt, fixture.source.romSHA256));
  assert.throws(() => decodeChrPage(new Uint8Array(1023)));
});

// These boundaries supplement natural capture using the recorded Japanese
// builder bytes and NES 8x16 format; they do not claim more original gameplay.
test("compact sprite X clipping keeps partial writes and regular OAM consumes wrapped slots", () => {
  const initial = new Uint8Array(256).fill(0xa5);
  const input = {
    x: 3,
    y: 1,
    flags: 5,
    offset: 252,
    remaining: 0,
    stride: 196,
  };
  const compact = appendSpriteOAM(catalog.frames.get(30), input, initial);
  assert.deepEqual(Array.from(compact.oam.subarray(252)), [249, 14, 7, 0xa5]);
  assert.equal(compact.offset, 252);
  assert.equal(compact.remaining, 0);
  const two = {
    code: 250,
    address: 0x9600,
    kind: "regular",
    parts: [
      { dy: -8, tile: 0x20, attr: 0, dx: 0, recoil: true },
      { dy: 0, tile: 0x22, attr: 1, dx: 8, recoil: false },
    ],
  };
  const filled = appendSpriteOAM(
    two,
    { ...input, x: 32, y: 40, flags: 8, stride: 4 },
    initial,
  );
  assert.deepEqual(Array.from(filled.oam.subarray(252)), [33, 0x20, 8, 32]);
  assert.deepEqual(Array.from(filled.oam.subarray(0, 4)), [40, 0x22, 1, 40]);
  assert.equal(filled.offset, 4);
  assert.equal(
    filled.remaining,
    254,
    "original inner builder can exhaust capacity before outer loop stops",
  );
  assert.equal(filled.oam.length, 256);
  assert.throws(() => appendSpriteOAM(two, { ...input, offset: 255 }, initial));
});

test("8x16 tile low-bit, row boundary and vertical flip bind the correct CHR slot", () => {
  assert.deepEqual(resolveSpriteRow(0x03, 0, 0, 0xb0), {
    slot: 4,
    tile: 2,
    row: 0,
  });
  assert.deepEqual(resolveSpriteRow(0x03, 8, 0, 0xb0), {
    slot: 4,
    tile: 3,
    row: 0,
  });
  assert.deepEqual(resolveSpriteRow(0x03, 0, 128, 0xb0), {
    slot: 4,
    tile: 3,
    row: 7,
  });
  assert.deepEqual(resolveSpriteRow(0xfe, 15, 0, 0xb0), {
    slot: 3,
    tile: 63,
    row: 7,
  });
  assert.deepEqual(resolveSpriteRow(0xff, 15, 0, 0xb0), {
    slot: 7,
    tile: 63,
    row: 7,
  });
  assert.throws(() => resolveSpriteRow(0, 16, 0, 0xb0));
});

test("scanline cap, OAM priority, left clipping and background-priority mask remain explicit", () => {
  const oam = new Uint8Array(256);
  for (let i = 0; i < 64; i++) oam[i * 4] = 0xf4;
  for (let i = 0; i < 9; i++)
    oam.set([19, 0, i === 0 ? 0x20 : 0, i * 8], i * 4);
  const page = new Uint8Array(4096).fill(1);
  const palette = new Uint8Array(32);
  palette[17] = 1;
  const rgb = new Uint8Array(192);
  rgb.set([20, 40, 60], 3);
  const options = {
    pages: new Map([["page", page]]),
    mapping: Array(8).fill("page"),
    palette,
    rgb,
    control: 0xb0,
    mask: 0x1e,
  };
  const result = rasterizeSpriteOAM(oam, options);
  assert.equal(result.rgba[20 * 256 * 4 + 3], 255);
  assert.equal(
    result.foreground[20 * 256],
    0,
    "behind-background pixels need game terrain composition",
  );
  assert.equal(result.rgba[(20 * 256 + 56) * 4 + 3], 255);
  assert.equal(
    result.rgba[(20 * 256 + 64) * 4 + 3],
    0,
    "ninth sprite is omitted for that scanline",
  );
  const clipped = rasterizeSpriteOAM(oam, { ...options, mask: 0x1a });
  assert.equal(clipped.rgba[20 * 256 * 4 + 3], 0);
  const hidden = rasterizeSpriteOAM(oam, { ...options, mask: 0 });
  assert.ok(hidden.rgba.every((v) => v === 0));
  assert.throws(() => rasterizeSpriteOAM(new Uint8Array(260), options));
});
