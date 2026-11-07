import type {
  GameClock,
  GameAudioPort,
} from "../../../web/src/game-api/module";
import { MatchAudio } from "./audio";
import { releaseAssetBase } from "./release-assets";
import type { ControllerProfile } from "../../../web/src/controllers/input";
import { loadAssets, assetURL } from "./assets";
import { combineLoadingProgress, loadingStage } from "./loading-progress";
import { MDInput, mdBits, type MDButton } from "./input";
import { InputTransport } from "./network-input";
import { NetworkScene } from "./network-scene";
import { loadNetworkWasm } from "./wasm";
import {
  FRAME_MS,
  type NetworkView,
  type CheckpointResponse,
  type InputAction,
  type Progress,
} from "./types";

type ClientOptions = {
  root: HTMLElement;
  canvas: HTMLCanvasElement;
  profile: ControllerProfile;
  roomID: string;
  userID: string;
  seat: number;
  epoch: string;
  arena: number;
  descriptorSHA: string;
  assetsBase: string;
  send: (action: InputAction) => Promise<void>;
  loadCheckpoint: () => Promise<CheckpointResponse>;
  onProgress: (progress: Progress) => void;
  onConnection: (connected: boolean) => void;
  onError: (error: Error) => void;
  inputWindow: () => number;
  physical?: (
    keyboard: Set<string>,
    at: number,
    enabled: boolean,
  ) => Set<string>;
  clearPhysical?: () => void;
  audio?: GameAudioPort;
  sound?: () => boolean;
  paused?: () => boolean;
};
function buttonsForMask(mask: number) {
  return new Set(
    Object.entries(mdBits)
      .filter(([, bit]) => (mask & bit) !== 0)
      .map(([name]) => name),
  );
}
function logicalMask(buttons: Set<string>) {
  let mask = 0;
  for (const button of buttons)
    if (Object.hasOwn(mdBits, button)) mask |= mdBits[button as MDButton];
  if ((mask & 3) === 3) mask &= ~3;
  if ((mask & 12) === 12) mask &= ~12;
  return mask;
}
/** Browser lifecycle/transport only; movement, combat and rendering stay in Go. */
export class NetworkClient {
  private input: MDInput;
  private transport: InputTransport;
  private scene?: NetworkScene;
  private options: ClientOptions;
  private abort = new AbortController();
  private context: CanvasRenderingContext2D;
  private image: ImageData;
  private view?: NetworkView;
  private version = 0;
  private clock?: GameClock;
  private enabled = false;
  private initialized = false;
  private closed = false;
  private recovering?: Promise<void>;
  private retryAt = 0;
  private failures = 0;
  private raf = 0;
  private lastStep = 0;
  private painted = false;
  private audio?: MatchAudio;
  constructor(options: ClientOptions) {
    options = { ...options, assetsBase: releaseAssetBase(options.assetsBase) };
    this.options = options;
    if (options.audio)
      this.audio = new MatchAudio(
        options.audio,
        options.assetsBase,
        options.epoch,
        options.arena,
      );
    document.addEventListener("visibilitychange", this.visibility);
    const context = options.canvas.getContext("2d", { alpha: false });
    if (!context) throw Error("浏览器无法显示比赛画面");
    this.context = context;
    this.image = context.createImageData(320, 224);
    this.input = new MDInput(options.root, options.profile, (remaining) => {
      if (this.closed) return;
      const keyboard = buttonsForMask(remaining),
        combined =
          options.physical?.(keyboard, performance.now(), this.enabled) ??
          keyboard,
        mask = logicalMask(combined);
      if (mask) {
        const event = this.transport?.sample(mask, this.scene?.inputFrame);
        if (event) this.scene?.queueInput(event);
      } else this.transport?.cancel();
    });
    this.transport = new InputTransport({
      send: options.send,
      inputWindow: options.inputWindow,
      onReset: () => {
        this.input?.cancelLocal();
        options.clearPhysical?.();
        this.scene?.release();
      },
    });
    options.canvas.width = 320;
    options.canvas.height = 224;
  }
  async initialize() {
    let runtimeProgress: Progress = { percent: 0, label: "加载游戏程序" },
      assetsProgress: Progress = { percent: 0, label: "读取资源清单" };
    const report = (label: string) =>
      this.options.onProgress(
        combineLoadingProgress(runtimeProgress, assetsProgress, label),
      );
    const [wasm, raw] = await Promise.all([
      loadNetworkWasm(
        assetURL(this.options.assetsBase, "/yuyuhakusho/v1/runtime"),
        (progress) => {
          runtimeProgress = progress;
          report(progress.label);
        },
      ),
      loadAssets(
        this.options.assetsBase,
        this.options.arena,
        this.options.descriptorSHA,
        this.abort.signal,
        (progress) => {
          assetsProgress = progress;
          report(progress.label);
        },
      ),
    ]);
    this.abort.signal.throwIfAborted();
    const downloaded = combineLoadingProgress(
      runtimeProgress,
      assetsProgress,
      "恢复比赛画面",
    );
    this.options.onProgress(loadingStage(downloaded, 92, "恢复比赛画面"));
    const asset = wasm.load(this.options.arena, raw);
    const scene = new NetworkScene(
      wasm,
      asset,
      this.options.seat,
      (view, codes) => this.audio?.consume(view, codes),
    );
    this.scene = scene;
    try {
      const result = await this.options.loadCheckpoint();
      this.abort.signal.throwIfAborted();
      this.validate(result.data.view);
      scene.restore(result.data.checkpoint, result.data.view);
      this.audio?.restore(result.data.view);
      this.view = result.data.view;
      this.version = result.version;
      this.transportBinding();
      this.paint();
      this.initialized = true;
      this.options.onProgress(loadingStage(downloaded, 100, "比赛画面已就绪"));
      this.raf = requestAnimationFrame(this.tick);
    } catch (error) {
      scene.dispose();
      this.scene = undefined;
      throw error;
    }
  }
  private validate(view: NetworkView) {
    if (
      !view ||
      view.protocol !== 1 ||
      view.epoch !== this.options.epoch ||
      view.options.arena !== this.options.arena ||
      view.resourceDescriptorSHA256 !== this.options.descriptorSHA ||
      view.localSeat !== this.options.seat ||
      !Array.isArray(view.inputs) ||
      !Array.isArray(view.seatSlots) ||
      !Array.isArray(view.options.characters) ||
      view.inputs.length < 2 ||
      view.inputs.length > 4
    )
      throw Error("对局或资源身份已变化，请重新连接");
  }
  private transportBinding() {
    const view = this.view;
    if (!view) return;
    const input = view.inputs[this.options.seat];
    const allowed =
      this.enabled &&
      view.phase !== "loading" &&
      view.phase !== "result" &&
      !input?.automated;
    this.transport.update({
      roomID: this.options.roomID,
      userID: this.options.userID,
      seat: this.options.seat,
      epoch: view.epoch,
      serverAt: view.serverAt,
      receivedSequence: input?.sequence ?? 0,
      appliedSequence: input?.appliedSequence ?? 0,
      canInput: allowed && !this.recovering,
      releaseAllowed: allowed,
      version: this.version,
    });
    this.input.setEnabled(
      this.enabled &&
        !this.recovering &&
        this.transport.connected &&
        view.phase !== "loading" &&
        view.phase !== "result" &&
        !input?.automated,
    );
  }
  update(
    view: NetworkView,
    version: number,
    enabled: boolean,
    clock: GameClock,
  ) {
    if (this.closed) return;
    this.enabled = enabled;
    this.clock = clock;
    if (!this.initialized) return;
    try {
      this.validate(view);
      if (version < this.version) return;
      this.version = version;
      this.view = view;
      this.transportBinding();
      if (!this.recovering) this.scene!.accept(view, this.transport.unapplied);
      this.refreshAudio();
    } catch (error) {
      void this.recover(error);
    }
  }
  private recover(reason: unknown) {
    if (this.closed || this.recovering || performance.now() < this.retryAt)
      return this.recovering ?? Promise.resolve();
    this.input.setEnabled(false);
    this.audio?.suspend();
    this.transport.cancel();
    this.options.onConnection(false);
    const pending = (async () => {
      const result = await this.options.loadCheckpoint();
      this.abort.signal.throwIfAborted();
      this.validate(result.data.view);
      this.scene!.restore(result.data.checkpoint, result.data.view);
      this.audio?.restore(result.data.view);
      this.view = result.data.view;
      this.version = result.version;
      this.failures = 0;
      this.paint();
    })()
      .catch((error) => {
        if (this.closed) return;
        this.failures++;
        this.retryAt =
          performance.now() +
          Math.min(5000, 300 * 2 ** Math.min(4, this.failures));
        if (this.failures >= 3)
          this.options.onError(
            error instanceof Error ? error : Error(String(reason)),
          );
      })
      .finally(() => {
        this.recovering = undefined;
        if (!this.closed) this.transportBinding();
      });
    this.recovering = pending;
    return pending;
  }
  private paint() {
    if (!this.scene) return;
    this.image.data.set(this.scene.pixels);
    this.context.putImageData(this.image, 0, 0);
    this.painted = true;
    const slot = this.view?.seatSlots[this.options.seat],
      local = this.scene.view?.players.find((player) => player.slot === slot);
    const root = this.options.root;
    root.dataset.localX = String(local?.X ?? "");
    root.dataset.localY = String(local?.Y ?? "");
    root.dataset.frame = this.scene.renderedFrame;
    root.dataset.authorityFrame = this.scene.authorityFrame.toString();
    root.dataset.connection = this.transport.connected
      ? "connected"
      : "recovering";
  }
  private tick = (now: number) => {
    if (this.closed) return;
    try {
      this.transport.pump();
      // Loading intentionally has no ticking authority until every human is
      // ready; terminal authority also stays still. Neither is a network stall.
      const waiting =
        this.view?.phase === "loading" || this.view?.phase === "result";
      const connected =
        !!this.view &&
        !this.recovering &&
        (waiting || this.transport.connected);
      this.options.onConnection(connected);
      const connection = connected ? "connected" : "recovering";
      this.refreshAudio(connected);
      if (this.options.root.dataset.connection !== connection)
        this.options.root.dataset.connection = connection;
      if (!connected) {
        this.input.setEnabled(false);
        if (!this.recovering && this.initialized && now >= this.retryAt)
          void this.recover(Error("等待权威帧恢复"));
      } else this.transportBinding();
      if (
        this.initialized &&
        !this.recovering &&
        now - this.lastStep >= FRAME_MS * 0.8
      ) {
        this.lastStep = now;
        const active =
          this.enabled &&
          connected &&
          this.view?.phase !== "loading" &&
          this.view?.phase !== "result" &&
          !this.view?.inputs[this.options.seat]?.automated;
        const logical = buttonsForMask(this.input.sample()),
          merged = this.options.physical?.(logical, now, !!active) ?? logical;
        const event = this.transport.sample(
          active ? logicalMask(merged) : 0,
          this.scene!.inputFrame,
        );
        if (event) this.scene!.queueInput(event);
        if (this.scene!.sample(now, this.clock) || !this.painted) this.paint();
      }
    } catch (error) {
      void this.recover(error);
    }
    this.raf = requestAnimationFrame(this.tick);
  };
  configure(profile: ControllerProfile) {
    this.transport.cancel();
    this.input.configure(profile);
    this.options.clearPhysical?.();
  }
  refreshAudio(connected = this.transport.connected) {
    if (this.view)
      this.audio?.sync(
        this.view,
        this.scene?.view,
        this.options.sound?.() ?? false,
        connected && !this.recovering,
        this.options.paused?.() ?? false,
      );
  }
  private visibility = () => {
    if (document.hidden) this.audio?.suspend();
    else if (this.initialized && !this.closed)
      void this.recover(Error("恢复当前比赛声音"));
  };
  pressPointer = (pointer: number, buttons: readonly string[]) =>
    this.input.pressPointer(pointer, buttons);
  releasePointer = (pointer: number, cancelled = false) =>
    this.input.releasePointer(pointer, cancelled);
  dispose() {
    if (this.closed) return;
    this.closed = true;
    document.removeEventListener("visibilitychange", this.visibility);
    this.audio?.dispose();
    this.abort.abort();
    cancelAnimationFrame(this.raf);
    this.transport.dispose();
    this.input.dispose();
    this.options.clearPhysical?.();
    this.scene?.dispose();
    this.scene = undefined;
  }
}
