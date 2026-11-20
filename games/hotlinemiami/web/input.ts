import type { GameInput, InputAction } from "./types.ts";

export function emptyInput(): GameInput {
  return { moveX: 0, moveY: 0, aim: 0, fire: false, throw: false, pickup: false, finish: false, interact: false };
}

type Binding = { identity: string; epoch: string; life: number; receivedSeq: number; enabled: boolean; window: number; frame?: number };

function hasEdge(input: GameInput) {
  return input.throw || input.pickup || input.finish || input.interact;
}

export class InputChannel {
  private send: (action: InputAction) => Promise<void>;
  private error: (message: string) => void;
  private binding: Binding = { identity: "", epoch: "", life: 0, receivedSeq: 0, enabled: false, window: 0 };
  private sequence = 0;
  private queue: InputAction[] = [];
  private inFlight = 0;
  private generation = 0;
  private disposed = false;

  constructor(send: (action: InputAction) => Promise<void>, error: (message: string) => void = () => {}) {
    this.send = send;
    this.error = error;
  }

  get queued() {
    return this.queue.length;
  }

  get seq() {
    return this.sequence;
  }

  update(binding: Binding) {
    const reset = binding.identity !== this.binding.identity || binding.epoch !== this.binding.epoch || binding.life !== this.binding.life;
    const suspension = binding.enabled !== this.binding.enabled || (binding.window > 0) !== (this.binding.window > 0);
    if (reset || suspension) {
      this.generation++;
      this.queue = [];
      this.inFlight = 0;
      this.sequence = reset ? binding.receivedSeq : Math.max(this.sequence, binding.receivedSeq);
    } else {
      this.sequence = Math.max(this.sequence, binding.receivedSeq);
    }
    this.binding = binding;
    if (!binding.enabled || binding.window < 1) this.queue = [];
    this.flush();
  }

  push(input: GameInput, prepare?: (action: InputAction) => void): InputAction | undefined {
    if (this.disposed || !this.binding.enabled || this.binding.window < 1) return;
    const action: InputAction = { type: "input", epoch: this.binding.epoch, life: this.binding.life, seq: ++this.sequence, viewFrame: this.binding.frame ?? 0, ...input };
    const tail = this.queue.at(-1);
    const merge = tail && !hasEdge(tail) && !hasEdge(action) && tail.fire === action.fire;
    if (!merge && this.queue.length >= 64) {
      this.error("连接积压，请等待恢复");
      return;
    }
    prepare?.(action);
    if (merge) this.queue[this.queue.length - 1] = action;
    else this.queue.push(action);
    this.flush();
    return action;
  }

  release() {
    this.queue = [];
    return this.push(emptyInput());
  }

  dispose() {
    this.disposed = true;
    this.generation++;
    this.queue = [];
  }

  private flush() {
    const generation = this.generation;
    while (this.binding.enabled && this.binding.window > this.inFlight && this.queue.length) {
      const action = this.queue.shift()!;
      this.inFlight++;
      void this.send(action).catch(error => {
        if (this.generation === generation && !this.disposed) this.error(error instanceof Error ? error.message : "操作发送失败");
      }).finally(() => {
        if (this.generation !== generation || this.disposed) return;
        this.inFlight--;
        this.flush();
      });
    }
  }
}
