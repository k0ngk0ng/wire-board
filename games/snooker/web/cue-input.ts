import type { Vec } from "./types";

export function pullPower(
  origin: Vec,
  point: Vec,
  angle: number,
  portrait: boolean,
  limit: number,
): number {
  const dx = portrait ? -Math.sin(angle) : Math.cos(angle),
    dy = portrait ? Math.cos(angle) : Math.sin(angle);
  const distance = (origin.x - point.x) * dx + (origin.y - point.y) * dy;
  return distance <= 6
    ? 0
    : Math.min(1, 0.02 + (0.98 * (distance - 6)) / Math.max(1, limit - 6));
}
export function cueTip(side: number, top: number) {
  const length = Math.hypot(side, top),
    scale = length > 0.9 ? 0.9 / length : 1;
  return { side: side * scale, top: top * scale };
}
