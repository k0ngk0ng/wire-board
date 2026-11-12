export type ContinuousLoopOptions = { volume?: number; rate?: number; seamOverlapMS?: number };
type Prepared = { buffer: AudioBuffer; sourceSha256: string; decodedFrames: number; overlapFrames: number };
type Voice = {
  key: string; sources: string[]; overlapMS: number; volume: number; rate: number;
  pending: Promise<boolean>; cancel: () => void; cancelled: Promise<boolean>;
  node?: AudioBufferSourceNode; gain?: GainNode; prepared?: Prepared;
  starts: number; startedAt?: number; error?: string;
};
const bounded = (value: number | undefined, previous: number, min: number, max: number) =>
  value !== undefined && Number.isFinite(value) ? Math.max(min, Math.min(max, value)) : previous;
const attempt = (action: () => void) => { try { action(); } catch { /* Continue peer cleanup. */ } };

/** Smooth only a decoded playback copy. No source file is changed, no silence
 * fade/normalization is added, and adjacent head samples meet at the new wrap.
 */
export function overlapContinuousLoop(input: Float32Array, sampleRate: number, milliseconds = 5): Float32Array<ArrayBuffer> {
  const size = Math.min(Math.round(sampleRate * milliseconds / 1000), Math.floor(input.length * .02));
  if (size < 2) return new Float32Array(input);
  const result = new Float32Array(input.length - size);
  result.set(input.subarray(size, input.length - size));
  const start = input.length - size * 2;
  for (let index = 0; index < size; index++) {
    const t = index / (size - 1), weight = t * t * (3 - 2 * t);
    result[start + index] = input[input.length - size + index] * (1 - weight) + input[index] * weight;
  }
  return result;
}

/** Platform-owned continuous sources feed one native media element. The
 * context's physical destination is deliberately never connected.
 */
export class NativeContinuousLoops {
  readonly owner: string;
  private readonly context: AudioContext;
  private readonly destination: MediaStreamAudioDestinationNode;
  private readonly media: HTMLAudioElement;
  private readonly available: () => boolean;
  private readonly voices = new Map<string, Voice>();
  private readonly buffers = new Map<string, Map<number, Promise<Prepared>>>();
  private readonly visibility = () => { if (!this.available()) this.stop(); };
  private watch?: ReturnType<typeof setInterval>;
  private closed = false;
  private paused = false;
  private outputPlaying = false;
  private outputGeneration = 0;
  private starting?: Promise<boolean>;

