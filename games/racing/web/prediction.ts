import { MAX_ROAD_SPEED, ROAD_DRAG } from "./dynamics.ts";
import type { Car, Point, Track } from "./types";
import { brakeLevel, type Drive } from "./input.ts";
import { clamp } from "./presentation.ts";
import { classicVehicle, stepClassicMotion, type ClassicMotionState } from "./classic-motion.ts";
import { classicRoadAt, classicRoadSurface, collideClassicRoad, type ClassicRuleTrack, type ClassicPropWorld } from "./classic-road.ts";
import { classicTerrainSurface, classicTerrainAt } from "./classic-terrain.ts";

type PredictionDrive = Drive & { protected?: boolean; extrapolated?: boolean };

function predictClassicCar(car: Car, input: PredictionDrive, track: ClassicRuleTrack, ms: number, serverNow: number, props?: ClassicPropWorld): Car {
  const c = { ...car };
  // Speculative contact is local to this replay. Only server snapshots update
  // confirmed world visibility and future authoritative collision filtering.
  const predictedProps = props ? { ...props, broken: props.broken.slice() } : undefined;
  const vehicle = classicVehicle(c.vehicle ?? "p911");
  let motion: ClassicMotionState = {
    x: c.x / 10, z: c.y / 10, vx: c.vx / 10, vz: c.vy / 10, angle: c.angle,
    yawRate: 0, steering: 0, longAccel: 0, lateralAccel: 0,
    rpm: c.rpm ?? vehicle.idleRPM, gear: c.gear ?? 1, shiftTimer: 0,
    throttle: 0, brake: 0, handbrake: 0, reverseTimer: 0, ...c.motion,
  };
  motion.x = c.x / 10; motion.z = c.y / 10;
  motion.vx = c.vx / 10; motion.vz = c.vy / 10; motion.angle = c.angle;
  let remaining = clamp(ms, 0, 250) / 1000;
  const protect = input.protected || (input.extrapolated && serverNow - c.lastInputAt > 750);
  while (remaining > 1e-7) {
    const dt = Math.min(.02, remaining);
    remaining -= dt;
    let road = classicRoadAt(track, c.x, c.y, c.height ?? 0, c.routeIndex ?? -1);
    if (protect) { motion.throttle = 0; motion.brake = 1; motion.reverseTimer = 0; }
    motion = stepClassicMotion(motion, {
      steer: protect ? 0 : input.steer,
      throttle: protect ? 0 : input.throttle,
      brakeAmount: protect ? 1 : brakeLevel(input),
      handbrake: protect ? false : input.drift,
      allowReverse: !protect,
    }, vehicle, classicTerrainSurface(track.terrain,c,classicRoadSurface(road, motion.angle)), dt);
    let ground=classicTerrainAt(track.terrain,motion.x*10,motion.z*10,c.height??road.height);
    road = classicRoadAt(track, motion.x * 10, motion.z * 10, ground.found?ground.height:c.height??road.height, road.index);
    const groundHeight=ground.found?ground.height:road.height,beforeCollisionX=motion.x,beforeCollisionZ=motion.z;
    motion = collideClassicRoad(track, motion, c.vehicle ?? "p911", groundHeight, predictedProps);
    c.x = motion.x * 10; c.y = motion.z * 10;
    c.vx = motion.vx * 10; c.vy = motion.vz * 10; c.angle = motion.angle;
    if(motion.x!==beforeCollisionX||motion.z!==beforeCollisionZ){
      ground=classicTerrainAt(track.terrain,c.x,c.y,groundHeight);
      road = classicRoadAt(track, c.x, c.y, ground.found?ground.height:groundHeight, road.index);
    }
    c.height = ground.found?ground.height:road.height;
    c.routeIndex = road.index;
    const normal=ground.found?ground:road;
    c.pitch = Math.atan(-(normal.nx * Math.cos(c.angle) + normal.nz * Math.sin(c.angle)) / Math.max(.1, normal.ny));
    c.roll = Math.atan(-(normal.nx * Math.sin(c.angle) - normal.nz * Math.cos(c.angle)) / Math.max(.1, normal.ny));
    c.offroad = road.side > road.right || road.side < -road.left;
  }
  return { ...c, motion, rpm: motion.rpm, gear: motion.gear, speed: Math.hypot(c.vx, c.vy),
    steer: input.steer, throttle: input.throttle, brake: input.brake, brakeAmount: brakeLevel(input), drift: input.drift };
}
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
  input: PredictionDrive,
  track: Track,
  ms: number,
  serverNow: number,
  props?: ClassicPropWorld,
): Car {
  const c = { ...car };
  const braking = brakeLevel(input);
  if (c.finishAt || c.resetUntil > serverNow) return c;
  if (c.vehicle && track.version === 3 && track.road && track.barriers)
    return predictClassicCar(c, input, track as ClassicRuleTrack, ms, serverNow, props);
  let remaining = clamp(ms, 0, 250) / 1000;
  while (remaining > 0) {
    const dt = Math.min(0.02, remaining);
    remaining -= dt;
    let forward = c.vx * Math.cos(c.angle) + c.vy * Math.sin(c.angle);
    c.angle +=
      ((input.steer * 1.65) / (1 + (Math.abs(forward) / 220) ** 2)) *
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
