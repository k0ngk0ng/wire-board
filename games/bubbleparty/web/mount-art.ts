import definition from "../assets/mounts-v4.json" with { type: "json" };
const normalizeCharacter = (value?: number) =>
  Number.isInteger(value) && value! >= 0 && value! < 10 ? value! : 0;

type Frame = {
  x: number;
  y: number;
  w: number;
  h: number;
  ax: number;
  ay: number;
};
type Clip = { frames: Frame[]; sequence: number[][] };
type Direction = "up" | "down" | "left" | "right";
const images = new Map<string, HTMLImageElement | null>();
const pending = new Map<string, Promise<void>>();
const mounts = new Map(definition.mounts.map((mount) => [mount.id, mount]));
export const mountName = (kind: string) => mounts.get(kind)?.name ?? "";
export const mountSourceCandidates = (base: string, file: string) => {
  const path = `/bubbleparty/v4/mounts/${file}`;
  return base ? [`${base.replace(/\/$/, "")}${path}`, path] : [path];
};
function getImage(base: string, file: string) {
  const key = `${base}:${file}`;
  if (!images.has(key) && !pending.has(key) && typeof Image !== "undefined") {
    pending.set(
      key,
      (async () => {
        for (const url of mountSourceCandidates(base, file)) {
          const image = new Image();
          const ok = await new Promise<boolean>((resolve) => {
            image.onload = () => resolve(image.naturalWidth > 0);
            image.onerror = () => resolve(false);
            image.src = url;
          });
          if (ok) {
            images.set(key, image);
            return;
          }
        }
        images.set(key, null);
      })(),
    );
  }
  return images.get(key) ?? null;
}
function frameAt(clip: Clip, now: number, moving: boolean) {
  if (!moving) return clip.frames[0];
  const duration = clip.sequence.reduce(
    (sum, [, weight]) => sum + weight * definition.tickMS,
    0,
  );
  let remaining = Math.max(0, now) % duration;
  for (const [index, weight] of clip.sequence) {
    if (remaining < weight * definition.tickMS) return clip.frames[index];
    remaining -= weight * definition.tickMS;
  }
  return clip.frames[0];
}
function paint(
  ctx: CanvasRenderingContext2D,
  image: HTMLImageElement,
  f: Frame,
  x: number,
  y: number,
  scale: number,
) {
  ctx.drawImage(
    image,
    f.x,
    f.y,
    f.w,
    f.h,
    x - f.ax * scale,
    y - f.ay * scale,
    f.w * scale,
    f.h * scale,
  );
}
export function drawMountItem(
  ctx: CanvasRenderingContext2D,
  base: string,
  kind: string,
  x: number,
  groundY: number,
  scale = 0.6,
): boolean {
  const mount = mounts.get(kind);
  if (!mount) return false;
  const image = getImage(base, mount.icon.file);
  if (!image) return false;
  paint(ctx, image, mount.icon.frames[0], x, groundY, scale);
  return true;
}
export function drawMountedAvatar(
  ctx: CanvasRenderingContext2D,
  base: string,
  pose: {
    mount: string;
    character?: number;
    team: number;
    dir: string;
    now: number;
    moving: boolean;
    reduced: boolean;
  },
  x: number,
  groundY: number,
  scale = 0.6,
): boolean {
  const mount = mounts.get(pose.mount);
  if (!mount) return false;
  const dir: Direction = ["up", "down", "left", "right"].includes(pose.dir)
    ? (pose.dir as Direction)
    : "down";
  const clip = mount.clips[dir],
    rider = definition.riders[normalizeCharacter(pose.character)].clips[dir];
  const shipImage = getImage(base, clip.file),
    riderImage = getImage(base, pose.team === 1 ? rider.blue : rider.red);
  // Paint both layers together. A missing image must never hide the rider;
  // the caller can retain its existing red/blue avatar fallback.
  if (!shipImage || !riderImage) return false;
  const moving = pose.moving && !pose.reduced;
  paint(ctx, shipImage, frameAt(clip, pose.now, moving), x, groundY, scale);
  paint(ctx, riderImage, frameAt(rider, pose.now, moving), x, groundY, scale);
  return true;
}