  private constructor(owner: string, context: AudioContext, destination: MediaStreamAudioDestinationNode,
    media: HTMLAudioElement, available: () => boolean) {
    this.owner = owner; this.context = context; this.destination = destination; this.media = media; this.available = available;
    media.dataset.platformContinuous = "";media.dataset.continuousOwner = owner;
    media.loop = false;media.playbackRate = 1;media.srcObject = destination.stream;
    media.onerror = () => this.stop();
    media.onpause = () => { this.outputPlaying = false;this.publish(); };
    if (typeof document !== "undefined") document.addEventListener?.("visibilitychange", this.visibility);
    this.watch = setInterval(() => { if (!this.available()) this.stop(); else this.publish(); }, 100);
    this.publish();
  }
  static prepare(owner: string, factory: () => HTMLAudioElement, available: () => boolean): NativeContinuousLoops | undefined {
    const ctor = globalThis.AudioContext ?? (globalThis as typeof globalThis & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (typeof ctor !== "function") return;
    let context: AudioContext | undefined, destination: MediaStreamAudioDestinationNode | undefined, media: HTMLAudioElement | undefined;
    try {
      context = new ctor({ latencyHint: "interactive" });
      destination = context.createMediaStreamDestination();media = factory();
      return new NativeContinuousLoops(owner, context, destination, media, available);
    } catch {
      attempt(() => media?.pause());attempt(() => { if (media) media.srcObject = null; });attempt(() => media?.remove());
      for (const track of destination?.stream.getTracks() ?? []) attempt(() => track.stop());
      if (context) void context.close().catch(() => {});
      return;
    }
  }
  get stopped() { return this.closed; }
  private same(voice: Voice, sources: readonly string[], overlapMS: number) {
    return voice.overlapMS === overlapMS && voice.sources.length === sources.length && voice.sources.every((source, i) => source === sources[i]);
  }
  failed(key: string, sources: readonly string[]): boolean {
    const voice = this.voices.get(key);
    return !this.closed && !!voice?.error && voice.sources.length === sources.length && voice.sources.every((source, i) => source === sources[i]);
  }
  loop(key: string, requested: readonly string[], options: ContinuousLoopOptions): Promise<boolean> {
    if (this.closed || this.paused || !this.available()) return Promise.resolve(false);
    const sources = [...new Set(requested.filter(Boolean))], overlapMS = bounded(options.seamOverlapMS, 5, 0, 10);
    const previous = this.voices.get(key);
    if (previous && this.same(previous, sources, overlapMS)) {
      this.tune(previous, options);
      if (previous.error) return Promise.resolve(false);
      return previous.node ? this.ensureOutput() : previous.pending;
    }
    if (previous) this.releaseVoice(previous);
    if (!sources.length || this.voices.size >= 32) return Promise.resolve(false);
    let cancel!: () => void;
    const cancelled = new Promise<boolean>(resolve => { cancel = () => resolve(false); });
    const voice: Voice = { key, sources, overlapMS, volume: bounded(options.volume, .15, 0, 1), rate: bounded(options.rate, 1, .25, 4),
      pending: Promise.resolve(false), cancel, cancelled, starts: 0 };
    this.voices.set(key, voice);
    voice.pending = Promise.race([this.load(voice), cancelled]);
    this.publish();return voice.pending;
  }
  private current(voice: Voice) { return !this.closed && this.voices.get(voice.key) === voice && this.available(); }
  private async decode(source: string, overlapMS: number): Promise<Prepared> {
    let variants = this.buffers.get(source);
    const existing = variants?.get(overlapMS);if (existing) return existing;
    if (!variants) {
      if (this.buffers.size >= 32) this.buffers.delete(this.buffers.keys().next().value!);
      variants = new Map();this.buffers.set(source, variants);
    }
    const pending = (async () => {
      let bytes: ArrayBuffer;
      const inline = /^data:audio\/[a-z0-9.+-]+;base64,([a-z0-9+/=]+)$/i.exec(source);
      if (inline) {
        const binary = atob(inline[1]);if (binary.length > 8 * 1024 * 1024) throw Error("continuous source too large");
        const data = new Uint8Array(binary.length);for (let i = 0; i < binary.length; i++) data[i] = binary.charCodeAt(i);
        bytes = data.buffer;
      } else {
        const response = await fetch(source);if (!response.ok) throw Error("continuous source failed");bytes = await response.arrayBuffer();
      }
      if (!bytes.byteLength || bytes.byteLength > 8 * 1024 * 1024) throw Error("continuous source size invalid");
      const digest = await crypto.subtle.digest("SHA-256", bytes);
      if (this.closed) throw Error("continuous source cancelled");
      const decoded = await this.context.decodeAudioData(bytes.slice(0));
      if (this.closed || decoded.numberOfChannels < 1 || decoded.numberOfChannels > 2 || decoded.duration > 30 || decoded.length > 2 * 1024 * 1024)
        throw Error("continuous decoded source invalid");
      const channels = Array.from({ length: decoded.numberOfChannels }, (_, i) => overlapContinuousLoop(decoded.getChannelData(i), decoded.sampleRate, overlapMS));
      const buffer = this.context.createBuffer(channels.length, channels[0].length, decoded.sampleRate);
      channels.forEach((channel, i) => buffer.getChannelData(i).set(channel));
      return { buffer, decodedFrames: decoded.length, overlapFrames: decoded.length - buffer.length,
        sourceSha256: [...new Uint8Array(digest)].map(value => value.toString(16).padStart(2, "0")).join("") };
    })();
    variants.set(overlapMS, pending);return pending;
  }
  private async load(voice: Voice): Promise<boolean> {
    for (const source of voice.sources) {
      try {
        const prepared = await this.decode(source, voice.overlapMS);
        if (!this.current(voice)) return false;
        const node = this.context.createBufferSource(), gain = this.context.createGain();
        voice.prepared = prepared;voice.node = node;voice.gain = gain;
        node.buffer = prepared.buffer;node.loop = true;
        node.playbackRate.setValueAtTime(voice.rate, this.context.currentTime);
        gain.gain.setValueAtTime(0, this.context.currentTime);
        node.connect(gain);gain.connect(this.destination);
        voice.startedAt = this.context.currentTime;node.start();voice.starts++;
        this.tune(voice, {});return await this.ensureOutput();
      } catch {
        if (!this.current(voice)) return false;
      }
    }
    voice.error = "decode-failed";this.publish();return false;
  }
  private tune(voice: Voice, options: ContinuousLoopOptions) {
    const previousVolume = voice.volume, previousRate = voice.rate;
    voice.volume = bounded(options.volume, voice.volume, 0, 1);voice.rate = bounded(options.rate, voice.rate, .25, 4);
    if (voice.node && voice.gain) {
      const at = this.context.currentTime;
      const target = (param: AudioParam, value: number, constant: number) => {
        if (typeof param.cancelAndHoldAtTime === "function") param.cancelAndHoldAtTime(at);
        else { const current = param.value;param.cancelScheduledValues(at);param.setValueAtTime(current, at); }
        param.setTargetAtTime(value, at, constant);
      };
      if (previousVolume !== voice.volume || !Object.keys(options).length) target(voice.gain.gain, voice.volume, .025);
      if (previousRate !== voice.rate) target(voice.node.playbackRate, voice.rate, .035);
    }
    this.publish();
  }
  private ensureOutput(force = false): Promise<boolean> {
    if (this.closed || this.paused || !this.available()) return Promise.resolve(false);
    if (this.outputPlaying && !this.media.paused && this.context.state === "running") return Promise.resolve(true);
    if (this.starting && !force) return this.starting;
    const generation = ++this.outputGeneration;
    // Invoke both native operations synchronously so a fresh platform unlock
    // retains its trusted gesture even if the context resume waits internally.
    let resume: Promise<void>, play: Promise<void>;
    try { resume = this.context.resume();play = this.media.play(); }
    catch { return Promise.resolve(false); }
    const pending = Promise.all([resume, play]).then(() => {
      if (this.closed || this.paused || !this.available()) { this.media.pause();return false; }
      if (generation !== this.outputGeneration) return false;
      this.outputPlaying = true;this.publish();return true;
    }).catch(() => { if (generation === this.outputGeneration) { this.outputPlaying = false;this.publish(); }return false; })
      .finally(() => { if (generation === this.outputGeneration) this.starting = undefined; });
    this.starting = pending;return pending;
  }
  unlock(): Promise<boolean> { return this.ensureOutput(true); }
  pause(owner: string) {
    if (owner !== this.owner || this.closed || this.paused) return;
    this.paused = true;this.outputGeneration++;this.starting = undefined;this.outputPlaying = false;
    this.media.pause();void this.context.suspend().catch(() => {});this.publish();
  }
  resume(owner: string): Promise<boolean> {
    if (owner !== this.owner || this.closed || !this.available()) return Promise.resolve(false);
    this.paused = false;return this.ensureOutput(true);
  }
  setVolume(level: number) { if (!this.closed) this.media.volume = level; }
  private publish() {
    if (this.closed) return;
    this.media.dataset.continuousState = this.paused ? "paused" : [...this.voices.values()].some(voice => voice.error) ? "fallback" : this.outputPlaying ? "playing" : "loading-or-blocked";
    this.media.dataset.contextState = this.context.state;
    this.media.dataset.continuousPublishedAt = String(typeof performance !== "undefined" ? performance.now() : 0);
    this.media.dataset.continuousVoices = JSON.stringify([...this.voices.values()].map(voice => ({ key: voice.key,
      sourceSha256: voice.prepared?.sourceSha256, rate: voice.node?.playbackRate.value, gain: voice.gain?.gain.value,
      targetRate: voice.rate, targetVolume: voice.volume, bufferFrames: voice.prepared?.buffer.length,
      decodedFrames: voice.prepared?.decodedFrames, overlapFrames: voice.prepared?.overlapFrames,
      sampleRate: voice.prepared?.buffer.sampleRate, starts: voice.starts, startedAt: voice.startedAt, error: voice.error })));
  }
  private releaseVoice(voice: Voice) {
    if (this.voices.get(voice.key) === voice) this.voices.delete(voice.key);
    voice.cancel();attempt(() => voice.node?.stop());attempt(() => voice.node?.disconnect());attempt(() => voice.gain?.disconnect());
  }
  stopKey(key: string) {
    const voice = this.voices.get(key);if (voice) this.releaseVoice(voice);
    if (!this.voices.size) this.stop();else this.publish();
  }
  stop() {
    if (this.closed) return;
    this.closed = true;this.outputGeneration++;this.outputPlaying = false;
    if (this.watch !== undefined) clearInterval(this.watch);
    if (typeof document !== "undefined") document.removeEventListener?.("visibilitychange", this.visibility);
    for (const voice of [...this.voices.values()]) this.releaseVoice(voice);
    this.buffers.clear();this.media.onerror = null;this.media.onpause = null;
    attempt(() => { this.media.muted = true;this.media.pause(); });attempt(() => { this.media.srcObject = null; });attempt(() => this.media.remove());
    for (const track of this.destination.stream.getTracks()) attempt(() => track.stop());
    void this.context.close().catch(() => {});
  }
}
