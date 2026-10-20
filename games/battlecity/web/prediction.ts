import type { Move } from "./input";
import type { Dir, TankView, View } from "./types";
export type TankPose = { x: number; y: number; dir: Dir };
export type TankWorld = {
  cells: readonly string[];
  broken: Uint8Array;
  shovelUntil: readonly number[];
  tanks: readonly TankView[];
  speed: number;
};
const pose = (t: TankView): TankPose => ({ x: t.x, y: t.y, dir: t.dir });
const ring = (team: number, x: number, y: number) =>
  (y === (team === 0 ? 0 : 12) && (x === 5 || x === 7)) ||
  (y === (team === 0 ? 1 : 11) && x >= 5 && x <= 7);
function free(p: TankPose, seat: number, w: TankWorld, at: number) {
  if (p.x < 0 || p.y < 0 || p.x > 192 || p.y > 192) return false;
  for (let y = Math.floor(p.y / 8); y <= Math.floor((p.y + 15.999) / 8); y++)
    for (
      let x = Math.floor(p.x / 8);
      x <= Math.floor((p.x + 15.999) / 8);
      x++
    ) {
      const cx = Math.floor(x / 2),
        cy = Math.floor(y / 2),
        code = Number(w.cells[cy]?.[cx] ?? 1);
      if (cx === 6 && (cy === 0 || cy === 12)) return false;
      const steel = w.shovelUntil.some(
        (until, team) => at < until && ring(team, cx, cy),
      );
      if (steel) {
        if (!w.broken[y * 26 + x]) return false;
      } else if (
        code === 3 ||
        ((code === 1 || code === 2) && !w.broken[y * 26 + x])
      )
        return false;
    }
  return !w.tanks.some(
    (t) =>
      t.seat !== seat &&
      t.alive &&
      t.x < p.x + 16 &&
      p.x < t.x + 16 &&
      t.y < p.y + 16 &&
      p.y < t.y + 16,
  );
}
// Cosmetic movement only; HP, bullets, respawn, pickups and wins stay server-owned.
export function predictTank(
  from: TankPose,
  tank: TankView,
  move: Move,
  w: TankWorld,
  ms: number,
  at: number,
): TankPose {
  let p = { ...from };
  if (!tank.alive || tank.hp <= 0 || tank.freezeUntil > at || move === "none")
    return p;
  const dir = ({ up: 0, right: 1, down: 2, left: 3 } as const)[move];
  let remaining = Math.max(0, Math.min(250, ms));
  while (remaining > 0) {
    const dt = Math.min(20, remaining) / 1000;
    remaining -= dt * 1000;
    if (p.dir !== dir) {
      const turn = { ...p, dir };
      if (dir === 1 || dir === 3) turn.y = Math.round(turn.y / 8) * 8;
      else turn.x = Math.round(turn.x / 8) * 8;
      if (free(turn, tank.seat, w, at)) p = turn;
    }
    const ice =
      Number(
        w.cells[Math.floor((p.y + 8) / 16)]?.[Math.floor((p.x + 8) / 16)],
      ) === 5;
    const amount = w.speed * (ice ? 1.1 : 1) * dt;
    const next = {
      ...p,
      x: Math.max(0, Math.min(192, p.x + [0, 1, 0, -1][p.dir] * amount)),
      y: Math.max(0, Math.min(192, p.y + [-1, 0, 1, 0][p.dir] * amount)),
    };
    if (free(next, tank.seat, w, at)) p = next;
  }
  return p;
}
function correctionPath(
  from: TankPose,
  target: TankPose,
  tank: TankView,
  w: TankWorld,
  at: number,
) {
  let p = { ...from };
  const distance = Math.hypot(target.x - from.x, target.y - from.y),
    steps = Math.ceil(distance);
  for (let i = 1; i <= steps; i++) {
    const next = {
      ...target,
      x: from.x + ((target.x - from.x) * i) / steps,
      y: from.y + ((target.y - from.y) * i) / steps,
    };
    if (!free(next, tank.seat, w, at)) break;
    p = next;
  }
  return p;
}
export class TankMotion {
  private scope = "";
  private stamp = -Infinity;
  private received = 0;
  private at = 0;
  private active = false;
  private authority?: TankPose;
  private local?: TankPose;
  private shown?: TankPose;
  private correction = { x: 0, y: 0 };
  frame(
    view: View,
    seat: number,
    move: Move,
    w: TankWorld,
    now: number,
    scope: string,
    enabled: boolean,
  ): TankPose {
    const tank = view.tanks[seat],
      base = pose(tank);
    const active =
      enabled &&
      view.phase === "playing" &&
      tank.alive &&
      tank.hp > 0 &&
      tank.freezeUntil <= view.serverAt;
    if (
      !this.local ||
      this.scope !== scope ||
      this.active !== active ||
      !active
    ) {
      this.scope = scope;
      this.stamp = view.serverAt;
      this.received = now;
      this.at = now;
      this.active = active;
      this.authority = base;
      this.local = base;
      this.shown = base;
      this.correction = { x: 0, y: 0 };
      return { ...base };
    }
    if (
      this.stamp !== view.serverAt ||
      this.authority?.x !== base.x ||
      this.authority?.y !== base.y
    ) {
      const old = this.shown!;
      this.correction =
        Math.hypot(old.x - base.x, old.y - base.y) < 24
          ? { x: old.x - base.x, y: old.y - base.y }
          : { x: 0, y: 0 };
      this.authority = base;
      this.local = base;
      this.stamp = view.serverAt;
      this.received = now;
    }
    const elapsed = Math.max(0, now - this.at),
      step = Math.min(40, elapsed, Math.max(0, this.received + 250 - this.at));
    this.at = now;
    this.local = predictTank(this.local, tank, move, w, step, view.serverAt);
    const blend = Math.exp(-elapsed / 70);
    this.correction.x *= blend;
    this.correction.y *= blend;
    this.shown = correctionPath(
      this.local,
      {
        ...this.local,
        x: this.local.x + this.correction.x,
        y: this.local.y + this.correction.y,
      },
      tank,
      w,
      view.serverAt,
    );
    return { ...this.shown };
  }
}
