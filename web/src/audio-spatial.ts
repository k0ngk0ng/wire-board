export type SpatialPoint = { x: number; y: number; z: number };
export type SpatialListener = SpatialPoint & { yaw: number };
export type SpatialOptions = {
  playbackID: string;
  source: SpatialPoint;
  listener: SpatialListener;
  volume?: number;
  maxDistance?: number;
};
export type NormalizedSpatialOptions = SpatialOptions & { volume: number; maxDistance: number; centered?: true };
type SpatialBuffer = { pending?: Promise<AudioBuffer>; abort: AbortController; bytes: number };
type SpatialVoice = {
  identity: string;
  key: string;
  options: NormalizedSpatialOptions;
  requestedAt: number;
  cancel: () => void;
  cancelled: Promise<boolean>;
  node?: AudioBufferSourceNode;
  gain?: GainNode;
  panner?: StereoPannerNode;
};
const attempt = (action: () => void) => { try { action(); } catch {} };
const now = () => typeof performance === "undefined" ? Date.now() : performance.now();
const pointValid = (point: SpatialPoint | undefined): point is SpatialPoint =>
  !!point && [point.x, point.y, point.z].every(value => Number.isFinite(value) && Math.abs(value) <= 1000000000);

export function spatialListenerValid(listener: SpatialListener | undefined): listener is SpatialListener {
  return pointValid(listener) && Number.isFinite(listener.yaw);
}

export function normalizeSpatialOptions(options: SpatialOptions | undefined): NormalizedSpatialOptions | undefined {
  if (!options || typeof options.playbackID !== "string" || !options.playbackID || options.playbackID.length > 256 ||
    !pointValid(options.source) || !spatialListenerValid(options.listener) ||
    (options.volume !== undefined && !Number.isFinite(options.volume)) ||
    (options.maxDistance !== undefined && (!Number.isFinite(options.maxDistance) || options.maxDistance <= 0))) return;
  return {
    playbackID: options.playbackID,
    source: { ...options.source },
    listener: { ...options.listener },
    volume: Math.max(0, Math.min(1, options.volume ?? 1)),
    maxDistance: options.maxDistance ?? 2400,
  };
}

export class NativeSpatialAudio {
  readonly owner: string;
  private readonly context: AudioContext;
  private readonly destination: MediaStreamAudioDestinationNode;
  private readonly media: HTMLAudioElement;
  private readonly available: () => boolean;
  private readonly voices = new Map<string, SpatialVoice>();
  private readonly buffers = new Map<string, SpatialBuffer>();
  private listener: SpatialListener = { x: 0, y: 0, z: 0, yaw: 0 };
  private decodedBytes = 0;
  private closed = false;
  private outputPlaying = false;
  private outputGeneration = 0;
  private starting?: Promise<boolean>;
  private readonly visibility = () => { if (!this.available()) this.stop(); };

  private constructor(owner: string, context: AudioContext, destination: MediaStreamAudioDestinationNode,
    media: HTMLAudioElement, available: () => boolean) {
    this.owner = owner;
    this.context = context;
    this.destination = destination;
    this.media = media;
    this.available = available;
    media.dataset.platformSpatial = "";
    media.dataset.spatialOwner = owner;
    media.loop = false;
    media.srcObject = destination.stream;
    media.onerror = () => this.stop();
    media.onpause = () => { this.outputPlaying = false; };
    if (typeof document !== "undefined") document.addEventListener("visibilitychange", this.visibility);
    this.publish();
  }

