import type { Arc, Fall, Point } from "./types";
const clamp = (v: number) => Math.max(0, Math.min(1, v));
export function arcAt(
  arc: Arc,
  shotAt: number,
  now: number,
  launchAt = shotAt,
): (Point & { angle: number }) | null {
  launchAt = arc.launchAt ?? launchAt;
  const path = arc.path;
  const end = arc.impactAt;
  if (!path.length || !end || now < launchAt || now >= end) return null;
  const elapsed = now - shotAt;
  const times =
    arc.pathTimes?.length === path.length
      ? arc.pathTimes
      : path.map(
          (_, i) =>
            launchAt -
            shotAt +
            ((end - launchAt) * i) / Math.max(1, path.length - 1),
        );
  let i = 1;
  while (i < times.length - 1 && times[i] < elapsed) i++;
  const a = path[Math.max(0, i - 1)] ?? path[0],
    b = path[i] ?? a;
  const t = clamp(
    (elapsed - (times[i - 1] ?? 0)) /
      Math.max(1, (times[i] ?? end - shotAt) - (times[i - 1] ?? 0)),
  );
  return {
    x: a.x + (b.x - a.x) * t,
    y: a.y + (b.y - a.y) * t,
    angle: Math.atan2(b.y - a.y, b.x - a.x),
  };
}
export function fallAt(f: Fall, now: number): Point {
  const t = clamp((now - f.at) / Math.max(1, f.endAt - f.at));
  return {
    x: f.fromX + (f.toX - f.fromX) * t,
    y: f.fromY + (f.toY - f.fromY) * t * t,
  };
}

/** Cancelled LastShot is kept by the server for diagnostics, never playback. */
export function renderableShot<T extends { cancelledAt?: number }>(
  shot: T | undefined,
): T | undefined {
  return shot?.cancelledAt ? undefined : shot;
}
