import type { AssetManifest, BinaryResource, ImageResource, MapAsset, MapRules, MotionFunction, Viewport } from "./types.ts";
import type { WeaponTiming } from "./attack.ts";

declare global {
  interface Window {
    Go?: new () => { importObject: WebAssembly.Imports; run: (instance: WebAssembly.Instance) => Promise<void> };
    hotlineMotion?: { init: (map: string) => boolean; move: MotionFunction; weapons: () => string; unarmed: () => string; viewport: () => unknown };
  }
}

export type LoadedAssets = {
  manifest: AssetManifest;
  map: MapAsset;
  rules: MapRules;
  images: Map<string, CanvasImageSource>;
  sounds: Map<string, string>;
  motion: MotionFunction;
  weapons: WeaponTiming[];
  unarmed: WeaponTiming;
  viewport: Viewport;
  music?: string;
};

const manifestCache = new Map<string, Promise<AssetManifest>>();
const manifestSizes = new WeakMap<AssetManifest, number>();
const byteCache = new Map<string, Uint8Array>();
const imageCache = new Map<string, CanvasImageSource>();
let runtime: Promise<void> | undefined;

export function assetURL(base: string, file: string) {
  if (file.startsWith("/") || /^https?:/.test(file)) return file;
  return `${base.replace(/\/$/, "")}/hotlinemiami/${file}`;
}

export function validateViewport(value: unknown): Viewport {
  if (!value || typeof value !== "object") throw Error("视野规则元数据无效");
  const { width, height } = value as Record<string, unknown>;
  if (typeof width !== "number" || typeof height !== "number" || !Number.isSafeInteger(width) || !Number.isSafeInteger(height) || width <= 0 || height <= 0 || width * 5 !== height * 8) throw Error("视野规则须为正整数且保持 8:5 比例");
  return { width, height };
}

export function loadManifest(base: string) {
  const url = assetURL(base, "manifest.json");
  let promise = manifestCache.get(url);
  if (!promise) {
    promise = fetch(url).then(async response => {
      if (!response.ok) throw Error(`素材清单读取失败（${response.status}）`);
      const bytes = await response.arrayBuffer();
      const data = JSON.parse(new TextDecoder().decode(bytes)) as AssetManifest;
      if (!data.maps?.length || !data.characters?.length || !data.images || !data.sprites) throw Error("素材清单不完整");
      manifestSizes.set(data, bytes.byteLength);
      return data;
    }).catch(error => {
      manifestCache.delete(url);
      throw error;
    });
    manifestCache.set(url, promise);
  }
  return promise;
}

async function readResource(base: string, resource: BinaryResource, signal: AbortSignal, report: (bytes: number) => void) {
  const url = assetURL(base, resource.file);
  const cached = byteCache.get(url);
  if (cached && cached.length === resource.bytes) {
    report(cached.length);
    return cached;
  }
  const response = await fetch(url, { signal });
  if (!response.ok) throw Error(`素材下载失败（${response.status}）`);
  const chunks: Uint8Array[] = [];
  let loaded = 0;
  const reader = response.body?.getReader();
  if (reader) {
    try {
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        chunks.push(value);
        loaded += value.length;
        report(loaded);
      }
    } finally {
      reader.releaseLock();
    }
  } else {
    const value = new Uint8Array(await response.arrayBuffer());
    chunks.push(value);
    loaded = value.length;
    report(loaded);
  }
  if (loaded !== resource.bytes) throw Error(`素材大小不符：${resource.file}`);
  const bytes = new Uint8Array(loaded);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.length;
  }
  if (resource.sha256) {
    const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", bytes));
    const actual = [...digest].map(value => value.toString(16).padStart(2, "0")).join("");
    if (actual !== resource.sha256) throw Error(`素材校验失败：${resource.file}`);
  }
  byteCache.set(url, bytes);
  return bytes;
}

async function imageFromBytes(key: string, bytes: Uint8Array) {
  const cached = imageCache.get(key);
  if (cached) return cached;
  try {
    let image: CanvasImageSource;
    if (typeof createImageBitmap === "function") image = await createImageBitmap(new Blob([new Uint8Array(bytes)], { type: "image/png" }));
    else {
      const native = new Image();
      native.src = key;
      await native.decode();
      image = native;
    }
    imageCache.set(key, image);
    return image;
  } catch {
    throw Error(`图像解码失败：${key}`);
  }
}

async function initializeRuntime(scriptURL: string, script: Uint8Array, wasm: Uint8Array) {
  if (!runtime) runtime = (async () => {
    if (!window.Go) {
      const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", new Uint8Array(script)));
      const integrity = btoa(String.fromCharCode(...digest));
      await new Promise<void>((resolve, reject) => {
        const element = document.createElement("script");
        element.src = scriptURL;
        element.crossOrigin = "anonymous";
        element.integrity = `sha256-${integrity}`;
        element.onload = () => { element.remove(); resolve(); };
        element.onerror = () => { element.remove(); reject(Error("预测运行时初始化失败")); };
        document.head.append(element);
      });
    }
    if (!window.Go) throw Error("未找到预测运行时");
    const go = new window.Go();
    const instance = await WebAssembly.instantiate(new Uint8Array(wasm), go.importObject);
    void go.run(instance.instance).catch(() => {});
    await new Promise<void>((resolve, reject) => {
      const deadline = performance.now() + 10000;
      const inspect = () => {
        if (window.hotlineMotion) resolve();
        else if (performance.now() >= deadline) reject(Error("预测规则初始化超时"));
        else setTimeout(inspect, 10);
      };
      inspect();
    });
  })().catch(error => { runtime = undefined; throw error; });
  await runtime;
}

