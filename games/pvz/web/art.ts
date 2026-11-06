export type SpriteFrame = { x: number; y: number; w: number; h: number; duration?: number };
export type SpriteDefinition = {
  src: string;
  frameWidth?: number;
  frameHeight?: number;
  columns?: number;
  frames?: number | SpriteFrame[];
  fps?: number;
  anchorX?: number;
  anchorY?: number;
  animations?: Record<string, SpriteDefinition>;
  loop?: boolean;
  preview?: SpriteDefinition;
  placement?: "scene";
  native?: { src: string; images: string[]; compression?: "gzip"; format?: string };
  bounds?: { x: number; y: number; width: number; height: number };
  attachments?: { plantFoot: { x: number; y: number }; spikeFoot: { x: number; y: number }; roofWestAdjustmentY: number; space: string };
};
export type ArtManifest = {
  version?: string;
  background: string;
  backgroundCrop?: { x: number; y: number; width: number; height: number };
  lawn?: { x: number; y: number; width: number; height: number };
  sprites: Record<string, SpriteDefinition>;
  images?: Record<string, string>;
  audio?: Record<string, string | string[]>;
  maps?: Record<string, { background: string; backgroundCrop?: { x: number; y: number; width: number; height: number }; lawn: { x: number; y: number; width: number; height: number }; rows: number; poolRows?: number[]; roofSlope?: number[] }>;
  byScene?: Record<string, string[]>;
  byUnit?: Record<string, string[]>;
  projectiles?: Record<string, SpriteDefinition>;
  music?: Record<string, MusicTheme>;
};
type NativeLayer = { src: string; m: [number, number, number, number, number, number]; alpha?: number };
type NativeAnimation = { kind?: string; clips: Record<string, { fps: number; loop: boolean; frames: NativeLayer[][]; bounds?: { x: number; y: number; width: number; height: number } }> };
export type DrawnPose = { kind: string; clip: string; index: number; frames: number; time: number; x: number; y: number };
export type Art = { manifest: ArtManifest; images: Map<string, HTMLImageElement>; native: Map<string, NativeAnimation>; loaded: Set<string>; pending: Map<string, Promise<void>>; drawnPoses: DrawnPose[]; base: string; scene: string };

export function assetURL(base: string, path: string) {
  if (/^(https?:|data:|blob:)/.test(path)) return path;
  return `${base.replace(/\/$/, "")}/${path.replace(/^\//, "")}`;
}

export async function loadArt(_legacyAssets: string, signal: AbortSignal, progress: (n: number, label: string) => void, scene = "day", kinds: string[] = []): Promise<Art> {
  const base = assetURL(import.meta.env.BASE_URL, "pvz/classic-v2");
  progress(5, "读取草坪与角色资源");
  const response = await fetch(assetURL(base, "manifest.json"), { signal });
  if (!response.ok) throw new Error(`原版资源清单加载失败（${response.status}）`);
  const manifest = await response.json() as ArtManifest;
  if ((!manifest.background && !manifest.maps?.[scene]?.background) || !manifest.sprites) throw new Error("原版资源清单不完整");
  manifest.sprites.zombie ??= manifest.sprites.normal;
  manifest.sprites.polevaulter ??= manifest.sprites.pole;
  manifest.sprites.coin ??= manifest.sprites.goldcoin;
  if (manifest.byUnit) {
    manifest.byUnit.zombie ??= manifest.byUnit.normal;
    manifest.byUnit.polevaulter ??= manifest.byUnit.pole;
    manifest.byUnit.coin ??= manifest.byUnit.goldcoin;
  }
  for (const kind of kinds) if (!manifest.sprites[kind]) throw new Error(`缺少经典角色素材：${kind}`);
  const art: Art = { manifest, images: new Map(), native: new Map(), loaded: new Set(), pending: new Map(), drawnPoses: [], base, scene };
  const sceneMap = manifest.maps?.[scene];
  if (sceneMap) { manifest.background = sceneMap.background; manifest.backgroundCrop = sceneMap.backgroundCrop; manifest.lawn = sceneMap.lawn; }
  const shared = ["shovel", "seedpacket", "seedbank", "shadow"].map(key => manifest.images?.[key]).filter((p): p is string => !!p);
  const music = manifest.music?.[scene]?.tracks.filter(track => track.required !== false).map(track => track.src) ?? [];
  const paths = [...new Set([manifest.background, ...(manifest.byScene?.[scene] ?? []), ...shared, ...music, ...unitPaths(art, [...kinds, "mower", "sun"])])];
  let done = 0;
  await Promise.all(paths.map(async path => {
    await loadAsset(art, path, signal);
    progress(10 + ++done / paths.length * 75, "加载场景、角色与原版音乐");
  }));
  return art;
}

