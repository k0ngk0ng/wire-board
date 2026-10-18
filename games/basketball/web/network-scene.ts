import type { BasketballRuntimeState, BasketballRuntimeView } from "./client";
import type { BasketballWasm } from "./wasm";
import type { BasketballMusicState } from "./audio";

export type RecordedVideoInput = {
  frame: number;
  held: number[];
  priorHeld: number[];
  pendingPressed: number[];
  automated: boolean[];
};
export type BasketballNetworkView = {
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
  runtime: BasketballRuntimeState;
  inputs: {
    sequence: number;
    buttons: number;
    appliedButtons: number;
    automated: boolean;
  }[];
  videoInputs: RecordedVideoInput[] | null;
  events: { sequence: number; videoFrame: number; ids: number[] }[];
  soundSequence: number;
  scores: number[];
};
export type BasketballVideoFrame = {
  frame: number;
  display: BasketballRuntimeState["display"];
};
type ReplayView = BasketballRuntimeView & { frames: BasketballVideoFrame[] };
type Options = {
  engine: BasketballWasm;
  controller?: number;
  onFrame: (frame: BasketballVideoFrame) => void;
  onSound?: (ids: readonly number[], sequence: number) => void;
  onReset?: (soundSequence: number, music?: BasketballMusicState) => void;
  onPredictionError?: (error: unknown) => void;
};
const frameMS = 1000 / 60;

/** Confirmed replay and optional local Go prediction. No basketball physics or
 * AI is implemented in JS. Server authority always replaces the replay anchor;
 * predicted sounds and results are never submitted back to the host.
 */
export class BasketballNetworkScene {
  private readonly options: Options;
  private authority?: BasketballNetworkView;
  private confirmed?: BasketballRuntimeView;
  private predicted?: BasketballRuntimeView;
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
  private reason = "";
  lastReplayState?: BasketballRuntimeState;

  constructor(options: Options) {
    if (
      options.controller !== undefined &&
      (!Number.isInteger(options.controller) ||
        options.controller < 0 ||
        options.controller > 3)
    )
      throw Error("Invalid local basketball controller");
    this.options = options;
  }
  get predictedState(): BasketballRuntimeState | undefined {
    return this.predicted?.state;
  }
  get resetReason(): string {
    return this.reason;
  }
  get presentationFrame(): number {
    return this.shownFrame;
  }
  get predicting(): boolean {
    return this.options.controller !== undefined && !!this.authority?.canInput;
  }

  private restore(
    state: BasketballRuntimeState,
    previous?: BasketballRuntimeView,
  ): BasketballRuntimeView {
    // Create before dispose so an invalid authority cannot destroy the current
    // usable session. Each scene owns at most two persistent Wasm sessions.
    const view = this.options.engine.request<BasketballRuntimeView>({
      op: "restore",
      checkpoint: { version: 1, state },
    });
    if (previous)
      this.options.engine.request({ op: "dispose", session: previous.session });
    return view;
  }
  private reset(
    view: BasketballNetworkView,
    version: number,
    now: number,
    reason: string,
  ) {
    this.confirmed = this.restore(view.runtime, this.confirmed);
    if (this.predicted) {
      this.options.engine.request({
        op: "dispose",
        session: this.predicted.session,
      });
      this.predicted = undefined;
    }
    this.authority = structuredClone(view);
    this.version = version;
    this.queue = [];
    this.pendingSounds = [];
    this.ownSamples.clear();
    this.localButtons = 0;
    this.soundWatermark = view.soundSequence;
    this.reason = reason;
    this.receivedAt = this.lastTickAt = now;
    this.nextPlaybackAt = now + 100;
    this.waitingForSnapshot = false;
    this.options.onReset?.(view.soundSequence, view.music);
    if (this.predicting) this.predicted = this.restore(view.runtime);
    this.present({
      frame: view.runtime.videoFrames,
      display: view.runtime.display,
    });
  }

