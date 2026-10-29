export type InputKind =
  "left" | "right" | "rotateCW" | "rotateCCW" | "softDrop" | "hardDrop";

export type InputEvent = { kind: InputKind };
export type Batch = { seq: number; events: InputEvent[] };

export const BATCH_MAX = 16;
export const SENT_MAX = 2;
export const PENDING_MAX = 24;
export const DAS_DELAY = 150;
export const DAS_INTERVAL = 50;
export const SOFT_DROP_DELAY = 50;
export const SOFT_DROP_INTERVAL = 50;

// Same batching discipline as the tetris module: two sent batches at most, so a
// slow server never receives an unbounded backlog of keystrokes.
export class InputQueue {
  private pendingEvents: InputEvent[] = [];
  private sentBatches: Batch[] = [];
  private nextSeq: number;

  constructor(serverSeq = 0) {
    this.nextSeq = serverSeq + 1;
  }

  push(kind: InputKind): boolean {
    if (this.pendingEvents.length >= PENDING_MAX) return false;
    this.pendingEvents.push({ kind });
    return true;
  }

  take(): Batch | null {
    if (this.sentBatches.length >= SENT_MAX || this.pendingEvents.length === 0)
      return null;
    const events = this.pendingEvents.splice(0, BATCH_MAX);
    const batch = { seq: this.nextSeq++, events };
    this.sentBatches.push(batch);
    return batch;
  }

  acknowledge(serverSeq: number) {
    this.nextSeq = Math.max(this.nextSeq, serverSeq + 1);
    this.sentBatches = this.sentBatches.filter(
      (batch) => batch.seq > serverSeq,
    );
  }

  reset(serverSeq: number) {
    this.pendingEvents = [];
    this.sentBatches = [];
    this.nextSeq = serverSeq + 1;
  }

  clearPending() {
    this.pendingEvents = [];
  }

  unconfirmed(acknowledged = 0): InputEvent[] {
    return [
      ...this.sentBatches
        .filter((batch) => batch.seq > acknowledged)
        .flatMap((batch) => batch.events),
      ...this.pendingEvents,
    ];
  }
}

export function repeatTimes(
  heldSince: number,
  now: number,
  delay: number,
  interval: number,
): number {
  const elapsed = now - heldSince;
  if (elapsed < delay) return 0;
  return 1 + Math.floor((elapsed - delay) / interval);
}
