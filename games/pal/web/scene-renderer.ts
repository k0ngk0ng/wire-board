import {
  renderRaster,
  type RasterGraph,
  type RasterBase,
} from "./raster-renderer";
import { readRNGArchive } from "./rng";
import { drawField, type FieldState } from "./field-rules";
import { applyScreenWave } from "./screen-wave";
import { EndingRenderer, type EndingFrame, type EndingPublication } from "./ending-renderer";
export type SceneDraw = {
  kind: number;
  a: number;
  b: number;
  c: number;
  d: number;
  x: number;
  baseY: number;
  sortY: number;
};
export type SceneFrame = {
  ending?: EndingFrame & { frames?: number; elapsedMS?: number; serial?: number; finished?: boolean };
  field?: FieldState;
  paletteEffect?: { palette: number; night: boolean; factors: number[]; redStep?: number };
  sceneReady?: boolean;
  clearColor?: number;
  map: number;
  cameraX: number;
  cameraY: number;
  palette: number;
  night: boolean;
  draws: SceneDraw[];
  overlay?: UICommand[];
  raster?: RasterGraph;
  battleAssets?: readonly BattleAsset[];
};
type BattleAsset = readonly [6 | 8, number];
export type UICommand = {
  kind: number;
  source: number;
  id: number;
  frame: number;
  x: number;
  y: number;
  color: number;
  small: number;
};
type Rect = { x: number; y: number; width: number; height: number };
type Atlas = { width: number; height: number; image: string; frames: Rect[] };
type IndexedAtlas = Atlas & { pixels: Uint8ClampedArray };
type Font = {
  height: number;
  glyphs: Uint8Array;
  widths: Uint8Array;
  small: Uint8Array;
};
type MapInfo = {
  id: number;
  width: number;
  height: number;
  tiles: string;
  atlas: string;
};
type MapData = MapInfo & { data: DataView };
type VisualIndex = {
  version: number;
  files: Record<string, { size: number; sha256: string; mime: string }>;
};
export type VisualPublication = EndingPublication;
type Palette = number[][];
function paletteFor(
  palettes: Palette[][],
  frame: SceneFrame,
): Palette | undefined {
  const effect = frame.paletteEffect;
  const base =
    palettes[effect?.palette ?? frame.palette]?.[
      (effect?.night ?? frame.night) ? 1 : 0
    ];
  if (!base) return base;
  const red = effect?.redStep ?? 0;
  const colors = red ? base.map((color, index) => {
    if (index === 0x4f) return color;
    const target = Math.trunc((color[0] + color[1] + color[2]) / 4) + 64;
    return [color[0] > target ? Math.max(target, color[0] - red * 8) : Math.min(target, color[0] + red * 8), Math.max(0, color[1] - red * 8), Math.max(0, color[2] - red * 8)];
  }) : base;
  if (!effect?.factors.length) return colors;
  return colors.map((color) =>
    color.map((value) => effect.factors.reduce((v, f) => (v * f) >> 6, value)),
  );
}
export type SceneLoadProgress = {
  percent: number;
  label: string;
  loaded: number;
  total: number;
};
function uiAsset(op: Pick<UICommand, "source" | "id">): {
  group: string;
  id: number;
} {
  switch (op.source) {
    case 1:
      return { group: "rgm", id: op.id };
    case 2:
      return { group: "data", id: 9 };
    case 3:
      return { group: "data", id: 12 };
    case 4:
      return { group: "fbp", id: op.id };
    case 5:
      return { group: "ball", id: op.id };
    case 6:
      return { group: "f", id: op.id };
    case 7:
      return { group: "abc", id: op.id };
    case 8:
      return { group: "fire", id: op.id };
    case 9:
      return { group: "data", id: 10 };
    default:
      throw Error("仙剑界面图像来源无效");
  }
}

function battleAtlases(assets: readonly BattleAsset[] = []) {
  if (!assets.length) return [];
  return [
    uiAsset({ source: 9, id: 0 }),
    ...assets.map(([source, id]) => uiAsset({ source, id })),
  ];
}