  update(view: BasketballNetworkView, version: number, now: number): void {
    if (this.closed) return;
    if (!Number.isSafeInteger(version) || version < 0 || !Number.isFinite(now))
      throw Error("Invalid scene clock/version");
    if (!this.authority || view.epoch !== this.authority.epoch) {
      this.reset(view, version, now, this.authority ? "epoch" : "initial");
      return;
    }
    if (
      version <= this.version ||
      view.runtime.videoFrames < this.confirmed!.state.videoFrames
    )
      return;
    if (this.waitingForSnapshot) {
      this.reset(view, version, now, "resume");
      return;
    }
    const previousFrame = this.confirmed!.state.videoFrames;
    const delta = view.runtime.videoFrames - previousFrame;
    const inputs = (view.videoInputs ?? []).filter(
      (input) =>
        input.frame > previousFrame && input.frame <= view.runtime.videoFrames,
    );
    if (
      delta > 120 ||
      inputs.length !== delta ||
      inputs.some((input, index) => input.frame !== previousFrame + index + 1)
    ) {
      this.reset(view, version, now, "gap");
      return;
    }
    const wasPredicting = this.predicting;
    const target =
      this.predicted?.state.videoFrames ?? view.runtime.videoFrames;
    let frames: BasketballVideoFrame[] = [];
    if (inputs.length) {
      const replay = this.options.engine.request<ReplayView>({
        op: "replay",
        session: this.confirmed!.session,
        inputs,
      });
      this.lastReplayState = replay.state;
      this.confirmed = replay;
      frames = replay.frames;
    }
    // A host cancellation/reclaim may modify input state after the last video
    // tick; its current snapshot is the exact anchor for the next interval.
    this.confirmed = this.restore(view.runtime, this.confirmed);
    this.authority = structuredClone(view);
    this.version = version;
    this.receivedAt = now;
    this.waitingForSnapshot = false;
    const knownSound =
      this.pendingSounds.at(-1)?.sequence ?? this.soundWatermark;
    for (const event of view.events)
      if (event.sequence > knownSound)
        this.pendingSounds.push({ ...event, ids: [...event.ids] });
    if (this.pendingSounds.length > 64) {
      this.pendingSounds = [];
      this.soundWatermark = view.soundSequence;
      this.options.onReset?.(view.soundSequence, view.music);
    }
    if (this.predicting) {
      this.queue = [];
      this.predicted = this.restore(view.runtime, this.predicted);
      const until = Math.min(
        Math.max(target, view.runtime.videoFrames),
        view.runtime.videoFrames + 12,
      );
      try {
        while (
          this.predicted.state.videoFrames < until &&
          !this.predicted.state.outcome?.confirmed
        ) {
          const frame = this.predicted.state.videoFrames + 1;
          this.advancePrediction(
            this.ownSamples.get(frame) ?? this.localButtons,
            false,
          );
        }
        this.presentPrediction();
      } catch (error) {
        this.predictionFailed(error);
      }
    } else {
      if (this.predicted) {
        this.options.engine.request({
          op: "dispose",
          session: this.predicted.session,
        });
        this.predicted = undefined;
      }
      if (wasPredicting) {
        this.queue = [];
        this.ownSamples.clear();
        this.nextPlaybackAt = now + frameMS;
      }
      this.queue.push(
        ...frames.filter((frame) => frame.frame > this.shownFrame),
      );
      if (this.queue.length > 120) {
        this.reset(view, version, now, "backlog");
        return;
      }
    }
    for (const frame of this.ownSamples.keys())
      if (frame <= view.runtime.videoFrames) this.ownSamples.delete(frame);
  }

  /** Sample only on video boundaries, so a 144 Hz RAF cannot consume a short
   * physical tap on a frame where the 60 Hz Go runtime does not advance. */
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
      this.localButtons = 0;
      this.waitingForSnapshot = true;
      this.lastTickAt = now;
      this.soundWatermark = this.authority.soundSequence;
      this.options.onReset?.(this.soundWatermark);
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
          !this.predicted.state.outcome?.confirmed &&
          !this.waitingForSnapshot &&
          now - this.receivedAt <= 250 &&
          this.predicted.state.videoFrames <
            this.authority.runtime.videoFrames + 12
        ) {
          try {
            this.advancePrediction(this.localButtons, true);
            this.presentPrediction();
          } catch (error) {
            this.predictionFailed(error);
          }
        }
      }
    }
    if (!this.predicting && !this.waitingForSnapshot) {
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
    const next = this.options.engine.request<BasketballRuntimeView>({
      op: "advance",
      session: this.predicted!.session,
      ticks: 1,
      held,
    });
    this.predicted = next;
    if (remember) this.ownSamples.set(next.state.videoFrames, buttons);
  }
  private presentPrediction() {
    const state = this.predicted!.state;
    // The host owns match completion. Do not show a forecast result screen.
    if (state.display.Visible.result && !this.authority!.runtime.outcome)
      return;
    this.present({ frame: state.videoFrames, display: state.display });
  }
  private predictionFailed(error: unknown) {
    if (this.predicted)
      this.options.engine.request({
        op: "dispose",
        session: this.predicted.session,
      });
    this.predicted = undefined;
    this.options.onPredictionError?.(error);
    // Confirmed server updates remain playable; no guessed rule or AI fallback.
    this.present({
      frame: this.authority!.runtime.videoFrames,
      display: this.authority!.runtime.display,
    });
  }
  private present(frame: BasketballVideoFrame) {
    this.shownFrame = frame.frame;
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
    if (this.closed || !this.predicted || this.options.controller === undefined)
      return;
    this.localButtons &= ~mask;
    for (const [frame, held] of this.ownSamples)
      this.ownSamples.set(frame, held & ~mask);
    this.predicted = this.options.engine.request<BasketballRuntimeView>({
      op: "cancelInput",
      session: this.predicted.session,
      controller: this.options.controller,
      buttons: mask,
    });
  }
  dispose(): void {
    if (this.closed) return;
    this.closed = true;
    for (const view of [this.confirmed, this.predicted])
      if (view)
        this.options.engine.request({ op: "dispose", session: view.session });
    this.confirmed = this.predicted = undefined;
    this.queue = [];
    this.pendingSounds = [];
    this.ownSamples.clear();
  }
}
