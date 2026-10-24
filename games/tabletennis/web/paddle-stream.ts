import type { Paddle } from "./types";
type Point = { x: number; y: number; z: number };
/** Cosmetic motion uses its own bounded sequence; no strike waits behind it. */
export class PaddleStream {
  private identity = "";
  private epoch = "";
  private ends = 0;
  private seq = 0;
  private generation = 0;
  private busy = false;
  private lastSent = -Infinity;
  private pending?: { point: Point; at: number };
  private sent?: Point;
  private readonly send: (a: Record<string, unknown>) => Promise<void>;
  constructor(send: (a: Record<string, unknown>) => Promise<void>) {
    this.send = send;
  }
  update(identity: string, epoch: string, ends: number, seq: number) {
    if (identity !== this.identity) {
      this.generation++;
      this.busy = false;
      this.pending = undefined;
      this.sent = undefined;
      this.lastSent = -Infinity;
      this.seq = seq;
    }
    this.identity = identity;
    this.epoch = epoch;
    this.ends = ends;
    this.seq = Math.max(this.seq, seq);
  }
  offer(point: Point, at: number) {
    if (
      this.sent &&
      Math.hypot(
        point.x - this.sent.x,
        point.y - this.sent.y,
        point.z - this.sent.z,
      ) < 0.006
    ) {
      this.pending = undefined;
      return;
    }
    this.pending = { point: { ...point }, at };
  }
  clear() {
    this.pending = undefined;
    this.sent = undefined;
  }
  flush(at: number): boolean {
    const p = this.pending;
    if (!p || this.busy || at - this.lastSent < 80) return false;
    this.pending = undefined;
    if (at - p.at > 180) return false;
    const generation = this.generation;
    this.busy = true;
    this.lastSent = at;
    this.sent = p.point;
    const done = () => {
      if (generation === this.generation) this.busy = false;
    };
    void this.send({
      type: "move",
      epoch: this.epoch,
      ends: this.ends,
      motionSeq: ++this.seq,
      racketX: p.point.x,
      racketY: p.point.y,
      racketZ: p.point.z,
      at: Math.round(p.at),
    }).then(done, done);
    return true;
  }
}
type Sample = Point & { at: number; seq: number; hitSeq: number };
/** Interpolate real position samples; extrapolation is short and bounded. */
export class RemotePaddles {
  private identity = "";
  private tracks: Sample[][] = [];
  update(identity: string, paddles: readonly Paddle[], now: number) {
    if (identity !== this.identity) {
      this.identity = identity;
      this.tracks = [];
    }
    paddles.forEach((p, i) => {
      const track = this.tracks[i] ?? (this.tracks[i] = []),
        last = track.at(-1);
      const at = Math.max(p.motionAt ?? now, p.result === "hit" ? p.at : 0),
        seq = p.motionSeq ?? 0,
        hitSeq = p.result === "hit" ? p.seq : 0;
      if (last && (at < last.at || (seq <= last.seq && hitSeq <= last.hitSeq)))
        return;
      track.push({ x: p.x, y: p.y ?? 0.9, z: p.z ?? 1.7, at, seq, hitSeq });
      if (track.length > 16) track.shift();
    });
  }
  sample(seat: number, at: number): Point {
    const list = this.tracks[seat];
    if (!list?.length) return { x: 0, y: 0.9, z: 1.7 };
    let a = list[0],
      b = a;
    for (const p of list) {
      if (p.at <= at) a = p;
      else {
        b = p;
        break;
      }
      b = p;
    }
    if (b.at > a.at) {
      const k = Math.max(0, Math.min(1, (at - a.at) / (b.at - a.at)));
      return {
        x: a.x + (b.x - a.x) * k,
        y: a.y + (b.y - a.y) * k,
        z: a.z + (b.z - a.z) * k,
      };
    }
    const prev = list.at(-2);
    if (prev && a.at > prev.at && at > a.at) {
      const k = Math.min(60, at - a.at) / (a.at - prev.at),
        limit = (n: number) => Math.max(-0.12, Math.min(0.12, n));
      return {
        x: a.x + limit((a.x - prev.x) * k),
        y: a.y + limit((a.y - prev.y) * k),
        z: a.z + limit((a.z - prev.z) * k),
      };
    }
    return { x: a.x, y: a.y, z: a.z };
  }
}
