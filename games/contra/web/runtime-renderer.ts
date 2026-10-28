import { rasterizeBackground, type BackgroundLayer } from "./background";
import { rasterizeSpriteOAM, resolveSpriteRow } from "./sprites";
import {
  fetchRuntimeAsset,
  type RuntimeManifest,
  type RuntimePresentation,
} from "./runtime-bridge";

type BackgroundCache = {
  bytes: Uint8Array;
  palette: Uint8Array;
  patterns: number[];
  scrollX: number;
  scrollY: number;
  control: number;
  mask: number;
  split: boolean;
  splitScroll: number;
  layer: BackgroundLayer;
};
function sameBytes(a: ArrayLike<number>, b: ArrayLike<number>): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}

export class RuntimeRenderer {
  private pages = new Map<string, Uint8Array>();
  private rgb: Uint8Array = new Uint8Array(192);
  private context: CanvasRenderingContext2D;
  private image: ImageData;
  private backgroundCache?: BackgroundCache;
  constructor(private canvas: HTMLCanvasElement) {
    const context = canvas.getContext("2d", { alpha: false });
    if (!context) throw Error("Canvas不可用");
    this.context = context;
    canvas.width = 256;
    canvas.height = 240;
    context.imageSmoothingEnabled = false;
    this.image = context.createImageData(256, 240);
  }
  async load(
    manifest: RuntimeManifest,
    progress: (percent: number, label: string) => void,
  ) {
    const entries = Object.entries(manifest.pages);
    this.backgroundCache = undefined;
    const rgb = await fetchRuntimeAsset(manifest.rgb);
    if (rgb.length !== 192) throw Error("RGB颜色表格式无效");
    this.rgb = rgb;
    let done = 0;
    await Promise.all(
      entries.map(async ([key, asset]) => {
        const pixels = await fetchRuntimeAsset(asset);
        if (pixels.length !== 4096 || pixels.some((v) => v > 3))
          throw Error("图形页格式无效");
        this.pages.set(key, pixels);
        progress(75 + (++done / entries.length) * 24, "加载首关与开场图形");
      }),
    );
    this.backgroundCache = undefined;
  }
  draw(view: RuntimePresentation) {
    if (view.Background.length !== 2048) throw Error("背景快照长度无效");
    const options = {
      pages: this.pages,
      mapping: view.Patterns.map(String),
      palette: Uint8Array.from(view.Palette),
      rgb: this.rgb,
      control: view.Scroll.NameTable,
      mask: view.Display.AppliedMask,
      validatedPages: true,
    };
    const cached = this.backgroundCache;
    // Sprite-zero raster effects depend on OAM as well as terrain. Keep their
    // original full path; ordinary and split backgrounds can reuse exact pixels.
    const reuse =
      !view.Display.RasterActive &&
      cached &&
      cached.scrollX === view.Scroll.X &&
      cached.scrollY === view.Scroll.Y &&
      cached.control === options.control &&
      cached.mask === options.mask &&
      cached.split === view.Display.SplitActive &&
      cached.splitScroll === view.Display.SplitScroll &&
      sameBytes(cached.bytes, view.Background) &&
      sameBytes(cached.palette, view.Palette) &&
      sameBytes(cached.patterns, view.Patterns);
    let background: BackgroundLayer;
    let data: Uint8Array;
    if (reuse) {
      data = cached.bytes;
      background = cached.layer;
    } else {
      data = Uint8Array.from(view.Background);
      const first = {
        tiles: data.subarray(0, 960),
        attributes: data.subarray(960, 1024),
      };
      const second = {
        tiles: data.subarray(1024, 1984),
        attributes: data.subarray(1984, 2048),
      };
      background = rasterizeBackground({
        ...options,
        screens: [first, second, first, second],
        scrollX: view.Scroll.X,
        scrollY: view.Scroll.Y,
      });
      if (view.Display.SplitActive) {
        const shifted = rasterizeBackground({
          ...options,
          screens: [first, second, first, second],
          scrollX: view.Display.SplitScroll,
          scrollY: view.Scroll.Y,
        });
        background.rgba.set(shifted.rgba.subarray(64 * 256 * 4), 64 * 256 * 4);
        background.opaque.set(shifted.opaque.subarray(64 * 256), 64 * 256);
      }
      if (!view.Display.RasterActive) {
        this.backgroundCache = {
          bytes: data,
          palette: options.palette,
          patterns: [...view.Patterns],
          scrollX: view.Scroll.X,
          scrollY: view.Scroll.Y,
          control: options.control,
          mask: options.mask,
          split: view.Display.SplitActive,
          splitScroll: view.Display.SplitScroll,
          layer: background,
        };
      }
    }
    const oam = Uint8Array.from(view.OAM);
    const sprites = rasterizeSpriteOAM(oam, options);
    if (view.Display.RasterActive) {
      const first = {
        tiles: data.subarray(0, 960),
        attributes: data.subarray(960, 1024),
      };
      const second = {
        tiles: data.subarray(1024, 1984),
        attributes: data.subarray(1984, 2048),
      };
      // Pinned FCEUX old-PPU reference: mask applies to the whole hit row;
      // background table control begins on the following row. Keep screen Y.
      let hit = -1;
      const top = oam[0] + 1,
        height = options.control & 32 ? 16 : 8;
      if ((options.mask & 24) === 24)
        for (let y = top; y < Math.min(240, top + height) && hit < 0; y++) {
          const row = resolveSpriteRow(
              oam[1],
              y - top,
              oam[2],
              options.control,
            ),
            page = this.pages.get(options.mapping[row.slot])!;
          const at = ((row.tile >> 3) * 8 + row.row) * 64 + (row.tile & 7) * 8;
          for (let col = 0; col < 8; col++) {
            const x = oam[3] + col;
            if (x >= 255 || (x < 8 && (options.mask & 6) !== 6)) continue;
            const value = page[at + (oam[2] & 64 ? 7 - col : col)];
            if (value !== 0 && background.opaque[y * 256 + x]) {
              hit = y;
              break;
            }
          }
        }
      if (hit < 0) throw Error("过场缺少真实sprite-zero命中");
      const maskOptions = { ...options, mask: view.Display.RasterMask };
      const maskedBackground = rasterizeBackground({
        ...maskOptions,
        screens: [first, second, first, second],
        scrollX: view.Scroll.X,
        scrollY: view.Scroll.Y,
      });
      const controlledBackground = rasterizeBackground({
        ...maskOptions,
        control: view.Display.RasterControl,
        screens: [first, second, first, second],
        scrollX: view.Scroll.X,
        scrollY: view.Scroll.Y,
      });
      background.rgba.set(
        maskedBackground.rgba.subarray(hit * 256 * 4, (hit + 1) * 256 * 4),
        hit * 256 * 4,
      );
      background.opaque.set(
        maskedBackground.opaque.subarray(hit * 256, (hit + 1) * 256),
        hit * 256,
      );
      background.rgba.set(
        controlledBackground.rgba.subarray((hit + 1) * 256 * 4),
        (hit + 1) * 256 * 4,
      );
      background.opaque.set(
        controlledBackground.opaque.subarray((hit + 1) * 256),
        (hit + 1) * 256,
      );
      const maskedSprites = rasterizeSpriteOAM(oam, maskOptions);
      sprites.rgba.set(
        maskedSprites.rgba.subarray(hit * 256 * 4),
        hit * 256 * 4,
      );
      sprites.foreground.set(
        maskedSprites.foreground.subarray(hit * 256),
        hit * 256,
      );
    }
    const rgba = this.image.data;
    rgba.set(background.rgba);
    for (let pixel = 0, at = 0; pixel < 256 * 240; pixel++, at += 4) {
      if (
        sprites.rgba[at + 3] &&
        (sprites.foreground[pixel] || !background.opaque[pixel])
      ) {
        rgba[at] = sprites.rgba[at];
        rgba[at + 1] = sprites.rgba[at + 1];
        rgba[at + 2] = sprites.rgba[at + 2];
        rgba[at + 3] = sprites.rgba[at + 3];
      }
    }
    this.context.putImageData(this.image, 0, 0);
  }
}
