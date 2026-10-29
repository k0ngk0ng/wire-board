import { MAX_ROAD_SPEED, ROAD_DRAG } from "./dynamics.ts";
import type { Car, Point, Track } from "./types";
import { brakeLevel, type Drive } from "./input.ts";
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
  const braking = brakeLevel(input);
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
    forward += (braking > 0 ? 0 : input.throttle) * 245 * dt;
    forward *= Math.exp(-(c.offroad ? 2.5 : ROAD_DRAG) * dt);
    if (input.drift) forward *= Math.exp(-1.2 * dt);
    if (braking > 0) forward *= Math.exp(-6 * braking * dt);
    const limit = c.offroad ? 110 : MAX_ROAD_SPEED;
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
    for (const o of track.props ?? []) {
      let dx = o.x - c.x,
        dy = o.y - c.y;
      if (Math.hypot(dx, dy) > o.radius + 23) continue;
      const along = clamp(dx * cos + dy * sin, -10, 10);
      dx = c.x + cos * along - o.x;
      dy = c.y + sin * along - o.y;
      const distance = Math.hypot(dx, dy),
        depth = o.radius + 13 - distance;
      if (depth <= 0) continue;
      const nx = distance > 0.00001 ? dx / distance : -cos,
        ny = distance > 0.00001 ? dy / distance : -sin;
      c.x += nx * depth;
      c.y += ny * depth;
      const inward = c.vx * nx + c.vy * ny;
      if (inward < 0) {
        c.vx -= inward * nx * 1.1;
        c.vy -= inward * ny * 1.1;
      }
    }
    c.speed = Math.hypot(c.vx, c.vy);
  }
  return {
    ...c,
    steer: input.steer,
    throttle: input.throttle,
    brake: input.brake,
    brakeAmount: braking,
    drift: input.drift,
  };
}
