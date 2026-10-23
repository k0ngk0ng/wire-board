export type HoldActionCallbacks = {
  onTap: () => unknown | Promise<unknown>;
  onHold: () => unknown | Promise<unknown>;
};
export type HoldActionTimers = {
  set: (callback: () => void, delayMS: number) => unknown;
  clear: (token: unknown) => void;
};
type Options = {
  getCallbacks: () => HoldActionCallbacks;
  onHoldingChange?: (holding: boolean) => void;
  delayMS?: number;
  timers?: HoldActionTimers;
};
type PointerPress = {
  id: number;
  x: number;
  y: number;
  cancelled: boolean;
  fired: boolean;
};

/** Only distinguishes input gestures; resource and action validation stays in the game. */
export function createHoldActionController(options: Options) {
  return new HoldActionController(options);
}

export class HoldActionController {
  private enabled = false;
  private identity = "";
  private disposed = false;
  private delayMS: number;
  private timer: unknown = null;
  private generation = 0;
  private active: PointerPress | null = null;
  private pointers = new Set<number>();
  private keys = new Set<string>();
  private longKeys = new Set<string>();
  private pointerClickPending = false;
  private suppressPointerClick = false;
  private keyboardClickPending = false;
  private suppressKeyboardClick = false;
  private _holding = false;
  private readonly timers: HoldActionTimers;
  private readonly options: Options;

  constructor(options: Options) {
    this.options = options;
    this.delayMS = this.delay(options.delayMS);
    this.timers = options.timers ?? {
      set: (fn, ms) => setTimeout(fn, ms),
      clear: (token) => clearTimeout(token as ReturnType<typeof setTimeout>),
    };
  }
  get holding() {
    return this._holding;
  }
  get pointerActive() {
    return this.pointers.size > 0;
  }
  private delay(value?: number) {
    return value !== undefined && Number.isFinite(value) && value >= 0
      ? value
      : 650;
  }
  private holdingChanged(value: boolean) {
    if (this._holding === value) return;
    this._holding = value;
    this.options.onHoldingChange?.(value);
  }
  private clearTimer() {
    this.generation++;
    if (this.timer !== null) this.timers.clear(this.timer);
    this.timer = null;
  }
  private invoke(kind: "onTap" | "onHold") {
    // The game owns user-facing errors. Never turn a failed hold into a tap,
    // or leave a rejected callback promise unhandled in an event listener.
    try {
      const result = this.options.getCallbacks()[kind]();
      void Promise.resolve(result).catch(() => {});
    } catch {
      /* The input is consumed even if the game callback fails. */
    }
  }
  configure({
    enabled,
    identity,
    delayMS,
  }: {
    enabled: boolean;
    identity: string;
    delayMS?: number;
  }) {
    const delay = this.delay(delayMS ?? this.delayMS);
    if (this.identity !== identity || !enabled || this.delayMS !== delay)
      this.cancel();
    this.enabled = enabled;
    this.identity = identity;
    this.delayMS = delay;
  }
  pointerDown(
    id: number,
    x: number,
    y: number,
    primary = true,
    button = 0,
    multiple = false,
  ) {
    if (this.disposed) return false;
    if (button !== 0) {
      if (this.active) this.cancel();
      return false;
    }
    const fresh = this.pointers.size === 0;
    if (fresh) this.suppressPointerClick = false;
    this.pointerClickPending = true;
    this.pointers.add(id);
    if (!primary || multiple || this.pointers.size > 1 || !fresh) {
      this.cancel();
      return false;
    }
    this.active = { id, x, y, cancelled: false, fired: false };
    if (!this.enabled) {
      this.cancel();
      return false;
    }
    this.clearTimer();
    const generation = this.generation;
    this.holdingChanged(true);
    this.timer = this.timers.set(() => {
      if (
        generation !== this.generation ||
        this.disposed ||
        !this.enabled ||
        !this.active ||
        this.active.id !== id ||
        this.active.cancelled ||
        this.active.fired
      )
        return;
      this.timer = null;
      this.active.fired = true;
      this.suppressPointerClick = true;
      this.holdingChanged(false);
      this.invoke("onHold");
    }, this.delayMS);
    return true;
  }
  additionalPointer(id: number) {
    if (!this.pointers.size || this.pointers.has(id)) return;
    this.pointers.add(id);
    this.cancel();
  }
  pointerMove(id: number, x: number, y: number, inside = true) {
    if (
      !this.active ||
      this.active.id !== id ||
      this.active.cancelled ||
      this.active.fired
    )
      return;
    if (!inside || Math.hypot(x - this.active.x, y - this.active.y) > 8)
      this.cancel();
  }
  pointerUp(id: number, x?: number, y?: number, inside = true) {
    if (!this.pointers.has(id)) return;
    if (x !== undefined && y !== undefined) this.pointerMove(id, x, y, inside);
    this.pointers.delete(id);
    if (this.active?.id === id) {
      this.clearTimer();
      this.active = null;
      this.holdingChanged(false);
    }
  }
  cancelPointer(id: number) {
    if (this.pointers.has(id)) this.cancel();
  }
  click(detail: number) {
    if (this.disposed) return true;
    if (detail === 0) {
      // Keyboard/AT activation must not inherit a touch or mouse suppression.
      if (this.active) this.cancel();
      const suppressed = this.suppressKeyboardClick;
      this.keyboardClickPending = false;
      this.suppressKeyboardClick = false;
      if (!this.enabled || suppressed) return true;
    } else {
      const suppressed = this.suppressPointerClick;
      this.pointerClickPending = false;
      this.suppressPointerClick = false;
      if (!this.enabled || suppressed) return true;
    }
    this.invoke("onTap");
    return false;
  }
  keyDown(key: string, shift = false, repeat = false) {
    if (key !== "Enter" && key !== " ") return false;
    if (this.disposed || !this.enabled) return true;
    if (repeat || this.keys.has(key)) return true;
    if (this.active) this.cancel();
    this.keys.add(key);
    this.keyboardClickPending = true;
    this.suppressKeyboardClick = false;
    if (key === "Enter" && shift) {
      this.longKeys.add(key);
      this.suppressKeyboardClick = true;
      this.invoke("onHold");
      return true;
    }
    return false;
  }
  keyUp(key: string) {
    this.keys.delete(key);
    if (this.longKeys.delete(key)) {
      // Shift+Enter is prevented on keydown, so Enter has no keyup click.
      this.suppressKeyboardClick = false;
      this.keyboardClickPending = false;
    } else if (key === "Enter" && !this.keyboardClickPending) {
      this.suppressKeyboardClick = false;
    }
  }
  cancel() {
    this.clearTimer();
    if (this.pointerClickPending || this.pointers.size)
      this.suppressPointerClick = true;
    if (this.keyboardClickPending || this.keys.size)
      this.suppressKeyboardClick = true;
    if (this.active) this.active.cancelled = true;
    this.holdingChanged(false);
  }
  resetInputs() {
    this.cancel();
    this.pointers.clear();
    this.active = null;
    this.keys.clear();
    this.longKeys.clear();
  }
  dispose() {
    this.resetInputs();
    this.disposed = true;
  }
  activate() {
    if (!this.disposed) return;
    this.disposed = false;
    this.pointerClickPending = false;
    this.suppressPointerClick = false;
    this.keyboardClickPending = false;
    this.suppressKeyboardClick = false;
  }
}