  static prepare(owner: string, factory: () => HTMLAudioElement, available: () => boolean): NativeSpatialAudio | undefined {
    const constructor = globalThis.AudioContext ?? (globalThis as typeof globalThis & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (typeof constructor !== "function") return;
    let context: AudioContext | undefined;
    let destination: MediaStreamAudioDestinationNode | undefined;
    let media: HTMLAudioElement | undefined;
    try {
      context = new constructor({ latencyHint: "interactive" });
      if (typeof context.createStereoPanner !== "function") throw Error("spatial audio unavailable");
      destination = context.createMediaStreamDestination();
      media = factory();
      return new NativeSpatialAudio(owner, context, destination, media, available);
    } catch {
      attempt(() => media?.pause());
      attempt(() => { if (media) media.srcObject = null; });
      attempt(() => media?.remove());
      for (const track of destination?.stream.getTracks() ?? []) attempt(() => track.stop());
      if (context) void context.close().catch(() => {});
      return;
    }
  }

  get stopped() { return this.closed; }

  play(key: string, sources: readonly string[], options: NormalizedSpatialOptions): Promise<boolean> {
    if (this.closed || !this.available()) return Promise.resolve(false);
    if (!options.centered) this.setListener(options.listener);
    const identity = JSON.stringify([!!options.centered, key]);
    const previous = this.voices.get(identity);
    if (previous) this.releaseVoice(previous);
    if (this.voices.size >= 32) this.releaseVoice(this.voices.values().next().value!);
    let cancel!: () => void;
    const cancelled = new Promise<boolean>(resolve => { cancel = () => resolve(false); });
    const voice: SpatialVoice = { identity, key, options, requestedAt: now(), cancel, cancelled };
    this.voices.set(identity, voice);
    const output = this.ensureOutput();
    const pending = this.load(voice, sources, output);
    this.publish();
    return Promise.race([pending, cancelled]);
  }

  private current(voice: SpatialVoice) {
    return !this.closed && this.available() && this.voices.get(voice.identity) === voice;
  }

  private async decode(source: string): Promise<AudioBuffer> {
    const previous = this.buffers.get(source);
    if (previous?.pending) {
      this.buffers.delete(source);
      this.buffers.set(source, previous);
      return previous.pending;
    }
    while (this.buffers.size >= 32) this.evictBuffer(this.buffers.keys().next().value!);
    const entry: SpatialBuffer = { abort: new AbortController(), bytes: 0 };
    entry.pending = (async () => {
      let bytes: ArrayBuffer;
      const inline = /^data:audio\/[a-z0-9.+-]+;base64,([a-z0-9+/=]+)$/i.exec(source);
      if (inline) {
        const binary = atob(inline[1]);
        if (!binary.length || binary.length > 2 * 1024 * 1024) throw Error("spatial source invalid");
        const data = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) data[i] = binary.charCodeAt(i);
        bytes = data.buffer;
      } else {
        const response = await fetch(source, { signal: entry.abort.signal });
        if (!response.ok) throw Error("spatial source failed");
        bytes = await this.readSource(response, entry.abort.signal);
      }
      if (entry.abort.signal.aborted || !bytes.byteLength || bytes.byteLength > 2 * 1024 * 1024 || this.closed) throw Error("spatial source invalid");
      const buffer = await this.context.decodeAudioData(bytes);
      if (entry.abort.signal.aborted || this.closed || !Number.isFinite(buffer.duration) || buffer.duration <= 0 || buffer.duration > 5 ||
        buffer.numberOfChannels < 1 || buffer.numberOfChannels > 2 || buffer.length > 960000) throw Error("spatial decoded source invalid");
      entry.bytes = buffer.length * buffer.numberOfChannels * 4;
      this.decodedBytes += entry.bytes;
      while (this.decodedBytes > 16 * 1024 * 1024 && this.buffers.size > 1) {
        const oldest = [...this.buffers.keys()].find(key => key !== source);
        if (!oldest) break;
        this.evictBuffer(oldest);
      }
      return buffer;
    })().catch(error => {
      if (this.buffers.get(source) === entry) this.evictBuffer(source);
      throw error;
    });
    this.buffers.set(source, entry);
    return entry.pending;
  }

  private async readSource(response: Response, signal: AbortSignal): Promise<ArrayBuffer> {
    const declared = Number(response.headers?.get("Content-Length"));
    if (Number.isFinite(declared) && declared > 2 * 1024 * 1024) throw Error("spatial source too large");
    if (!response.body) return response.arrayBuffer();
    const reader = response.body.getReader();
    const chunks: Uint8Array[] = [];
    let size = 0;
    try {
      for (;;) {
        const chunk = await reader.read();
        if (signal.aborted) throw Error("spatial source cancelled");
        if (chunk.done) break;
        size += chunk.value.byteLength;
        if (size > 2 * 1024 * 1024) {
          void reader.cancel().catch(() => {});
          throw Error("spatial source too large");
        }
        chunks.push(chunk.value);
      }
      const bytes = new Uint8Array(size);
      let offset = 0;
      for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
      return bytes.buffer;
    } finally {
      reader.releaseLock();
    }
  }

  private evictBuffer(source: string) {
    const entry = this.buffers.get(source);
    if (!entry) return;
    this.buffers.delete(source);
    this.decodedBytes -= entry.bytes;
    entry.abort.abort();
  }

  private async load(voice: SpatialVoice, sources: readonly string[], output: Promise<boolean>): Promise<boolean> {
    for (const source of sources) {
      try {
        const buffer = await this.decode(source);
        if (!this.current(voice) || now() - voice.requestedAt > 250) break;
        if (!(await output) || !this.current(voice) || now() - voice.requestedAt > 250) break;
        const node = this.context.createBufferSource();
        const gain = this.context.createGain();
        const panner = this.context.createStereoPanner();
        voice.node = node;
        voice.gain = gain;
        voice.panner = panner;
        node.buffer = buffer;
        node.loop = false;
        node.connect(gain);
        gain.connect(panner);
        panner.connect(this.destination);
        node.onended = () => { if (this.current(voice)) this.releaseVoice(voice); };
        this.tune(voice);
        node.start();
        this.publish();
        return true;
      } catch {
        if (!this.current(voice)) break;
      }
    }
    this.releaseVoice(voice);
    return false;
  }

