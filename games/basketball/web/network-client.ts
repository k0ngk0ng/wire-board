import type {
  GameAudioPort,
  GameClock,
} from "../../../web/src/game-api/module";
import type { ControllerProfile } from "../../../web/src/controllers/input";
import { BasketballInput } from "./input";
import {
  BasketballAudio,
  type BasketballAudioOptions,
  type BasketballMusicState,
} from "./audio";
import { BasketballRenderer, loadBasketballAssetsWithFallback } from "./renderer";
import type { PackedPostResultRows } from "./postresult-raster";
import { loadBasketballWasm } from "./wasm";
import {
  BasketballNetworkInput,
  type BasketballInputBatch,
} from "./network-input";
import {
  BasketballNetworkScene,
  type BasketballNetworkView,
  type BasketballVideoFrame,
} from "./network-scene";

export type BasketballNetworkClientOptions = {
  root: HTMLElement;
  canvas: HTMLCanvasElement;
  profile: ControllerProfile;
  roomID: string;
  userID: string;
  // Authenticated room seat is independent from the original actor/controller.
  seat?: number;
  controller?: number;
  runtimeURL: string;
  assetsURL: string;
  postResult?: boolean;
  audio: GameAudioPort;
  audioOptions?: BasketballAudioOptions;
  send: (action: BasketballInputBatch) => Promise<void>;
  inputWindow?: () => number;
  onError: (error: unknown) => void;
  onConnection?: (connected: boolean) => void;
};

/** Live platform adapter: one user's physical sources, reliable HTTP input,
 * authoritative playback, local Wasm prediction, renderer and platform audio.
 */
export class BasketballNetworkClient {
  private readonly options: BasketballNetworkClientOptions;
  private readonly input: BasketballInput;
  private readonly transport: BasketballNetworkInput;
  private readonly scene: BasketballNetworkScene;
  private readonly sound: BasketballAudio;
  private readonly renderer: BasketballRenderer;
  private frameRequest = 0;
  private enabled = false;
  private closed = false;
  private inputGeneration = 0;
  private courtKey = "";
  private imageKey = "";
  private connection?: boolean;
  private musicState?: BasketballMusicState;
  private clock?: GameClock;
  private audioGeneration = 0;
  private soundSequence = 0;
  private recoveryPending = false;
  private lastViewAt = 0;
  private viewVersion = -1;
  private hasAuthority = false;

