export type Drive = {
  steer: number;
  throttle: number;
  brake: boolean;
  drift: boolean;
};
export const neutral: Drive = {
  steer: 0,
  throttle: 0,
  brake: false,
  drift: false,
};
const clamp = (x: number, lo: number, hi: number) =>
  Math.max(lo, Math.min(hi, Number.isFinite(x) ? x : 0));
const same = (a: Drive, b: Drive) =>
  a.steer === b.steer &&
  a.throttle === b.throttle &&
  a.brake === b.brake &&
  a.drift === b.drift;
export function wheelSteer(x: number, left: number, width: number) {
  const raw = clamp((x - left - width / 2) / Math.max(1, width * 0.42), -1, 1);
  return Math.abs(raw) < 0.07 ? 0 : raw;
}
type Sync = {
  identity?: string;
  scope: string;
  epoch: string;
  seq: number;
  enabled: boolean;
};
type Request = Drive & {
  type: "drive";
  epoch: string;
  seq: number;
  token: number;
};
// This queue retains intent, never game state. At most one platform act is in
// flight and every newer gesture replaces the unsent state (including release).
export class LatestDrive {
  value: Drive = { ...neutral };
  private snapshot?: Sync;
  private sent?: Drive;
  private pending?: Request;
  private at = -Infinity;
  private seq = 0;
  private token = 0;
  sync(s: Sync) {
    if (
      !this.snapshot ||
      s.scope !== this.snapshot.scope ||
      s.enabled !== this.snapshot.enabled
    ) {
      this.value = { ...neutral };
      this.sent = undefined;
      this.pending = undefined;
      this.at = -Infinity;
      if (
        !this.snapshot ||
        (s.identity ?? s.scope) !==
          (this.snapshot.identity ?? this.snapshot.scope)
      )
        this.seq = s.seq;
    }
    this.seq = Math.max(this.seq, s.seq);
    this.snapshot = s;
  }
  set(v: Drive) {
    if (!this.snapshot?.enabled) return;
    this.value = {
      steer: clamp(v.steer, -1, 1),
      throttle: clamp(v.throttle, 0, 1),
      brake: !!v.brake,
      drift: !!v.drift,
    };
  }
  next(now: number): Request | null {
    if (!this.snapshot?.enabled || this.pending) return null;
    const changed = !this.sent || !same(this.sent, this.value);
    const released = changed && same(this.value, neutral);
    if (!changed && same(this.value, neutral)) return null;
    if (!released && now - this.at < (changed ? 100 : 300)) return null;
    this.sent = { ...this.value };
    this.at = now;
    this.pending = {
      ...this.value,
      type: "drive",
      epoch: this.snapshot.epoch,
      seq: ++this.seq,
      token: ++this.token,
    };
    return this.pending;
  }
  complete(token: number) {
    if (this.pending?.token === token) this.pending = undefined;
  }
}
