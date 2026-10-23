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
import { BasketballRenderer, loadBasketballAssets } from "./renderer";
import type { LoadingProgress } from "./loading";
import type { PackedPostResultRows } from "./postresult-raster";
import { loadBasketballWasm } from "./wasm";
import {
  BasketballNetworkInput,
  type BasketballInputBatch,
} from "./network-input";
import {
  BasketballNetworkScene,
  type BasketballNetworkView,
  type BasketballSyncSnapshot,
  type BasketballVideoFrame,
} from "./network-scene";

export type BasketballCheckpointResponse = {
  roomId: string;
  version: number;
  serverInstance: string;
  serverNow: number;
  data: BasketballSyncSnapshot;
};

export type BasketballNetworkClientOptions = {
  epoch: string;
  loadCheckpoint: () => Promise<BasketballCheckpointResponse>;
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
  court?: number;
  onProgress?: (p: LoadingProgress) => void;
  audio: GameAudioPort;
  audioOptions?: BasketballAudioOptions;
  send: (action: BasketballInputBatch) => Promise<void>;
  inputWindow?: () => number;
  onError: (error: unknown) => void;
  onConnection?: (connected: boolean) => void;
};

/** Live platform adapter: one user's physical sources, shared ordered input,
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
  private pendingCourt?: {
    view: BasketballNetworkView;
    version: number;
    enabled: boolean;
    clock?: GameClock;
  };
  private courtLoading = false;
  private syncing = false;
  private syncViews: Array<{ view: BasketballNetworkView; version: number }> =
    [];
  private syncRequest?: Promise<boolean>;
  private desired?: {
    view: BasketballNetworkView;
    version: number;
    enabled: boolean;
    clock?: GameClock;
  };

  static async create(
    options: BasketballNetworkClientOptions,
  ): Promise<BasketballNetworkClient> {
    let engineProgress = 0,
      artProgress = 0,
      initial = true;
    const report = (part: "engine" | "art", p: LoadingProgress) => {
      if (part === "engine") engineProgress = p.percent;
      else artProgress = p.percent;
      options.onProgress?.({
        ...p,
        percent: initial
          ? Math.min(98, engineProgress * 0.55 + artProgress * 0.43)
          : p.percent,
      });
    };
    const [engine, assets] = await Promise.all([
      loadBasketballWasm(options.runtimeURL, (p) => report("engine", p)),
      loadBasketballAssets(options.assetsURL, {
        postResult: options.postResult,
        court: options.court,
        onProgress: (p) => report("art", p),
      }),
    ]);
    initial = false;
    return new BasketballNetworkClient(
      options,
      new BasketballRenderer(options.canvas, assets, (projection) =>
        engine.request<PackedPostResultRows>({
          op: "projectPresentation",
          projection,
        }),
      ),
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
      onPredictionError: (error) =>
        console.warn("Basketball replay needs recovery", error),
      onResync: () => {
        void this.initialize().catch((error) => {
          if (!this.closed) this.options.onError(error);
        });
      },
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
  /** Initial entry and explicit recovery only; ordinary snapshots never call
   * this endpoint. The caller owns the client before initialization begins. */
  private setConnection(value: boolean) {
    if (this.connection !== value) {
      this.connection = value;
      this.options.onConnection?.(value);
    }
  }
  initialize(): Promise<boolean> {
    if (this.closed) return Promise.resolve(false);
    if (this.syncRequest) return this.syncRequest;
    this.syncing = true;
    this.syncViews = [];
    this.enabled = false;
    this.input.setEnabled(false);
    this.scene.cancel();
    this.setConnection(false);
    const pending = (async () => {
      for (let attempt = 0; attempt < 3; attempt++) {
        const reply = await this.options.loadCheckpoint();
        if (this.closed) return false;
        const { view, checkpoint } = reply.data;
        if (
          reply.roomId !== this.options.roomID ||
          view.epoch !== this.options.epoch ||
          view.localActor !== (this.options.controller ?? -1)
        )
          throw Error("对局或控制席位已改变");
        await this.renderer.ensureCourt(view.court);
        if (this.closed) return false;
        this.clock = this.desired?.clock;
        this.musicState = view.music;
        this.hasAuthority = true;
        this.scene.bootstrap(
          checkpoint,
          { ...view, canInput: false },
          reply.version,
          performance.now(),
          true,
        );
        let latest = { view, version: reply.version },
          complete = true;
        const received = this.syncViews;
        this.syncViews = [];
        for (const item of received) {
          if (item.version <= latest.version || item.view.epoch !== view.epoch)
            continue;
          if (
            !this.scene.update(
              { ...item.view, canInput: false },
              item.version,
              performance.now(),
            )
          ) {
            complete = false;
            break;
          }
          latest = item;
        }
        if (!complete) continue;
        this.scene.finishRecovery(performance.now());
        const enabled = this.desired?.enabled ?? false,
          clock = this.desired?.clock;
        this.viewVersion = latest.version;
        this.desired = { ...latest, enabled, clock };
        this.syncing = false;
        this.update(latest.view, latest.version, enabled, clock);
        this.setConnection(true);
        return true;
      }
      throw Error("同步数据未能及时接续，请重新连接。");
    })();
    this.syncRequest = pending;
    void pending
      .finally(() => {
        if (this.syncRequest === pending) {
          this.syncRequest = undefined;
          this.syncing = false;
        }
      })
      .catch(() => {});
    return pending;
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
    if (this.closed) return;
    if (!this.desired || version >= this.desired.version)
      this.desired = { view, version, enabled, clock };
    if (this.syncing) {
      if (!this.syncViews.length || version > this.syncViews.at(-1)!.version)
        this.syncViews.push({ view, version });
      if (this.syncViews.length > 64) this.syncViews.shift();
      return;
    }
    // A checkpoint can be newer than React's current room snapshot. Apply
    // local enable/disable immediately against that newer authority instead
    // of waiting for another network frame (notably after switching users).
    if (
      version < this.viewVersion &&
      this.desired?.version === this.viewVersion
    ) {
      this.update(this.desired.view, this.desired.version, enabled, clock);
      return;
    }
    if (
      this.closed ||
      version < this.viewVersion ||
      (this.pendingCourt && version < this.pendingCourt.version)
    )
      return;
    const court = view.court;
    if (!this.renderer.hasCourt(court) || this.courtLoading) {
      this.pendingCourt = { view, version, enabled, clock };
      this.enabled = false;
      this.input.setEnabled(false);
      this.scene.cancel();
      if (!this.courtLoading) {
        this.courtLoading = true;
        void this.renderer
          .ensureCourt(court)
          .then(() => {
            this.courtLoading = false;
            if (this.closed) return;
            const pending = this.pendingCourt;
            this.pendingCourt = undefined;
            if (pending)
              this.update(
                pending.view,
                pending.version,
                pending.enabled,
                pending.clock,
              );
            if (!this.courtLoading)
              this.options.onProgress?.({ percent: 100, label: "球场已就绪" });
          })
          .catch((error) => {
            this.courtLoading = false;
            if (!this.closed) this.options.onError(error);
          });
      }
      return;
    }
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
      if (!this.courtLoading && !this.syncing)
        this.scene.tick(performance.now(), () => {
          const generation = this.inputGeneration,
            held = this.input.sample();
          this.transport.sample(held);
          return generation === this.inputGeneration ? held : 0;
        });
      const connected =
        !this.syncing &&
        !this.scene.needsCheckpoint &&
        (!this.enabled || this.transport.connected);
      this.setConnection(connected);
    } catch (error) {
      this.input.cancel();
      this.options.onError(error);
    }
    this.frameRequest = requestAnimationFrame(this.tick);
  };
  private present(frame: BasketballVideoFrame) {
    const d = frame.display;
    this.options.canvas.dataset.videoFrame = String(frame.frame);
    const actor = this.options.controller;
    if (actor !== undefined && d.HasVisible) {
      // Diagnostics report the actual committed drawing coordinates.
      const object = d.Visible.objects[actor];
      this.options.canvas.dataset.playerAction = String(object.action);
      this.options.canvas.dataset.ballAction = String(
        d.Visible.objects[6]?.action,
      );
      this.options.canvas.dataset.ballWorldX = String(
        d.Visible.objects[6]?.position.x,
      );
      this.options.canvas.dataset.playerX = String(object.projected.x);
      this.options.canvas.dataset.playerWorldX = String(object.position.x);
      this.options.canvas.dataset.playerVisible = String(
        d.Visible.drawOrder.includes(actor),
      );
      this.options.canvas.dataset.playerY = String(object.projected.y);
      this.options.canvas.dataset.cameraX = String(d.Visible.camera.x);
    }
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
    this.syncViews = [];
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
