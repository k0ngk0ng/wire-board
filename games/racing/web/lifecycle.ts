type State = {
  race: string;
  transport: number;
  enabled: boolean;
  hidden: boolean;
  phase: string;
  snapshotAge?: number;
};
// Losing input authority or the transport releases the car. Reconnecting does
// not count as a new user gesture, even with automatic acceleration selected.
export class ControlLifecycle {
  private previous?: State;
  private paused = true;
  sync(s: State) {
    if (!this.previous || this.previous.race !== s.race)
      this.paused = s.phase !== "countdown" || s.hidden || !s.enabled;
    else if (
      this.previous.transport !== s.transport ||
      this.previous.enabled !== s.enabled ||
      s.hidden
    )
      this.paused = true;
    if ((s.snapshotAge ?? 0) > 1000) this.paused = true;
    this.previous = s;
    return this.paused;
  }
  suspend() {
    this.paused = true;
  }
  networkFailed() {
    this.paused = true;
  }
  wake() {
    if (
      this.previous?.enabled &&
      !this.previous.hidden &&
      (this.previous.snapshotAge ?? 0) <= 1000
    )
      this.paused = false;
    return this.paused;
  }
}
