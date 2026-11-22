import type { GameInput, InputAction } from './types.ts';

export function emptyInput(yaw = 0, pitch = 0): GameInput {
  return { moveX: 0, moveY: 0, yaw, pitch, fire: false, secondary: false, jump: false, crouch: false, walk: false, reload: false, use: false, drop: false, select: '' };
}

type Binding = { identity: string; epoch: string; life: number; receivedSeq: number; enabled: boolean; window: number; frame: number };
const edgeKeys = ['fire', 'secondary', 'jump', 'reload', 'use', 'drop', 'flashlight'] as const;
export function hasInputEdge(previous: GameInput, input: GameInput, cancel = false) { return cancel || !!input.select || edgeKeys.some(key => previous[key] !== input[key]); }
export type InputClock = { now: () => number; set: (run: () => void, delay: number) => number; clear: (timer: number) => void };
const nativeClock: InputClock = { now: () => performance.now(), set: (run, delay) => setTimeout(run, delay), clear: timer => clearTimeout(timer) };

function requiresRecovery(error: unknown): boolean {
  let current = error;
  for (let depth = 0; depth < 4 && current && typeof current === 'object'; depth++) {
    const value = current as { status?: number; transientSync?: boolean; message?: string; cause?: unknown };
    if (value.status === 409 || value.transientSync || /传输中已过期|重新同步|实时连接|确认超时/.test(value.message ?? '')) return true;
    current = value.cause;
  }
  return false;
}

export class InputChannel {
  private send: (action: InputAction) => Promise<void>;
  private error: (message: string) => void;
  private clock: InputClock;
  private binding: Binding = { identity: '', epoch: '', life: 0, receivedSeq: 0, enabled: false, window: 0, frame: 0 };
  private sequence = 0;
  private queue: { action: InputAction; edge: boolean }[] = [];
  private inFlight = 0;
  private generation = 0;
  private disposed = false;
  private previous = emptyInput();
  private continuousAt = -10000;
  private timer: number | undefined;
  private recoverAfter: number | undefined;
  private recovery: () => void;

  constructor(send: (action: InputAction) => Promise<void>, error: (message: string) => void = () => {}, clock: InputClock = nativeClock, recovery: () => void = () => {}) {
    this.send = send;
    this.error = error;
    this.clock = clock;
    this.recovery = recovery;
  }

  get queued() { return this.queue.length; }
  get seq() { return this.sequence; }
  get recovering() { return this.recoverAfter !== undefined; }

  update(binding: Binding) {
    const reset = binding.identity !== this.binding.identity || binding.epoch !== this.binding.epoch || binding.life !== this.binding.life;
    if (this.recoverAfter !== undefined && (reset || binding.frame > this.recoverAfter)) { this.recoverAfter = undefined; this.error(''); }
    if (this.recoverAfter !== undefined) binding = { ...binding, enabled: false, window: 0 };
    const suspension = binding.enabled !== this.binding.enabled || (binding.window > 0) !== (this.binding.window > 0);
    if (reset || suspension) {
      this.generation++;
      this.clearTimer();
      this.queue = [];
      this.inFlight = 0;
      this.previous = emptyInput();
      this.continuousAt = -10000;
      this.sequence = reset ? binding.receivedSeq : Math.max(this.sequence, binding.receivedSeq);
    } else this.sequence = Math.max(this.sequence, binding.receivedSeq);
    this.binding = binding;
    if (!binding.enabled || binding.window < 1) { this.queue = []; this.clearTimer(); }
    this.flush();
  }

  push(input: GameInput, prepare?: (action: InputAction) => void, cancel = false): InputAction | undefined {
    if (this.disposed || !this.binding.enabled || this.binding.window < 1) return;
    const edge = hasInputEdge(this.previous, input, cancel);
    const tail = this.queue.at(-1);
    const replace = tail && !tail.edge;
    if (!replace && this.queue.length >= 64) { this.error('连接积压，请等待恢复'); return; }
    const action: InputAction = { ...input, type: 'input', epoch: this.binding.epoch, life: this.binding.life, seq: ++this.sequence, viewFrame: this.binding.frame, ...(cancel ? { cancel: true } : {}) };
    prepare?.(action);
    this.previous = { ...input };
    if (replace) this.queue[this.queue.length - 1] = { action, edge };
    else this.queue.push({ action, edge });
    this.flush();
    return action;
  }

  release(yaw = 0, pitch = 0) { this.queue = []; this.clearTimer(); return this.push(emptyInput(yaw, pitch), undefined, true); }
  dispose() { this.disposed = true; this.generation++; this.queue = []; this.clearTimer(); }
  private clearTimer() { if (this.timer !== undefined) { this.clock.clear(this.timer); this.timer = undefined; } }

  private flush() {
    this.clearTimer();
    const generation = this.generation;
    while (!this.disposed && this.binding.enabled && this.inFlight < Math.min(8, this.binding.window) && this.queue.length) {
      const next = this.queue[0];
      const delay = 10 - (this.clock.now() - this.continuousAt);
      if (!next.edge && delay > 0) {
        this.timer = this.clock.set(() => { this.timer = undefined; if (generation === this.generation) this.flush(); }, delay);
        break;
      }
      this.queue.shift();
      if (!next.edge) this.continuousAt = this.clock.now();
      this.inFlight++;
      void this.send(next.action).catch(error => {
        if (generation !== this.generation || this.disposed) return;
        if (requiresRecovery(error)) {
          this.recoverAfter = this.binding.frame;
          this.generation++;
          this.clearTimer();
          this.queue = [];
          this.inFlight = 0;
          this.binding = { ...this.binding, enabled: false, window: 0 };
          this.previous = emptyInput();
          this.recovery();
        }
        this.error(error instanceof Error ? error.message : '操作发送失败');
      }).finally(() => {
        if (generation !== this.generation || this.disposed) return;
        this.inFlight--;
        this.flush();
      });
    }
  }
}
