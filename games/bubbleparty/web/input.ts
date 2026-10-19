export type Move = "none" | "up" | "down" | "left" | "right";
export type Control = { move: Move; bomb: boolean };
export type InputRequest = Control & {
  type: "input";
  epoch: string;
  seq: number;
  token: number;
};

export const NEUTRAL: Control = { move: "none", bomb: false };
const same = (a: Control, b: Control) => a.move === b.move && a.bomb === b.bomb;

type Sync = { scope: string; epoch: string; seq: number; enabled: boolean };

// One in-flight intent at a time. Continuous direction is coalesced; the bomb
// flag is an edge that the caller clears immediately after sending.
export class LatestInput {
  value: Control = { ...NEUTRAL };
  private snapshot?: Sync;
  private sent?: Control;
  private pending?: InputRequest;
  private at = -Infinity;
  private seq = 0;
  private token = 0;

  sync(next: Sync) {
    const previous = this.snapshot;
    if (!previous || next.scope !== previous.scope || next.epoch !== previous.epoch) {
      this.value = { ...NEUTRAL };
      this.sent = undefined;
      this.pending = undefined;
      this.at = -Infinity;
      this.seq = Math.max(0, next.seq);
    } else if (next.enabled !== previous.enabled) {
      this.value = { ...NEUTRAL };
      this.sent = undefined;
      this.pending = undefined;
      this.at = -Infinity;
    }
    this.seq = Math.max(this.seq, next.seq);
    this.snapshot = next;
  }

  set(value: Control) {
    if (!this.snapshot?.enabled) return;
    this.value = { move: value.move, bomb: !!value.bomb };
  }

  next(now: number, force = false): InputRequest | null {
    if (!this.snapshot?.enabled || this.pending) return null;
    const changed = !this.sent || !same(this.sent, this.value);
    const released = changed && same(this.value, NEUTRAL);
    if (!changed && same(this.value, NEUTRAL)) return null;
    if (!force && !released && now - this.at < (changed ? 100 : 280)) return null;
    this.sent = { ...this.value };
    this.at = now;
    this.pending = {
      ...this.value,
      type: "input",
      epoch: this.snapshot.epoch,
      seq: ++this.seq,
      token: ++this.token,
    };
    return this.pending;
  }

  complete(token: number) {
    if (this.pending?.token === token) this.pending = undefined;
  }

  reset(seq = 0) {
    this.value = { ...NEUTRAL };
    this.sent = undefined;
    this.pending = undefined;
    this.at = -Infinity;
    this.seq = Math.max(0, seq);
  }

  release() {
    this.value = { ...NEUTRAL };
  }
}

export const keyMove = (key: string): Move | null => {
  switch (key) {
    case "ArrowUp":
    case "w":
    case "W":
      return "up";
    case "ArrowDown":
    case "s":
    case "S":
      return "down";
    case "ArrowLeft":
    case "a":
    case "A":
      return "left";
    case "ArrowRight":
    case "d":
    case "D":
      return "right";
    default:
      return null;
  }
};

export const isBombKey = (key: string) =>
  key === " " || key === "j" || key === "J" || key === "k" || key === "K";

export function padDirection(
  clientX: number,
  clientY: number,
  rect: DOMRect,
): Move {
  const x = ((clientX - rect.left) / Math.max(1, rect.width)) * 2 - 1;
  const y = ((clientY - rect.top) / Math.max(1, rect.height)) * 2 - 1;
  if (Math.max(Math.abs(x), Math.abs(y)) < 0.22) return "none";
  if (Math.abs(x) >= Math.abs(y)) return x < 0 ? "left" : "right";
  return y < 0 ? "up" : "down";
}
