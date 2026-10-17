export type BasketballInputBinding = {
  roomID: string;
  userID: string;
  seat: number;
  epoch: string;
  serverAt: number;
  sequence: number;
  canInput: boolean;
  version?: number;
};
export type BasketballInputEvent = {
  sequence: number;
  buttons: number;
  offsetMS: number;
  release: boolean;
  cancelMask?: number;
};
export type BasketballInputBatch = {
  type: "inputs";
  epoch: string;
  seat: number;
  basedOn: number;
  sequence: number;
  events: BasketballInputEvent[];
};
type ResetReason = "binding" | "disabled" | "timeout" | "network" | "disposed";
type Queued = Omit<BasketballInputEvent, "offsetMS"> & { at: number };
type Flight = { sequence: number; priority: boolean; generation: number };
type Options = {
  send: (action: BasketballInputBatch) => Promise<void>;
  /** Must clear physical sources without recursively calling cancel(). */
  onReset: (reason: ResetReason) => void;
  now?: () => number;
};

/** Browser transport only. Rules, video ticks and results remain in Go.
 * The host samples once per animation frame and supplies every room snapshot.
 * Explicit cancellation may overtake a pending normal HTTP request; both carry
 * the same unacknowledged release until either confirms it.
 */
export class BasketballNetworkInput {
  private readonly options: Options;
  private readonly now: () => number;
  private binding?: BasketballInputBinding;
  private generation = 0;
  private sequence = 0;
  private acknowledged = 0;
  private queue: Queued[] = [];
  private flights = new Set<Flight>();
  private held = 0;
  private lastProgressAt = 0;
  private lastInputAt = 0;
  private retryAt = 0;
  private closed = false;

  constructor(options: Options) {
    this.options = options;
    this.now = options.now ?? (() => performance.now());
  }
  get pendingCount(): number {
    return this.queue.length;
  }
  get connected(): boolean {
    return (
      !this.closed &&
      !!this.binding?.canInput &&
      this.now() - this.lastProgressAt <= 750
    );
  }

  update(next: BasketballInputBinding): void {
    if (this.closed) return;
    if (
      !Number.isSafeInteger(next.serverAt) ||
      next.serverAt < 0 ||
      !Number.isSafeInteger(next.sequence) ||
      next.sequence < 0 ||
      !Number.isInteger(next.seat) ||
      next.seat < -1 ||
      next.seat > 3
    ) {
      throw new Error("Invalid basketball input binding");
    }
    const old = this.binding;
    const changed =
      !old ||
      old.roomID !== next.roomID ||
      old.userID !== next.userID ||
      old.seat !== next.seat ||
      old.epoch !== next.epoch;
    if (changed) {
      this.generation++;
      this.flights.clear();
      this.queue = [];
      this.held = 0;
      this.sequence = this.acknowledged = next.sequence;
      this.lastProgressAt = this.lastInputAt = this.now();
      this.retryAt = 0;
      this.binding = { ...next, canInput: next.canInput && next.seat >= 0 };
      this.options.onReset("binding");
      return;
    }
    if (
      next.serverAt < old.serverAt ||
      next.sequence < this.acknowledged ||
      (next.version !== undefined &&
        old.version !== undefined &&
        next.version < old.version)
    )
      return;
    this.expire();
    if (next.serverAt > old.serverAt) this.lastProgressAt = this.now();
    this.binding = { ...next, canInput: next.canInput && next.seat >= 0 };
    this.acknowledge(next.sequence);
    if (!this.binding.canInput) {
      this.generation++;
      this.flights.clear();
      this.queue = [];
      this.held = 0;
      if (old.canInput) this.options.onReset("disabled");
      return;
    }
    this.flush();
  }

  /** Sample the merged logical mask, including the platform turbo sampler. */
  sample(buttons: number): void {
    if (!Number.isInteger(buttons) || buttons < 0 || buttons > 255)
      throw new Error("Invalid basketball buttons");
    if (this.closed || !this.binding?.canInput) return;
    if (this.expire() || !this.connected) {
      this.flush();
      return;
    }
    const now = this.now();
    if (
      buttons !== this.held ||
      (buttons !== 0 && now - this.lastInputAt >= 250)
    ) {
      if (this.queue.length >= 64) {
        this.resetGesture("timeout");
        this.flush(true);
        return;
      }
      this.held = buttons;
      this.lastInputAt = now;
      if (
        this.queue.length === 1 &&
        this.queue[0].release &&
        now - this.queue[0].at > 500
      ) {
        // The release remains unacknowledged, but no playable events remain
        // from that old interval. Carry it immediately before the new gesture.
        this.queue[0] = { ...this.queue[0], at: now };
      }
      this.queue.push({
        sequence: ++this.sequence,
        buttons,
        at: now,
        release: false,
      });
    }
    this.flush();
  }

