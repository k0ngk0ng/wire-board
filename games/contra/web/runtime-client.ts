import type { ControllerProfile } from "../../../web/src/controllers/input";
import type { GameAudioPort } from "../../../web/src/game-api/module";
import { ContraAudioAdapter, type ContraAudioResources } from "./audio";
import {
  runtimeStageManifest,
  runtimePresentationManifest,
  type RuntimePresentation,
  type RuntimeBridge,
  type RuntimeCheckpoint,
  type RuntimeManifest,
  type RuntimePoll,
  type RuntimeSnapshot,
} from "./runtime-bridge";
import { RuntimeInput } from "./runtime-input";
import { RuntimeRenderer } from "./runtime-renderer";
import { RuntimePresentationQueue } from "./runtime-presentation";

export interface RuntimeTransport {
  inputEnabled?(): boolean;
  sequenceBase?(): number;
  release?(sequence: number): Promise<void>;
  dispose?(): void;
  checkpoint(): Promise<RuntimeCheckpoint>;
  ready(): Promise<void>;
  input(held: number, sequence: number): Promise<void>;
  poll(after: string): Promise<RuntimePoll>;
}
export type RuntimeEvidence = {
  checked: number;
  resyncs: number;
  checkpoints: RuntimeCheckpoint[];
  inputs: { sequence: number; mask: number }[];
  frames: {
    frame: string;
    revision: string;
    kind: string;
    pads: [number, number];
    digest: string;
    phase: string;
    x: number;
    y: number;
    projectiles: number;
  }[];
};
type Options = {
  root: HTMLElement;
  canvas: HTMLCanvasElement;
  profile: ControllerProfile;
  audio: GameAudioPort;
  audioResources: ContraAudioResources;
  manifest: RuntimeManifest;
  bridge: RuntimeBridge;
  transport: RuntimeTransport;
  initial: RuntimeSnapshot;
  onProgress: (percent: number, label: string) => void;
  onSnapshot: (value: RuntimeSnapshot) => void;
  onError: (error: Error) => void;
};

/** Native server frames are authoritative. Wasm replays their accepted pads and
 * must match the Go-canonical digest before a frame reaches the Canvas/audio.
 * A formal Board can supply its existing authenticated room transport here.
 */
