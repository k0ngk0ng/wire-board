import type { SpriteRasterOptions } from "./sprites.ts";

export type BackgroundScreen = Readonly<{
  /** 32 columns × 30 rows of 8×8 pattern tiles. */
  tiles: Uint8Array;
  /** Four 16×16 palette quadrants per byte, eight bytes per attribute row. */
  attributes: Uint8Array;
}>;

export type BackgroundRasterOptions = SpriteRasterOptions &
  Readonly<{
    /** Logical screens at top-left, top-right, bottom-left and bottom-right.
     * Repeated references express mirroring without copying or mutating maps.
     */
    screens: readonly BackgroundScreen[];
    scrollX: number;
    scrollY: number;
  }>;

export type BackgroundLayer = Readonly<{
  rgba: Uint8ClampedArray;
  /** Nonzero pattern pixels, used by sprites drawn behind terrain. */
  opaque: Uint8Array;
}>;

/** Draw normalized tiles, palette attributes and indexed pattern pages. This
 * performs presentation only: no CPU, mapper, collision or game-state updates.
 * The scene owner supplies the map/scroll state that belongs to this frame.
 */
export function rasterizeBackground(
  options: BackgroundRasterOptions,
): BackgroundLayer {
  if (
    options.screens.length !== 4 ||
    options.mapping.length !== 8 ||
    options.palette.length !== 32 ||
    options.rgb.length !== 192
  )
    throw new RangeError("Invalid background display dimensions");
  for (const screen of options.screens)
    if (screen.tiles.length !== 960 || screen.attributes.length !== 64)
      throw new RangeError(
        "Background screen must contain 32×30 tiles and 64 attributes",
      );
  for (const value of [
    options.scrollX,
    options.scrollY,
    options.control,
    options.mask,
  ])
    if (!Number.isInteger(value) || value < 0 || value > 255)
      throw new RangeError("Invalid background view value");
  if (options.scrollY >= 240)
    throw new RangeError(
      "Background vertical origin must be within a 240-pixel screen",
    );
  const mapped = options.mapping.map((id) => {
    const page = options.pages.get(id);
    if (!page || page.length !== 4096)
      throw new Error("Missing indexed background page");
    return page;
  });
  const rgba = new Uint8ClampedArray(256 * 240 * 4);
  const opaque = new Uint8Array(256 * 240);
  const colorMask = options.mask & 1 ? 0x30 : 0x3f;
  const patternBase = options.control & 16 ? 4 : 0;
  const originX = options.control & 1;
  const originY = (options.control >> 1) & 1;
  for (let y = 0; y < 240; y++) {
    const sourceY = y + options.scrollY;
    const pixelY = sourceY % 240;
    const tileY = pixelY >> 3;
    const screenY = originY ^ (sourceY >= 240 ? 1 : 0);
    for (let x = 0; x < 256; x++) {
      const pixel = y * 256 + x;
      let color = options.palette[0] & colorMask;
      if (options.mask & 8 && (x >= 8 || options.mask & 2)) {
        const sourceX = x + options.scrollX;
        const pixelX = sourceX & 255;
        const tileX = pixelX >> 3;
        const screenX = originX ^ (sourceX >= 256 ? 1 : 0);
        const screen = options.screens[screenY * 2 + screenX];
        const tile = screen.tiles[tileY * 32 + tileX];
        const localTile = tile & 63;
        const page = mapped[patternBase + (tile >> 6)];
        const pattern =
          page[
            ((localTile >> 3) * 8 + (pixelY & 7)) * 64 +
              (localTile & 7) * 8 +
              (pixelX & 7)
          ];
        if (pattern) {
          const attribute = screen.attributes[(tileY >> 2) * 8 + (tileX >> 2)];
          const palette = (attribute >> (((tileY & 2) << 1) | (tileX & 2))) & 3;
          color = options.palette[palette * 4 + pattern] & colorMask;
          opaque[pixel] = 1;
        }
      }
      const output = pixel * 4;
      rgba[output] = options.rgb[color * 3];
      rgba[output + 1] = options.rgb[color * 3 + 1];
      rgba[output + 2] = options.rgb[color * 3 + 2];
      rgba[output + 3] = 255;
    }
  }
  return { rgba, opaque };
}

/** Combine the two original presentation layers, retaining opaque-background
 * priority for sprites such as players in water. Both input buffers are kept.
 */
export function composeScene(
  background: BackgroundLayer,
  sprites: Readonly<{
    rgba: Uint8ClampedArray;
    foreground: Uint8Array;
  }>,
): Uint8ClampedArray {
  const pixels = 256 * 240;
  if (
    background.rgba.length !== pixels * 4 ||
    sprites.rgba.length !== pixels * 4 ||
    background.opaque.length !== pixels ||
    sprites.foreground.length !== pixels
  )
    throw new RangeError("Scene layers have inconsistent dimensions");
  const image = new Uint8ClampedArray(background.rgba);
  for (let pixel = 0; pixel < pixels; pixel++) {
    const at = pixel * 4;
    if (
      sprites.rgba[at + 3] &&
      (sprites.foreground[pixel] || !background.opaque[pixel])
    )
      image.set(sprites.rgba.subarray(at, at + 4), at);
  }
  return image;
}
