import { DisplayReconciliation } from "./reconciliation";
import type { BasketballCheckpoint, BasketballRuntimeState } from "./client";
import type { BasketballWasm } from "./wasm";
import type { BasketballMusicState } from "./audio";

export type RecordedVideoInput = {
  frame: number;
  held: number[];
  priorHeld: number[];
  pendingPressed: number[];
  automated: boolean[];
};
export type InputWindow = { from: number; runs: number[][] };
export type BasketballNetworkView = {
  protocol: number;
  frame: number;
  court: number;
  syncRevision: number;
  digest: string;
  replay: InputWindow;
  latches: [number, number];
  loading?: { ready: boolean[] };
  presentationAutomaticAt?: number;
  attributes?: import("./campaign-types").CampaignAttributes;
  campaign?: import("./campaign-types").CampaignView;
  preparation?: import("./campaign-types").CampaignPreparationView;
  canPrepare?: boolean;
  options?: { court: string; mode: string };
  seatActors?: number[];
  localActor?: number;
  music?: BasketballMusicState;
  epoch: string;
  serverAt: number;
  phase: string;
  canInput: boolean;
  inputs: {
    sequence: number;
    buttons: number;
    appliedButtons: number;
    automated: boolean;
  }[];
  events: { sequence: number; videoFrame: number; ids: number[] }[];
  soundSequence: number;
  scores: number[];
};
export type BasketballSyncSnapshot = {
  view: BasketballNetworkView;
  checkpoint: BasketballCheckpoint;
};
export type BasketballVideoFrame = {
  frame: number;
  display: BasketballRuntimeState["display"];
};
export type CompactRuntimeView = BasketballVideoFrame & {
  session: number;
  finished: boolean;
  hasOutcome: boolean;
  digest?: string;
  frames?: BasketballVideoFrame[];
};
type Options = {
  engine: BasketballWasm;
  controller?: number;
  onFrame: (frame: BasketballVideoFrame) => void;
  onSound?: (ids: readonly number[], sequence: number) => void;
  onReset?: (sequence: number, music?: BasketballMusicState) => void;
  onPredictionError?: (error: unknown) => void;
  onResync?: (reason: string) => void;
};
const frameMS = 1000 / 60;
const flags = (view: BasketballNetworkView) =>
  view.inputs.map((input) => input.automated);
function windowEnd(window: InputWindow): number {
  if (
    !window ||
    !Number.isSafeInteger(window.from) ||
    window.from < 0 ||
    !Array.isArray(window.runs) ||
    window.runs.length > 120
  )
    throw Error("Invalid input window");
  let count = 0;
  for (const run of window.runs) {
    if (
      run.length !== 5 ||
      !run.every(Number.isSafeInteger) ||
      run[0] < 1 ||
      run[0] > 120 ||
      run.slice(1, 4).some((n) => n < 0 || n > 0xffffffff) ||
      run[4] < 0 ||
      run[4] > 15
    )
      throw Error("Invalid input run");
    count += run[0];
  }
  if (
    count > 120 ||
    (count > 0 && window.from === 0) ||
    (count === 0 && window.from !== 0) ||
    !Number.isSafeInteger(window.from + count)
  )
    throw Error("Invalid input window range");
  return count ? window.from + count - 1 : 0;
}

/** Input-driven local simulation. Full state crosses JS/Wasm only when an
 * explicit checkpoint is installed; confirmed/predicted forks stay native. */