export class RuntimeClient {
  readonly input: RuntimeInput;
  readonly evidence: RuntimeEvidence = {
    checked: 0,
    resyncs: 0,
    checkpoints: [],
    inputs: [],
    frames: [],
  };
  private renderer: RuntimeRenderer;
  private audio: ContraAudioAdapter;
  private presentation: RuntimePresentationQueue;
  private snapshot: RuntimeSnapshot;
  private localSession = 0;
  private request = 0;
  private closed = false;
  private ready = false;
  private polling = false;
  private resync = false;
  private frameRequest = 0;
  private lastPoll = 0;
  private lastInput = 0;
  private held = -1;
  private inputChain = Promise.resolve();
  private stages = new Map<number, Promise<void>>();
  private loadedPages = new Set<string>();
  private lifetime = new AbortController();
  constructor(private options: Options) {
    this.snapshot = options.initial;
    this.renderer = new RuntimeRenderer(options.canvas);
    this.audio = new ContraAudioAdapter(
      options.audio,
      options.audioResources,
      options.initial.epoch,
    );
    this.presentation = new RuntimePresentationQueue((snapshot, resync) => {
      if (this.closed || document.hidden) return;
      this.renderer.draw(snapshot.presentation);
      this.audio.update(snapshot.audio, {
        resync,
        finalized: snapshot.finalized,
      });
      this.options.onSnapshot(snapshot);
    });
    this.input = new RuntimeInput(options.profile, options.root, () =>
      this.send(0, true),
    );
    this.input.setEnabled(false);
    document.addEventListener("visibilitychange", this.visibility);
  }
  async initialize() {
    await this.prepareStage(this.options.initial.level);
    do {
      await this.whenVisible();
      if (this.closed) return;
      await this.restore();
    } while (!this.closed && document.hidden);
    if (this.closed) return;
    await this.options.transport.ready();
    if (this.closed) return;
    this.ready = true;
    this.input.setEnabled(true);
    this.options.root.focus({ preventScroll: true });
    this.options.onProgress(100, "游戏已就绪");
    this.frameRequest = requestAnimationFrame(this.tick);
  }
  private whenVisible(): Promise<void> {
    if (!document.hidden || this.closed) return Promise.resolve();
    return new Promise((resolve) => {
      const ready = () => {
        if (document.hidden && !this.closed) return;
        document.removeEventListener("visibilitychange", ready);
        this.lifetime.signal.removeEventListener("abort", ready);
        resolve();
      };
      document.addEventListener("visibilitychange", ready);
      this.lifetime.signal.addEventListener("abort", ready, { once: true });
    });
  }
  prepareStage(level: number): Promise<void> {
    const index = Math.min(level, 7);
    let pending = this.stages.get(index);
    if (!pending) {
      pending = this.loadGraphics(
        runtimeStageManifest(this.options.manifest, index),
      ).catch((error) => {
        this.stages.delete(index);
        throw error;
      });
      this.stages.set(index, pending);
    }
    return pending;
  }
  private async loadGraphics(manifest: RuntimeManifest) {
    await this.renderer.load(manifest, this.options.onProgress);
    for (const key of Object.keys(manifest.pages ?? {}))
      this.loadedPages.add(key);
  }
  async preparePresentation(view: RuntimePresentation) {
    if (!view.Patterns?.some((page) => !this.loadedPages.has(String(page))))
      return;
    const manifest = runtimePresentationManifest(this.options.manifest, view);
    manifest.pages = Object.fromEntries(
      Object.entries(manifest.pages).filter(
        ([key]) => !this.loadedPages.has(key),
      ),
    );
    await this.loadGraphics(manifest);
  }
  private async restore() {
    const checkpoint = await this.options.transport.checkpoint();
    if (this.closed) return;
    if (checkpoint.epoch !== this.options.initial.epoch)
      throw Error("恢复快照属于另一局");
    if (checkpoint.rulesBuildID !== this.options.manifest.rulesBuildID)
      throw Error("规则程序已改变，请重新加载");
    const revision = (checkpoint.state as { revision?: string })?.revision;
    if (
      this.localSession &&
      revision &&
      BigInt(revision) < BigInt(this.snapshot.revision)
    ) {
      this.presentation.reset(this.snapshot, performance.now());
      return;
    }
    this.evidence.checkpoints.push(checkpoint);
    if (this.evidence.checkpoints.length > 8) this.evidence.checkpoints.shift();
    if (this.localSession)
      this.options.bridge.call({
        op: "dispose",
        session: this.localSession,
        epoch: this.snapshot.epoch,
      });
    const local = this.options.bridge.call<RuntimeSnapshot>({
      op: "restore",
      epoch: checkpoint.epoch,
      checkpoint,
    });
    this.localSession = local.session;
    this.snapshot = { ...local, session: this.options.initial.session };
    await this.preparePresentation(local.presentation);
    if (!this.closed) this.presentation.reset(this.snapshot, performance.now());
  }
  private visibility = () => {
    this.input.cancel();
    if (document.hidden) {
      this.presentation.clear();
      this.audio.suspend();
      this.options.audio.stop();
    } else this.resync = true;
  };
  private send(mask: number, force = false) {
    if (this.closed || !this.ready) return;
    if (
      !force &&
      (this.snapshot.finalized ||
        this.options.transport.inputEnabled?.() === false)
    )
      return;
    const now = performance.now();
    if (!force && mask === this.held && now - this.lastInput < 250) return;
    this.held = mask;
    this.lastInput = now;
    const sequence = (this.request =
      Math.max(this.request, this.options.transport.sequenceBase?.() ?? 0) + 1);
    this.evidence.inputs.push({ sequence, mask });
    if (this.evidence.inputs.length > 4096) this.evidence.inputs.shift();
    this.inputChain = this.inputChain
      .then(() =>
        force && this.options.transport.release
          ? this.options.transport.release(sequence)
          : this.options.transport.input(mask, sequence),
      )
      .catch((e) => this.fail(e));
  }
  private tick = (now: number) => {
    if (this.closed) return;
    this.frameRequest = requestAnimationFrame(this.tick);
    if (!this.ready || document.hidden) return;
    try {
      if (!this.resync) this.presentation.sample(now);
      const enabled =
        !this.snapshot.finalized &&
        this.options.transport.inputEnabled?.() !== false;
      this.input.setEnabled(enabled);
      if (enabled) this.send(this.input.sample(now));
    } catch (error) {
      this.fail(error);
      return;
    }
    if (!this.polling && now - this.lastPoll >= 16) {
      this.lastPoll = now;
      this.polling = true;
      void this.poll()
        .catch((e) => this.fail(e))
        .finally(() => {
          this.polling = false;
        });
    }
  };
  private async poll() {
    if (this.resync) {
      await this.restore();
      if (this.closed) return;
      this.evidence.resyncs++;
      this.resync = false;
      return;
    }
    const response = await this.options.transport.poll(this.snapshot.revision);
    if (this.closed) return;
    if (response.snapshot.epoch !== this.options.initial.epoch)
      throw Error("对局身份已改变");
    if (response.checkpoint) {
      await this.restore();
      if (this.closed) return;
      this.evidence.resyncs++;
      return;
    }
    for (const accepted of response.frames) {
      const remote = accepted.snapshot;
      if (remote && remote.epoch !== this.snapshot.epoch)
        throw Error("输入记录属于另一局");
      const kind = accepted.kind ?? "pads",
        op = kind === "present-paused" ? "paused" : kind;
      const revision = remote?.revision ?? accepted.revision,
        frame = remote?.frame ?? accepted.frame,
        digest = remote?.digest ?? accepted.digest;
      if (!revision || !frame || !digest) throw Error("缺少权威重放记录");
      const local = this.options.bridge.call<RuntimeSnapshot>({
        op,
        session: this.localSession,
        epoch: this.snapshot.epoch,
        revision,
        frame,
        ...(kind === "pads" ? { pads: accepted.pads } : {}),
      });
      if (local.digest !== digest)
        throw Error(
          `原生与Wasm状态不同：修订${revision} ${local.digest}/${digest}`,
        );
      this.snapshot = { ...local, session: this.options.initial.session };
      this.evidence.checked++;
      this.evidence.frames.push({
        frame,
        revision,
        kind,
        pads: accepted.pads ?? [0, 0],
        digest,
        phase: local.phase,
        x: local.players[0].x,
        y: local.players[0].y,
        projectiles: local.projectiles,
      });
      if (this.evidence.frames.length > 4096) this.evidence.frames.shift();
      await this.preparePresentation(local.presentation);
      if (this.closed) return;
      this.presentation.push(this.snapshot);
    }
  }

