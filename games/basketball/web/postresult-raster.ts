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

export function rasterizePostResult(assets: PostResultRasterAssets, frame: PostResultRasterFrame): Uint8ClampedArray {
  if (assets.indices.length !== assets.pages.length * 512 * 8 * 4 || assets.rgb.length !== 64 || new Set(assets.pages).size !== assets.pages.length) throw new Error("Invalid scene index atlas");
  for (const color of assets.rgb) {
    if (color.length !== 3) throw new Error("Invalid scene RGB color");
    color.forEach(v => integer(v, 0, 255, "RGB color"));
  }
  const array = (values: number[], length: number, max: number, name: string) => {
    if (values.length !== length) throw new Error(`Invalid scene ${name} length`);
    values.forEach(v => integer(v, 0, max, name));
  };
  array(frame.nametables, 4096, 255, "nametables");
  array(frame.palette, 32, 63, "palette");
  array(frame.oam, 256, 255, "OAM");
  const pages = new Map(assets.pages.map((page, index) => [page, index]));
  let end = 0;
  for (const band of frame.bands) {
    if (band.y0 !== end) throw new Error("Scene bands must cover the frame in order");
    integer(band.y1, band.y0 + 1, 240, "band end");
    integer(band.nametable, 0, 3, "nametable");
    integer(band.scrollX, -65535, 65535, "horizontal scroll");
    integer(band.scrollY, -65535, 65535, "vertical scroll");
    integer(band.mask, 0, 255, "display mask");
    for (const banks of [band.backgroundCHR, band.spriteCHR]) {
      array(banks, 4, 255, "CHR pages");
      if (banks.some(page => !pages.has(page))) throw new Error("Missing scene CHR page");
    }
    end = band.y1;
  }
  if (end !== 240) throw new Error("Scene bands do not cover the frame");
  const pixel = (banks: number[], tile: number, x: number, y: number) => {
    const page = pages.get(banks[tile >> 6])!;
    const value = assets.indices[((page * 8 + y) * 512 + (tile & 63) * 8 + x) * 4];
    if (value > 3) throw new Error("Invalid scene texture index");
    return value;
  };
  const output = new Uint8ClampedArray(256 * 240 * 4);
  for (const band of frame.bands) for (let y = band.y0; y < band.y1; y++) {
    const sprites: number[] = [];
    for (let n = 0; n < 64 && sprites.length < 8; n++) {
      const top = frame.oam[n * 4] + 1;
      if (y >= top && y < top + 8) sprites.push(n * 4);
    }
    for (let x = 0; x < 256; x++) {
      let background = 0, paletteIndex = 0;
      if ((band.mask & 8) && (x >= 8 || (band.mask & 2))) {
        const wx = mod(x + band.scrollX + (band.nametable & 1) * 256, 512);
        const wy = mod(y + band.scrollY + (band.nametable >> 1) * 240, 480);
        const base = (Math.floor(wy / 240) * 2 + Math.floor(wx / 256)) * 1024;
        const px = wx % 256, py = wy % 240, tx = px >> 3, ty = py >> 3;
        const tile = frame.nametables[base + ty * 32 + tx];
        background = pixel(band.backgroundCHR, tile, px & 7, py & 7);
        const attr = frame.nametables[base + 960 + (ty >> 2) * 8 + (tx >> 2)];
        const group = (attr >> (((ty & 2) << 1) + (tx & 2))) & 3;
        if (background) paletteIndex = group * 4 + background;
      }
      if ((band.mask & 16) && (x >= 8 || (band.mask & 4))) for (const at of sprites) {
        const left = frame.oam[at + 3];
        if (x < left || x >= left + 8) continue;
        const attr = frame.oam[at + 2], dx = x - left, dy = y - frame.oam[at] - 1;
        const index = pixel(band.spriteCHR, frame.oam[at + 1], attr & 64 ? 7 - dx : dx, attr & 128 ? 7 - dy : dy);
        if (!index) continue;
        if (!(attr & 32) || !background) paletteIndex = 16 + (attr & 3) * 4 + index;
        break;
      }
      let color = frame.palette[paletteIndex];
      if (band.mask & 1) color &= 0x30;
      const rgb = assets.rgb[color], at = (y * 256 + x) * 4;
      output[at] = rgb[0]; output[at + 1] = rgb[1]; output[at + 2] = rgb[2]; output[at + 3] = 255;
    }
  }
  return output;
}
