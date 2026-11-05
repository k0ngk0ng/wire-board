import type {
  GameAudioStreamConfig,
  GameAudioStreamHandle,
  GameAudioStreamSegment,
  GameAudioStreamStatus,
} from "./game-api/audio-stream";

type Bytes = Uint8Array<ArrayBuffer>;
export type NormalizedStreamConfig = {
  playbackID: string;
  mimeType: string;
  sampleRate: number;
  initSegment: Bytes;
  maxForwardSamples: number;
  retainBackSamples: number;
};
function copyBytes(value: Uint8Array): Bytes {
  const copy = new Uint8Array(value.byteLength);
  copy.set(value);
  return copy;
}
function equalBytes(a: Uint8Array, b: Uint8Array) {
  return a.length === b.length && a.every((value, index) => value === b[index]);
}
export function normalizeStreamConfig(
  input: GameAudioStreamConfig,
): NormalizedStreamConfig | undefined {
  if (
    !input ||
    typeof input.playbackID !== "string" ||
    !input.playbackID.trim() ||
    input.playbackID.length > 256 ||
    typeof input.mimeType !== "string" ||
    !input.mimeType ||
    input.mimeType.length > 512 ||
    input.sampleRate !== 44100 ||
    !(input.initSegment instanceof Uint8Array) ||
    !input.initSegment.byteLength ||
    input.initSegment.byteLength > 65536
  )
    return;
  const maxForwardSamples = input.maxForwardSamples ?? 66150;
  const retainBackSamples = input.retainBackSamples ?? 88200;
  if (
    !Number.isSafeInteger(maxForwardSamples) ||
    maxForwardSamples < 1 ||
    maxForwardSamples > 441000 ||
    !Number.isSafeInteger(retainBackSamples) ||
    retainBackSamples < 11025 ||
    retainBackSamples > 441000
  )
    return;
  try {
    return {
      playbackID: input.playbackID,
      mimeType: input.mimeType,
      sampleRate: input.sampleRate,
      initSegment: copyBytes(input.initSegment),
      maxForwardSamples,
      retainBackSamples,
    };
  } catch {
    return;
  }
}

type Pending = {
  segment: GameAudioStreamSegment & { data: Bytes };
  promise: Promise<boolean>;
  resolve(value: boolean): void;
};

// Owns one native stream epoch. There is one pending operation and only one
// retained accepted segment; public status values are copied interpretations.
export class NativeAudioStream implements GameAudioStreamHandle {
  readonly ready: Promise<boolean>;
  private resolveReady!: (value: boolean) => void;
  private initialized = false;
  private closed = false;
  private paused = false;
  private started = false;
  private ending = false;
  private eos = false;
  private ended = false;
  private failure?: string;
  private nextSeq = 1;
  private acceptedEndSample = 0;
  private last?: Pending["segment"];
  private pending?: Pending;
  private buffer?: SourceBuffer;
  private cancelUpdate?: () => void;
  private endPromise?: Promise<boolean>;
  private playPending = false;
  private playGeneration = 0;
  private readonly listeners: (() => void)[] = [];
  private readonly config: NormalizedStreamConfig;
  private readonly source: MediaSource;
  private readonly media: HTMLAudioElement;
  private readonly url: string;
  private readonly available: () => boolean;
  private readonly release: () => void;

  private constructor(
    config: NormalizedStreamConfig,
    source: MediaSource,
    media: HTMLAudioElement,
    url: string,
    available: () => boolean,
    release: () => void,
  ) {
    this.config = config;
    this.source = source;
    this.media = media;
    this.url = url;
    this.available = available;
    this.release = release;
    this.ready = new Promise((resolve) => {
      this.resolveReady = resolve;
    });
  }

  static prepare(
    config: NormalizedStreamConfig,
    factory: () => HTMLAudioElement,
    available: () => boolean,
    release: () => void,
  ): NativeAudioStream | undefined {
    let media: HTMLAudioElement | undefined, url: string | undefined;
    try {
      if (
        typeof MediaSource === "undefined" ||
        !MediaSource.isTypeSupported(config.mimeType)
      )
        return;
      const source = new MediaSource();
      url = URL.createObjectURL(source);
      media = factory();
      return new NativeAudioStream(
        config,
        source,
        media,
        url,
        available,
        release,
      );
    } catch {
      if (url) URL.revokeObjectURL(url);
      media?.remove();
      return;
    }
  }

  matches(other: NormalizedStreamConfig) {
    const own = this.config;
    return (
      own.playbackID === other.playbackID &&
      own.mimeType === other.mimeType &&
      own.sampleRate === other.sampleRate &&
      own.maxForwardSamples === other.maxForwardSamples &&
      own.retainBackSamples === other.retainBackSamples &&
      equalBytes(own.initSegment, other.initSegment)
    );
  }

