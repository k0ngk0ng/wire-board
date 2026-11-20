import type { MotionFunction } from "./types.ts";

type MotionPlayer = { x: number; y: number; aim: number; life: number; alive: boolean; downUntil: number; busyUntil?: number; appliedSeq: number };
type MotionInput = { seq: number; moveX: number; moveY: number; aim: number };
type PendingInput = { frame: number; input: MotionInput };

const neutral = (): MotionInput => ({ seq: 0, moveX: 0, moveY: 0, aim: 0 });

export class MotionPrediction {
  private motion: MotionFunction;
  private player: MotionPlayer = { x: 0, y: 0, aim: 0, life: 0, alive: false, downUntil: 0, appliedSeq: 0 };
  private frame = 0;
  private originFrame = 0;
  private originAt = 0;
  private identity = "";
  private pending: PendingInput[] = [];
  private applied = neutral();
  private doors = "[]";
  private glass = "[]";
  private enabled = true;
  private immediateTarget = 0;
  predictedFrame = 0;

  constructor(motion: MotionFunction) {
    this.motion = motion;
  }

  get pendingCount() {
    return this.pending.length;
  }

  confirm(player: MotionPlayer, frame: number, at: number, identity: string, doors = "[]", glass = "[]") {
    const reset = identity !== this.identity || player.life !== this.player.life;
    if (!reset && frame < this.frame) return;
    if (reset) {
      this.pending = [];
      this.applied = neutral();
      this.originAt = at;
      this.originFrame = frame;
      this.predictedFrame = frame;
      this.immediateTarget = frame;
    } else {
      for (const pending of this.pending)
        if (pending.input.seq <= player.appliedSeq) this.applied = pending.input;
      this.pending = this.pending.filter(pending => pending.input.seq > player.appliedSeq);
      const clockFrame = this.targetFrame(at);
      if (frame > clockFrame) this.originFrame += frame - clockFrame;
    }
    if (!player.alive) {
      this.pending = [];
      this.applied = neutral();
    }
    this.player = { ...player };
    this.frame = frame;
    this.identity = identity;
    this.doors = doors;
    this.glass = glass;
  }

  input(input: MotionInput, at: number) {
    if (!this.player.alive || !this.enabled) return;
    const frame = Math.max(this.frame + 1, this.predictedFrame + 1, this.targetFrame(at) + 1);
    this.pending.push({ frame, input: { ...input } });
    this.immediateTarget = frame;
    if (this.pending.length > 128) this.pending = this.pending.slice(-128);
  }

  sample(at: number) {
    const clockFrame = this.targetFrame(at);
    const visibleInput = Math.min(this.immediateTarget, clockFrame + 1);
    const target = this.enabled ? Math.min(this.frame + 9, Math.max(this.frame, clockFrame, visibleInput, this.predictedFrame)) : this.frame;
    let x = this.player.x, y = this.player.y, aim = this.player.aim;
    let input = this.applied;
    for (let frame = this.frame + 1; frame <= target; frame++) {
      for (const pending of this.pending) if (pending.frame <= frame) input = pending.input;
      aim = input.seq ? input.aim : aim;
      if (this.player.alive && frame >= this.player.downUntil && frame >= (this.player.busyUntil ?? 0)) {
        const next = this.motion(x, y, aim, input.moveX, input.moveY, 1, this.doors, this.glass);
        x = next[0];
        y = next[1];
      }
    }
    this.predictedFrame = target;
    return { ...this.player, x, y, aim };
  }

  clear(at: number) {
    this.pending = [];
    this.applied = neutral();
    this.originAt = at;
    this.originFrame = this.frame;
    this.predictedFrame = this.frame;
    this.immediateTarget = this.frame;
  }

  setEnabled(enabled: boolean, at: number) {
    if (enabled === this.enabled) return;
    this.enabled = enabled;
    this.clear(at);
  }

  private targetFrame(at: number) {
    return this.originFrame + Math.max(0, Math.trunc((Math.trunc(at) - Math.trunc(this.originAt)) * 60 / 1000));
  }
}

export class EventCursor {
  private identity = "";
  private cursor = 0;

  consume<T extends { id: number }>(identity: string, events: T[]): T[] {
    if (identity !== this.identity) {
      this.identity = identity;
      this.cursor = events.reduce((highest, event) => Math.max(highest, event.id), 0);
      return [];
    }
    const next = events.filter(event => event.id > this.cursor).sort((a, b) => a.id - b.id);
    for (const event of next) this.cursor = Math.max(this.cursor, event.id);
    return next;
  }
}
