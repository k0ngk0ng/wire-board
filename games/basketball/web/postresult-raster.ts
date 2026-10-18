/** Indexed scene artwork only. Scene logic supplies bands, tile maps and OAM;
 * this renderer neither reads the ROM nor executes game/machine instructions. */
export interface PostResultRasterAssets {
  pages: number[];
  /** RGBA pixels from the exported 512px-wide index atlas; red holds 0..3. */
  indices: Uint8ClampedArray;
  rgb: number[][];
}
export interface PostResultRasterBand {
  y0: number; y1: number;
  nametable: number;
  /** Screen-relative offsets; a lower band can reset its row with -y0. */
  scrollX: number; scrollY: number;
  backgroundCHR: number[]; spriteCHR: number[];
  mask: number;
}
export interface PostResultRasterFrame {
  /** Four already-mirrored virtual nametables, including attribute bytes. */
  nametables: number[];
  palette: number[];
  oam: number[];
  bands: PostResultRasterBand[];
}

const integer = (v: number, min: number, max: number, name: string) => {
  if (!Number.isInteger(v) || v < min || v > max) throw new Error(`Invalid scene ${name}`);
};
const mod = (n: number, d: number) => ((n % d) + d) % d;

export interface PostResultTileSegment {
  x: number; length: number; page: number; tile: number;
  row: number; column: number; palette: number; mask: number;
}
export interface PostResultRasterRows {
  palette: number[]; oam: number[];
  rows: Array<{ y: number; segments: PostResultTileSegment[]; spriteCHR: number[]; mask: number }>;
}

/** Render resolved tile-row references. Fetch timing is resolved upstream;
 * this path consumes no bus events, machine state or reference screenshots. */
export function rasterizePostResultRows(assets: PostResultRasterAssets, frame: PostResultRasterRows): Uint8ClampedArray {
  if (assets.indices.length !== assets.pages.length * 512 * 8 * 4 || assets.rgb.length !== 64 || new Set(assets.pages).size !== assets.pages.length) throw new Error("Invalid scene index atlas");
  if (frame.palette.length !== 32 || frame.oam.length !== 256 || frame.rows.length !== 240) throw new Error("Invalid scene row frame");
  frame.palette.forEach(v => integer(v, 0, 63, "palette"));
  frame.oam.forEach(v => integer(v, 0, 255, "OAM"));
  for (const rgb of assets.rgb) {
    if (rgb.length !== 3) throw new Error("Invalid scene RGB color");
    rgb.forEach(v => integer(v, 0, 255, "RGB color"));
  }
  const pages = new Map(assets.pages.map((page, index) => [page, index]));
  const pixel = (page: number, tile: number, row: number, column: number) => {
    const at = pages.get(page);
    if (at === undefined) throw new Error("Missing scene CHR page");
    const value = assets.indices[((at * 8 + row) * 512 + tile * 8 + column) * 4];
    if (value > 3) throw new Error("Invalid scene texture index");
    return value;
  };
  const output = new Uint8ClampedArray(256 * 240 * 4);
  for (let y = 0; y < 240; y++) {
    const row = frame.rows[y];
    if (row.y !== y || row.spriteCHR.length !== 4 || row.spriteCHR.some(p => !pages.has(p))) throw new Error("Invalid scene raster row");
    integer(row.mask, 0, 255, "display mask");
    const background = new Uint8Array(256), sprites: number[] = [];
    let end = 0;
    for (const segment of row.segments) {
      if (segment.x !== end || !pages.has(segment.page)) throw new Error("Invalid scene tile coverage or page");
      integer(segment.length, 1, 8, "segment length");
      integer(segment.column, 0, 8 - segment.length, "tile column");
      integer(segment.row, 0, 7, "tile row");
      integer(segment.tile, 0, 63, "tile");
      integer(segment.palette, 0, 3, "palette group");
      integer(segment.mask, 0, 255, "display mask");
      end += segment.length;
      if (end > 256) throw new Error("Scene tile row overflow");
      for (let dx = 0; dx < segment.length; dx++) {
        const x = segment.x + dx;
        if (!(segment.mask & 8) || (x < 8 && !(segment.mask & 2))) continue;
        const index = pixel(segment.page, segment.tile, segment.row, segment.column + dx);
        if (index) background[x] = segment.palette * 4 + index;
      }
    }
    if (end !== 256) throw new Error("Incomplete scene tile row");
    for (let n = 0; n < 64 && sprites.length < 8; n++) {
      const top = frame.oam[n * 4] + 1;
      if (y >= top && y < top + 8) sprites.push(n * 4);
    }
    for (let x = 0; x < 256; x++) {
      let paletteIndex = background[x];
      if ((row.mask & 16) && (x >= 8 || (row.mask & 4))) for (const at of sprites) {
        const left = frame.oam[at + 3];
        if (x < left || x >= left + 8) continue;
        const attr = frame.oam[at + 2], tile = frame.oam[at + 1], dx = x - left, dy = y - frame.oam[at] - 1;
        const index = pixel(row.spriteCHR[tile >> 6], tile & 63, attr & 128 ? 7 - dy : dy, attr & 64 ? 7 - dx : dx);
        if (!index) continue;
        if (!(attr & 32) || !paletteIndex) paletteIndex = 16 + (attr & 3) * 4 + index;
        break;
      }
      let color = frame.palette[paletteIndex];
      if (row.mask & 1) color &= 0x30;
      const rgb = assets.rgb[color], at = (y * 256 + x) * 4;
      output[at] = rgb[0]; output[at + 1] = rgb[1]; output[at + 2] = rgb[2]; output[at + 3] = 255;
    }
  }
  return output;
}