  begin() {
    const listen = (
      target: EventTarget,
      name: string,
      callback: EventListener,
    ) => {
      target.addEventListener(name, callback);
      this.listeners.push(() => target.removeEventListener(name, callback));
    };
    listen(this.source, "sourceopen", () => {
      void this.initialize();
    });
    listen(this.source, "sourceclose", () => {
      if (!this.closed) this.fail("native-source-closed");
    });
    listen(this.media, "error", () => this.fail("native-media-error"));
    listen(this.media, "ended", () => {
      if (!this.closed) this.ended = true;
    });
    this.media.dataset.platformStream = "";
    this.media.loop = false;
    queueMicrotask(() => {
      if (this.closed) return;
      try {
        this.media.src = this.url;
      } catch {
        this.fail("native-source-load-failed");
      }
    });
  }

  private async initialize() {
    if (this.closed || this.buffer) return;
    try {
      this.buffer = this.source.addSourceBuffer(this.config.mimeType);
      this.buffer.mode = "segments";
      if (
        !(await this.update(() =>
          this.buffer!.appendBuffer(this.config.initSegment.buffer),
        ))
      )
        return;
      if (this.closed) return;
      this.initialized = true;
      this.resolveReady(true);
    } catch {
      this.fail("native-source-initialization-failed");
    }
  }

  private update(action: () => void): Promise<boolean> {
    if (
      this.closed ||
      !this.buffer ||
      this.buffer.updating ||
      this.cancelUpdate
    )
      return Promise.resolve(false);
    const buffer = this.buffer;
    return new Promise((resolve) => {
      let settled = false;
      const finish = (ok: boolean) => {
        if (settled) return;
        settled = true;
        buffer.removeEventListener("updateend", done);
        buffer.removeEventListener("error", failed);
        buffer.removeEventListener("abort", aborted);
        this.cancelUpdate = undefined;
        resolve(ok);
      };
      const done = () => finish(!this.closed);
      const failed = () => {
        finish(false);
        this.fail("native-buffer-error");
      };
      const aborted = () => {
        finish(false);
        if (!this.closed) this.fail("native-buffer-aborted");
      };
      this.cancelUpdate = () => finish(false);
      buffer.addEventListener("updateend", done);
      buffer.addEventListener("error", failed);
      buffer.addEventListener("abort", aborted);
      try {
        action();
      } catch {
        finish(false);
        this.fail("native-buffer-update-failed");
      }
    });
  }

  private time() {
    return Number.isFinite(this.media.currentTime) &&
      this.media.currentTime >= 0
      ? this.media.currentTime
      : 0;
  }
  private ranges(): [number, number][] {
    try {
      const ranges = this.buffer?.buffered;
      return ranges
        ? Array.from(
            { length: ranges.length },
            (_, index) =>
              [ranges.start(index), ranges.end(index)] as [number, number],
          )
        : [];
    } catch {
      return [];
    }
  }
  private equalSegment(a: GameAudioStreamSegment, b: GameAudioStreamSegment) {
    return (
      a.sequence === b.sequence &&
      a.startSample === b.startSample &&
      a.sampleFrames === b.sampleFrames &&
      equalBytes(a.data, b.data)
    );
  }

  append(input: GameAudioStreamSegment): Promise<boolean> {
    if (
      !input ||
      !Number.isSafeInteger(input.sequence) ||
      input.sequence < 1 ||
      input.sequence > 0xffffffff ||
      !Number.isSafeInteger(input.startSample) ||
      input.startSample < 0 ||
      !Number.isSafeInteger(input.sampleFrames) ||
      input.sampleFrames < 1 ||
      !Number.isSafeInteger(input.startSample + input.sampleFrames) ||
      !(input.data instanceof Uint8Array) ||
      !input.data.byteLength ||
      input.data.byteLength > 1024 * 1024 ||
      this.closed ||
      !this.available()
    )
      return Promise.resolve(false);
    if (this.pending)
      return this.equalSegment(this.pending.segment, input)
        ? this.pending.promise
        : Promise.resolve(false);
    if (this.ending || this.eos || this.ended) return Promise.resolve(false);
    if (this.last && this.equalSegment(this.last, input))
      return Promise.resolve(true);
    if (
      input.sequence !== this.nextSeq ||
      input.startSample !== this.acceptedEndSample ||
      input.startSample +
        input.sampleFrames -
        this.time() * this.config.sampleRate >
        this.config.maxForwardSamples
    )
      return Promise.resolve(false);
    let segment: Pending["segment"];
    try {
      segment = {
        sequence: input.sequence,
        startSample: input.startSample,
        sampleFrames: input.sampleFrames,
        data: copyBytes(input.data),
      };
    } catch {
      return Promise.resolve(false);
    }
    let resolve!: (value: boolean) => void;
    const promise = new Promise<boolean>((done) => {
      resolve = done;
    });
    const pending: Pending = { segment, promise, resolve };
    this.pending = pending;
    void this.commit(pending);
    return promise;
  }

