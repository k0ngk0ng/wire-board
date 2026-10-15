import type { Car, Point, Track } from "./types";
import type { Drive } from "./input";
import { clamp } from "./presentation.ts";
export function roadDistance(track: Pick<Track, "points">, p: Point) {
  let best = Infinity;
  for (let i = 0; i < track.points.length; i++) {
    const a = track.points[i],
      b = track.points[(i + 1) % track.points.length],
      dx = b.x - a.x,
      dy = b.y - a.y;
    const t = clamp(
      ((p.x - a.x) * dx + (p.y - a.y) * dy) / (dx * dx + dy * dy || 1),
      0,
      1,
    );
    best = Math.min(best, Math.hypot(p.x - a.x - t * dx, p.y - a.y - t * dy));
  }
  return best;
}
// Cosmetic, bounded replay of the driving equations. Authoritative collisions,
// checkpoints, laps and results always remain the server's values.
export function predictCar(
  car: Car,
  input: Drive,
  track: Track,
  ms: number,
  serverNow: number,
): Car {
  const c = { ...car };
  if (c.finishAt || c.resetUntil > serverNow) return c;
  let remaining = clamp(ms, 0, 250) / 1000;
  while (remaining > 0) {
    const dt = Math.min(0.02, remaining);
    remaining -= dt;
    let forward = c.vx * Math.cos(c.angle) + c.vy * Math.sin(c.angle);
    c.angle +=
      input.steer *
      (1.65 + 0.002 * Math.abs(forward)) *
      Math.min(Math.abs(forward) / 65, 1) *
      (input.drift ? 1.18 : 1) *
      dt;
    const cos = Math.cos(c.angle),
      sin = Math.sin(c.angle);
    forward = c.vx * cos + c.vy * sin;
    let side = -c.vx * sin + c.vy * cos;
    c.offroad = roadDistance(track, c) > track.roadWidth / 2;
    forward += input.throttle * 245 * dt;
    forward *= Math.exp(-(c.offroad ? 2.5 : 0.55) * dt);
    if (input.brake) forward *= Math.exp(-6 * dt);
    const limit = c.offroad ? 110 : 310;
    forward = clamp(forward, 0, limit);
    side *= Math.exp(-(c.offroad ? 4 : input.drift ? 1.35 : 9) * dt);
    c.vx = forward * cos - side * sin;
    c.vy = forward * sin + side * cos;
    const velocity = Math.hypot(c.vx, c.vy);
    if (velocity > limit) {
      c.vx *= limit / velocity;
      c.vy *= limit / velocity;
    }
    c.x = clamp(c.x + c.vx * dt, 13, track.width - 13);
    c.y = clamp(c.y + c.vy * dt, 13, track.height - 13);
    c.speed = Math.hypot(c.vx, c.vy);
  }
  return c;
}
