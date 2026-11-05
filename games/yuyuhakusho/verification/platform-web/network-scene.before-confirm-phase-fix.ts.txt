import {
  frameNumber,
  FRAME_MS,
  MAX_PREDICTION,
  type NetworkView,
  type LocalEvent,
} from "./types";
import type { GameClock } from "../../../web/src/game-api/module";
import {
  replay,
  type NetworkWasm,
  type RuntimeReply,
  type RuntimeView,
} from "./wasm";

type Scheduled = LocalEvent & { frame: bigint };
/** Two isolated Go sessions: authority is replayed; only its fork predicts. */
export class NetworkScene {
  readonly pixels = new Uint8Array(320 * 224 * 4);
  private confirmed = 0;
  private predicted = 0;
  private authority?: NetworkView;
  private confirmedFrame = 0n;
  private predictedFrame = 0n;
  private queue: Scheduled[] = [];
  private held = [0, 0, 0, 0];
  private automated = 0;
  private localSeat: number;
  private current?: RuntimeView;
  private rendered = "";
  private inputTarget = 0n;
  private replayTarget = 0n;
  private tickAt?: number;
  private closed = false;
  constructor(
    private wasm: NetworkWasm,
    private asset: number,
    seat: number,
  ) {
    this.localSeat = seat;
  }
  get frame() {
    return this.predictedFrame;
  }
  get inputFrame() {
    const target =
      this.predictedFrame > this.replayTarget
        ? this.predictedFrame
        : this.replayTarget;
    return target + 1n;
  }
  get view() {
    return this.current;
  }
  get renderedFrame() {
    return this.rendered;
  }
  get authorityFrame() {
    return this.confirmedFrame;
  }
  private check(reply: RuntimeReply, view: NetworkView) {
    if (
      !reply.view ||
      reply.view.frame !== view.frame ||
      reply.rulesDigest !== view.digest ||
      reply.presentationDigest !== view.presentationDigest
    )
      throw Error("权威状态校验不同，需要重新同步");
  }
  restore(checkpoint: string, view: NetworkView, pending: LocalEvent[] = []) {
    if (this.closed || typeof checkpoint !== "string" || checkpoint.length < 1)
      throw Error("恢复状态无效");
    const result = this.wasm.request({
      op: "restoreCheckpoint",
      asset: this.asset,
      checkpoint,
    });
    const session = result.session;
    if (!session) throw Error("未能恢复对局");
    try {
      this.check(
        { ...result, ...this.wasm.request({ op: "digests", session }) },
        view,
      );
    } catch (error) {
      this.wasm.request({ op: "dispose", session });
      throw error;
    }
    this.dropSessions();
    this.confirmed = session;
    this.authority = view;
    this.confirmedFrame = frameNumber(view.frame);
    this.rebuild(pending);
    this.replayTarget = this.confirmedFrame;
    this.tickAt = undefined;
    // Preview draws F+1 on a detached fork. It never advances authority F.
    const preview = this.wasm.request(
      { op: "preview", session: this.confirmed },
      this.pixels,
    );
    this.rendered = preview.renderedFrame ?? "";
  }
  accept(view: NetworkView, pending: LocalEvent[]) {
    if (
      !this.authority ||
      view.epoch !== this.authority.epoch ||
      view.resourceDescriptorSHA256 !==
        this.authority.resourceDescriptorSHA256 ||
      view.options.arena !== this.authority.options.arena
    )
      throw Error("对局身份已变化");
    const target = frameNumber(view.frame);
    if (target < this.confirmedFrame) return false;
    if (target === this.confirmedFrame) {
      this.check(
        this.wasm.request({ op: "digests", session: this.confirmed }),
        view,
      );
      this.authority = view;
      return false;
    }
    if (target - this.confirmedFrame > 120n) throw Error("权威帧超出重放窗口");
    const previousTarget = this.predictedFrame;
    const result = replay(this.wasm, this.confirmed, view.replay, view.frame);
    this.check(result, view);
    this.confirmedFrame = target;
    this.authority = view;
    this.rebuild(pending);
    const limit = this.confirmedFrame + BigInt(MAX_PREDICTION);
    this.replayTarget =
      previousTarget > this.confirmedFrame
        ? previousTarget
        : this.confirmedFrame;
    if (this.replayTarget > limit) this.replayTarget = limit;
    if (previousTarget < this.confirmedFrame) this.tickAt = undefined;
    return true;
  }
  private rebuild(pending: LocalEvent[]) {
    if (this.predicted)
      this.wasm.request({ op: "dispose", session: this.predicted });
    const fork = this.wasm.request({ op: "fork", session: this.confirmed });
    if (!fork.session) throw Error("无法创建本地预测");
    this.predicted = fork.session;
    this.predictedFrame = this.confirmedFrame;
    this.current = fork.view;
    this.held = [0, 0, 0, 0];
    this.automated = 0;
    this.authority!.inputs.forEach((input, seat) => {
      this.held[seat] = input.appliedButtons;
      if (input.automated) this.automated |= 1 << seat;
    });
    let next = this.confirmedFrame + 1n;
    this.queue = pending
      .slice(0, 64)
      .sort((a, b) => a.sequence - b.sequence)
      .map((event) => {
        const frame =
          event.frame !== undefined && event.frame > next ? event.frame : next;
        next = frame + 1n;
        return { ...event, frame };
      });
    this.inputTarget = this.queue[0]?.frame ?? this.confirmedFrame;
  }
  queueInput(event: LocalEvent) {
    if (this.closed || this.localSeat < 0) return;
    const frame =
        event.frame !== undefined && event.frame > this.inputFrame
          ? event.frame
          : this.inputFrame,
      previous = this.queue.at(-1)?.frame;
    this.queue.push({
      ...event,
      frame:
        previous !== undefined && previous >= frame ? previous + 1n : frame,
    });
    if (this.queue.length > 64) throw Error("本地输入超过上限");
    this.inputTarget = this.queue[0]?.frame ?? frame;
  }
  release() {
    this.queue = [];
    if (this.localSeat >= 0) this.held[this.localSeat] = 0;
    this.inputTarget = this.predictedFrame + 1n;
  }
  sample(now: number, _clock: GameClock | undefined) {
    const view = this.authority;
    if (
      this.closed ||
      !view ||
      !this.predicted ||
      view.phase === "loading" ||
      view.phase === "result"
    )
      return false;
    const limit = this.confirmedFrame + BigInt(MAX_PREDICTION);
    let changed = false,
      steps = 0;
    const advance = (target: bigint) => {
      while (
        this.predictedFrame < target &&
        this.predictedFrame < limit &&
        steps++ < MAX_PREDICTION
      ) {
        const next = this.predictedFrame + 1n;
        // One transition per tick preserves press/release taps even after correction.
        if (this.queue[0] && this.queue[0].frame <= next) {
          const event = this.queue.shift()!;
          if (this.localSeat >= 0) this.held[this.localSeat] = event.buttons;
        }
        const result = this.wasm.request(
          {
            op: "step",
            session: this.predicted,
            held: this.held,
            automated: this.automated,
          },
          this.pixels,
        );
        if (!result.view) throw Error("本地画面不可用");
        this.current = result.view;
        this.predictedFrame = frameNumber(result.view.frame);
        this.rendered = result.renderedFrame ?? "";
        changed = true;
        if (result.view.completed || this.predictedFrame < next) break;
      }
    };
    // Correct to the previous local target without consuming native clock
    // ticks. Packet age is not a reason to instantly spend all12 lead frames.
    advance(this.replayTarget);
    this.replayTarget = this.predictedFrame;
    this.inputTarget = this.queue[0]?.frame ?? this.predictedFrame;
    if (this.current?.completed) return changed;
    if (this.tickAt === undefined) this.tickAt = now - FRAME_MS;
    const due = Math.max(0, Math.floor((now - this.tickAt + 1e-6) / FRAME_MS));
    const beforeNative = this.predictedFrame;
    let target = beforeNative + BigInt(Math.min(MAX_PREDICTION, due));
    if (this.inputTarget > target) target = this.inputTarget;
    if (target > limit) target = limit;
    advance(target);
    const advanced = Number(this.predictedFrame - beforeNative);
    this.tickAt += advanced * FRAME_MS;
    // A genuinely stalled authority freezes at the same fixed cap. Discard
    // elapsed debt so its return cannot create an ever-growing catch-up burst.
    if (this.predictedFrame >= limit && due > advanced) this.tickAt = now;
    this.inputTarget = this.queue[0]?.frame ?? this.predictedFrame;
    return changed;
  }
  private dropSessions() {
    for (const session of [this.predicted, this.confirmed])
      if (session) this.wasm.request({ op: "dispose", session });
    this.predicted = this.confirmed = 0;
  }
  dispose() {
    if (this.closed) return;
    this.closed = true;
    this.dropSessions();
    this.wasm.request({ op: "disposeAssets", asset: this.asset });
  }
}
