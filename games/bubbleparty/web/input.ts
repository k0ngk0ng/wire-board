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

// Movement is coalesced, but a bomb edge stays queued until a request carries
// it. Releasing a short tap must not erase it behind an in-flight request.
export class LatestInput {
  value: Control = { ...NEUTRAL };
  private snapshot?: Sync;
  private sent?: Control;
  private pending = new Map<number, InputRequest>();
  private at = -Infinity;
  private seq = 0;
  private token = 0;

  sync(next: Sync) {
    const previous = this.snapshot;
    if (
      !previous ||
      next.scope !== previous.scope ||
      next.epoch !== previous.epoch
    ) {
      this.value = { ...NEUTRAL };
      this.sent = undefined;
      this.pending.clear();
      this.at = -Infinity;
      this.seq = Math.max(0, next.seq);
    } else if (next.enabled !== previous.enabled) {
      this.value = { ...NEUTRAL };
      this.sent = undefined;
      this.pending.clear();
      this.at = -Infinity;
    }
    this.seq = Math.max(this.seq, next.seq);
    this.snapshot = next;
  }

  set(value: Control) {
    if (!this.snapshot?.enabled) return;
    this.value = { move: value.move, bomb: this.value.bomb || !!value.bomb };
  }

  next(now: number, force = false, window = 1): InputRequest | null {
    if (
      !this.snapshot?.enabled ||
      this.pending.size >= Math.max(1, Math.min(8, window))
    )
      return null;
    const changed = !this.sent || !same(this.sent, this.value);
    const released = changed && same(this.value, NEUTRAL);
    if (!changed && same(this.value, NEUTRAL)) return null;
    if (!force && !released && now - this.at < (changed ? 40 : 200))
      return null;
    this.sent = { ...this.value };
    this.at = now;
    const request: InputRequest = {
      ...this.value,
      type: "input",
      epoch: this.snapshot.epoch,
      seq: ++this.seq,
      token: ++this.token,
    };
    this.value = { ...this.value, bomb: false };
    this.pending.set(request.token, request);
    return request;
  }

  complete(token: number) {
    this.pending.delete(token);
  }

  reset(seq = 0) {
    this.value = { ...NEUTRAL };
    this.sent = undefined;
    this.pending.clear();
    this.at = -Infinity;
    this.seq = Math.max(0, seq);
  }

  release() {
    this.value = { ...NEUTRAL };
  }
}

// Track physical keys, not just the last direction. A turn released while a
// previous key is still held resumes that key; OS repeat does not reorder it.
export class HeldDirections {
  private held = new Map<string, Move>();
  press(key: string): Move {
    const move = keyMove(key);
    key = key.toLowerCase();
    if (move && !this.held.has(key)) this.held.set(key, move);
    return this.current();
  }
  release(key: string): Move {
    this.held.delete(key.toLowerCase());
    return this.current();
  }
  clear() {
    this.held.clear();
  }
  private current(): Move {
    return [...this.held.values()].at(-1) ?? "none";
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