function unitPaths(art: Art, kinds: string[]) {
  return [...new Set(kinds.flatMap(kind => {
    const definition = art.manifest.sprites[kind];
    if (!definition) return [];
    return art.manifest.byUnit?.[kind] ?? (definition.native ? [definition.native.src, ...definition.native.images] : [definition.src, ...Object.values(definition.animations ?? {}).map(a => a.src)]).filter((p): p is string => !!p);
  }))];
}

async function loadAsset(art: Art, path: string, signal?: AbortSignal) {
  if (art.loaded.has(path)) return;
  const previous = art.pending.get(path);
  if (previous) return previous;
  const native = Object.values(art.manifest.sprites).some(definition => definition.native?.src === path);
  const music = Object.values(art.manifest.music ?? {}).flatMap(scene => scene.tracks).find(track => track.src === path);
  if (music || /\.(flac|ogg|wav|mp3)$/i.test(path)) {
    const task = (async () => {
      const response = await fetch(assetURL(art.base, path), { signal });
      if (!response.ok) throw new Error(`音乐加载失败：${path}`);
      const bytes = await response.arrayBuffer();
      if (music?.bytes !== undefined && bytes.byteLength !== music.bytes) throw new Error("原版音乐文件不完整");
      if (music?.sha256) {
        const hash = [...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes))].map(n => n.toString(16).padStart(2, "0")).join("");
        if (hash !== music.sha256) throw new Error("原版音乐与资源清单不匹配");
      }
      art.loaded.add(path);
    })().finally(() => art.pending.delete(path));
    art.pending.set(path, task); return task;
  }
  if (native) {
    const task = (async () => {
      const response = await fetch(assetURL(art.base, path), { signal });
      if (!response.ok) throw new Error(`动画加载失败：${path}`);
      const bytes = new Uint8Array(await response.arrayBuffer());
      const decoded = bytes[0] === 0x1f && bytes[1] === 0x8b ? await new Response(new Blob([bytes]).stream().pipeThrough(new DecompressionStream("gzip"))).text() : new TextDecoder().decode(bytes);
      const data = JSON.parse(decoded) as NativeAnimation;
      if (!data.clips || !Object.values(data.clips).every(clip => Array.isArray(clip.frames) && Number.isFinite(clip.fps))) throw new Error(`动画格式无效：${path}`);
      art.native.set(path, data); art.loaded.add(path);
    })().finally(() => art.pending.delete(path));
    art.pending.set(path, task); return task;
  }
  const task = new Promise<void>((resolve, reject) => {
    const image = new Image(); image.crossOrigin = "anonymous";
    const abort = () => { image.src = ""; reject(new DOMException("Aborted", "AbortError")); };
    if (signal?.aborted) { abort(); return; }
    signal?.addEventListener("abort", abort, { once: true });
    image.onload = () => { signal?.removeEventListener("abort", abort); art.images.set(path, image); art.loaded.add(path); resolve(); };
    image.onerror = () => { signal?.removeEventListener("abort", abort); reject(new Error(`图片加载失败：${path}`)); };
    image.src = assetURL(art.base, path);
  }).finally(() => art.pending.delete(path));
  art.pending.set(path, task);
  return task;
}

export async function ensureUnits(art: Art, kinds: string[], signal?: AbortSignal) {
  for (const kind of kinds) if (!art.manifest.sprites[kind]) throw new Error(`缺少经典角色素材：${kind}`);
  await Promise.all(unitPaths(art, kinds).map(path => loadAsset(art, path, signal)));
}
export async function ensureMusic(art: Art, src: string) { await loadAsset(art, src); }
export function unitReady(art: Art, kind: string) { return !!art.manifest.sprites[kind] && unitPaths(art, [kind]).every(path => art.loaded.has(path)); }

export async function previewFrame(art: Art, kind: string, signal?: AbortSignal) {
  const definition = art.manifest.sprites[kind];
  if (!definition) return null;
  const preview = definition.preview ?? definition;
  // A thumbnail may be shared by a deck slot and an open catalog. Retiring one
  // view must not cancel the other view's in-flight image.
  await loadAsset(art, preview.src);
  if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
  return spriteFrame(art, kind, 0, undefined, preview);
}

export function spriteFrame(art: Art, kind: string, time: number, animation?: string, override?: SpriteDefinition) {
  const definition = art.manifest.sprites[kind];
  const sprite = override ?? (animation ? definition?.animations?.[animation] ?? definition : definition);
  if (!sprite) return null;
  const image = art.images.get(sprite.src);
  if (!image) return null;
  const frames = sprite.frames;
  const count = Array.isArray(frames) ? frames.length : frames ?? 1;
  const elapsed = Math.floor(Math.max(0, time) * (sprite.fps ?? 12) / 1000);
  const index = sprite.loop === false ? Math.min(count - 1, elapsed) : elapsed % Math.max(1, count);
  const w = sprite.frameWidth ?? image.naturalWidth;
  const h = sprite.frameHeight ?? image.naturalHeight;
  const frame = Array.isArray(frames) ? frames[index] : { x: (index % (sprite.columns ?? count)) * w, y: Math.floor(index / (sprite.columns ?? count)) * h, w, h };
  return { sprite, image, frame };
}

