import {
  characters,
  normalizeCharacter,
  characterSourceCandidates,
} from "./character-definitions.ts";
import { clipDuration, frameAt, type AvatarClip } from "./avatar-animation.ts";
export type CharacterClipKey =
  "up" | "down" | "left" | "right" | "idle" | "death";
type Atlas = {
  failed?: boolean;
  images: Partial<Record<CharacterClipKey, HTMLImageElement>>;
  promise: Promise<{ loaded: number; failed: CharacterClipKey[] }>;
};
const atlases = new Map<string, Atlas>();
const identity = (base: string, id: number, team: number) =>
  `${base}:${normalizeCharacter(id)}:${team === 1 ? 1 : 0}`;
export function loadCharacter(
  base: string,
  id: number,
  team: number,
  retryFailed = false,
): Atlas["promise"] {
  const key = identity(base, id, team),
    cached = atlases.get(key);
  if (cached && (!retryFailed || !cached.failed)) return cached.promise;
  const images: Atlas["images"] = {};
  const atlas: Atlas = {
    images,
    promise: Promise.resolve({ loaded: 0, failed: [] }),
  };
  atlases.set(key, atlas);
  const clips = characters[normalizeCharacter(id)].clips;
  atlas.promise = Promise.all(
    (Object.keys(clips) as CharacterClipKey[]).map(async (kind) => {
      if (typeof Image === "undefined") return kind;
      const file = clips[kind][team === 1 ? "blue" : "red"];
      for (const url of characterSourceCandidates(base, file)) {
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
  ).then((results) => {
    atlas.failed = results.some((x) => x !== null);
    return {
      loaded: results.filter((x) => x === null).length,
      failed: results.filter((x): x is CharacterClipKey => x !== null),
    };
  });
  return atlas.promise;
}
export function getCharacterImage(
  base: string,
  id: number,
  team: number,
  kind: CharacterClipKey,
): HTMLImageElement | null {
  const image = atlases.get(identity(base, id, team))?.images[kind];
  return image?.complete && image.naturalWidth > 0 ? image : null;
}
export const characterDeathDuration = (id?: number) =>
  clipDuration(characters[normalizeCharacter(id)].clips.death);
export type CharacterPose = {
  character?: number;
  team: number;
  dir: string;
  moving: boolean;
  trapped: boolean;
  dead: boolean;
  deathAge: number;
  now: number;
  reduced: boolean;
};
export function drawCharacterArt(
  ctx: CanvasRenderingContext2D,
  base: string,
  pose: CharacterPose,
  x: number,
  groundY: number,
  scale = 0.6,
): boolean {
  const id = normalizeCharacter(pose.character),
    character = characters[id];
  let kind: CharacterClipKey;
  if (pose.dead || pose.trapped) kind = "death";
  else if (!pose.moving && pose.dir === "down") kind = "idle";
  else
    kind = (
      ["up", "down", "left", "right"].includes(pose.dir) ? pose.dir : "down"
    ) as CharacterClipKey;
  const clip: AvatarClip = character.clips[kind];
  let index = 0;
  if (pose.trapped)
    index = pose.reduced
      ? 0
      : character.lockedFrames[
          Math.floor(pose.now / 400) % character.lockedFrames.length
        ];
  else if (pose.dead) index = frameAt(clip, pose.deathAge, false);
  else if (!pose.reduced && (pose.moving || kind === "idle"))
    index = frameAt(clip, pose.now, true);
  const image = getCharacterImage(base, id, pose.team, kind);
  if (!image) {
    void loadCharacter(base, id, pose.team);
    return false;
  }
  const f = clip.frames[index] ?? clip.frames[0];
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
export function drawCharacterFallback(
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
  const character = normalizeCharacter(id);
  if (character === 2 || character === 6) {
    for (const dx of [-10, 10]) {
      ctx.fillStyle = character === 6 ? color : "#f4f0e4";
      ctx.beginPath();
      ctx.arc(dx, -29, character === 6 ? 4 : 5, 0, Math.PI * 2);
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
  if (character === 1) {
    ctx.strokeStyle = "#e3d9bd";
    ctx.lineWidth = 2.5;
    ctx.strokeRect(-10, -27, 8, 5);
    ctx.strokeRect(2, -27, 8, 5);
  }
  if (character === 3) {
    ctx.fillStyle = "#f8ecce";
    ctx.fillRect(-2, -31, 4, 13);
  }
  if (character === 4 || character === 5) {
    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.moveTo(-14, -22);
    ctx.quadraticCurveTo(0, -42, 14, -22);
    ctx.lineTo(11, -16);
    ctx.lineTo(8, -25);
    ctx.lineTo(-8, -25);
    ctx.lineTo(-11, -16);
    ctx.closePath();
    ctx.fill();
    if (character === 5) {
      ctx.strokeStyle = "#f5f1df";
      ctx.lineWidth = 2;
      ctx.strokeRect(-8, -16, 7, 6);
      ctx.strokeRect(1, -16, 7, 6);
    } else {
      ctx.fillStyle = "#f1ce75";
      ctx.beginPath();
      ctx.moveTo(-8, -5);
      ctx.lineTo(8, -5);
      ctx.lineTo(13, 0);
      ctx.lineTo(-13, 0);
      ctx.closePath();
      ctx.fill();
    }
  }
  if (character === 6) {
    ctx.fillStyle = "#87d7d4";
    ctx.beginPath();
    ctx.moveTo(-6, -7);
    ctx.lineTo(6, -7);
    ctx.lineTo(0, -1);
    ctx.closePath();
    ctx.fill();
    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.moveTo(-10, -6);
    ctx.lineTo(-16, 0);
    ctx.lineTo(-6, 0);
    ctx.closePath();
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(10, -6);
    ctx.lineTo(16, 0);
    ctx.lineTo(6, 0);
    ctx.closePath();
    ctx.fill();
  }
  if (character === 7) {
    ctx.fillStyle = "#f4f0e4";
    ctx.fillRect(-12, -23, 24, 4);
    ctx.fillRect(-10, -5, 20, 5);
  }
  if (character === 8 || character === 9) {
    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.moveTo(-15, -24);
    ctx.lineTo(0, -42);
    ctx.lineTo(15, -24);
    ctx.closePath();
    ctx.fill();
    ctx.fillStyle = "#f4f0e4";
    ctx.fillRect(-14, -25, 28, 4);
  }
  ctx.fillStyle = color;
  ctx.fillRect(-8, -3, 6, 3);
  ctx.fillRect(2, -3, 6, 3);
  ctx.restore();
}