  cancel(mask = 255): void {
    if (!Number.isInteger(mask) || mask < 1 || mask > 255)
      throw new Error("Invalid basketball cancellation mask");
    if (this.closed || !this.binding?.canInput) return;
    if (
      this.held === 0 &&
      this.queue.length === 1 &&
      this.queue[0].release &&
      (this.queue[0].cancelMask ?? 255) === 255
    ) {
      this.flush(true);
      return;
    }
    // One release covers all earlier unacknowledged events. Even an idle
    // local sampler may need to clear a server latch after focus changes.
    if (mask !== 255 && this.queue.length >= 63) {
      this.resetGesture("timeout");
      this.flush(true);
      return;
    }
    const event: Queued = {
      sequence: ++this.sequence,
      buttons: 0,
      at: this.now(),
      release: true,
    };
    if (mask === 255) this.queue = [event];
    else {
      event.cancelMask = mask;
      this.queue.push(event);
    }
    this.held &= ~mask;
    this.lastInputAt = this.now();
    this.retryAt = 0;
    this.flush(true);
  }

  private resetGesture(reason: ResetReason): void {
    this.queue = [
      { sequence: ++this.sequence, buttons: 0, at: this.now(), release: true },
    ];
    this.held = 0;
    this.lastInputAt = this.now();
    this.retryAt = 0;
    this.options.onReset(reason);
  }
  private expire(): boolean {
    if (this.closed || !this.binding?.canInput) return false;
    const now = this.now();
    const stale = now - this.lastProgressAt > 750;
    if (
      (stale && (this.held !== 0 || this.queue.some((e) => !e.release))) ||
      this.queue.some((e) => !e.release && now - e.at > 500)
    ) {
      this.resetGesture("timeout");
      // If snapshots stopped too, keep only this cancellation for recovery.
      this.flush(true);
      return true;
    }
    return false;
  }
  private acknowledge(sequence: number): void {
    this.sequence = Math.max(this.sequence, sequence);
    this.acknowledged = Math.max(this.acknowledged, sequence);
    this.queue = this.queue.filter(
      (event) => event.sequence > this.acknowledged,
    );
    for (const flight of this.flights)
      if (flight.sequence <= this.acknowledged) this.flights.delete(flight);
  }

  private flush(priority = false): void {
    if (!this.connected || !this.queue.length || this.now() < this.retryAt)
      return;
    if ([...this.flights].some((flight) => flight.priority === priority))
      return;
    const last = this.queue[this.queue.length - 1];
    // Do not issue a second identical cancellation as an ordinary request.
    if ([...this.flights].some((flight) => flight.sequence >= last.sequence))
      return;
    const firstAt = this.queue[0].at;
    const action: BasketballInputBatch = {
      type: "inputs",
      epoch: this.binding!.epoch,
      seat: this.binding!.seat,
      basedOn: this.binding!.serverAt,
      sequence: last.sequence,
      events: this.queue.map((event) => ({
        sequence: event.sequence,
        buttons: event.buttons,
        offsetMS: Math.round(event.at - firstAt),
        release: event.release,
        ...(event.cancelMask !== undefined
          ? { cancelMask: event.cancelMask }
          : {}),
      })),
    };
    const flight: Flight = {
      sequence: last.sequence,
      priority,
      generation: this.generation,
    };
    this.flights.add(flight);
    let sent: Promise<void>;
    try {
      sent = this.options.send(action);
    } catch (error) {
      sent = Promise.reject(error);
    }
    void sent.then(
      () => {
        if (this.closed || flight.generation !== this.generation) return;
        this.flights.delete(flight);
        this.acknowledge(flight.sequence);
        if (!this.expire()) this.flush();
      },
      () => {
        if (this.closed || flight.generation !== this.generation) return;
        this.flights.delete(flight);
        if (
          flight.sequence <= this.acknowledged ||
          this.queue.some(
            (event) => event.release && event.sequence > flight.sequence,
          )
        )
          return;
        if (action.events.at(-1)?.release) {
          // A failing cancellation is retried by a later sample/snapshot,
          // never recursively inside its failing Promise callback.
          this.retryAt = this.now() + 250;
          return;
        }
        this.resetGesture("network");
        this.flush(true);
      },
    );
  }

  dispose(): void {
    if (this.closed) return;
    this.cancel();
    this.closed = true;
    this.generation++;
    this.queue = [];
    this.flights.clear();
    this.held = 0;
    this.options.onReset("disposed");
  }
}
