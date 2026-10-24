import type { Segment, Stroke, Vec, View } from "./types";
import { sampleFlight } from "./timeline.ts";
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v));
function acceleration(v: Vec, spin: number): Vec {
  return {
    x: -0.12 * v.x,
    y: -9.81 - 1.6 * spin * Math.min(1.5, Math.abs(v.z) / 4),
    z: -0.12 * v.z,
  };
}
export function contactPoint(view: View, seat: number, at: number): Vec {
  const dir = (view.options.teams[seat] ^ view.ends) === 0 ? 1 : -1;
  if (
    view.expected === seat &&
    (view.phase === "serve" || view.phase === "serve_toss")
  )
    return { x: dir * 0.34, y: 1.1, z: dir * 1.28 };
  if (view.expected === seat && view.phase === "rally" && view.flight)
    return sampleFlight(
      view.flight,
      Math.max(Math.round(at), view.flight.contactStart),
    );
  return {
    x: dir * (view.paddles?.[seat]?.x ?? 0) * 0.9,
    y: 0.9,
    z: dir * 1.7,
  };
}
export function canReach(
  view: View,
  seat: number,
  stroke: Pick<Stroke, "racketX">,
  at: number,
): boolean {
  if (view.phase === "serve") return true;
  if (!view.flight) return false;
  const point = sampleFlight(
    view.flight,
    Math.max(Math.round(at), view.flight.contactStart),
  );
  const dir = (view.options.teams[seat] ^ view.ends) === 0 ? 1 : -1;
  return Math.abs(point.x - dir * (stroke.racketX ?? 0) * 0.9) <= 0.22;
}

/** A short, display-only launch arc. It contains no score, collision, next turn
 * or result and is discarded as soon as the server supplies the new ball. */
export function predictLaunch(
  view: View,
  seat: number,
  stroke: Stroke,
  at: number,
): Segment {
  const from = view.options.teams[seat] ^ view.ends,
    dir = from === 0 ? 1 : -1,
    serve = view.phase === "serve";
  const contact = serve
    ? Math.round(at) + 180
    : Math.max(Math.round(at), view.flight!.contactStart);
  let kind: string = stroke.kind,
    spin = kind === "chop" ? -0.95 : kind === "block" ? 0.05 : 0.9;
  let p: Vec, v: Vec;
  if (serve) {
    p = { x: dir * 0.34, y: 1.1, z: dir * 1.28 };
    const t = 0.24;
    v = {
      x: -dir * (0.6 + 0.18 * stroke.aim),
      y: (0.78 - p.y + 0.5 * 9.81 * t * t) / t,
      z: -dir * (2.1 + 1.2 * stroke.power),
    };
  } else {
    p = sampleFlight(view.flight!, contact);
    if (kind === "drive" && p.y > 1.18 && stroke.power > 0.86) {
      kind = "smash";
      spin = 0.4;
    }
    let t = 0.86 - 0.3 * stroke.power;
    if (kind === "chop") t += 0.13;
    if (kind === "block") t += 0.05;
    if (kind === "smash") t -= 0.08;
    const timing = clamp(
      (contact - view.flight!.contactStart - 130) / 260,
      -1,
      1,
    );
    const targetX =
        dir *
        (stroke.aim * 0.7625 * (0.92 + 0.17 * stroke.power) +
          timing * stroke.power * 0.25),
      targetZ =
        -dir *
        (0.65 +
          0.48 * stroke.power +
          Math.max(0, timing) * stroke.power * 0.32);
    const vx = (targetX - p.x) / (t - 0.06 * t * t),
      vz = (targetZ - p.z) / (t - 0.06 * t * t),
      a = acceleration({ x: vx, y: 0, z: vz }, spin);
    let vy = (0.78 - p.y - 0.5 * a.y * t * t) / t;
    const incoming = view.flight!.spin;
    if (incoming < -0.3 && kind === "drive")
      vy -= -incoming * Math.abs(timing) * stroke.power * 0.9;
    if (kind === "block") vy -= incoming * 0.18;
    v = { x: vx, y: vy, z: vz };
  }
  const a = acceleration(v, spin);
  let horizon = 0.25;
  // Stop at a possible contact. Never predict a bounce or a net outcome.
  const discriminant = v.y * v.y - 2 * a.y * (p.y - 0.78);
  if (discriminant >= 0) {
    for (const t of [
      (-v.y - Math.sqrt(discriminant)) / a.y,
      (-v.y + Math.sqrt(discriminant)) / a.y,
    ])
      if (t > 0 && v.y + a.y * t < 0) horizon = Math.min(horizon, t);
  }
  if (p.z * v.z < 0) {
    const t = -p.z / v.z;
    const y = p.y + v.y * t + 0.5 * a.y * t * t;
    if (t > 0 && y <= 0.76 + 0.1525 + 0.02) horizon = Math.min(horizon, t);
  }
  return { at: contact, until: contact + horizon * 1000, p, v, a };
}
function authority(view: View, at: number): Vec {
  if (view.flight) return sampleFlight(view.flight, at);
  const dir = (view.options.teams[view.expected] ^ view.ends) === 0 ? 1 : -1;
  const toss =
    view.phase === "serve_toss"
      ? Math.max(
          0,
          Math.sin(clamp((at - view.phaseUntil + 180) / 180, 0, 1) * Math.PI),
        ) * 0.18
      : 0;
  return { x: dir * 0.34, y: 1.1 + toss, z: dir * 1.28 };
}
export class BallDisplay {
  get active(): boolean {
    return !!this.preview;
  }
  private preview?: {
    segment: Segment;
    identity: string;
    epoch: string;
    rally: number;
    shot: number;
  };
  private shown?: Vec;
  private correction?: { x: number; y: number; z: number; at: number };
  start(
    view: View,
    seat: number,
    stroke: Stroke,
    at: number,
    identity: string,
  ) {
    if (!canReach(view, seat, stroke, at)) {
      this.reset();
      return;
    }
    this.preview = {
      segment: predictLaunch(view, seat, stroke, at),
      identity,
      epoch: view.epoch,
      rally: view.rally,
      shot: view.shot,
    };
  }
  reset() {
    this.preview = undefined;
    this.shown = undefined;
    this.correction = undefined;
  }
  sample(view: View, at: number, identity: string): Vec {
    const target = authority(view, at),
      p = this.preview;
    if (p) {
      const reset =
        p.identity !== identity ||
        p.epoch !== view.epoch ||
        p.rally !== view.rally ||
        !["serve", "serve_toss", "rally"].includes(view.phase);
      if (reset) {
        this.reset();
        return target;
      }
      if (view.shot !== p.shot) {
        if (
          this.shown &&
          Math.hypot(
            this.shown.x - target.x,
            this.shown.y - target.y,
            this.shown.z - target.z,
          ) < 0.75
        ) {
          this.correction = {
            x: this.shown.x - target.x,
            y: this.shown.y - target.y,
            z: this.shown.z - target.z,
            at,
          };
        }
        this.preview = undefined;
      } else if (at >= p.segment.at) {
        const position = sampleFlight({ segments: [p.segment] }, at);
        this.shown = position;
        return position;
      }
    }
    const c = this.correction;
    if (c) {
      const factor = Math.exp(-Math.max(0, at - c.at) / 65);
      if (factor < 0.01) this.correction = undefined;
      else {
        target.x += c.x * factor;
        target.y += c.y * factor;
        target.z += c.z * factor;
      }
    }
    this.shown = target;
    return target;
  }
}