export class SceneRenderer {
  private ending: EndingRenderer;
  private raw = new Map<string, Promise<ArrayBuffer>>();
  private sizes = new Map<string, number>();
  private counts = new Map<string, number>();
  private planned = false;
  private manifestBytes = 0;
  private index?: Promise<VisualIndex>;
  private palettes?: Promise<Palette[][]>;
  private maps = new Map<number, Promise<MapData>>();
  private atlases = new Map<string, Promise<IndexedAtlas>>();
  private preparedBattleAtlases = new Set<string>();
  private pendingBattleAtlases = new Map<string, Promise<void>>();
  private battleWorkers = Array.from({ length: 4 }, () => Promise.resolve());
  private nextBattleWorker = 0;
  private font?: Promise<Font>;
  private movies = new Map<number, Promise<Uint8Array[]>>();
  constructor(
    private base: string,
    private options: {
      signal?: AbortSignal;
      progress?: (value: SceneLoadProgress) => void;
      publication?: VisualPublication;
      endingPublication?: EndingPublication;
      prepareEndingAudio?: (url: string) => Promise<boolean>;
    } = {},
  ) {
    this.base = base.replace(/\/$/, "");
    this.ending = new EndingRenderer(this.base.replace(/\/v1$/, "/ending-v1"), { ...this.options, publication: this.options.endingPublication, prepareAudio: this.options.prepareEndingAudio });
  }

  endingAudioSource(clip: number) { return this.ending.audioSource(clip); }

  private async catalog(): Promise<VisualIndex> {
    this.index ??= this.download("index.json", (n) => {
      this.manifestBytes = n;
    })
      .then(async (buffer) => {
        if (this.options.publication) {
          const file = this.options.publication.files["index.json"], digest = [...new Uint8Array(await crypto.subtle.digest("SHA-256", buffer))].map(value => value.toString(16).padStart(2, "0")).join("");
          if (file.sha256 !== this.options.publication.assetID || digest !== file.sha256 || buffer.byteLength !== file.bytes) throw Error("仙剑素材清单校验失败");
        }
        const result: VisualIndex = JSON.parse(
          new TextDecoder().decode(buffer),
        );
        if (result.version !== 1 || !result.files)
          throw Error("仙剑素材版本不兼容");
        return result;
      })
      .catch((error) => {
        this.index = undefined;
        throw error;
      });
    return this.index;
  }

