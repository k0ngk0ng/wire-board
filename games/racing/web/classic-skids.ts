type SkidCar = {
  vx: number; vy: number; angle: number; speed: number;
  brake: boolean; brakeAmount?: number; drift: boolean; offroad: boolean;
  finishAt: number; protected?: boolean; resetUntil: number;
  motion?: { longAccel?: number; brake?: number; handbrake?: number };
};
export type ClassicSkidStrength = { front: number; rear: number; cause: "none" | "brake" | "slip" | "handbrake" };
export type ClassicSkidPoint = { x: number; y: number; z: number; nx: number; ny: number; nz: number };
export type ClassicSkidContact = { wheel: number; front: boolean; width: number; point: ClassicSkidPoint; valid?: boolean; surface?: number };
export type ClassicSkidSegment = { slot: number; seat: number; wheel: number; born: number; strength: number; vertices: number[]; surface?: number };
const clamp = (value: number, low = 0, high = 1) => Math.max(low, Math.min(high, value));
const finite = (value: number | undefined, fallback = 0) => Number.isFinite(value) ? value! : fallback;
const looseSurfaces = new Set([2,3,4,11,5,13,9,15]);

/** Unknown native IDs stay unknown; VROAD distance is not a material label. */
export function classicSkidOnSurface(found: boolean, surface: number): boolean {
  return found && !looseSurfaces.has(surface);
}

/** Motion and applied pedal state, never steering input alone, create rubber. */
export function classicSkidStrength(car: SkidCar, serverNow: number): ClassicSkidStrength {
  const none: ClassicSkidStrength = { front: 0, rear: 0, cause: "none" };
  if (![car.vx,car.vy,car.angle].every(Number.isFinite)) return none;
  const speed = Math.hypot(finite(car.vx), finite(car.vy)) / 10;
  if (speed < 3 || car.protected || car.finishAt || car.resetUntil > serverNow) return none;
  const forward = (car.vx * Math.cos(car.angle) + car.vy * Math.sin(car.angle)) / 10;
  const side = Math.abs((-car.vx * Math.sin(car.angle) + car.vy * Math.cos(car.angle)) / 10);
  const slip = speed >= 5 && side > 1.3 ? clamp((Math.atan2(side, Math.max(1, Math.abs(forward))) - .13) / .25) : 0;
  const brake = finite(car.motion?.brake, car.brake ? (car.brakeAmount ?? 1) : 0);
  const hard = speed >= 5 && brake >= .65 ? clamp((-finite(car.motion?.longAccel) * Math.sign(forward || 1) - 4.5) / 5) * brake : 0;
  const hand = finite(car.motion?.handbrake, car.drift ? 1 : 0);
  if (hand > .25) return { front: hard, rear: Math.max(.3, hand, slip, hard), cause: "handbrake" };
  if (hard > .05) return { front: hard, rear: Math.max(hard, slip), cause: "brake" };
  if (slip > .05) return { front: slip > .7 ? slip * .35 : 0, rear: slip, cause: "slip" };
  return none;
}

/** A six-vertex strip in world decimetres; each end uses its own road normal. */
export function classicSkidQuad(a: ClassicSkidPoint, b: ClassicSkidPoint, width: number): number[] | null {
  const dx = b.x - a.x, dy = b.y - a.y, dz = b.z - a.z;
  if (![...Object.values(a), ...Object.values(b), width].every(Number.isFinite) || Math.hypot(dx, dy, dz) < 1e-5 || width <= 0) return null;
  const sides = (p: ClassicSkidPoint) => {
    let x = dy * p.nz - dz * p.ny, y = dz * p.nx - dx * p.nz, z = dx * p.ny - dy * p.nx;
    const scale = width / (2 * Math.max(1e-8, Math.hypot(x, y, z)));
    x *= scale; y *= scale; z *= scale;
    return [[p.x-x,p.y-y,p.z-z],[p.x+x,p.y+y,p.z+z]];
  };
  const [al,ar] = sides(a), [bl,br] = sides(b);
  return [...al,...ar,...bl,...bl,...ar,...br];
}

/** Bounded shared history for every visible vehicle, with per-wheel continuity. */
export class ClassicSkidTrail {
  readonly capacity: number;
  readonly lifetime: number;
  written = 0;
  private next = 0;
  private readonly marks: Array<ClassicSkidSegment | undefined>;
  private readonly previous = new Map<string, { point: ClassicSkidPoint; at: number }>();
  constructor(capacity = 2048, lifetime = 16000) {
    this.capacity = Math.max(1, Math.floor(capacity)); this.lifetime = lifetime;
    this.marks = new Array(this.capacity);
  }
  sample(seat: number, contacts: readonly ClassicSkidContact[], strength: ClassicSkidStrength, now: number, reduced = false): ClassicSkidSegment[] {
    const created: ClassicSkidSegment[] = [];
    for (const contact of contacts) {
      const key = `${seat}:${contact.wheel}`, level = contact.front ? strength.front : strength.rear;
      if (reduced || contact.valid === false || level <= .05 || !Number.isFinite(now)) { this.previous.delete(key); continue; }
      const old = this.previous.get(key), point = contact.point;
      if (old && now >= old.at && now - old.at <= 250) {
        const distance = Math.hypot(point.x-old.point.x, point.y-old.point.y, point.z-old.point.z);
        if (distance < 1) continue;
        const vertices = distance <= 80 && Math.abs(point.y-old.point.y) <= 30 ? classicSkidQuad(old.point,point,contact.width) : null;
        if (vertices) {
          const mark = { slot: this.next, seat, wheel: contact.wheel, born: now, strength: clamp(level), vertices,
            ...(contact.surface === undefined ? {} : { surface: contact.surface }) };
          this.marks[this.next] = mark; this.next = (this.next + 1) % this.capacity; this.written++;
          created.push(mark);
        }
      }
      this.previous.set(key, { point: { ...point }, at: now });
    }
    return created;
  }
  live(now: number): ClassicSkidSegment[] { return this.marks.filter((mark): mark is ClassicSkidSegment => !!mark && now >= mark.born && now-mark.born < this.lifetime); }
  remove(seat: number) { for (const key of this.previous.keys()) if (key.startsWith(`${seat}:`)) this.previous.delete(key); }
  clear() { this.previous.clear(); this.marks.fill(undefined); this.next = 0; this.written = 0; }
}
