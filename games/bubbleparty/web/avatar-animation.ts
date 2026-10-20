export type AvatarFrame = {
  x: number;
  y: number;
  w: number;
  h: number;
  ax: number;
  ay: number;
};
export type AvatarClip = {
  red: string;
  blue: string;
  frames: AvatarFrame[];
  sequence: number[][];
};
// Original frame order is retained. Its raw time unit is not established;
// this is the presentation timing used by the reconstruction.
export const AVATAR_TICK_MS = 20;
export const clipDuration = (clip: AvatarClip) =>
  clip.sequence.reduce((n, p) => n + p[1] * AVATAR_TICK_MS, 0);
export function frameAt(
  clip: AvatarClip,
  elapsed: number,
  loop = true,
): number {
  const duration = clipDuration(clip);
  let at = loop
    ? Math.max(0, elapsed) % duration
    : Math.min(Math.max(0, elapsed), duration - 1);
  for (const [frame, weight] of clip.sequence) {
    const ms = weight * AVATAR_TICK_MS;
    if (at < ms) return frame;
    at -= ms;
  }
  return clip.sequence[clip.sequence.length - 1][0];
}
export const normalizeAvatar = (value?: number) =>
  Number.isInteger(value) && value! >= 0 && value! < 4 ? value! : 0;
export const avatarSourceCandidates = (base: string, file: string) => {
  const local = `/bubbleparty/v3/avatars/${file}`;
  return base ? [`${base.replace(/\/$/, "")}${local}`, local] : [local];
};
