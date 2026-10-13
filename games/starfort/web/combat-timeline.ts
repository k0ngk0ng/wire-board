// Keep authoritative snapshots intact; only their presentation is buffered.
// A shot starts on receipt. Health, shields and removals commit on arrival.
export const FLIGHT_MS = 180;
export class CombatTimeline<T> {
  private current: T;
  private pending: { value: T; at: number }[] = [];
  constructor(initial: T) {
    this.current = initial;
  }
  push(value: T, receivedAt: number) {
    this.pending.push({ value, at: receivedAt + FLIGHT_MS });
  }
  reset(value: T) {
    this.current = value;
    this.pending = [];
  }
  read(at: number): T {
    while (this.pending.length && this.pending[0].at <= at)
      this.current = this.pending.shift()!.value;
    return this.current;
  }
}
