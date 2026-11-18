import { applyRNGFrame } from "./rng";
import { applyScreenWave } from "./screen-wave";
import type { SceneDraw, UICommand } from "./scene-renderer";
export type RasterBase = {
  sceneReady: boolean;
  clearColor: number;
  map: number;
  cameraX: number;
  cameraY: number;
  draws: SceneDraw[];
  overlay: UICommand[];
};
export type RasterGraph = {
  version: number;
  root: number;
  nodes: number[][];
  base?: RasterBase;
  backup?: RasterBase;
};
export type RasterAsset = {
  width: number;
  height: number;
  frames: { x: number; y: number; width: number; height: number }[];
  pixels: Uint8ClampedArray;
};
export type RasterFont = {
  height: number;
  glyphs: Uint8Array;
  widths: Uint8Array;
  small: Uint8Array;
};

// Evaluate original indexed drawing operations. Inputs are immutable surface
// versions, so a backup never observes later writes to its source. Buffers with
// one remaining consumer can be reused without retaining a full frame history.
export async function renderRaster(
  graph: RasterGraph,
  loadAsset: (source: number, id: number) => Promise<RasterAsset>,
  loadFont: () => Promise<RasterFont>,
  movies: {
    seed?: (base: RasterBase) => Promise<Uint8Array>;
    movie?: (id: number) => Promise<Uint8Array[]>;
  } = {},
): Promise<Uint8Array> {
  const { nodes, root } = graph;
  if (
    graph.version !== 1 ||
    nodes.length < 1 ||
    nodes.length > 16384 ||
    root < 1 ||
    root > nodes.length
  )
    throw Error("仙剑绘制状态无效");
  const refs = new Uint32Array(nodes.length + 1),
    assets = new Map<string, RasterAsset>();
  const wanted = new Map<string, [number, number]>();
  const wantedMovies = new Set<number>(),
    clips = new Map<number, Uint8Array[]>();
  for (let i = 0; i < nodes.length; i++) {
    const n = nodes[i];
    if (
      n.length !== 16 ||
      n.some((v) => !Number.isInteger(v)) ||
      n[0] < 0 ||
      n[0] > 11 ||
      n[1] < 0 ||
      n[1] > i ||
      n[2] < 0 ||
      n[2] > i ||
      n[3] < 1 ||
      n[3] > 4096 ||
      n[4] < 1 ||
      n[4] > 4096
    )
      throw Error("仙剑绘制依赖无效");
    if (n[0] === 11 && (n[1] === 0 || n[2] !== 0 || n[5] < 0 || n[5] > 255 || n[6] < 0 || n[6] > 255)) throw Error("仙剑颜色替换状态无效");
    if (n[1]) refs[n[1]]++;
    if (n[2]) refs[n[2]]++;
    if (n[0] === 1) wanted.set(`${n[5]}/${n[6]}`, [n[5], n[6]]);
    if (n[0] === 10) wantedMovies.add(n[5]);
  }
  refs[root]++;
  await Promise.all(
    [...wanted].map(async ([key, [source, id]]) =>
      assets.set(key, await loadAsset(source, id)),
    ),
  );
  await Promise.all(
    [...wantedMovies].map(async (id) => {
      if (!movies.movie) throw Error("仙剑演出资源未就绪");
      clips.set(id, await movies.movie(id));
    }),
  );
  const font = nodes.some((n) => n[0] === 2) ? await loadFont() : undefined;
  const buffers: (Uint8Array | undefined)[] = Array(nodes.length + 1);
  for (let i = 0; i < nodes.length; i++) {
    const n = nodes[i],
      kind = n[0],
      width = n[3],
      height = n[4],
      a = buffers[n[1]],
      b = buffers[n[2]];
    const base = (id: number) => {
      const value = buffers[id];
      if (!id) return new Uint8Array(width * height);
      if (!value || value.length !== width * height)
        throw Error("仙剑绘制缓冲尺寸无效");
      return refs[id] === 1 ? value : value.slice();
    };
    let out: Uint8Array;
    switch (kind) {
      case 0:
        out = new Uint8Array(width * height);
        out.fill(n[5]);
        break;
      case 1: {
        out = base(n[1]);
        const asset = assets.get(`${n[5]}/${n[6]}`),
          rect = asset?.frames[n[7]];
        if (!asset || !rect) throw Error("仙剑绘制图块不存在");
        const dx = n[8],
          dy = n[9],
          mode = n[10],
          color = n[11],
          shift = n[12];
        if (mode < 0 || mode > 3) throw Error("仙剑绘制颜色方式无效");
        for (let y = Math.max(0, -dy); y < rect.height && dy + y < height; y++)
          for (
            let x = Math.max(0, -dx);
            x < rect.width && dx + x < width;
            x++
          ) {
            const src = ((rect.y + y) * asset.width + rect.x + x) * 4;
            if (!asset.pixels[src + 3]) continue;
            const p = (dy + y) * width + dx + x,
              value = asset.pixels[src];
            if (mode === 1) out[p] = (out[p] & 240) | ((out[p] & 15) >>> 1);
            else if (mode >= 2) {
              const low = Math.max(0, Math.min(15, (value & 15) + shift));
              out[p] = low | (mode === 3 ? color : value & 240);
            } else out[p] = value;
          }
        break;
      }
      case 2: {
        out = base(n[1]);
        const code = n[5] >= 0xf900 ? n[5] - 0xf900 + 0xd800 : n[5],
          small = !!n[9];
        if (
          !font ||
          code < 0 ||
          code >= 65534 ||
          (code >= 0xd800 && code < 0xf900 && n[5] < 0xf900) ||
          (small && code >= 256)
        )
          break;
        const wide = !small && font.widths[code] === 32,
          w = wide ? 16 : 8,
          h = small ? 8 : font.height,
          dx = n[6],
          dy = Math.max(0, n[7]);
        for (let y = 0; y < h && dy + y < height; y++)
          for (let x = 0; x < w; x++) {
            if (dx + x < 0 || dx + x >= width) continue;
            const bits = small
              ? font.small[code * 8 + y]
              : font.glyphs[code * 32 + (wide ? y * 2 + (x >>> 3) : y)];
            if (bits & (1 << (small ? x : 7 - (x & 7))))
              out[(dy + y) * width + dx + x] = n[8];
          }
        break;
      }
      case 3: {
        out = base(n[2]);
        if (!a) throw Error("仙剑复制来源无效");
        const sw = nodes[n[1] - 1][3],
          [sx, sy, dx, dy, w, h] = n.slice(5, 11);
        for (let y = 0; y < h; y++)
          out.set(
            a.subarray((sy + y) * sw + sx, (sy + y) * sw + sx + w),
            (dy + y) * width + dx,
          );
        break;
      }
      case 4: {
        out = base(n[1]);
        const [x, y, w, h, color] = n.slice(5, 10);
        for (let row = y; row < y + h; row++)
          out.fill(color, row * width + x, row * width + x + w);
        break;
      }
      case 5: {
        out = base(n[1]);
        if (!n[5]) break;
        for (let p = 0; p < out.length; p++) {
          const low = ((out[p] & 15) + n[5]) & 255;
          out[p] = (out[p] & 240) | (low & 128 ? 0 : low & 112 ? 15 : low);
        }
        break;
      }
      case 6: {
        out = base(n[1]);
        if (width !== 320 || height !== 200)
          throw Error("仙剑波动画面尺寸无效");
        applyScreenWave(out, n[5], n[6]);
        break;
      }
      case 7: {
        out = base(n[2]);
        if (!a || !b || width !== 320 || height !== 200)
          throw Error("仙剑渐变来源无效");
        for (let p = n[6]; p < out.length; p += 6) {
          let low = out[p] & 15;
          if (n[5] > 0) {
            const target = a[p] & 15;
            if (target > low) low++;
            else if (target < low) low--;
          }
          out[p] = (a[p] & 240) | low;
        }
        break;
      }
      case 8: {
        out = base(n[2]);
        if (!a) throw Error("仙剑缩放来源无效");
        const sw = nodes[n[1] - 1][3],
          sh = nodes[n[1] - 1][4],
          [sx, sy, w, h, dx, dy, dw, dh] = n.slice(5, 13);
        for (let y = 0; y < dh; y++)
          for (let x = 0; x < dw; x++) {
            const fromX = sx + Math.trunc((x * w) / dw),
              fromY = sy + Math.trunc((y * h) / dh),
              toX = dx + x,
              toY = dy + y;
            if (
              fromX >= 0 &&
              fromY >= 0 &&
              fromX < sw &&
              fromY < sh &&
              toX >= 0 &&
              toY >= 0 &&
              toX < width &&
              toY < height
            )
              out[toY * width + toX] = a[fromY * sw + fromX];
          }
        break;
      }
      case 9: {
        const seed =
          n[5] === 0 ? graph.base : n[5] === 1 ? graph.backup : undefined;
        if (!seed || !movies.seed) throw Error("仙剑演出起始画面不存在");
        out = (await movies.seed(seed)).slice();
        if (width !== 320 || height !== 200 || out.length !== 64000)
          throw Error("仙剑演出起始画面尺寸无效");
        break;
      }
      case 11: {
        if (!a) throw Error("仙剑颜色替换来源无效");
        out = base(n[1]);
        for (let i = 0; i < out.length; i++) if (out[i] === n[5]) out[i] = n[6];
        break;
      }
      case 10: {
        out = base(n[1]);
        const frames = clips.get(n[5]),
          first = n[6],
          last = n[7];
        if (
          width !== 320 ||
          height !== 200 ||
          !n[1] ||
          !frames ||
          first < 0 ||
          last < first ||
          last >= frames.length
        )
          throw Error("仙剑演出播放范围无效");
        for (let frame = first; frame <= last; frame++) {
          if (!frames[frame].length) throw Error("仙剑演出已结束");
          applyRNGFrame(frames[frame], out);
        }
        break;
      }
      default:
        throw Error("仙剑绘制方式无效");
    }
    if (n[1] && !--refs[n[1]]) buffers[n[1]] = undefined;
    if (n[2] && !--refs[n[2]]) buffers[n[2]] = undefined;
    buffers[i + 1] = out;
  }
  const result = buffers[root];
  if (nodes[root - 1][3] !== 320 || nodes[root - 1][4] !== 200 || !result)
    throw Error("仙剑最终画面尺寸无效");
  return result;
}
