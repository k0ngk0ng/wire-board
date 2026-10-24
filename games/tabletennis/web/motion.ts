import type { Stroke } from "./types";
type Point = { x: number; y: number; at: number };
const clamp = (n: number, lo: number, hi: number) =>
  Math.max(lo, Math.min(hi, n));
/** Live paddle velocity, independent of mouse buttons or pointer release. */
export class PaddleMotion {
  private points: Point[] = [];
  clear() {
    this.points = [];
  }
  move(x: number, y: number, at: number) {
    const last = this.points.at(-1);
    if (last && (at <= last.at || at - last.at > 120)) this.clear();
    this.points.push({ x, y, at });
    while (this.points.length > 2 && this.points[1].at < at - 80)
      this.points.shift();
  }
  sample(at: number, shortSide: number) {
    const first = this.points[0],
      last = this.points.at(-1);
    const idle = {
      active: !!last,
      moving: false,
      settled:
        !!first &&
        !!last &&
        at - first.at >= 60 &&
        (at - last.at >= 60 || last.at - first.at >= 60),
      at: last?.at ?? 0,
      velocity: { x: 0, y: 0 },
      stroke: { kind: "block", aim: 0, power: 0.35 } as Stroke,
    };
    if (!first || !last || last.at === first.at || at - last.at > 50)
      return idle;
    let velocityOrigin = this.points[Math.max(0, this.points.length - 2)];
    for (let i = this.points.length - 2; i >= 0; i--) {
      velocityOrigin = this.points[i];
      if (last.at - velocityOrigin.at >= 12) break;
    }
    const extent = Math.max(160, shortSide),
      seconds = Math.max(0.004, (last.at - velocityOrigin.at) / 1000);
    const dx = last.x - velocityOrigin.x,
      dy = last.y - velocityOrigin.y;
    const velocity = { x: dx / extent / seconds, y: dy / extent / seconds };
    const speed = Math.hypot(velocity.x, velocity.y);
    if (
      Math.hypot(last.x - first.x, last.y - first.y) <
        Math.max(4, extent * 0.012) ||
      speed < 0.08
    )
      return idle;
    return {
      active: true,
      moving: true,
      settled: false,
      at: last.at,
      velocity,
      stroke: {
        kind: velocity.y > 0.12 ? "chop" : "drive",
        aim: clamp(
          (velocity.x / Math.max(0.3, Math.abs(velocity.y))) * 0.75,
          -1,
          1,
        ),
        power: clamp(0.25 + speed * 0.28, 0.25, 1),
        lift: clamp(-velocity.y / Math.max(0.1, speed), 0, 1),
      } as Stroke,
    };
  }
}