export class BasketballNetworkScene {
  private options: Options;
  private authority?: BasketballNetworkView;
  private confirmed?: CompactRuntimeView;
  private predicted?: CompactRuntimeView;
  private version = -1;
  private closed = false;
  private queue: BasketballVideoFrame[] = [];
  private pendingSounds: BasketballNetworkView["events"] = [];
  private soundWatermark = 0;
  private shownFrame = -1;
  private nextPlaybackAt = 0;
  private lastTickAt = 0;
  private receivedAt = 0;
  private localButtons = 0;
  private ownSamples = new Map<number, number>();
  private waitingForSnapshot = false;
  private afterGap = false;
  private reason = "";
  private smoothing = new DisplayReconciliation();
  private lastPresented?: BasketballVideoFrame;
  constructor(options: Options) {
    if (
      options.controller !== undefined &&
      (!Number.isInteger(options.controller) ||
        options.controller < 0 ||
        options.controller > 3)
    )
      throw Error("Invalid controller");
    this.options = options;
  }
  get resetReason() {
    return this.reason;
  }
  get presentationFrame() {
    return this.shownFrame;
  }
  get predicting() {
    return (
      this.options.controller !== undefined &&
      !!this.authority?.canInput &&
      !this.waitingForSnapshot
    );
  }
  get needsCheckpoint() {
    return this.waitingForSnapshot || !this.confirmed;
  }
  // Explicit diagnostics only. Ordinary update/tick never exports simulation.
  get lastReplayState(): BasketballRuntimeState | undefined {
    return this.inspect(this.confirmed);
  }
  get predictedState(): BasketballRuntimeState | undefined {
    return this.inspect(this.predicted);
  }
  private inspect(view?: CompactRuntimeView) {
    return view
      ? this.options.engine.request<{ checkpoint: BasketballCheckpoint }>({
          op: "snapshot",
          session: view.session,
        }).checkpoint.state
      : undefined;
  }
  private forget(view?: CompactRuntimeView) {
    if (view)
      this.options.engine.request({ op: "dispose", session: view.session });
  }
  private fork(): CompactRuntimeView {
    return this.options.engine.request({
      op: "fork",
      session: this.confirmed!.session,
      target: this.predicted?.session,
      compact: true,
    });
  }
  bootstrap(
    checkpoint: BasketballCheckpoint,
    view: BasketballNetworkView,
    version: number,
    now: number,
  ): void {
    if (this.closed) return;
    if (
      view.protocol !== 1 ||
      !Number.isFinite(now) ||
      !Number.isSafeInteger(version)
    )
      throw Error("Unsupported basketball sync protocol");
    const next = this.options.engine.request<CompactRuntimeView>({
      op: "restore",
      checkpoint,
      compact: true,
      verify: true,
    });
    if (next.frame !== view.frame || next.digest !== view.digest) {
      this.forget(next);
      throw Error("Basketball checkpoint checksum differs");
    }
    this.forget(this.confirmed);
    this.forget(this.predicted);
    this.confirmed = next;
    this.predicted = undefined;
    this.authority = view;
    this.version = version;
    this.waitingForSnapshot = false;
    this.afterGap = false;
    this.queue = [];
    this.pendingSounds = [];
    this.ownSamples.clear();
    this.localButtons = 0;
    this.soundWatermark = view.soundSequence;
    this.receivedAt = this.lastTickAt = now;
    this.nextPlaybackAt = now + frameMS;
    this.reason = "checkpoint";
    this.smoothing.reset();
    this.lastPresented = undefined;
    this.options.onReset?.(view.soundSequence, view.music);
    if (this.predicting) this.predicted = this.fork();
    this.present(next);
  }
  private recover(reason: string): false {
    if (!this.waitingForSnapshot) {
      this.reason = reason;
      this.smoothing.reset();
      this.waitingForSnapshot = true;
      this.queue = [];
      this.pendingSounds = [];
      this.ownSamples.clear();
      this.localButtons = 0;
      this.options.onReset?.(this.authority?.soundSequence ?? 0);
      this.options.onResync?.(reason);
    }
    return false;
  }
  update(view: BasketballNetworkView, version: number, now: number): boolean {
    if (this.closed) return false;
    if (!Number.isSafeInteger(version) || version < 0 || !Number.isFinite(now))
      throw Error("Invalid scene clock/version");
    if (!this.authority || !this.confirmed) return this.recover("initial");
    if (view.epoch !== this.authority.epoch) return this.recover("epoch");
    if (version < this.version || view.frame < this.confirmed.frame)
      return true;
    if (this.waitingForSnapshot) return false;
    if (view.protocol !== 1) return this.recover("protocol");
    if (view.syncRevision !== this.authority.syncRevision)
      return this.recover("revision");
    if (version === this.version && view.canInput === this.authority.canInput)
      return true;
    const previous = this.authority,
      wasPredicting = this.predicting;
    const oldFrame = this.confirmed.frame,
      target = this.predicted?.frame ?? view.frame;
    try {
      const end = windowEnd(view.replay);
      if (
        view.frame > oldFrame &&
        (view.replay.from > oldFrame + 1 || end !== view.frame)
      )
        return this.recover("gap");
      const changed =
        view.frame !== oldFrame ||
        view.digest !== previous.digest ||
        view.latches.some((n, i) => n !== previous.latches[i]) ||
        flags(view).some((n, i) => n !== previous.inputs[i].automated);
      if (changed) {
        const result = this.options.engine.request<CompactRuntimeView>({
          op: "replay",
          session: this.confirmed.session,
          window: view.replay,
          automated: flags(view),
          latches: view.latches,
          compact: true,
          verify: true,
          captureFrames: !(
            this.options.controller !== undefined && view.canInput
          ),
        });
        if (result.frame !== view.frame) return this.recover("gap");
        if (result.digest !== view.digest) return this.recover("checksum");
        this.confirmed = result;
      }
      this.authority = view;
      this.version = version;
      this.receivedAt = now;
      if (this.afterGap) {
        this.queue = [];
        this.pendingSounds = [];
        this.soundWatermark = view.soundSequence;
        this.options.onReset?.(view.soundSequence, view.music);
        this.nextPlaybackAt = now + frameMS;
      } else {
        const known =
          this.pendingSounds.at(-1)?.sequence ?? this.soundWatermark;
        for (const event of view.events)
          if (event.sequence > known) this.pendingSounds.push(event);
      }
      if (this.predicting) {
        if (changed || !wasPredicting || !this.predicted) {
          this.queue = [];
          this.predicted = this.fork();
          const until = Math.min(Math.max(target, view.frame), view.frame + 12);
          while (this.predicted.frame < until && !this.predicted.finished)
            this.advancePrediction(
              this.ownSamples.get(this.predicted.frame + 1) ??
                this.localButtons,
              false,
            );
          if (this.lastPresented?.frame === this.predicted.frame)
            this.smoothing.correct(this.lastPresented, this.predicted, now);
          this.presentPrediction(now);
        }
      } else {
        this.forget(this.predicted);
        this.predicted = undefined;
        this.smoothing.reset();
        if (wasPredicting || this.afterGap) {
          this.queue = [];
          this.ownSamples.clear();
          this.present(this.confirmed);
          this.nextPlaybackAt = now + frameMS;
        } else if (changed)
          this.queue.push(
            ...(this.confirmed.frames ?? []).filter(
              (frame) => frame.frame > this.shownFrame,
            ),
          );
        if (this.queue.length > 120) return this.recover("backlog");
      }
      this.afterGap = false;
      for (const frame of this.ownSamples.keys())
        if (frame <= view.frame) this.ownSamples.delete(frame);
      return true;
    } catch (error) {
      this.options.onPredictionError?.(error);
      return this.recover("replay");
    }
  }
  tick(now: number, sample?: () => number): void {
    if (
      this.closed ||
      !this.authority ||
      !Number.isFinite(now) ||
      now < this.lastTickAt
    )
      return;
    if (now - this.lastTickAt > 250) {
      this.queue = [];
      this.pendingSounds = [];
      this.ownSamples.clear();
      this.cancel();
      this.afterGap = true;
      this.lastTickAt = now;
      this.options.onReset?.(this.authority.soundSequence);
      return;
    }
    while (now + 1e-6 >= this.lastTickAt + frameMS) {
      this.lastTickAt += frameMS;
      if (this.predicting) {
        this.localButtons = sample?.() ?? 0;
        if (
          !Number.isInteger(this.localButtons) ||
          this.localButtons < 0 ||
          this.localButtons > 255
        )
          throw Error("Invalid prediction buttons");
        if (
          this.predicted &&
          !this.predicted.finished &&
          !this.afterGap &&
          now - this.receivedAt <= 250 &&
          this.predicted.frame < this.authority.frame + 12
        ) {
          try {
            this.advancePrediction(this.localButtons, true);
            this.presentPrediction();
          } catch (error) {
            this.options.onPredictionError?.(error);
            this.recover("prediction");
          }
        }
      }
    }
    if (!this.predicting && !this.waitingForSnapshot && !this.afterGap) {
      while (now + 1e-6 >= this.nextPlaybackAt) {
        const frame = this.queue.shift();
        if (!frame) {
          this.nextPlaybackAt = now + frameMS;
          break;
        }
        this.nextPlaybackAt += frameMS;
        this.present(frame);
      }
    }
  }
  private advancePrediction(buttons: number, remember: boolean) {
    const held = this.authority!.inputs.map((input) =>
      input.automated ? 0 : input.appliedButtons,
    );
    held[this.options.controller!] = buttons;
    this.predicted = this.options.engine.request<CompactRuntimeView>({
      op: "advance",
      session: this.predicted!.session,
      ticks: 1,
      held,
      compact: true,
    });
    if (remember) this.ownSamples.set(this.predicted.frame, buttons);
  }
  private presentPrediction(now = this.lastTickAt) {
    if (this.predicted!.display.Visible.result && !this.confirmed!.hasOutcome)
      return;
    this.present(this.smoothing.frame(this.predicted!, now));
  }
  private present(frame: BasketballVideoFrame) {
    this.shownFrame = frame.frame;
    this.lastPresented = frame;
    this.options.onFrame(frame);
    while (
      this.pendingSounds.length &&
      this.pendingSounds[0].videoFrame <= frame.frame
    ) {
      const event = this.pendingSounds.shift()!;
      if (event.sequence <= this.soundWatermark) continue;
      this.soundWatermark = event.sequence;
      this.options.onSound?.(event.ids, event.sequence);
    }
  }
  cancel(mask = 255): void {
    if (this.closed || this.options.controller === undefined) return;
    this.localButtons &= ~mask;
    for (const [frame, held] of this.ownSamples)
      this.ownSamples.set(frame, held & ~mask);
    for (const name of ["confirmed", "predicted"] as const) {
      const view = this[name];
      if (view)
        this[name] = this.options.engine.request<CompactRuntimeView>({
          op: "cancelInput",
          session: view.session,
          controller: this.options.controller,
          buttons: mask,
          compact: true,
        });
    }
  }
  dispose(): void {
    if (this.closed) return;
    this.closed = true;
    this.forget(this.confirmed);
    this.forget(this.predicted);
    this.confirmed = this.predicted = undefined;
    this.queue = [];
    this.pendingSounds = [];
    this.ownSamples.clear();
  }
}
