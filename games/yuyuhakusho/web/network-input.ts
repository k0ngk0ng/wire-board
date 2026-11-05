import type { InputAction, LocalEvent } from "./types";
export type InputBinding = {
  roomID: string;
  userID: string;
  seat: number;
  epoch: string;
  serverAt: number;
  receivedSequence: number;
  appliedSequence: number;
  canInput: boolean;
  releaseAllowed?: boolean;
  version?: number;
};
type Queued = LocalEvent & { release: boolean };
type Flight = { sequence: number; generation: number; release: boolean };
type Options = {
  send: (action: InputAction) => Promise<void>;
  onReset: () => void;
  now?: () => number;
  inputWindow?: () => number;
};
const MAX_SEQUENCE = Number.MAX_SAFE_INTEGER;
export class InputTransport {
  private options: Options;
  private now: () => number;
  private binding?: InputBinding;
  private generation = 0;
  private sequence = 0;
  private received = 0;
  private applied = 0;
  private queue: Queued[] = [];
  private history: LocalEvent[] = [];
  private flights = new Set<Flight>();
  private value = 0;
  private closed = false;
  private lastProgress = 0;
  private lastSent = 0;
  private retryAt = 0;
  constructor(options: Options) {
    this.options = options;
    this.now = options.now ?? (() => performance.now());
  }
  get held() {
    return this.value;
  }
  get pendingCount() {
    return this.queue.length;
  }
  get unapplied() {
    return this.history.map((event) => ({ ...event }));
  }
  get connected() {
    return (
      !this.closed && !!this.binding && this.now() - this.lastProgress <= 750
    );
  }
  update(next: InputBinding) {
    if (this.closed) return;
    if (
      !Number.isSafeInteger(next.serverAt) ||
      next.serverAt < 0 ||
      !Number.isSafeInteger(next.receivedSequence) ||
      next.receivedSequence < 0 ||
      !Number.isSafeInteger(next.appliedSequence) ||
      next.appliedSequence < 0 ||
      next.appliedSequence > next.receivedSequence ||
      !Number.isInteger(next.seat) ||
      next.seat < -1 ||
      next.seat > 3
    )
      throw Error("输入确认格式无效");
    const old = this.binding,
      changed =
        !old ||
        old.roomID !== next.roomID ||
        old.userID !== next.userID ||
        old.seat !== next.seat ||
        old.epoch !== next.epoch;
    if (changed) {
      this.generation++;
      this.queue = [];
      this.history = [];
      this.flights.clear();
      this.value = 0;
      this.sequence = this.received = next.receivedSequence;
      this.applied = next.appliedSequence;
      this.lastProgress = this.lastSent = this.now();
      this.binding = { ...next, canInput: next.canInput && next.seat >= 0 };
      this.options.onReset();
      return;
    }
    if (
      next.serverAt < old.serverAt ||
      next.receivedSequence < this.received ||
      next.appliedSequence < this.applied ||
      (next.version !== undefined &&
        old.version !== undefined &&
        next.version < old.version)
    )
      return;
    if (next.serverAt > old.serverAt || next.receivedSequence > this.received)
      this.lastProgress = this.now();
    this.binding = { ...next, canInput: next.canInput && next.seat >= 0 };
    this.received = next.receivedSequence;
    this.applied = next.appliedSequence;
    this.sequence = Math.max(this.sequence, this.received);
    this.queue = this.queue.filter((item) => item.sequence > this.received);
    this.history = this.history.filter((item) => item.sequence > this.applied);
    if (!this.binding.canInput) {
      if (old.canInput) this.cancel();
      this.options.onReset();
      return;
    }
    this.expire();
    this.flush();
  }
  sample(buttons: number, frame?: bigint): LocalEvent | undefined {
    if (!Number.isInteger(buttons) || buttons < 0 || buttons > 4095)
      throw Error("MD 输入无效");
    if (this.closed || !this.binding?.canInput) return;
    if (this.expire() || !this.connected) {
      this.flush();
      return;
    }
    const at = this.now();
    if (buttons === this.value && (buttons === 0 || at - this.lastSent < 250))
      return;
    if (this.queue.length >= 64 || this.history.length >= 64) {
      this.cancel();
      return;
    }
    const sequence = this.next();
    if (sequence === undefined) return;
    this.value = buttons;
    this.lastSent = at;
    const event = { sequence, buttons, at, frame };
    this.queue.push({ ...event, release: false });
    this.history.push(event);
    this.flush();
    return { ...event };
  }
  private next() {
    if (this.sequence >= MAX_SEQUENCE) {
      this.options.onReset();
      return;
    }
    return ++this.sequence;
  }
  cancel = () => {
    if (this.closed || !this.binding) return;
    const needed =
      this.value !== 0 ||
      this.queue.length > 0 ||
      this.history.length > 0 ||
      this.flights.size > 0;
    this.options.onReset();
    this.value = 0;
    this.queue = [];
    this.history = [];
    if (!needed || this.binding.releaseAllowed === false) {
      this.generation++;
      this.flights.clear();
      return;
    }
    const sequence = this.next();
    if (sequence === undefined) return;
    const event = { sequence, buttons: 0, at: this.now(), release: true };
    this.queue.push(event);
    this.history.push(event);
    this.lastSent = this.now();
    this.flush(true);
  };
  private expire() {
    if (!this.binding?.canInput) return false;
    const oldest = this.queue[0];
    if (
      this.now() - this.lastProgress > 750 ||
      (oldest && this.now() - oldest.at > 500)
    ) {
      if (this.value !== 0 || this.queue.some((item) => !item.release))
        this.cancel();
      return true;
    }
    return false;
  }
  private flush(priority = false) {
    if (this.closed || !this.binding || this.now() < this.retryAt) return;
    const window = Math.max(
      1,
      Math.min(8, Math.floor(this.options.inputWindow?.() ?? 1)),
    );
    const ordinary = [...this.flights].filter((item) => !item.release).length;
    for (const item of this.queue) {
      if (
        item.sequence <= this.received ||
        [...this.flights].some(
          (flight) =>
            flight.sequence === item.sequence &&
            flight.generation === this.generation,
        )
      )
        continue;
      if (item.release) {
        if ([...this.flights].some((flight) => flight.release)) continue;
      } else if (
        priority ||
        ordinary +
          [...this.flights].filter((flight) => !flight.release).length -
          ordinary >=
          window
      )
        break;
      if (!item.release && !this.binding.canInput) continue;
      const flight = {
        sequence: item.sequence,
        generation: this.generation,
        release: item.release,
      };
      this.flights.add(flight);
      const action: InputAction = item.release
        ? {
            type: "release",
            epoch: this.binding.epoch,
            sequence: item.sequence,
          }
        : {
            type: "input",
            epoch: this.binding.epoch,
            sequence: item.sequence,
            buttons: item.buttons,
          };
      void this.options
        .send(action)
        .then(() => {
          if (this.closed || flight.generation !== this.generation) return;
          this.queue = this.queue.filter(
            (queued) => queued.sequence !== item.sequence,
          );
          this.lastProgress = this.now();
        })
        .catch(() => {
          if (this.closed || flight.generation !== this.generation) return;
          this.retryAt = this.now() + 100;
        })
        .finally(() => {
          this.flights.delete(flight);
          if (!this.closed && flight.generation === this.generation)
            this.flush();
        });
    }
  }
  pump() {
    if (this.closed) return;
    this.expire();
    this.flush();
  }
  dispose() {
    if (this.closed) return;
    this.cancel();
    this.closed = true;
    this.generation++;
    this.queue = [];
    this.history = [];
    this.flights.clear();
  }
}