export async function loadBattleAssets(base: string, mapID: string, characters: string[], signal: AbortSignal, report: (percent: number, label: string, bytes?: { loaded: number; total: number }) => void, suppliedRules?: MapRules): Promise<LoadedAssets> {
  report(0, "读取素材清单");
  const manifest = await loadManifest(base);
  signal.throwIfAborted();
  const map = manifest.maps.find(candidate => candidate.id === mapID);
  if (!map) throw Error("未找到当前地图素材");
  let rules = map.rules ?? suppliedRules;
  let ruleBytes = 0;
  if (!rules && map.rulesFile) {
    const response = await fetch(assetURL(base, map.rulesFile), { signal });
    if (!response.ok) throw Error("地图碰撞数据读取失败");
    const bytes = await response.arrayBuffer();
    ruleBytes = bytes.byteLength;
    rules = JSON.parse(new TextDecoder().decode(bytes)) as MapRules;
  }
  if (!rules) throw Error("缺少地图碰撞数据");
  if (!manifest.runtime) throw Error("缺少预测运行时清单");
  const imageIDs = new Set<string>([map.image]);
  for (const character of manifest.characters.filter(candidate => characters.includes(candidate.id))) {
    for (const key of [character.sprite, character.attack, character.dead, character.overlay, ...Object.values(character.poses ?? {})]) {
      if (key && manifest.sprites[key]) imageIDs.add(manifest.sprites[key].image);
    }
  }
  for (const weapon of manifest.weapons) {
    const sprite = manifest.sprites[weapon.sprite];
    if (sprite) imageIDs.add(sprite.image);
  }
  const files = new Map<string, BinaryResource>();
  for (const id of imageIDs) {
    const resource = manifest.images[id];
    if (!resource) throw Error(`缺少图像：${id}`);
    files.set(`image:${id}`, resource);
  }
  files.set("runtime:script", manifest.runtime.script);
  files.set("runtime:wasm", manifest.runtime.wasm);
  const audio = manifest.sounds ?? manifest.audio ?? {};
  for (const [id, resource] of Object.entries(audio)) files.set(`sound:${id}`, resource);
  if (manifest.music) files.set("music", manifest.music);
  const unique = new Map<string, { resource: BinaryResource; ids: string[]; url: string }>();
  const resourceURLs = new Map<string, string>();
  for (const [id, resource] of files) {
    const url = assetURL(base, resource.file);
    const key = resource.sha256 ? `${resource.sha256}:${resource.bytes}` : url;
    const existing = unique.get(key);
    if (existing) {
      if (existing.resource.bytes !== resource.bytes || existing.resource.sha256 !== resource.sha256) throw Error("素材同地址的校验信息不一致");
      existing.ids.push(id);
      resourceURLs.set(id, existing.url);
    } else {
      unique.set(key, { resource, ids: [id], url });
      resourceURLs.set(id, url);
    }
  }
  const metadataBytes = (manifestSizes.get(manifest) ?? 0) + ruleBytes;
  const total = metadataBytes + [...unique.values()].reduce((sum, file) => sum + file.resource.bytes, 0);
  const counts = new Map<string, number>([["metadata", metadataBytes]]);
  const data = new Map<string, Uint8Array>();
  let cursor = 0;
  const entries = [...unique.entries()];
  await Promise.all(Array.from({ length: Math.min(5, entries.length) }, async () => {
    while (cursor < entries.length) {
      const [url, file] = entries[cursor++];
      const bytes = await readResource(base, file.resource, signal, loaded => {
        counts.set(url, loaded);
        const read = [...counts.values()].reduce((sum, value) => sum + value, 0);
        report(Math.trunc(read * 92 / Math.max(1, total)), "加载战场与人物", { loaded: read, total });
      });
      for (const id of file.ids) data.set(id, bytes);
    }
  }));
  signal.throwIfAborted();
  const images = new Map<string, CanvasImageSource>();
  for (const id of imageIDs) images.set(id, await imageFromBytes(resourceURLs.get(`image:${id}`)!, data.get(`image:${id}`)!));
  report(96, "初始化预测规则", { loaded: total, total });
  await initializeRuntime(assetURL(base, manifest.runtime.script.file), data.get("runtime:script")!, data.get("runtime:wasm")!);
  if (!window.hotlineMotion?.init(JSON.stringify(rules))) throw Error("地图预测规则初始化失败");
  const sounds = new Map<string, string>();
  for (const id of Object.keys(audio)) sounds.set(id, resourceURLs.get(`sound:${id}`)!);
  signal.throwIfAborted();
  const weapons = JSON.parse(window.hotlineMotion.weapons()) as WeaponTiming[];
  const unarmed = JSON.parse(window.hotlineMotion.unarmed()) as WeaponTiming;
  if (typeof window.hotlineMotion.viewport !== "function") throw Error("缺少视野规则元数据");
  const viewport = validateViewport(window.hotlineMotion.viewport());
  const music = manifest.music ? assetURL(base, manifest.music.file) : undefined;
  return { manifest, map, rules, images, sounds, motion: window.hotlineMotion.move, weapons, unarmed, viewport, music };
}

export function imageResource(manifest: AssetManifest, reference: string): { resource: ImageResource; frame?: { x: number; y: number; w: number; h: number } } | undefined {
  const resource = manifest.images[reference];
  if (resource) return { resource };
  const sprite = manifest.sprites[reference];
  if (sprite) return { resource: manifest.images[sprite.image], frame: sprite.frames[0] };
  return undefined;
}
