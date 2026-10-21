import rawProjectiles from "../data/projectiles.json" with { type: "json" };
import rawEffects from "../data/projectile-effects.json" with { type: "json" };
import type { Blast, Event, Shot } from "./types";
type NativeFrame = {
  x: number;
  y: number;
  w: number;
  h: number;
  ax: number;
  ay: number;
};
type Sheet = {
  path: string;
  frames: NativeFrame[];
  primaryStart: number;
  specialStart: number;
  directions: number;
  animationFrames: number;
};
const projectiles = rawProjectiles as unknown as Record<string, Sheet>;
const effects = rawEffects as unknown as Record<
  string,
  { path: string; frames: NativeFrame[] }
>;
export const projectileEffectPaths = Object.values(effects).map(
  (sheet) => sheet.path,
);
export function projectilePaths(tank: number) {
  return projectiles[String(tank)] ? [projectiles[String(tank)].path] : [];
}
export function projectileFrame(
  tank: number,
  mode: "primary" | "special",
  angle: number,
  age: number,
  reduced: boolean,
) {
  const sheet = projectiles[String(tank)];
  if (!sheet) return null;
  const direction =
    ((Math.round(-angle / ((Math.PI * 2) / sheet.directions)) %
      sheet.directions) +
      sheet.directions) %
    sheet.directions;
  const phase = reduced
    ? 0
    : Math.floor(Math.max(0, age) / 90) % sheet.animationFrames;
  const index =
    (mode === "special" ? sheet.specialStart : sheet.primaryStart) +
    direction * sheet.animationFrames +
    phase;
  return { ...sheet.frames[index], path: sheet.path, index };
}
export function effectFrame(
  kind: "mine" | "homming" | "ion",
  age: number,
  reduced: boolean,
  beam = false,
) {
  const sheet = effects[kind];
  let index = 0;
  if (kind === "ion")
    index = beam
      ? 10 + (reduced ? 0 : Math.floor(Math.max(0, age) / 60) % 8)
      : Math.min(9, reduced ? 0 : Math.floor(Math.max(0, age) / 70));
  else if (!reduced)
    index =
      Math.floor(Math.max(0, age) / (kind === "mine" ? 90 : 100)) %
      (kind === "mine" ? 4 : 8);
  // The native reticle cycles eight phases, with no image in its final phase.
  const frame = sheet.frames[index];
  return frame ? { ...frame, path: sheet.path, index } : null;
}
export function blastTimestamp(
  blast: Pick<Blast, "x" | "y" | "at">,
  shot: Pick<Shot, "impactAt"> | undefined,
  events: Event[],
) {
  if (blast.at !== undefined) return blast.at;
  const event = [...events]
    .reverse()
    .find(
      (e) =>
        e.type === "explode" &&
        Math.abs((e.x ?? 0) - blast.x) < 2 &&
        Math.abs((e.y ?? 0) - blast.y) < 2,
    );
  return event?.at ?? shot?.impactAt ?? 0;
}
// Behavior descriptions, not claims about the original localized weapon names.
const secondaryDescriptions: Record<number, string> = {
  1: "燃烧弹",
  2: "毒箭",
  3: "红色重炮",
  4: "三发散射",
  5: "毒雾弹",
  6: "锁定导弹",
  7: "九发齐射",
  8: "三束波动光束",
  9: "双发地雷",
  10: "离子炮",
  11: "移动封锁弹",
  12: "双发光弹",
  13: "九发锁定导弹",
  14: "同队范围治疗",
};
export const secondaryDescription = (tank: number) =>
  secondaryDescriptions[tank] ?? "特殊弹";