export function rasterizePostResult(assets: PostResultRasterAssets, frame: PostResultRasterFrame): Uint8ClampedArray {
  if (frame.nametables.length !== 4096) throw new Error("Invalid scene nametables");
  frame.nametables.forEach(v => integer(v, 0, 255, "nametables"));
  const rows: PostResultRasterRows["rows"] = [];
  let end = 0;
  for (const band of frame.bands) {
    if (band.y0 !== end) throw new Error("Scene bands must cover the frame in order");
    integer(band.y1, band.y0 + 1, 240, "band end");
    integer(band.nametable, 0, 3, "nametable");
    integer(band.scrollX, -65535, 65535, "horizontal scroll");
    integer(band.scrollY, -65535, 65535, "vertical scroll");
    if (band.backgroundCHR.length !== 4) throw new Error("Invalid scene background pages");
    for (let y = band.y0; y < band.y1; y++) {
      const segments: PostResultTileSegment[] = [];
      for (let x = 0; x < 256;) {
        const wx = mod(x + band.scrollX + (band.nametable & 1) * 256, 512);
        const wy = mod(y + band.scrollY + (band.nametable >> 1) * 240, 480);
        const base = (Math.floor(wy / 240) * 2 + Math.floor(wx / 256)) * 1024;
        const px = wx % 256, py = wy % 240, tx = px >> 3, ty = py >> 3;
        const tile = frame.nametables[base + ty * 32 + tx];
        const attr = frame.nametables[base + 960 + (ty >> 2) * 8 + (tx >> 2)];
        const length = Math.min(8 - (px & 7), 256 - x);
        segments.push({ x, length, page: band.backgroundCHR[tile >> 6], tile: tile & 63,
          row: py & 7, column: px & 7, palette: (attr >> (((ty & 2) << 1) + (tx & 2))) & 3, mask: band.mask });
        x += length;
      }
      rows.push({ y, segments, spriteCHR: band.spriteCHR, mask: band.mask });
    }
    end = band.y1;
  }
  if (end !== 240) throw new Error("Scene bands do not cover the frame");
  return rasterizePostResultRows(assets, { rows, palette: frame.palette, oam: frame.oam });
}
