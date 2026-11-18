import { api } from "../../../web/src/ui/board-host";
import serverRelease from "../assets/server-release.json";
import PalLocalWorker from "./local-worker?worker&inline";
import type { GameClock } from "../../../web/src/game-api/module";
import type { PalView } from "./view";
import type { LocalAction, LocalMeta, LocalInputRequest } from "./local-types";
import type { SceneFrame } from "./scene-renderer";

const wall = () => performance.timeOrigin + performance.now();
function coreURL(core: string) {
  for (const catalog of Object.values(serverRelease.catalogs)) {
    const files = catalog.files as Record<string, { url: string }>;
    const file = files[`cores/${core}.wasm`];
    if (file) return file.url;
  }
  if (
    import.meta.env.MODE === "pal-e2e" &&
    ["127.0.0.1", "localhost"].includes(location.hostname)
  )
    return new URL(`/pal-e2e-core/${core}.wasm`, location.origin).href;
  throw Error("当前存档的本地核心尚未发布");
}

export class LocalClient {
  private worker?: Worker;
  private meta?: LocalMeta;
  private clock?: GameClock;
  private audio?: PalView["audio"];
  private ready = false;
  private active = false;
  private closed = false;
  private loading?: Promise<void>;
  private generation = 0;
  private revision = 0;
  private frame?: PalView;
  private recoveries = 0;
  private reason = "";
  private records = new Map<
    number,
    NonNullable<LocalMeta["records"]>[number]
  >();
  private staged = new Map<
    number,
    { committed: boolean; action?: LocalAction }
  >();
  previews: SceneFrame[] = [];
  private preparing = 0;
  private lead = 0;
  constructor(
    private room: string,
    private paint: (
      frame: PalView,
      reason: "step" | "confirm" | "reset",
    ) => void,
    private action: (action: LocalAction) => void,
    private error: (message: string) => void,
    private prepare: (frames: SceneFrame[]) => Promise<void>,
  ) {}
  private time() {
    const clock = this.clock;
    return clock
      ? clock.serverNow +
          (clock.paused
            ? 0
            : Math.max(0, performance.now() - clock.receivedAt) + this.lead)
      : wall();
  }
  status() {
    return {
      ready: this.usable(),
      active: this.active,
      generation: this.generation,
      revision: this.revision,
      recoveries: this.recoveries,
      reason: this.reason,
    };
  }
  usable() {
    return this.ready && this.preparing === 0;
  }
  private replayMeta(meta: LocalMeta) {
    return {
      ...meta,
      checkpoint: undefined,
      records: [...this.records.values()]
        .filter(
          (record) =>
            record.revision > this.revision && record.revision <= meta.revision,
        )
        .sort((a, b) => a.revision - b.revision),
    };
  }
  accept(view: PalView, clock: GameClock, roundTripMS = 0) {
    this.lead = Number.isFinite(roundTripMS)
      ? Math.max(0, Math.min(250, roundTripMS))
      : 0;
    this.clock = clock;
    this.audio = view.audio;
    if (this.meta?.generation !== view.local?.generation) this.records.clear();
    this.meta = view.local;
    for (const record of view.local?.records ?? [])
      this.records.set(record.revision, record);
    while (this.records.size > 64)
      this.records.delete(this.records.keys().next().value!);
    if (!this.ready || !this.worker || !view.local) return;
    if (view.local.generation !== this.generation) {
      void this.refresh().catch(this.report);
      return;
    }
    if (view.local.revision < this.revision) return;
    this.worker.postMessage({
      type: "confirm",
      meta: this.replayMeta(view.local),
      now: this.time(),
      sentAt: wall(),
    });
  }
  private report = (error: unknown) => {
    if (!this.closed)
      this.error(
        error instanceof Error ? error.message : "无法恢复仙剑本地操作",
      );
  };
  start() {
    return this.refresh();
  }
  private refresh() {
    if (this.loading) return this.loading;
    this.loading = this.initialize().finally(() => {
      this.loading = undefined;
    });
    return this.loading;
  }
  private async initialize() {
    this.ready = false;
    this.staged.clear();
    const response = (await api(`/rooms/${this.room}/sync`)) as {
      roomId: string;
      data: PalView;
    };
    if (this.closed) return;
    const meta = response.data.local;
    if (response.roomId !== this.room || !meta?.checkpoint)
      throw Error("无法读取本人的仙剑同步检查点");
    this.worker?.terminate();
    const worker = new PalLocalWorker();
    this.worker = worker;
    this.audio = response.data.audio;
    this.generation = meta.generation;
    this.revision = meta.revision;
    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(
        () => reject(Error("仙剑本地核心加载超时")),
        25000,
      );
      worker.onerror = (event) => {
        clearTimeout(timeout);
        reject(Error(event.message || "仙剑本地核心无法执行"));
      };
      worker.onmessage = (event) => {
        if (this.closed || this.worker !== worker) return;
        const message = event.data;
        if (message.type === "prepare") {
          this.previews = message.frames;
          if (this.ready) {
            this.preparing++;
            void this.prepare(this.previews)
              .catch(this.report)
              .finally(() => {
                this.preparing--;
              });
          }
        } else if (message.type === "frame") {
          this.frame = { ...(message.frame as PalView), audio: this.audio };
          this.revision = Math.max(
            this.revision,
            this.frame.local?.revision ?? 0,
          );
          for (const revision of this.records.keys())
            if (revision <= this.revision) this.records.delete(revision);
          this.paint(this.frame, message.reason);
        } else if (message.type === "action") {
          const action = message.action as LocalAction;
          if (!message.preview) this.action(action);
          else {
            const staged = this.staged.get(action.seq);
            if (staged?.committed) {
              this.staged.delete(action.seq);
              this.action(action);
            } else if (staged) staged.action = action;
          }
        } else if (message.type === "ready") {
          const latest =
            this.meta &&
            this.meta.generation === this.generation &&
            this.meta.revision >= this.revision
              ? this.meta
              : meta;
          worker.postMessage({
            type: "confirm",
            meta: this.replayMeta(latest),
            now: this.time(),
            sentAt: wall(),
          });
        } else if (message.type === "confirmed") {
          clearTimeout(timeout);
          this.ready = true;
          worker.postMessage({ type: "active", value: this.active });
          resolve();
        } else if (message.type === "recover") {
          this.recoveries++;
          this.reason = message.reason;
          clearTimeout(timeout);
          if (!this.ready) {
            reject(Error(message.reason));
            return;
          }
          this.ready = false;
          if (this.active) void this.refresh().catch(this.report);
        }
      };
      worker.postMessage({
        type: "init",
        core: coreURL(meta.core),
        checkpoint: meta.checkpoint,
        meta: { ...meta, checkpoint: undefined },
        now: this.time(),
        sentAt: wall(),
      });
    });
  }
  submit(input: LocalInputRequest) {
    if (input.cancelPreview) {
      this.staged.delete(input.seq);
      this.worker?.postMessage({
        type: "cancel",
        seq: input.seq,
        now: this.time(),
        sentAt: wall(),
      });
      return;
    }
    if (input.commitPreview) {
      const staged = this.staged.get(input.seq);
      if (staged?.action) {
        this.staged.delete(input.seq);
        this.action(staged.action);
      } else if (staged) staged.committed = true;
      return;
    }
    if (input.interrupt && this.meta && !this.closed) {
      this.action({
        type: "local_release",
        epoch: this.meta.generation,
        seq: input.seq,
        direction: 4,
        keys: 0,
        boundary: this.meta.boundary ?? "release",
        controlAt: this.meta.serverAt,
      });
      this.recover();
      return;
    }
    if (!this.ready || !this.worker || !this.active) return;
    if (input.preview) this.staged.set(input.seq, { committed: false });
    this.worker.postMessage({
      type: "input",
      input,
      now: this.time(),
      sentAt: wall(),
    });
  }
  setActive(value: boolean) {
    if (this.active === value || this.closed) return;
    this.active = value;
    if (value && !this.ready && !this.loading) {
      void this.refresh().catch(this.report);
      return;
    }
    this.worker?.postMessage({ type: "active", value });
  }
  recover() {
    this.ready = false;
    this.worker?.postMessage({ type: "active", value: false });
    if (this.active) void this.refresh().catch(this.report);
  }
  dispose() {
    this.closed = true;
    this.worker?.terminate();
    this.worker = undefined;
  }
}
