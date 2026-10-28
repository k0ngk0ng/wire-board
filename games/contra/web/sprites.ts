/** Pure presentation data for the recovered Japanese Contra sprite format.
 * No ROM, emulator, DOM, audio player or resource loading is required here.
 */
export type SpritePart = Readonly<{
  dx: number;
  dy: number;
  tile: number;
  attr: number;
  /** Shared component tails stop inheriting the torso recoil displacement. */
  recoil: boolean;
}>;

export type SpriteFrame = Readonly<{
  code: number;
  /** Original bank-4/5 address, retained only as source provenance. */
  address: number;
  kind: "blank" | "compact" | "regular";
  parts: readonly SpritePart[];
}>;

export type SpriteCatalog = Readonly<{
  sourceRomSHA256: string;
  anchor: "original-sprite-position";
  frames: ReadonlyMap<number, SpriteFrame>;
}>;

function record(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new TypeError("Expected a sprite descriptor object");
  return value as Record<string, unknown>;
}

function integer(value: unknown, min: number, max: number): number {
  if (
    typeof value !== "number" ||
    !Number.isInteger(value) ||
    value < min ||
    value > max
  )
    throw new RangeError(`Sprite value must be an integer in ${min}..${max}`);
  return value;
}

/** Validate normalized, source-bound descriptors produced by asset extraction.
 * Coordinates stay relative to the original logical sprite position. They are
 * not re-centered per animation frame, which would introduce visible jitter.
 */
export function parseSpriteCatalog(
  value: unknown,
  expectedRomSHA256: string,
): SpriteCatalog {
  const input = record(value);
  if (
    !/^[a-f0-9]{64}$/.test(expectedRomSHA256) ||
    input.sourceRomSHA256 !== expectedRomSHA256 ||
    input.format !== "contra-j-sprites-v1" ||
    input.anchor !== "original-sprite-position"
  )
    throw new Error("Sprite descriptor identity or coordinate format differs");
  if (
    !Array.isArray(input.frames) ||
    input.frames.length < 1 ||
    input.frames.length > 255
  )
    throw new RangeError("Invalid sprite frame count");
  const frames = new Map<number, SpriteFrame>();
  for (const entry of input.frames) {
    const frame = record(entry);
    const code = integer(frame.code, 1, 255);
    const address = integer(frame.address, 0x95d8, 0xbfff);
    if (frames.has(code)) throw new Error("Duplicate sprite code");
    if (
      frame.kind !== "blank" &&
      frame.kind !== "compact" &&
      frame.kind !== "regular"
    )
      throw new Error("Unknown sprite component format");
    if (!Array.isArray(frame.parts) || frame.parts.length > 63)
      throw new RangeError("Invalid sprite part count");
    const parts = frame.parts.map((entry): SpritePart => {
      const part = record(entry);
      if (typeof part.recoil !== "boolean")
        throw new TypeError("Invalid recoil flag");
      return Object.freeze({
        dy: integer(part.dy, -128, 127),
        tile: integer(part.tile, 0, 255),
        attr: integer(part.attr, 0, 255),
        dx: integer(part.dx, -128, 127),
        recoil: part.recoil,
      });
    });
    if (
      (frame.kind === "blank" && parts.length !== 0) ||
      (frame.kind === "regular" && parts.length === 0) ||
      (frame.kind === "compact" &&
        (parts.length !== 1 ||
          parts[0].dx !== -4 ||
          parts[0].dy !== -8 ||
          parts[0].recoil))
    )
      throw new Error("Sprite parts do not match the declared format");
    frames.set(
      code,
      Object.freeze({
        code,
        address,
        kind: frame.kind,
        parts: Object.freeze(parts),
      }),
    );
  }
  return Object.freeze({
    sourceRomSHA256: expectedRomSHA256,
    anchor: "original-sprite-position",
    frames,
  });
}

export type SpritePlacement = Readonly<{
  x: number;
  y: number;
  flags: number;
  offset: number;
  /** Original remaining-slot counter (63 means all 64 slots are available). */
  remaining: number;
  stride: number;
}>;

const signedByte = (value: number) => (value & 128 ? value - 256 : value);

/** Append one normalized frame using original $9284..$936D presentation rules.
 * The returned OAM buffer is a copy. The inner builder completes its frame even
 * if its counter becomes negative (bit 7); an outer scene builder then stops.
 * Hardware-visible output always remains exactly 64 four-byte entries.
 */
export function appendSpriteOAM(
  frame: SpriteFrame,
  placement: SpritePlacement,
  before: Uint8Array,
) {
  if (before.length !== 256)
    throw new RangeError("OAM contains exactly 64 entries");
  for (const value of [
    placement.x,
    placement.y,
    placement.flags,
    placement.offset,
    placement.remaining,
  ])
    integer(value, 0, 255);
  if (
    placement.offset % 4 !== 0 ||
    (placement.stride !== 4 && placement.stride !== 196)
  )
    throw new RangeError("Invalid OAM slot or write stride");
  const oam = new Uint8Array(before);
  let offset = placement.offset,
    remaining = placement.remaining;
  for (const part of frame.parts) {
    let y: number, x: number, attr: number;
    if (frame.kind === "compact") {
      y = (placement.y - 8) & 255;
      x = placement.x - 4;
      attr = part.attr | placement.flags;
    } else {
      let effects = placement.flags & 0xc8;
      if (!part.recoil) effects &= 0xf7;
      let relativeY = (part.dy + (effects & 8 ? 1 : 0)) & 255;
      if (effects & 128) relativeY = (0xf0 - relativeY) & 255;
      y = (relativeY + placement.y) & 255;
      const paletteMask = placement.flags & 4 ? 0xfc : 0xff;
      const override = placement.flags & (placement.flags & 4 ? 0x23 : 0x20);
      attr = ((part.attr & paletteMask) | override) ^ effects;
      let relativeX = part.dx & 255;
      if (effects & 64) relativeX = (0xf8 - relativeX) & 255;
      x = placement.x + signedByte(relativeX);
    }
    // Original writes these bytes before horizontal clipping. A rejected final
    // tile leaves a partial slot which the scene's unused-slot filler hides.
    oam[offset] = y;
    oam[offset + 1] = part.tile;
    oam[offset + 2] = attr;
    if (x >= 0 && x < 256) {
      oam[offset + 3] = x;
      remaining = (remaining - 1) & 255;
      offset = (offset + placement.stride) & 255;
    }
  }
  return { oam, offset, remaining };
}