  setListener(listener: SpatialListener) {
    if (!spatialListenerValid(listener) || this.closed) return;
    this.listener = { ...listener };
    for (const voice of this.voices.values()) this.tune(voice);
    this.publish();
  }

  private tune(voice: SpatialVoice) {
    if (!voice.gain || !voice.panner) return;
    const dx = voice.options.centered ? 0 : voice.options.source.x - this.listener.x;
    const dy = voice.options.centered ? 0 : voice.options.source.y - this.listener.y;
    const dz = voice.options.centered ? 0 : voice.options.source.z - this.listener.z;
    const distance = Math.hypot(dx, dy, dz);
    const horizontal = Math.hypot(dx, dy);
    const pan = horizontal ? Math.max(-1, Math.min(1, (dx * Math.sin(this.listener.yaw) - dy * Math.cos(this.listener.yaw)) / horizontal)) : 0;
    const attenuation = Math.max(0, 1 - distance / voice.options.maxDistance);
    voice.panner.pan.setValueAtTime(pan, this.context.currentTime);
    voice.gain.gain.setValueAtTime(voice.options.volume * attenuation * attenuation, this.context.currentTime);
  }

  private ensureOutput(): Promise<boolean> {
    if (this.closed || !this.available()) return Promise.resolve(false);
    if (this.outputPlaying && !this.media.paused && this.context.state === "running") return Promise.resolve(true);
    if (this.starting) return this.starting;
    const generation = ++this.outputGeneration;
    try {
      const resume = this.context.resume();
      const play = this.media.play();
      this.starting = Promise.all([resume, play]).then(() => {
        if (this.closed || !this.available() || generation !== this.outputGeneration) { this.media.pause(); return false; }
        this.outputPlaying = true;
        this.publish();
        return true;
      }).catch(() => false).finally(() => { if (generation === this.outputGeneration) this.starting = undefined; });
      return this.starting;
    } catch {
      return Promise.resolve(false);
    }
  }

  unlock() { return this.ensureOutput(); }
  setVolume(level: number) { if (!this.closed) this.media.volume = level; }

  stopKey(key?: string) {
    for (const voice of [...this.voices.values()]) if (!voice.options.centered && (key === undefined || voice.key === key)) this.releaseVoice(voice);
    if (!this.voices.size) {
      for (const [source, entry] of this.buffers) if (!entry.bytes) this.evictBuffer(source);
    }
    this.publish();
  }

  private releaseVoice(voice: SpatialVoice) {
    if (this.voices.get(voice.identity) === voice) this.voices.delete(voice.identity);
    voice.cancel();
    if (voice.node) voice.node.onended = null;
    attempt(() => voice.node?.stop());
    attempt(() => voice.node?.disconnect());
    attempt(() => voice.gain?.disconnect());
    attempt(() => voice.panner?.disconnect());
    this.publish();
  }

  private publish() {
    if (this.closed) return;
    this.media.dataset.spatialState = this.outputPlaying ? "playing" : "loading-or-blocked";
    this.media.dataset.spatialBuffers = String(this.buffers.size);
    this.media.dataset.spatialDecodedBytes = String(this.decodedBytes);
    this.media.dataset.spatialVoices = JSON.stringify([...this.voices.values()].map(voice => ({ key: voice.key,
      playbackID: voice.options.playbackID, source: voice.options.source, pan: voice.panner?.pan.value,
      gain: voice.gain?.gain.value, started: !!voice.node })));
  }

  stop() {
    if (this.closed) return;
    this.closed = true;
    this.outputGeneration++;
    this.outputPlaying = false;
    this.starting = undefined;
    if (typeof document !== "undefined") document.removeEventListener("visibilitychange", this.visibility);
    for (const voice of [...this.voices.values()]) this.releaseVoice(voice);
    for (const source of [...this.buffers.keys()]) this.evictBuffer(source);
    this.media.onerror = null;
    this.media.onpause = null;
    attempt(() => { this.media.muted = true; this.media.pause(); });
    attempt(() => { this.media.srcObject = null; });
    attempt(() => this.media.remove());
    for (const track of this.destination.stream.getTracks()) attempt(() => track.stop());
    void this.context.close().catch(() => {});
  }
}
