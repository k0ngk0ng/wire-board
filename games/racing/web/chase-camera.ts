import type { Car } from "./types";
export type Vec3 = { x: number; y: number; z: number };
export type ChasePose = { eye: Vec3; target: Vec3; fov: number; near: number };
const finite = (n: number, fallback = 0) => (Number.isFinite(n) ? n : fallback);
export function chasePose(
  car: Pick<Car, "x" | "y" | "angle" | "speed">,
  rear: boolean,
  aspect: number,
): ChasePose {
  const heading = finite(car.angle),
    speed = Math.max(0, Math.min(310, finite(car.speed))) / 310;
  const sign = rear ? -1 : 1,
    dx = Math.cos(heading) * sign,
    dz = Math.sin(heading) * sign;
  const distance = aspect < 1 ? 90 : 92;
  return {
    eye: {
      x: finite(car.x) - dx * distance,
      y: 30 + speed * 3,
      z: finite(car.y) - dz * distance,
    },
    target: { x: finite(car.x) + dx * 110, y: 12, z: finite(car.y) + dz * 110 },
    fov: aspect < 1 ? 73 : 64 + speed * 4,
    near: 2,
  };
}
export function projectPoint(
  p: Vec3,
  c: ChasePose,
  width: number,
  height: number,
): { x: number; y: number; depth: number } | null {
  const fx = c.target.x - c.eye.x,
    fy = c.target.y - c.eye.y,
    fz = c.target.z - c.eye.z,
    fl = Math.hypot(fx, fy, fz);
  const f = { x: fx / fl, y: fy / fl, z: fz / fl },
    rl = Math.hypot(f.z, f.x),
    r = { x: -f.z / rl, y: 0, z: f.x / rl };
  const u = {
    x: r.z * f.y - r.y * f.z,
    y: r.x * f.z - r.z * f.x,
    z: r.y * f.x - r.x * f.y,
  };
  // Screen vertical points downward, opposite the camera up vector.
  const dx = p.x - c.eye.x,
    dy = p.y - c.eye.y,
    dz = p.z - c.eye.z,
    depth = dx * f.x + dy * f.y + dz * f.z;
  if (depth < c.near) return null;
  const lens = height / (2 * Math.tan((c.fov * Math.PI) / 360));
  return {
    x: width / 2 + ((dx * r.x + dy * r.y + dz * r.z) * lens) / depth,
    y: height / 2 + ((dx * u.x + dy * u.y + dz * u.z) * lens) / depth,
    depth,
  };
}
export function automaticGear(speed: number): number {
  const s = Math.max(0, finite(speed));
  return 1 + [50, 105, 165, 225, 280].filter((n) => s >= n).length;
}
export class RearViewHold {
  private held = new Set<string>();
  get active() {
    return this.held.size > 0;
  }
  press(source: string) {
    this.held.add(source);
  }
  release(source: string) {
    this.held.delete(source);
  }
  clear() {
    this.held.clear();
  }
}

export function clipPolygon(points: Vec3[], pose: ChasePose): Vec3[] {
  const dx = pose.target.x - pose.eye.x,
    dy = pose.target.y - pose.eye.y,
    dz = pose.target.z - pose.eye.z,
    l = Math.hypot(dx, dy, dz),
    near = pose.near + 0.01;
  const depth = (p: Vec3) =>
    ((p.x - pose.eye.x) * dx +
      (p.y - pose.eye.y) * dy +
      (p.z - pose.eye.z) * dz) /
    l;
  const out: Vec3[] = [];
  for (let i = 0; i < points.length; i++) {
    const p = points[i],
      q = points[(i + 1) % points.length],
      a = depth(p),
      b = depth(q);
    if (a >= near) out.push({ ...p });
    if (a >= near !== b >= near) {
      const t = (near - a) / (b - a);
      out.push({
        x: p.x + (q.x - p.x) * t,
        y: p.y + (q.y - p.y) * t,
        z: p.z + (q.z - p.z) * t,
      });
    }
  }
  return out;
}