/** Decode a 1 KiB NES bitplane page into the 64×64 indexed atlas layout.
 * Production can supply the same index pixels from the published page PNG;
 * this helper also supports source-bound extraction and oracle verification.
 */
export function decodeChrPage(bitplanes: Uint8Array): Uint8Array {
  if (bitplanes.length !== 1024)
    throw new RangeError("CHR page must contain 64 tiles");
  const pixels = new Uint8Array(4096);
  for (let tile = 0; tile < 64; tile++) {
    for (let y = 0; y < 8; y++) {
      const low = bitplanes[tile * 16 + y],
        high = bitplanes[tile * 16 + y + 8];
      for (let x = 0; x < 8; x++)
        pixels[((tile >> 3) * 8 + y) * 64 + (tile & 7) * 8 + x] =
          ((low >> (7 - x)) & 1) | (((high >> (7 - x)) & 1) << 1);
    }
  }
  return pixels;
}

/** Resolve a displayed row to a mapped CHR slot and atlas-local tile/row.
 * In 8×16 mode the tile low bit selects the pattern table; vertical flip
 * exchanges the two 8×8 tiles as well as reversing their pixel rows.
 */
export function resolveSpriteRow(
  tile: number,
  displayRow: number,
  attr: number,
  control: number,
) {
  for (const value of [tile, attr, control]) integer(value, 0, 255);
  const height = control & 32 ? 16 : 8;
  integer(displayRow, 0, height - 1);
  const row = attr & 128 ? height - 1 - displayRow : displayRow;
  const globalTile =
    height === 16
      ? (tile & 1) * 256 + (tile & 254) + (row >> 3)
      : (control & 8 ? 256 : 0) + tile;
  return { slot: globalTile >> 6, tile: globalTile & 63, row: row & 7 };
}

export type SpriteRasterOptions = Readonly<{
  pages: ReadonlyMap<string, Uint8Array>;
  mapping: readonly string[];
  palette: Uint8Array;
  /** Current source/device RGB color table (64 entries), supplied by caller. */
  rgb: Uint8Array;
  control: number;
  mask: number;
}>;

/** Produce an RGBA sprite layer and its per-pixel foreground priority.
 * Terrain composition uses rgba alpha plus foreground to respect sprites
 * behind opaque backgrounds. No result, collision or animation state changes.
 */
export function rasterizeSpriteOAM(
  oam: Uint8Array,
  options: SpriteRasterOptions,
) {
  if (
    oam.length !== 256 ||
    options.mapping.length !== 8 ||
    options.palette.length !== 32 ||
    options.rgb.length !== 192
  )
    throw new RangeError("Invalid sprite display data dimensions");
  integer(options.control, 0, 255);
  integer(options.mask, 0, 255);
  const mapped = options.mapping.map((id) => {
    const page = options.pages.get(id);
    if (!page || page.length !== 4096 || page.some((pixel) => pixel > 3))
      throw new Error("Missing or invalid indexed CHR page");
    return page;
  });
  const rgba = new Uint8ClampedArray(256 * 240 * 4);
  const foreground = new Uint8Array(256 * 240);
  if (!(options.mask & 16))
    return { rgba, foreground, width: 256, height: 240 };
  const height = options.control & 32 ? 16 : 8;
  for (let line = 0; line < 240; line++) {
    const visible: number[] = [];
    for (let slot = 0; slot < 64 && visible.length < 8; slot++) {
      const top = oam[slot * 4] + 1;
      if (top <= line && line < top + height) visible.push(slot);
    }
    // Earlier OAM entries take priority. Draw them last, preserving transparent
    // pixels rather than letting an empty pixel erase a later opaque sprite.
    for (let i = visible.length - 1; i >= 0; i--) {
      const offset = visible[i] * 4;
      const attr = oam[offset + 2],
        x = oam[offset + 3];
      const row = resolveSpriteRow(
        oam[offset + 1],
        line - oam[offset] - 1,
        attr,
        options.control,
      );
      const page = mapped[row.slot];
      const atlasRow =
        ((row.tile >> 3) * 8 + row.row) * 64 + (row.tile & 7) * 8;
      for (let col = 0; col < 8; col++) {
        const screenX = x + col;
        if (screenX >= 256 || (screenX < 8 && !(options.mask & 4))) continue;
        const value = page[atlasRow + (attr & 64 ? 7 - col : col)];
        if (value === 0) continue;
        let color = options.palette[16 + (attr & 3) * 4 + value] & 63;
        if (options.mask & 1) color &= 0x30;
        const pixel = line * 256 + screenX;
        rgba[pixel * 4] = options.rgb[color * 3];
        rgba[pixel * 4 + 1] = options.rgb[color * 3 + 1];
        rgba[pixel * 4 + 2] = options.rgb[color * 3 + 2];
        rgba[pixel * 4 + 3] = 255;
        foreground[pixel] = attr & 32 ? 0 : 1;
      }
    }
  }
  return { rgba, foreground, width: 256, height: 240 };
}