export function drawSprite(context: CanvasRenderingContext2D, art: Art, kind: string, time: number, x: number, y: number, scale = 1, animation?: string, override?: SpriteDefinition) {
  const definition = art.manifest.sprites[kind];
  if (!override && definition?.native) return drawNative(context, art, definition, time, x, y, scale, animation);
  const selected = spriteFrame(art, kind, time, animation, override);
  if (!selected) return false;
  const { image, frame, sprite } = selected;
  const w = frame.w * scale, h = frame.h * scale;
  const ax = sprite.anchorX ?? frame.w / 2, ay = sprite.anchorY ?? frame.h;
  context.drawImage(image, frame.x, frame.y, frame.w, frame.h, x - ax * scale, y - ay * scale, w, h);
  return true;
}

export function hasAnimation(art: Art, kind: string, animation: string) {
  const definition = art.manifest.sprites[kind];
  return !!definition?.animations?.[animation] || !!(definition?.native && art.native.get(definition.native.src)?.clips[animation]);
}
export function animationDuration(art: Art, kind: string, animation: string) {
  const definition = art.manifest.sprites[kind];
  const native = definition?.native && art.native.get(definition.native.src)?.clips[animation];
  if (native) return native.frames.length / native.fps * 1000;
  const clip = definition?.animations?.[animation];
  if (!clip) return 0;
  return (Array.isArray(clip.frames) ? clip.frames.length : clip.frames ?? 1) / (clip.fps ?? 12) * 1000;
}

const poses = new WeakMap<Art, { frames: Map<string, HTMLCanvasElement>; bytes: number }>();
function drawNative(context: CanvasRenderingContext2D, art: Art, definition: SpriteDefinition, time: number, x: number, y: number, scale: number, animation = "idle") {
  const data = art.native.get(definition.native!.src);
  if (!data) return false;
  const name = data.clips[animation] ? animation : data.clips.idle ? "idle" : Object.keys(data.clips)[0];
  const clip = data.clips[name]; if (!clip?.frames.length) return false;
  const frame = Math.floor(Math.max(0, time) * clip.fps / 1000);
  const index = clip.loop ? frame % clip.frames.length : Math.min(clip.frames.length - 1, frame);
  if (art.drawnPoses.length < 256 && (!clip.loop || ["attack", "eating"].includes(name) || name.startsWith("boss-"))) art.drawnPoses.push({ kind: data.kind ?? definition.native!.src, clip: name, index, frames: clip.frames.length, time, x, y });
  const layers = clip.frames[index], ax = definition.anchorX ?? 0, ay = definition.anchorY ?? 0;
  const compose = (target: CanvasRenderingContext2D) => {
    for (const layer of layers) {
      const image = art.images.get(layer.src); if (!image) continue;
      target.save(); target.globalAlpha *= layer.alpha ?? 1; target.transform(...layer.m); target.drawImage(image, 0, 0); target.restore();
    }
  };
  const bounds = clip.bounds ?? definition.bounds;
  if (bounds && bounds.width > 0 && bounds.height > 0) {
    let cache = poses.get(art); if (!cache) { cache = { frames: new Map(), bytes: 0 }; poses.set(art, cache); }
    const key = `${definition.native!.src}:${name}:${index}`;
    let pose = cache.frames.get(key);
    if (!pose) {
      const bx = Math.floor(bounds.x), by = Math.floor(bounds.y);
      pose = document.createElement("canvas"); pose.width = Math.ceil(bounds.x + bounds.width) - bx; pose.height = Math.ceil(bounds.y + bounds.height) - by;
      const target = pose.getContext("2d")!; target.translate(-bx, -by); compose(target); cache.frames.set(key, pose); cache.bytes += pose.width * pose.height * 4;
      while (cache.frames.size > 1 && (cache.frames.size > 96 || cache.bytes > 32 * 1024 * 1024)) {
        const first = cache.frames.keys().next().value!, previous = cache.frames.get(first)!;
        cache.bytes -= previous.width * previous.height * 4; cache.frames.delete(first);
      }
    } else { cache.frames.delete(key); cache.frames.set(key, pose); }
    context.drawImage(pose, x + (Math.floor(bounds.x) - ax) * scale, y + (Math.floor(bounds.y) - ay) * scale, pose.width * scale, pose.height * scale);
  } else {
    context.save(); context.translate(x - ax * scale, y - ay * scale); context.scale(scale, scale); compose(context); context.restore();
  }
  return true;
}
import type { MusicTheme } from "./music-state";
