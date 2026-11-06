import type { Player, Point } from "./types.ts";
const clamp = (n: number, a: number, b: number) => Math.max(a, Math.min(b, n));
export class Motion {
  private player?: Player;
  private identity = "";
  private tick = -1;
  private authAt = 0;
  private anchorAt = 0;
  private anchor: Point = { x: 500, y: 440 };
  private correction: Point = { x: 0, y: 0 };
  private dx = 0;
  private dy = 0;
  private manual = false;
  update(player: Player, tick: number, epoch: string, at: number) {
    if (
      player.goal &&
      (!this.player?.goal ||
        player.goal.x !== this.player.goal.x ||
        player.goal.y !== this.player.goal.y)
    )
      this.manual = false;
    const identity = `${epoch}/${player.room}/${player.hp <= 0}`;
    if (identity !== this.identity) {
      this.identity = identity;
      this.player = player;
      this.tick = tick;
      this.authAt = at;
      this.anchorAt = at;
      this.anchor = { x: player.x, y: player.y };
      this.correction = { x: 0, y: 0 };
      this.dx = 0;
      this.dy = 0;
      this.manual = false;
      return;
    }
    if (tick === this.tick) {
      this.player = player;
      return;
    }
    const old = this.sample(at);
    this.tick = tick;
    this.player = player;
    this.authAt = at;
    this.anchorAt = at;
    this.anchor = { x: player.x, y: player.y };
    this.correction =
      Math.hypot(old.x - player.x, old.y - player.y) < 110
        ? { x: old.x - player.x, y: old.y - player.y }
        : { x: 0, y: 0 };
  }
  input(dx: number, dy: number, at: number) {
    if (dx === this.dx && dy === this.dy && this.manual) return;
    const point = this.sample(at);
    this.anchor = point;
    this.anchorAt = at;
    this.correction = { x: 0, y: 0 };
    this.dx = dx;
    this.dy = dy;
    this.manual = true;
  }
  sample(at: number): Point {
    const p = this.player;
    if (!p) return this.anchor;
    if (p.hp <= 0) return { x: p.x, y: p.y };
    const dt =
      clamp(
        at - this.anchorAt,
        0,
        Math.max(0, 250 - (this.anchorAt - this.authAt)),
      ) / 1000;
    let dx = this.manual ? this.dx : p.dx,
      dy = this.manual ? this.dy : p.dy;
    const goal = !this.manual ? p.goal : undefined;
    if (goal) {
      const d = Math.hypot(goal.x - this.anchor.x, goal.y - this.anchor.y);
      dx = d > 1 ? (goal.x - this.anchor.x) / d : 0;
      dy = d > 1 ? (goal.y - this.anchor.y) / d : 0;
    }
    const length = Math.hypot(dx, dy);
    if (length > 1) {
      dx /= length;
      dy /= length;
    }
    let step = 155 * dt * (p.carry ? 0.72 : 1) * (p.hp < 35 ? 0.8 : 1);
    if (goal)
      step = Math.min(
        step,
        Math.hypot(goal.x - this.anchor.x, goal.y - this.anchor.y),
      );
    const decay = Math.exp(-Math.max(0, at - this.anchorAt) / 90);
    return {
      x: clamp(this.anchor.x + dx * step + this.correction.x * decay, 145, 855),
      y: clamp(this.anchor.y + dy * step + this.correction.y * decay, 270, 565),
    };
  }
}
