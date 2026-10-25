import type { Segment, Stroke, Vec, View } from "./types";
import { sampleFlight, localPoint } from "./timeline.ts";
import {
  paddleRadiusPixels,
  pointerOverBall,
  paddleAtScreen,
  paddleReachRadius,
} from "./view-space.ts";
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v));
function acceleration(v: Vec, spin: number): Vec {
  return {
    x: -0.12 * v.x,
    y: -9.81 - 1.6 * spin * Math.min(1.5, Math.abs(v.z) / 4),
    z: -0.12 * v.z,
  };
}
type PointerSample = { point: { x: number; y: number }; at: number };
export function sweptPointerContact(
  view: View,
  seat: number,
  from: PointerSample,
  to: PointerSample,
  width: number,
  height: number,
): PointerSample | undefined {
  const duration = to.at - from.at;
  if (
    (view.phase !== "rally" && view.phase !== "serve") ||
    view.expected !== seat ||
    (view.phase === "rally" && !view.flight) ||
    duration <= 0 ||
    duration > 80
  )
    return;
  const distance = Math.hypot(
    (to.point.x - from.point.x) * width,
    (to.point.y - from.point.y) * height,
  );
  if (distance < 1) return;
  const steps = Math.min(
    48,
    Math.max(
      1,
      Math.ceil(duration / 4),
      Math.ceil(distance / (paddleRadiusPixels(width) / 2)),
    ),
  );
  const side = view.options.teams[seat] ^ view.ends;
  const openAt =
    view.phase === "serve" ? view.controls.openAt : view.flight!.contactStart;
  for (let i = 0; i <= steps; i++) {
    const f = i / steps,
      at = from.at + duration * f;
    if (at < openAt || at > view.controls.closeAt) continue;
    const point = {
      x: from.point.x + (to.point.x - from.point.x) * f,
      y: from.point.y + (to.point.y - from.point.y) * f,
    };
    const ball = localPoint(contactPoint(view, seat, at), side);
    if (!pointerOverBall(point, ball, width, height)) continue;
    const z = Math.max(0, Math.min(view.reachDepth ?? 1.7, ball.z)),
      p = paddleAtScreen(point, z, width, height);
    if (
      canReach(
        view,
        seat,
        {
          racketX: p.x / 0.9,
          racketY: p.y,
          racketRadius: paddleReachRadius(point, z, width, height),
        },
        at,
      )
    )
      return { point, at };
  }
}
export function returnWindowOpen(
  view: View,
  at: number,
  motion: { moving: boolean; settled: boolean; touching?: boolean },
): boolean {
  return (
    view.phase === "rally" &&
    !!view.flight &&
    at >= view.flight.contactStart &&
    at <= view.controls.closeAt &&
    (motion.moving || motion.settled || !!motion.touching)
  );
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
  stroke: Pick<Stroke, "racketX" | "racketY" | "racketRadius">,
  at: number,
): boolean {
  if (view.phase === "serve")
    return (
      stroke.racketY === undefined ||
      Math.hypot((stroke.racketX ?? 0) * 0.9 - 0.34, stroke.racketY - 1.1) <=
        (stroke.racketRadius ?? 0.22)
    );
  if (!view.flight) return false;
  const point = sampleFlight(
    view.flight,
    Math.max(Math.round(at), view.flight.contactStart),
  );
  const dir = (view.options.teams[seat] ^ view.ends) === 0 ? 1 : -1;
  return (
    Math.hypot(
      point.x - dir * (stroke.racketX ?? 0) * 0.9,
      point.y - (stroke.racketY ?? point.y),
    ) <= (stroke.racketRadius ?? 0.22)
  );
}

/** A short, display-only launch arc. It contains no score, collision, next turn
 * or result and is discarded as soon as the server supplies the new ball. */
export function predictLaunch(
  view: View,
  seat: number,
  stroke: Stroke,
  at: number,
): Segment & { spin: number } {
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
      z: -dir * (2.0 + 2.1 * stroke.power),
    };
  } else {
    p = sampleFlight(view.flight!, contact);
    if (kind === "drive" && p.y > 1.18 && stroke.power > 0.86) {
      kind = "smash";
      spin = 0.4;
    }
    let t = 0.98 - 0.71 * stroke.power;
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
      targetZ = -dir * (0.95 + Math.max(0, timing) * 0.2);
    if ((stroke.lift ?? 0) > 0.25 && p.y < 1.1 && kind === "drive") {
      const fraction = clamp(Math.abs(p.z / (targetZ - p.z)), 0.04, 0.96),
        baseline = p.y + (0.78 - p.y) * fraction;
      t = Math.max(
        t,
        Math.sqrt(
          (2 * Math.max(0, 0.76 + 0.1525 + 0.02 + 0.04 - baseline)) /
            (9.81 * fraction * (1 - fraction)),
        ),
      );
    }
    const vx = (targetX - p.x) / (t - 0.06 * t * t),
      vz = (targetZ - p.z) / (t - 0.06 * t * t),
      a = acceleration({ x: vx, y: 0, z: vz }, spin);
    let vy = (0.78 - p.y - 0.5 * a.y * t * t) / t;
    const incoming = view.flight!.spin;
    if (incoming < -0.3 && kind === "drive")
      vy -=
        -incoming *
        Math.abs(timing) *
        stroke.power *
        0.9 *
        (1 - (stroke.lift ?? 0));
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
  return { at: contact, until: contact + horizon * 1000, p, v, a, spin };
}
function planeTimes(p: number, v: number, a: number, target: number): number[] {
  if (Math.abs(a) < 1e-9)
    return Math.abs(v) < 1e-9 ? [] : [(target - p) / v].filter((t) => t > 1e-6);
  const d = v * v - 2 * a * (p - target);
  if (d < 0) return [];
  return [(-v - Math.sqrt(d)) / a, (-v + Math.sqrt(d)) / a]
    .filter((t) => t > 1e-6)
    .sort((x, y) => x - y);
}
/** A clear net crossing is a force-segment boundary, not a predicted impact. */
export function predictPath(
  view: View,
  seat: number,
  stroke: Stroke,
  at: number,
): { segments: Segment[] } {
  const launch = predictLaunch(view, seat, stroke, at),
    segments: Segment[] = [launch];
  const limit = launch.at + 250;
  for (const t of planeTimes(launch.p.z, launch.v.z, launch.a.z, 0)) {
    const crossAt = launch.at + Math.max(1, Math.round(t * 1000));
    if (crossAt >= launch.until) break;
    const p = {
      x: launch.p.x + launch.v.x * t + 0.5 * launch.a.x * t * t,
      y: launch.p.y + launch.v.y * t + 0.5 * launch.a.y * t * t,
      z: 0,
    };
    if (Math.abs(p.x) > 0.8125) continue;
    if (p.y <= 0.76 + 0.1525 + 0.02) break;
    const v = {
      x: launch.v.x + launch.a.x * t,
      y: launch.v.y + launch.a.y * t,
      z: launch.v.z + launch.a.z * t,
    };
    const a = acceleration(v, launch.spin);
    let until = limit;
    for (const hit of planeTimes(p.y, v.y, a.y, 0.78))
      if (v.y + a.y * hit < 0)
        until = Math.min(until, crossAt + Math.max(1, Math.round(hit * 1000)));
    launch.until = crossAt;
    segments.push({ at: crossAt, until, p, v, a });
    break;
  }
  return { segments };
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
    path: { segments: Segment[] };
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
      path: predictPath(view, seat, stroke, at),
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
      } else if (at >= p.path.segments[0].at) {
        const position = sampleFlight(p.path, at);
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
