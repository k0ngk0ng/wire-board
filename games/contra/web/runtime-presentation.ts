import type { RuntimeSnapshot } from "./runtime-bridge";

const frameMS = 1000 / 60;
/** Schedules already verified states only. It never predicts input or physics.
 * The two-frame start buffer absorbs the platform's three-frame 50ms batches.
 * An inactive or overloaded display resumes from the newest accepted state and
 * seeks audio instead of replaying stale effects. */
export class RuntimePresentationQueue {
  private queue: RuntimeSnapshot[] = [];
  private nextAt: number | undefined;
  private sampledAt: number | undefined;
  private paintedAt: number | undefined;
  private overloaded = false;
  private show: (snapshot: RuntimeSnapshot, resync: boolean) => void;
  constructor(show: (snapshot: RuntimeSnapshot, resync: boolean) => void) {
    this.show = show;
  }
  clear() {
    this.queue = [];
    this.nextAt = undefined;
    this.sampledAt = undefined;
    this.paintedAt = undefined;
    this.overloaded = false;
  }
  reset(snapshot: RuntimeSnapshot, now: number) {
    this.clear();
    this.sampledAt = now;
    this.paintedAt = now;
    this.show(snapshot, true);
  }
  presentNow(snapshot: RuntimeSnapshot, now: number) {
    this.clear();
    this.sampledAt = now;
    this.paintedAt = now;
    this.show(snapshot, false);
  }
  push(snapshot: RuntimeSnapshot) {
    this.queue.push(snapshot);
    if (this.queue.length > 12) {
      this.queue = [snapshot];
      this.overloaded = true;
    }
  }
  sample(now: number) {
    const delayed = this.sampledAt !== undefined && now - this.sampledAt > 250;
    this.sampledAt = now;
    if (!this.queue.length) {
      this.nextAt = undefined;
      return;
    }
    const starved = this.paintedAt !== undefined && now - this.paintedAt > 250;
    if (delayed || starved || this.overloaded) {
      const latest = this.queue[this.queue.length - 1];
      this.reset(latest, now);
      return;
    }
    this.nextAt ??= now + 2 * frameMS;
    if (now + 0.05 < this.nextAt) return;
    const snapshot = this.queue.shift()!;
    this.show(snapshot, false);
    this.paintedAt = now;
    this.nextAt = Math.max(this.nextAt + frameMS, now - frameMS);
  }
}