  static async create(
    options: BasketballNetworkClientOptions,
  ): Promise<BasketballNetworkClient> {
    const [engine, assets] = await Promise.all([
      loadBasketballWasm(options.runtimeURL),
      loadBasketballAssetsWithFallback(options.assetsURL, undefined, {postResult: options.postResult}),
    ]);
    return new BasketballNetworkClient(
      options,
      new BasketballRenderer(options.canvas, assets, projection => engine.request<PackedPostResultRows>({op: "projectPresentation", projection})),
      engine,
    );
  }
  private constructor(
    options: BasketballNetworkClientOptions,
    renderer: BasketballRenderer,
    engine: Awaited<ReturnType<typeof loadBasketballWasm>>,
  ) {
    this.options = options;
    this.renderer = renderer;
    this.sound = new BasketballAudio(options.audio, options.audioOptions);
    this.scene = new BasketballNetworkScene({
      engine,
      controller: options.controller,
      onFrame: (frame) => this.present(frame),
      onSound: (ids, seq) => {
        this.soundSequence = seq;
        if (
          ids.some(
            (id) =>
              id === 0 ||
              id === 253 ||
              id === 255 ||
              id === this.musicState?.trackID,
          )
        )
          this.audioGeneration++;
        const generation = this.audioGeneration;
        void this.sound.consume(ids, seq).then((report) => {
          if (this.closed || generation !== this.audioGeneration) return;
          if (
            report.failed.some(
              (id) => id === 255 || id === this.musicState?.trackID,
            )
          )
            this.recoveryPending = true;
          else if (
            ids.some(
              (id) => id === 0 || id === 255 || id === this.musicState?.trackID,
            )
          )
            this.recoveryPending = false;
        });
      },
      onReset: (seq, music) => {
        this.audioGeneration++;
        this.soundSequence = seq;
        this.sound.reset(seq);
        this.recoveryPending = false;
        if (music) {
          this.musicState = music;
          this.restoreMusic();
        }
      },
      onPredictionError: options.onError,
    });
    this.transport = new BasketballNetworkInput({
      send: options.send,
      inputWindow: options.inputWindow,
      onReset: () => {
        this.inputGeneration++;
        this.input?.cancelLocal();
        this.scene.cancel();
      },
    });
    this.input = new BasketballInput(options.profile, options.root, (mask) => {
      if (this.closed) return;
      this.transport.cancel(mask);
      this.scene.cancel(mask);
    });
    this.input.setEnabled(false);
    window.addEventListener("pointerdown", this.recoverOnGesture, true);
    window.addEventListener("keydown", this.recoverOnGesture, true);
    this.frameRequest = requestAnimationFrame(this.tick);
  }
  readonly controllerSink = {
    pressPointer: (id: number, buttons: readonly string[]) => {
      if (this.closed || !this.enabled) return;
      this.options.root.focus({ preventScroll: true });
      this.input.pressPointer(id, buttons);
    },
    releasePointer: (id: number, cancelled: boolean) => {
      if (!this.closed) this.input.releasePointer(id, cancelled);
    },
  };
  update(
    view: BasketballNetworkView,
    version: number,
    enabled: boolean,
    clock?: GameClock,
  ): void {
    if (this.closed || version < this.viewVersion) return;
    this.hasAuthority = true;
    this.viewVersion = version;
    this.lastViewAt = performance.now();
    this.clock = clock;
    this.musicState = view.music;
    const actor = this.options.controller ?? -1;
    const seat = this.options.seat ?? this.options.controller ?? -1;
    const allowed = enabled && seat >= 0 && view.canInput;
    this.transport.update({
      roomID: this.options.roomID,
      userID: this.options.userID,
      seat,
      epoch: view.epoch,
      serverAt: view.serverAt,
      sequence: actor >= 0 ? view.inputs[actor].sequence : 0,
      canInput: allowed,
      version,
    });
    if (this.enabled !== allowed) {
      this.enabled = allowed;
      this.input.setEnabled(allowed);
    }
    this.scene.update(
      { ...view, canInput: allowed },
      version,
      performance.now(),
    );
  }
  configure(profile: ControllerProfile): void {
    if (!this.closed) this.input.configure(profile);
  }
  private restoreMusic() {
    const state = this.musicState;
    if (!state || state.controlSequence > this.soundSequence) return;
    const generation = this.audioGeneration;
    const now = this.clock
      ? this.clock.serverNow +
        Math.max(0, performance.now() - this.clock.receivedAt)
      : state.at;
    void this.sound.restoreMusic(state, now).then((played) => {
      if (this.closed || generation !== this.audioGeneration) return;
      this.recoveryPending = !played && !state.paused && state.trackID !== 0;
    });
  }
  private recoverOnGesture = (event: Event) => {
    if (
      !this.closed &&
      event.isTrusted &&
      this.recoveryPending &&
      performance.now() - this.lastViewAt < 1000
    )
      this.restoreMusic();
  };
  private tick = () => {
    if (this.closed) return;
    try {
      this.scene.tick(performance.now(), () => {
        const generation = this.inputGeneration,
          held = this.input.sample();
        this.transport.sample(held);
        return generation === this.inputGeneration ? held : 0;
      });
      const connected = !this.enabled || this.transport.connected;
      if (connected !== this.connection) {
        this.connection = connected;
        this.options.onConnection?.(connected);
      }
    } catch (error) {
      this.input.cancel();
      this.options.onError(error);
    }
    this.frameRequest = requestAnimationFrame(this.tick);
  };
  private present(frame: BasketballVideoFrame) {
    const d = frame.display;
    this.options.canvas.dataset.videoFrame = String(frame.frame);
    if (!d.HasVisible) {
      this.options.canvas.getContext("2d")?.clearRect(0, 0, 256, 240);
      this.imageKey = "";
      return;
    }
    // Reconciliation can roll back a previously predicted court write without
    // changing its serial. Include retained writes, not just the counter.
    const key = JSON.stringify([
      d.Visible.court,
      d.CourtEpoch,
      d.CourtSerial,
      d.CourtColumns,
      d.CourtPatches,
    ]);
    if (key !== this.courtKey) {
      this.renderer.resetCourt(d.Visible.court);
      this.renderer.commitCourtEvents([
        ...(d.CourtColumns ?? []),
        ...(d.CourtPatches ?? []).map((patch) => ({ patch })),
      ]);
      this.courtKey = key;
      this.imageKey = "";
    }
    const image = JSON.stringify(d.Visible);
    if (image !== this.imageKey) {
      this.renderer.render(d.Visible);
      this.imageKey = image;
    }
  }
  dispose(): void {
    if (this.closed) return;
    this.closed = true;
    this.audioGeneration++;
    window.removeEventListener("pointerdown", this.recoverOnGesture, true);
    window.removeEventListener("keydown", this.recoverOnGesture, true);
    cancelAnimationFrame(this.frameRequest);
    this.input.dispose();
    this.transport.dispose();
    this.scene.dispose();
    // A cancelled asynchronous loader never acquired audio ownership. Its
    // late disposal must not stop a newer client sharing the platform port.
    if (this.hasAuthority) this.sound.dispose();
  }
}
