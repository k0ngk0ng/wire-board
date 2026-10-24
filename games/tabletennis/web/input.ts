import type { Stroke, SwingAction, CancelAction } from "./types";
const clamp = (x: number, a: number, b: number) => Math.max(a, Math.min(b, x));
export function classifyStroke(
  dx: number,
  dy: number,
  durationMS: number,
  shortSide: number,
): Stroke {
  const extent = Math.max(160, shortSide);
  const distance = Math.hypot(dx, dy);
  if (distance < Math.max(8, extent * 0.018))
    return { kind: "block", aim: 0, power: 0.35 };
  return {
    kind: dy > extent * 0.025 ? "chop" : "drive",
    aim: clamp((dx / Math.max(extent * 0.06, Math.abs(dy))) * 0.9, -1, 1),
    power: clamp(
      0.2 + (distance / extent / (Math.max(60, durationMS) / 1000)) * 0.42,
      0.2,
      1,
    ),
  };
}
export type InputBinding = {
  retrySeq?: number;
  identity: string;
  epoch: string;
  rally: number;
  shot: number;
  seq: number;
  serverAt: number;
  enabled: boolean;
};
type Options = {
  send: (a: SwingAction | CancelAction) => Promise<void>;
  onError?: (error: string) => void;
};
/** Discrete strike transport. The platform owns WS/HTTP; the game owns the
 * ball token. Snapshot sequence confirmation releases the in-flight request;
 * a ball stays consumed until its token changes. Old completions are inert.
 */
export class StrokeChannel {
  private binding?: InputBinding;
  private generation = 0;
  private sequence = 0;
  private attemptSeq = 0;
  private pending?: SwingAction;
  private locked = false;
  private closed = false;
  private readonly options: Options;
  constructor(options: Options) {
    this.options = options;
  }
  update(next: InputBinding) {
    if (this.closed) return;
    const old = this.binding;
    const identity =
      !old || old.identity !== next.identity || old.epoch !== next.epoch;
    if (!identity && next.serverAt < old.serverAt) return;
    if (identity || old.rally !== next.rally || old.shot !== next.shot) {
      this.generation++;
      this.pending = undefined;
      this.locked = false;
      this.attemptSeq = 0;
      if (identity) this.sequence = next.seq;
    }
    this.sequence = Math.max(this.sequence, next.seq);
    if (this.pending && next.seq >= this.pending.seq) this.pending = undefined;
    if (this.attemptSeq > 0 && (next.retrySeq ?? 0) >= this.attemptSeq)
      this.locked = false;
    this.binding = { ...next };
  }
  swing(stroke: Stroke, at: number): boolean {
    const b = this.binding;
    if (this.closed || !b?.enabled || this.locked) return false;
    const action: SwingAction = {
      ...stroke,
      type: "swing",
      epoch: b.epoch,
      rally: b.rally,
      shot: b.shot,
      seq: ++this.sequence,
      at: Math.round(at),
    };
    this.attemptSeq = action.seq;
    this.pending = action;
    this.locked = true;
    const generation = this.generation;
    void this.options.send(action).then(
      () => {
        if (generation === this.generation && this.pending === action)
          this.pending = undefined;
      },
      (error: unknown) => {
        if (
          generation !== this.generation ||
          this.closed ||
          this.binding!.seq >= action.seq
        )
          return;
        if (this.pending === action) this.pending = undefined;
        this.locked = false;
        this.options.onError?.(
          error instanceof Error ? error.message : "挥拍未能提交，请重试",
        );
      },
    );
    return true;
  }
  cancel(emit = true) {
    const b = this.binding;
    if (this.closed || !b || (!this.pending && !this.locked)) return;
    const action: CancelAction = {
      type: "cancel",
      epoch: b.epoch,
      rally: b.rally,
      shot: b.shot,
      seq: ++this.sequence,
    };
    this.generation++;
    this.pending = undefined;
    this.locked = false;
    if (emit) void this.options.send(action).catch(() => {});
  }
  dispose() {
    this.cancel();
    this.closed = true;
    this.generation++;
    this.binding = undefined;
  }
}