  private fail(reason: unknown) {
    if (this.closed) return;
    this.send(0, true);
    this.ready = false;
    this.presentation.clear();
    this.input.setEnabled(false);
    this.audio.stop();
    this.options.audio.stop();
    this.options.onError(
      reason instanceof Error ? reason : new Error(String(reason)),
    );
  }
  get missingAudio() {
    return this.audio.missingResources;
  }
  dispose() {
    if (this.closed) return;
    this.input.cancel();
    this.closed = true;
    this.lifetime.abort();
    this.presentation.clear();
    cancelAnimationFrame(this.frameRequest);
    this.input.dispose();
    this.audio.dispose();
    this.options.audio.stop();
    void this.inputChain.finally(() => this.options.transport.dispose?.());
    document.removeEventListener("visibilitychange", this.visibility);
    if (this.localSession)
      this.options.bridge.call({
        op: "dispose",
        session: this.localSession,
        epoch: this.snapshot.epoch,
      });
  }
}

/** Private development HTTP adapter. Production room integration supplies its
 * own RuntimeTransport and never exposes a client checkpoint-write endpoint. */
export function developmentTransport(
  initial: RuntimeSnapshot,
): RuntimeTransport {
  const call = async <T>(
    path: string,
    extra: Record<string, unknown> = {},
  ): Promise<T> => {
    const response = await fetch("/runtime/" + path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        session: initial.session,
        epoch: initial.epoch,
        ...extra,
      }),
    });
    const value = await response.json();
    if (!response.ok) throw Error(value.error ?? "运行层请求失败");
    return value as T;
  };
  return {
    checkpoint: () => call("checkpoint"),
    ready: () => call("ready"),
    input: async (held, sequence) => {
      await call("input", { pads: [held, 0], sequence });
    },
    poll: (revision) => call("poll", { revision }),
  };
}
