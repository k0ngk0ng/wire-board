import source from "./art.json";
import hd from "./hd.json";
import appearances from "./avatar-art.json";
import projectiles from "./projectile-art.json";
import elite from "./elite-art.json";
import type { View, PetDefinition } from "./types";

export const resourceRoot = "https://cdn.ichenj.com/stoneage";
export const assetURL = (file: string) =>
  `${resourceRoot}/${enhancedResources[file] ? hd.basePath : appearances.basePath}/${file}`;
export const imagePixelScale = hd.pixelScale;
export type SpriteFrame = {
  file: string;
  x: number;
  y: number;
  width: number;
  height: number;
  sound: number;
};
export type SpriteAction = {
  direction: number;
  action: number;
  ticks: number;
  frames: SpriteFrame[];
};
const art = {
  ...source,
  sprites: { ...source.sprites, ...appearances.sprites, ...elite.sprites },
  resources: {
    ...source.resources,
    ...appearances.resources,
    ...projectiles.resources,
    ...elite.resources,
  },
} as {
  ui: Record<string, { normal: string; pressed: string }>;
  sprites: Record<string, SpriteAction[]>;
  resources: Record<string, { bytes: number; sha256: string }>;
};
export const nativeButtons = art.ui;
export const projectileBitmaps: Record<
  string,
  {
    file: string;
    width: number;
    height: number;
    xoffset: number;
    yoffset: number;
  }
> = projectiles.bitmaps;
export const avatarAppearances = appearances.avatars;
// Selection previews face the viewer; battle directions remain team-specific.
export const previewDirection = 7;
const enhancedResources: Record<string, { bytes: number; sha256: string }> =
  hd.resources;
const resources: Record<string, { bytes: number; sha256: string }> = {
  ...art.resources,
  ...enhancedResources,
};
const images = new Map<string, HTMLImageElement>();
const inFlight = new Map<string, Promise<HTMLImageElement>>();
export const imageFor = (file: string) => images.get(file);
export function spriteAction(graphic: number, direction = 3, action = 3) {
  const actions = art.sprites[String(graphic)] ?? [];
  return (
    actions.find((a) => a.direction === direction && a.action === action) ??
    actions.find((a) => a.direction === direction && a.action === 3)
  );
}
export const petPreview = (pet: PetDefinition) =>
  spriteAction(pet.graphic, previewDirection)?.frames[0]?.file;
export function neededAssets(v: View): string[] {
  const files = new Set<string>();
  if (v.phase === "selection") {
    for (const p of [...v.pool.pets, ...v.pool.rides]) {
      const f = petPreview(p);
      if (f) files.add(f);
    }
    for (const avatar of avatarAppearances) {
      files.add(avatar.preview);
    }
    for (const ride of v.pool.rides)
      for (const rider of ride.riders ?? []) {
        const frame = spriteAction(rider.graphic, previewDirection)?.frames[0];
        if (frame) files.add(frame.file);
      }
    for (const g of [
      ...v.pool.hats,
      ...v.pool.weapons,
      ...v.pool.clothes,
      ...v.pool.accessories,
    ])
      files.add(g.icon);
  } else {
    const weapons = v.players.map(
      (p) => v.pool.weapons.find((w) => w.id === p.loadout?.weapon)?.weaponType,
    );
    for (const [id, frame] of Object.entries(projectileBitmaps))
      if (
        (Number(id) < 25700 && weapons.includes(4)) ||
        (Number(id) >= 25700 && weapons.includes(19))
      )
        files.add(frame.file);
    files.add("battle/battle_00.png");
    for (const pair of Object.values(art.ui)) {
      files.add(pair.normal);
      files.add(pair.pressed);
    }
    const graphics = new Set<number>();
    for (const p of v.players) {
      graphics.add(p.body.graphic);
      graphics.add(p.pet.graphic);
      const ride = v.pool.rides.find((r) => r.id === p.loadout?.ride);
      const rider = ride?.riders?.find((r) => r.avatar === p.loadout?.avatar);
      if (rider) graphics.add(rider.graphic);
    }
    for (const graphic of graphics)
      for (const action of art.sprites[String(graphic)] ?? [])
        for (const frame of action.frames) files.add(frame.file);
  }
  return [...files];
}
export async function loadAssets(
  files: readonly string[],
  signal: AbortSignal,
  progress: (loaded: number, total: number) => void,
) {
  const unique = [...new Set(files)];
  const counts = new Map<string, number>();
  const total = unique.reduce((sum, file) => {
    const r = resources[file];
    if (!r) throw Error(`素材清单缺少 ${file}`);
    return sum + r.bytes;
  }, 0);
  progress(0, total);
  const update = (file: string, bytes: number) => {
    counts.set(file, bytes);
    progress(
      [...counts.values()].reduce((a, b) => a + b, 0),
      total,
    );
  };
  let next = 0;
  await Promise.all(
    Array.from({ length: Math.min(8, unique.length) }, async () => {
      while (next < unique.length) {
        signal.throwIfAborted();
        const file = unique[next++],
          metadata = resources[file];
        if (images.has(file)) {
          update(file, metadata.bytes);
          continue;
        }
        let promise = inFlight.get(file);
        if (!promise) {
          promise = (async () => {
            const response = await fetch(assetURL(file), {
              cache: "force-cache",
              signal: AbortSignal.any([signal, AbortSignal.timeout(30000)]),
            });
            if (!response.ok)
              throw Error(`石器素材加载失败（${response.status}）`);
            const chunks: Uint8Array[] = [];
            let received = 0;
            const reader = response.body?.getReader();
            if (reader) {
              for (;;) {
                const { done, value } = await reader.read();
                if (done) break;
                chunks.push(value);
                received += value.byteLength;
                update(file, received);
              }
            } else {
              const value = new Uint8Array(await response.arrayBuffer());
              chunks.push(value);
              received = value.byteLength;
              update(file, received);
            }
            if (received !== metadata.bytes)
              throw Error("石器素材大小与索引不一致，请重试");
            const bytes = new Uint8Array(received);
            let offset = 0;
            for (const chunk of chunks) {
              bytes.set(chunk, offset);
              offset += chunk.byteLength;
            }
            const digest = [
              ...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)),
            ]
              .map((n) => n.toString(16).padStart(2, "0"))
              .join("");
            if (digest !== metadata.sha256)
              throw Error("石器素材版本与索引不一致，请重试");
            const image = new Image();
            image.crossOrigin = "anonymous";
            image.src = assetURL(file);
            await image.decode();
            signal.throwIfAborted();
            images.set(file, image);
            return image;
          })();
          inFlight.set(file, promise);
          void promise.finally(() => inFlight.delete(file)).catch(() => {});
        }
        await promise;
        signal.throwIfAborted();
        update(file, metadata.bytes);
      }
    }),
  );
  return { loaded: total, total };
}