  private async commit(pending: Pending) {
    let accepted = false;
    try {
      if (!(await this.ready) || this.closed || this.pending !== pending)
        return;
      const cutoff =
        this.time() - this.config.retainBackSamples / this.config.sampleRate;
      if (cutoff > 0 && this.ranges().some(([start]) => start < cutoff)) {
        if (!(await this.update(() => this.buffer!.remove(0, cutoff)))) return;
      }
      if (
        this.closed ||
        !(await this.update(() =>
          this.buffer!.appendBuffer(pending.segment.data.buffer),
        ))
      )
        return;
      const ahead = Math.max(
        0,
        ...this.ranges().map(([, end]) => end - this.time()),
      );
      if (
        ahead >
        this.config.maxForwardSamples / this.config.sampleRate + 0.05
      ) {
        this.fail("native-buffer-exceeds-forward-bound");
        return;
      }
      this.acceptedEndSample =
        pending.segment.startSample + pending.segment.sampleFrames;
      this.nextSeq++;
      this.last = pending.segment;
      accepted = true;
      this.play();
    } finally {
      if (this.pending === pending) this.pending = undefined;
      pending.resolve(accepted && !this.closed);
    }
  }

  private play() {
    if (
      this.closed ||
      this.paused ||
      this.ended ||
      !this.acceptedEndSample ||
      this.playPending ||
      !this.available()
    )
      return;
    if (this.started && !this.media.paused) return;
    const generation = ++this.playGeneration;
    this.playPending = true;
    try {
      void this.media
        .play()
        .then(() => {
          if (
            this.closed ||
            this.paused ||
            !this.available() ||
            generation !== this.playGeneration
          ) {
            this.media.pause();
            return;
          }
          this.playPending = false;
          this.started = true;
        })
        .catch(() => {
          if (this.closed || this.paused || generation !== this.playGeneration)
            return;
          this.playPending = false;
          this.fail("native-play-failed");
        });
    } catch {
      this.playPending = false;
      this.fail("native-play-failed");
    }
  }

  pause() {
    if (this.closed) return;
    this.paused = true;
    this.playGeneration++;
    this.playPending = false;
    this.media.pause();
  }
  resume(): Promise<boolean> {
    if (this.closed || !this.available() || !this.paused)
      return Promise.resolve(false);
    this.paused = false;
    this.play();
    return Promise.resolve(true);
  }
  setVolume(value: number) {
    this.media.volume = value;
  }

  end(): Promise<boolean> {
    if (this.endPromise) return this.endPromise;
    if (this.closed) return Promise.resolve(false);
    this.ending = true;
    const pending = this.pending?.promise;
    this.endPromise = (async () => {
      if (!(await this.ready) || this.closed) return false;
      if ((pending && !(await pending)) || this.closed) return false;
      try {
        if (this.source.readyState !== "open") return false;
        this.source.endOfStream();
        this.eos = true;
        this.play();
        return true;
      } catch {
        this.fail("native-end-failed");
        return false;
      }
    })();
    return this.endPromise;
  }

  status(): GameAudioStreamStatus {
    const phase = this.failure
      ? "failed"
      : this.closed
        ? "stopped"
        : this.ended
          ? "ended"
          : this.paused
            ? "paused"
            : !this.initialized
              ? "opening"
              : this.eos
                ? "draining"
                : this.started
                  ? "playing"
                  : "ready";
    return {
      phase,
      playbackID: this.config.playbackID,
      sampleRate: this.config.sampleRate,
      maxForwardSamples: this.config.maxForwardSamples,
      retainBackSamples: this.config.retainBackSamples,
      nextSeq: this.nextSeq,
      acceptedEndSample: this.acceptedEndSample,
      updating: this.buffer?.updating ?? false,
      pendingBytes: this.pending?.segment.data.byteLength ?? 0,
      currentTime: this.time(),
      buffered: this.ranges(),
      ...(this.failure ? { error: this.failure } : {}),
    };
  }

  private fail(reason: string) {
    if (this.closed) return;
    this.failure = reason;
    this.stop();
  }
  stop() {
    if (this.closed) return;
    this.closed = true;
    this.playGeneration++;
    this.resolveReady(false);
    this.pending?.resolve(false);
    this.pending = undefined;
    this.cancelUpdate?.();
    for (const dispose of this.listeners.splice(0)) dispose();
    try {
      if (this.buffer?.updating) this.buffer.abort();
    } catch {
      /* Detached native buffer. */
    }
    this.media.pause();
    this.media.muted = true;
    this.media.removeAttribute("src");
    this.media.load();
    this.media.remove();
    URL.revokeObjectURL(this.url);
    this.last = undefined;
    this.release();
  }
}