  private report() {
    if (!this.planned) return;
    const total =
      this.manifestBytes + [...this.sizes.values()].reduce((a, b) => a + b, 0);
    const loaded =
      this.manifestBytes + [...this.counts.values()].reduce((a, b) => a + b, 0);
    this.options.progress?.({
      percent: total ? (loaded / total) * 100 : 0,
      label: "加载原版素材",
      loaded,
      total,
    });
  }
  private url(name: string) {
    if (!this.options.publication) return `${this.base}/${name}`;
    const file = this.options.publication.files[name];
    if (!file || !file.url.startsWith("https://cdn.ichenj.com/bga/") || /[?#\\]/.test(file.url) || file.url.includes("/../") || !Number.isSafeInteger(file.bytes) || file.bytes <= 0 || file.bytes > 67108864 || !/^[a-f0-9]{64}$/.test(file.sha256)) throw Error("仙剑素材发布记录无效");
    return file.url;
  }

  private async download(name: string, count: (bytes: number) => void) {
    const file = this.options.publication?.files[name];
    const response = await fetch(this.url(name), {
      signal: this.options.signal ? AbortSignal.any([this.options.signal, AbortSignal.timeout(45000)]) : AbortSignal.timeout(45000),
    });
    if (!response.ok) throw Error(`仙剑素材加载失败：${name}`);
    if (file && response.headers.get("Content-Type")?.split(";")[0] !== file.mime) throw Error("仙剑素材格式不匹配");
    const maximum = file?.bytes ?? 67108864;
    const reader = response.body?.getReader();
    if (!reader) {
      const bytes = await response.arrayBuffer();
      if (bytes.byteLength > maximum) throw Error("仙剑素材超过大小限制");
      count(bytes.byteLength);
      return bytes;
    }
    const chunks: Uint8Array[] = [];
    let size = 0;
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        size += value.length;
        if (size > maximum) { await reader.cancel(); throw Error("仙剑素材超过大小限制"); }
        chunks.push(value);
        count(size);
      }
    } finally {
      reader.releaseLock();
    }
    const joined = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) {
      joined.set(chunk, offset);
      offset += chunk.length;
    }
    return joined.buffer;
  }

  private async bytes(name: string): Promise<ArrayBuffer> {
    const index = await this.catalog();
    const file = index.files[name];
    if (!file) throw Error(`仙剑素材不存在：${name}`);
    if (!Number.isSafeInteger(file.size) || file.size <= 0 || file.size > 67108864) throw Error("仙剑素材清单无效");
    const published = this.options.publication?.files[name];
    if (this.options.publication && (!published || published.sha256 !== file.sha256 || published.bytes !== file.size || published.mime !== file.mime)) throw Error("仙剑素材发布记录不匹配");
    this.sizes.set(name, file.size);
    this.report();
    if (!this.raw.has(name))
      this.raw.set(
        name,
        (async () => {
          const data = await this.download(name, (n) => {
            this.counts.set(name, Math.min(n, file.size));
            this.report();
          });
          const digest = [
            ...new Uint8Array(await crypto.subtle.digest("SHA-256", data)),
          ]
            .map((n) => n.toString(16).padStart(2, "0"))
            .join("");
          if (data.byteLength !== file.size || digest !== file.sha256)
            throw Error(`仙剑素材校验失败：${name}`);
          return data;
        })().catch((error) => {
          this.raw.delete(name);
          this.counts.delete(name);
          this.report();
          throw error;
        }),
      );
    return this.raw.get(name)!;
  }

  async prepare(frame: SceneFrame) {
    if (frame.ending?.clip) {
      await this.ending.prepare(frame.ending);
      return;
    }
    const index = await this.catalog(),
      metadata = new Set<string>(["fonts.json"]),
      files = new Set<string>(["palettes.json"]);
    const addAtlas = (group: string, id: number) =>
      metadata.add(`atlases/${group}/${id}.json`);
    const addScene = (scene: RasterBase | SceneFrame) => {
      if (scene.sceneReady !== false) {
        metadata.add(`maps/${scene.map}.json`);
        addAtlas("gop", scene.map);
        for (const draw of scene.draws)
          if (draw.kind === 0) addAtlas("mgo", draw.a);
      }
      for (const op of scene.overlay ?? [])
        if (op.kind === 1) {
          const a = uiAsset(op);
          addAtlas(a.group, a.id);
        }
    };
    if (frame.raster) {
      for (const node of frame.raster.nodes)
        if (node[0] === 1) {
          const a = uiAsset({ source: node[5], id: node[6] });
          addAtlas(a.group, a.id);
        } else if (node[0] === 9) {
          const seed = node[5] === 0 ? frame.raster.base : frame.raster.backup;
          if (!seed) throw Error("仙剑演出起始画面不存在");
          addScene(seed);
        } else if (node[0] === 10) {
          metadata.add(`movies/${node[5]}.json`);
        }
    } else {
      metadata.add(`maps/${frame.map}.json`);
      addAtlas("gop", frame.map);
      for (const draw of frame.draws)
        if (draw.kind === 0) addAtlas("mgo", draw.a);
      for (const actor of frame.field?.actors ?? []) addAtlas("mgo", actor[2]);
      for (const op of frame.overlay ?? [])
        if (op.kind === 1) {
          const a = uiAsset(op);
          addAtlas(a.group, a.id);
        }
    }
    await Promise.all(
      [...metadata].map(async (name) => {
        const value = await this.json<Record<string, unknown>>(name);
        for (const key of [
          "image",
          "tiles",
          "glyphs",
          "widths",
          "small",
          "data",
        ])
          if (typeof value[key] === "string") files.add(value[key] as string);
      }),
    );
    for (const name of files) {
      const file = index.files[name];
      if (!file) throw Error(`仙剑素材不存在：${name}`);
      this.sizes.set(name, file.size);
    }
    for (const { group, id } of battleAtlases(frame.battleAssets)) {
      for (const ext of ["json", "png"]) {
        const name = `atlases/${group}/${id}.${ext}`;
        const file = index.files[name];
        if (file) this.sizes.set(name, file.size);
      }
    }
    this.planned = true;
    this.report();
    await Promise.all([
      ...[...files].map((name) => this.bytes(name)),
      this.prepareBattle(frame.battleAssets),
    ]);
    await this.loadFont();
  }

  battleReady(assets?: readonly BattleAsset[]) {
    return battleAtlases(assets).every(({ group, id }) =>
      this.preparedBattleAtlases.has(`${group}/${id}`),
    );
  }

  async prepareBattle(assets?: readonly BattleAsset[]) {
    await Promise.all(battleAtlases(assets).map(({ group, id }) => {
      const key = `${group}/${id}`;
      if (this.preparedBattleAtlases.has(key)) return;
      const pending = this.pendingBattleAtlases.get(key);
      if (pending) return pending;
      const lane = this.nextBattleWorker++ % this.battleWorkers.length;
      const task = this.battleWorkers[lane].then(async () => {
        const index = await this.catalog();
        if (index.files[`atlases/${key}.json`]) await this.atlas(group, id);
        this.preparedBattleAtlases.add(key);
      }).finally(() => this.pendingBattleAtlases.delete(key));
      this.battleWorkers[lane] = task.catch(() => {});
      this.pendingBattleAtlases.set(key, task);
      return task;
    }));
  }

  private async json<T>(name: string): Promise<T> {
    return JSON.parse(new TextDecoder().decode(await this.bytes(name))) as T;
  }

  private async map(id: number): Promise<MapData> {
    if (!this.maps.has(id))
      this.maps.set(
        id,
        (async () => {
          const info = await this.json<MapInfo>(`maps/${id}.json`);
          const data = await this.bytes(info.tiles);
          const stream = new Blob([data])
            .stream()
            .pipeThrough(new DecompressionStream("gzip"));
          const decoded = await new Response(stream).arrayBuffer();
          if (
            decoded.byteLength !== 65536 ||
            info.width !== 64 ||
            info.height !== 128
          )
            throw Error("仙剑地图数据不完整");
          return { ...info, data: new DataView(decoded) };
        })().catch((error) => {
          this.maps.delete(id);
          throw error;
        }),
      );
    return this.maps.get(id)!;
  }

  private async atlas(group: string, id: number): Promise<IndexedAtlas> {
    const key = `${group}/${id}`;
    if (!this.atlases.has(key))
      this.atlases.set(
        key,
        (async () => {
          const meta = await this.json<Atlas>(`atlases/${group}/${id}.json`);
          const bitmap = await createImageBitmap(
            new Blob([await this.bytes(meta.image)], { type: "image/png" }),
            { premultiplyAlpha: "none", colorSpaceConversion: "none" },
          );
          const canvas = document.createElement("canvas");
          canvas.width = meta.width;
          canvas.height = meta.height;
          const ctx = canvas.getContext("2d", { willReadFrequently: true });
          if (
            !ctx ||
            bitmap.width !== meta.width ||
            bitmap.height !== meta.height
          )
            throw Error("仙剑图集尺寸不匹配");
          ctx.drawImage(bitmap, 0, 0);
          bitmap.close();
          const image = ctx.getImageData(0, 0, canvas.width, canvas.height);
          return { ...meta, pixels: image.data };
        })().catch((error) => {
          this.atlases.delete(key);
          throw error;
        }),
      );
    return this.atlases.get(key)!;
  }

  private async decoded(name: string, size: number): Promise<Uint8Array> {
    const raw = await this.bytes(name);
    const stream = new Blob([raw])
      .stream()
      .pipeThrough(new DecompressionStream("gzip"));
    const data = new Uint8Array(await new Response(stream).arrayBuffer());
    if (data.length !== size) throw Error("仙剑字体数据不完整");
    return data;
  }
  private async movie(id: number): Promise<Uint8Array[]> {
    if (!this.movies.has(id))
      this.movies.set(
        id,
        (async () => {
          const meta = await this.json<{
            version: number;
            width: number;
            height: number;
            frames: number;
            data: string;
          }>(`movies/${id}.json`);
          if (
            meta.version !== 1 ||
            meta.width !== 320 ||
            meta.height !== 200 ||
            meta.frames < 1 ||
            meta.frames > 4096
          )
            throw Error("仙剑演出资源版本无效");
          const stream = new Blob([await this.bytes(meta.data)])
            .stream()
            .pipeThrough(new DecompressionStream("gzip"));
          const raw = new Uint8Array(await new Response(stream).arrayBuffer());
          if (raw.length > 16 * 1024 * 1024) throw Error("仙剑演出资源过大");
          const frames = readRNGArchive(raw);
          if (frames.length !== meta.frames) throw Error("仙剑演出帧数不匹配");
          return frames;
        })().catch((error) => {
          this.movies.delete(id);
          throw error;
        }),
      );
    return this.movies.get(id)!;
  }
  private async loadFont(): Promise<Font> {
    this.font ??= (async () => {
      const meta = await this.json<{
        version: number;
        height: number;
        count: number;
        stride: number;
        glyphs: string;
        widths: string;
        small: string;
      }>("fonts.json");
      if (
        meta.version !== 1 ||
        meta.height !== 16 ||
        meta.count !== 65536 ||
        meta.stride !== 32
      )
        throw Error("仙剑字体版本不兼容");
      const [glyphs, widths, small] = await Promise.all([
        this.decoded(meta.glyphs, 65536 * 32),
        this.decoded(meta.widths, 65536),
        this.decoded(meta.small, 256 * 8),
      ]);
      return { height: meta.height, glyphs, widths, small };
    })().catch((error) => {
      this.font = undefined;
      throw error;
    });
    return this.font;
  }

  async paint(
    canvas: HTMLCanvasElement,
    frame: SceneFrame,
    current: () => boolean = () => true,
    predicted?: FieldState,
  ): Promise<void> {
    if (frame.ending?.clip) {
      await this.ending.prepare(frame.ending);
      await this.ending.paint(canvas, frame.ending, current);
      return;
    }
    const predictionFrames =
      predicted && frame.field
        ? (predicted.frame - frame.field.frame) >>> 0
        : 0;
    if (
      predicted &&
      predictionFrames <= 3 &&
      !frame.raster &&
      frame.sceneReady !== false
    ) {
      const map = await this.map(frame.map),
        sprites = new Map<number, IndexedAtlas>();
      await Promise.all(
        [...new Set(predicted.actors.map((a) => a[2]))].map(async (id) => {
          sprites.set(id, await this.atlas("mgo", id));
        }),
      );
      const elapsed = predictionFrames;
      const waves = frame.draws
        .filter((draw) => draw.kind === 2)
        .flatMap((draw) => {
          const level = draw.a + draw.c * elapsed;
          return level > 0 && level < 256
            ? [{ ...draw, a: level, b: (draw.b + elapsed) % 32 }]
            : [];
        });
      frame = {
        ...frame,
        cameraX: predicted.camera[0],
        cameraY: predicted.camera[1],
        draws: [
          ...waves,
          ...drawField(predicted, {
            sprite: (asset, id) => sprites.get(asset)?.frames[id],
            height: (x, y, half, layer) => {
              if (x < 0 || x >= 64 || y < 0 || y >= 128 || half < 0 || half > 1)
                return 0;
              return (
                (map.data.getUint32(((y * 64 + x) * 2 + half) * 4, true) >>>
                  (layer ? 24 : 8)) &
                15
              );
            },
          }),
        ],
      };
    }
    this.palettes ??= this.json<Palette[][]>("palettes.json").catch((error) => {
      this.palettes = undefined;
      throw error;
    });
    if (frame.raster) {
      const palettes = await this.palettes,
        palette = paletteFor(palettes, frame);
      if (!palette || palette.length !== 256) throw Error("仙剑调色板不存在");
      const pixels = await renderRaster(
        frame.raster,
        (source, id) => {
          const asset = uiAsset({ source, id });
          return this.atlas(asset.group, asset.id);
        },
        () => this.loadFont(),
        {
          seed: (base) => this.scenePixels(base),
          movie: (id) => this.movie(id),
        },
      );
      if (current()) this.present(canvas, pixels, palette);
      return;
    }
    const [pixels, palettes] = await Promise.all([
      this.scenePixels(frame),
      this.palettes,
    ]);
    const palette = paletteFor(palettes, frame);
    if (!palette || palette.length !== 256) throw Error("仙剑调色板不存在");
    if (current()) this.present(canvas, pixels, palette);
  }

  private async scenePixels(
    frame: RasterBase | SceneFrame,
  ): Promise<Uint8Array> {
    const [map, terrain] =
      frame.sceneReady === false
        ? [
            { data: new DataView(new ArrayBuffer(65536)) },
            {
              width: 1,
              height: 1,
              image: "",
              frames: [],
              pixels: new Uint8ClampedArray(),
            },
          ]
        : await Promise.all([
            this.map(frame.map),
            this.atlas("gop", frame.map),
          ]);
    const sprites = new Map<number, IndexedAtlas>();
    await Promise.all(
      [
        ...new Set(frame.draws.filter((op) => op.kind === 0).map((op) => op.a)),
      ].map(async (id) => {
        sprites.set(id, await this.atlas("mgo", id));
      }),
    );
    const overlay = frame.overlay ?? [],
      ui = new Map<string, IndexedAtlas>();
    await Promise.all(
      overlay
        .filter((op) => op.kind === 1)
        .map(async (op) => {
          const { group, id } = uiAsset(op),
            key = `${group}/${id}`;
          ui.set(key, await this.atlas(group, id));
        }),
    );
    const font = overlay.some((op) => op.kind === 0)
      ? await this.loadFont()
      : undefined;
    const pixels = new Uint8Array(320 * 200);
    pixels.fill(frame.clearColor ?? 0);
    const tile = (
      x: number,
      y: number,
      half: number,
      layer: number,
    ): number | undefined => {
      if (x < 0 || x >= 64 || y < 0 || y >= 128 || half < 0 || half > 1)
        return undefined;
      let data = map.data.getUint32(((y * 64 + x) * 2 + half) * 4, true);
      if (layer) data >>>= 16;
      const index = (data & 255) | ((data >>> 4) & 256);
      return layer ? index - 1 : index;
    };
    const blit = (
      atlas: IndexedAtlas,
      id: number,
      x: number,
      y: number,
      shadow = false,
    ) => {
      const rect = atlas.frames[id];
      if (!rect || !rect.width || !rect.height) return;
      for (let sy = Math.max(0, -y); sy < rect.height && y + sy < 200; sy++)
        for (let sx = Math.max(0, -x); sx < rect.width && x + sx < 320; sx++) {
          const src = ((rect.y + sy) * atlas.width + rect.x + sx) * 4,
            dst = (y + sy) * 320 + x + sx;
          if (atlas.pixels[src + 3])
            pixels[dst] = shadow
              ? (pixels[dst] & 0xf0) | ((pixels[dst] & 15) >>> 1)
              : atlas.pixels[src];
        }
    };
    const startY = Math.trunc(frame.cameraY / 16) - 1,
      endY = Math.trunc((frame.cameraY + 200) / 16) + 2;
    const startX = Math.trunc(frame.cameraX / 32) - 1,
      endX = Math.trunc((frame.cameraX + 320) / 32) + 2;
    if (frame.sceneReady !== false)
      for (let layer = 0; layer < 2; layer++)
        for (let y = startY; y < endY; y++)
          for (let half = 0; half < 2; half++)
            for (let x = startX; x < endX; x++) {
              let id = tile(x, y, half, layer);
              if (id === undefined || !terrain.frames[id]) {
                if (layer) continue;
                id = tile(0, 0, 0, 0);
              }
              if (id !== undefined)
                blit(
                  terrain,
                  id,
                  x * 32 + half * 16 - 16 - frame.cameraX,
                  y * 16 + half * 8 - 8 - frame.cameraY,
                );
            }
    for (const op of frame.sceneReady === false ? [] : frame.draws) {
      if (op.kind === 2) {
        applyScreenWave(pixels, op.a, op.b);
      } else if (op.kind === 0) {
        const atlas = sprites.get(op.a);
        const rect = atlas?.frames[op.b];
        if (atlas && rect) blit(atlas, op.b, op.x, op.baseY - rect.height);
      } else if (op.kind === 1) {
        const id = tile(op.a, op.b, op.c, op.d),
          rect = id === undefined ? undefined : terrain.frames[id];
        if (id !== undefined && rect)
          blit(terrain, id, op.x, op.baseY - rect.height);
      }
    }
    for (const op of overlay) {
      if (op.kind === 1) {
        const { group, id } = uiAsset(op);
        const atlas = ui.get(`${group}/${id}`);
        if (!atlas) throw Error("仙剑界面图像未就绪");
        blit(atlas, op.frame, op.x, op.y, !!op.color);
      } else if (op.kind === 0 && font) {
        if ((op.id >= 0xd800 && op.id < 0xf900) || op.id >= 65534) continue;
        const code = op.id >= 0xf900 ? op.id - 0xf900 + 0xd800 : op.id;
        if (code < 0 || code >= 65536 || (op.small && code >= 256))
          throw Error("仙剑字形编号无效");
        const wide = !op.small && font.widths[code] === 32,
          width = wide ? 16 : 8,
          height = op.small ? 8 : font.height;
        for (let y = 0; y < height && Math.max(0, op.y) + y < 200; y++)
          for (let x = 0; x < width; x++) {
            const dx = op.x + x,
              dy = Math.max(0, op.y) + y;
            if (dx < 0 || dx >= 320) continue;
            const bits = op.small
              ? font.small[code * 8 + y]
              : font.glyphs[code * 32 + (wide ? y * 2 + (x >>> 3) : y)];
            if (bits & (1 << (op.small ? x : 7 - (x & 7))))
              pixels[dy * 320 + dx] = op.color;
          }
      } else throw Error("仙剑界面绘制指令无效");
    }
    return pixels;
  }

  private present(
    canvas: HTMLCanvasElement,
    pixels: Uint8Array,
    palette: Palette,
  ) {
    const ctx = canvas.getContext("2d");
    if (!ctx) throw Error("无法初始化仙剑画面");
    if (canvas.width !== 320) canvas.width = 320;
    if (canvas.height !== 200) canvas.height = 200;
    ctx.imageSmoothingEnabled = false;
    const rgba = new Uint8ClampedArray(320 * 200 * 4);
    for (let i = 0; i < pixels.length; i++) {
      const color = palette[pixels[i]];
      rgba[i * 4] = color[0];
      rgba[i * 4 + 1] = color[1];
      rgba[i * 4 + 2] = color[2];
      rgba[i * 4 + 3] = 255;
    }
    ctx.putImageData(new ImageData(rgba, 320, 200), 0, 0);
  }
}
