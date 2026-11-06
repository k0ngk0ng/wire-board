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
};
export type ArtManifest = {
  version?: string;
  background: string;
  backgroundCrop?: { x: number; y: number; width: number; height: number };
  lawn?: { x: number; y: number; width: number; height: number };
  sprites: Record<string, SpriteDefinition>;
  images?: Record<string, string>;
  audio?: Record<string, string | string[]>;
};
export type Art = { manifest: ArtManifest; images: Map<string, HTMLImageElement>; base: string };

export function assetURL(base: string, path: string) {
  if (/^(https?:|data:|blob:)/.test(path)) return path;
  return `${base.replace(/\/$/, "")}/${path.replace(/^\//, "")}`;
}

export async function loadArt(assets: string, signal: AbortSignal, progress: (n: number, label: string) => void): Promise<Art> {
  const base = assetURL(assets, "pvz/classic-v1");
  progress(5, "读取草坪与角色资源");
  const response = await fetch(assetURL(base, "manifest.json"), { signal });
  if (!response.ok) throw new Error(`原版资源清单加载失败（${response.status}）`);
  const manifest = await response.json() as ArtManifest;
  if (!manifest.background || !manifest.sprites) throw new Error("原版资源清单不完整");
  manifest.sprites.zombie ??= manifest.sprites.normal;
  manifest.sprites.polevaulter ??= manifest.sprites.pole;
  for (const kind of ["sunflower", "peashooter", "repeater", "snowpea", "wallnut", "potatomine", "cherrybomb", "tallnut", "zombie", "conehead", "buckethead", "newspaper", "polevaulter", "football"])
    if (!manifest.sprites[kind]?.src) throw new Error(`缺少经典角色资源：${kind}`);
  const paths = [...new Set([manifest.background, ...Object.values(manifest.images ?? {}), ...Object.values(manifest.sprites).flatMap(s => [s.src, ...Object.values(s.animations ?? {}).map(a => a.src)])])];
  const images = new Map<string, HTMLImageElement>();
  let done = 0;
  await Promise.all(paths.map(async path => {
    const image = new Image();
    image.crossOrigin = "anonymous";
    const url = assetURL(base, path);
    await new Promise<void>((resolve, reject) => {
      const abort = () => { image.src = ""; reject(new DOMException("Aborted", "AbortError")); };
      signal.addEventListener("abort", abort, { once: true });
      image.onload = () => { signal.removeEventListener("abort", abort); resolve(); };
      image.onerror = () => { signal.removeEventListener("abort", abort); reject(new Error(`图片加载失败：${path}`)); };
      image.src = url;
    });
    if (signal.aborted) throw new DOMException("Aborted", "AbortError");
    images.set(path, image);
    progress(10 + ++done / paths.length * 75, "加载经典草坪与角色");
  }));
  return { manifest, images, base };
}

export function spriteFrame(art: Art, kind: string, time: number, animation?: string) {
  const definition = art.manifest.sprites[kind];
  const sprite = animation ? definition?.animations?.[animation] ?? definition : definition;
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

export function drawSprite(context: CanvasRenderingContext2D, art: Art, kind: string, time: number, x: number, y: number, scale = 1, animation?: string) {
  const selected = spriteFrame(art, kind, time, animation);
  if (!selected) return false;
  const { image, frame, sprite } = selected;
  const w = frame.w * scale, h = frame.h * scale;
  const ax = sprite.anchorX ?? frame.w / 2, ay = sprite.anchorY ?? frame.h;
  context.drawImage(image, frame.x, frame.y, frame.w, frame.h, x - ax * scale, y - ay * scale, w, h);
  return true;
}
