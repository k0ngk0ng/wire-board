import definition from "../assets/avatars-v3.json" with { type: "json" };
import {
  avatarSourceCandidates,
  clipDuration,
  frameAt,
  normalizeAvatar,
  type AvatarClip,
} from "./avatar-animation.ts";
export const avatarCatalog = definition;
export type AvatarClipKey = "up" | "down" | "left" | "right" | "idle" | "death";
type Atlas = {
  images: Partial<Record<AvatarClipKey, HTMLImageElement>>;
  promise: Promise<{ loaded: number; failed: AvatarClipKey[] }>;
};
const atlases = new Map<string, Atlas>();
const identity = (base: string, id: number, team: number) =>
  `${base}:${normalizeAvatar(id)}:${team === 1 ? 1 : 0}`;
export const avatarName = (id?: number) =>
  definition.avatars[normalizeAvatar(id)].name;
export function loadAvatar(
  base: string,
  id: number,
  team: number,
): Atlas["promise"] {
  const key = identity(base, id, team),
    cached = atlases.get(key);
  if (cached) return cached.promise;
  const images: Atlas["images"] = {};
  const atlas: Atlas = {
    images,
    promise: Promise.resolve({ loaded: 0, failed: [] }),
  };
  atlases.set(key, atlas);
  const clips = definition.avatars[normalizeAvatar(id)].clips;
  atlas.promise = Promise.all(
    (Object.keys(clips) as AvatarClipKey[]).map(async (kind) => {
      if (typeof Image === "undefined") return kind;
      const file = clips[kind][team === 1 ? "blue" : "red"];
      for (const url of avatarSourceCandidates(base, file)) {
        const image = new Image();
        const ok = await new Promise<boolean>((resolve) => {
          image.onload = () => resolve(image.naturalWidth > 0);
          image.onerror = () => resolve(false);
          image.src = url;
        });
        if (ok) {
          images[kind] = image;
          return null;
        }
      }
      return kind;
    }),
  ).then((results) => ({
    loaded: results.filter((x) => x === null).length,
    failed: results.filter((x): x is AvatarClipKey => x !== null),
  }));
  return atlas.promise;
}
export function getAvatarImage(
  base: string,
  id: number,
  team: number,
  kind: AvatarClipKey,
): HTMLImageElement | null {
  const image = atlases.get(identity(base, id, team))?.images[kind];
  return image?.complete && image.naturalWidth > 0 ? image : null;
}
export const avatarDeathDuration = (id?: number) =>
  clipDuration(definition.avatars[normalizeAvatar(id)].clips.death);
export type AvatarPose = {
  avatar?: number;
  team: number;
  dir: string;
  moving: boolean;
  trapped: boolean;
  dead: boolean;
  deathAge: number;
  now: number;
  reduced: boolean;
};
export function drawAvatarArt(
  ctx: CanvasRenderingContext2D,
  base: string,
  pose: AvatarPose,
  x: number,
  groundY: number,
  scale = 0.6,
): boolean {
  const id = normalizeAvatar(pose.avatar),
    avatar = definition.avatars[id];
  let kind: AvatarClipKey;
  if (pose.dead || pose.trapped) kind = "death";
  else if (!pose.moving && pose.dir === "down") kind = "idle";
  else
    kind = (
      ["up", "down", "left", "right"].includes(pose.dir) ? pose.dir : "down"
    ) as AvatarClipKey;
  const clip: AvatarClip = avatar.clips[kind];
  let index = 0;
  if (pose.trapped)
    index = pose.reduced
      ? 0
      : avatar.lockedFrames[
          Math.floor(pose.now / 400) % avatar.lockedFrames.length
        ];
  else if (pose.dead) index = frameAt(clip, pose.deathAge, false);
  else if (!pose.reduced && (pose.moving || kind === "idle"))
    index = frameAt(clip, pose.now, true);
  const image = getAvatarImage(base, id, pose.team, kind);
  if (!image) {
    void loadAvatar(base, id, pose.team);
    return false;
  }
  const f = clip.frames[index];
  ctx.drawImage(
    image,
    f.x,
    f.y,
    f.w,
    f.h,
    x - f.ax * scale,
    groundY - f.ay * scale,
    f.w * scale,
    f.h * scale,
  );
  return true;
}
export function drawAvatarFallback(
  ctx: CanvasRenderingContext2D,
  id: number | undefined,
  team: number,
  x: number,
  groundY: number,
  scale = 1,
) {
  const color = team === 1 ? "#1765d5" : "#cf263b";
  ctx.save();
  ctx.translate(x, groundY);
  ctx.scale(scale, scale);
  ctx.fillStyle = color;
  ctx.strokeStyle = team === 1 ? "#123a73" : "#6e1824";
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.ellipse(0, -15, 13, 17, 0, 0, Math.PI * 2);
  ctx.fill();
  ctx.stroke();
  const avatar = normalizeAvatar(id);
  if (avatar === 2) {
    for (const dx of [-10, 10]) {
      ctx.fillStyle = "#f4f0e4";
      ctx.beginPath();
      ctx.arc(dx, -29, 5, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
    }
  }
  ctx.fillStyle = "#ffdab0";
  ctx.beginPath();
  ctx.ellipse(0, -12, 9, 7, 0, 0, Math.PI * 2);
  ctx.fill();
  ctx.fillStyle = "#24383d";
  ctx.fillRect(-4, -14, 2, 3);
  ctx.fillRect(3, -14, 2, 3);
  if (avatar === 1) {
    ctx.strokeStyle = "#e3d9bd";
    ctx.lineWidth = 2.5;
    ctx.strokeRect(-10, -27, 8, 5);
    ctx.strokeRect(2, -27, 8, 5);
  }
  if (avatar === 3) {
    ctx.fillStyle = "#f8ecce";
    ctx.fillRect(-2, -31, 4, 13);
  }
  ctx.fillStyle = color;
  ctx.fillRect(-8, -3, 6, 3);
  ctx.fillRect(2, -3, 6, 3);
  ctx.restore();
}
