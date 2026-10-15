import { toneDataURL, type Sound } from "./audio-tones.ts";
export type { Sound } from "./audio-tones.ts";
type Clip = { src: string; fallback?: string };
type Task = {
  owner: string;
  priority: number;
  clips: Clip[];
  starting: boolean;
};
type LoopOptions = { volume?: number; rate?: number };
type LoopVoice = {
  owner: string;
  key: string;
  sources: string[];
  media: HTMLAudioElement;
  sourceIndex: number;
  starting: boolean;
  pending: Promise<boolean>;
  volume: number;
  rate: number;
  frame?: number;
};
const SILENCE =
  "data:audio/wav;base64,UklGRiYAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQIAAACAgA==";

// Native effect and optional music channels share media-volume behavior,
// gesture unlock and mute cancellation; game rules only select clip URLs.
export class GameAudio {
  private media?: HTMLAudioElement;
  private background?: GameAudio;
  private backgroundOwner?: string;
  private backgroundSources = "";
  private loops = new Map<string, LoopVoice>();
  private factory: () => HTMLAudioElement;
  private allowed = true;
  private primed = false;
  private serial = 0;
  private task?: Task;
  private assets = "";
  constructor(factory: () => HTMLAudioElement = () => new Audio()) {
    this.factory = factory;
  }
  get enabled() {
    return this.allowed;
  }
  private createElement() {
    const media = this.factory();
    media.dataset.platformAudio = "";
    media.preload = "none";
    media.hidden = true;
    media.muted = !this.allowed;
    if (typeof document !== "undefined") document.body.append(media);
    return media;
  }
  private element() {
    this.media ??= this.createElement();
    return this.media;
  }
  mount() {
    this.element();
  }
  setAssetsBase(value: string) {
    this.assets = value.replace(/\/$/, "");
  }
  setEnabled(value: boolean) {
    this.allowed = value;
    if (!value) this.stop();
  }
  private available() {
    return (
      this.allowed && (typeof document === "undefined" || !document.hidden)
    );
  }
  private playbackSession() {
    try {
      const session = (
        navigator as Navigator & { audioSession?: { type: string } }
      ).audioSession;
      if (session) session.type = "playback";
    } catch {
      /* HTML media remains the fallback where Audio Session is absent. */
    }
  }
  async unlock(): Promise<boolean> {
    if (!this.available()) return false;
    void this.background?.unlock();
    if (this.primed || this.task) return true;
    const media = this.element(),
      serial = this.serial;
    this.playbackSession();
    media.src = SILENCE;
    media.muted = true;
    try {
      await media.play();
      if (serial !== this.serial || !this.allowed) return false;
      this.primed = true;
      if (media.src === SILENCE) {
        media.pause();
        media.removeAttribute("src");
        media.load();
      }
      return true;
    } catch {
      if (serial === this.serial) this.primed = false;
      return false;
    }
  }
  stop(owner?: string) {
    this.stopEffects(owner);
    for (const voice of this.loops.values())
      if (owner === undefined || voice.owner === owner) this.releaseLoop(voice);
    this.background?.stop(owner);
    if (owner === undefined || this.backgroundOwner === owner) {
      this.backgroundOwner = undefined;
      this.backgroundSources = "";
    }
  }
  private stopEffects(owner?: string) {
    if (owner !== undefined && this.task?.owner !== owner) return;
    this.serial++;
    this.task = undefined;
    if (this.media) {
      this.media.onended = null;
      this.media.onerror = null;
      this.media.muted = true;
      this.media.pause();
      this.media.removeAttribute("src");
      this.media.load();
    }
  }
  dispose() {
    this.stop();
    this.background?.dispose();
    this.media?.remove();
    this.media = undefined;
    this.primed = false;
  }
  channel(owner: string) {
    return {
      play: (sources: readonly string[]) =>
        this.start(
          sources.map((src) => ({ src })),
          owner,
          1,
        ),
      playWithFallback: (sources: readonly string[]) =>
        sources.length
          ? this.start([{ src: sources[0], fallback: sources[1] }], owner, 1)
          : Promise.resolve(false),
      stop: () => this.stop(owner),
      music: (sources: readonly string[]) => this.startMusic(sources, owner),
      stopMusic: () => this.background?.stop(owner),
      loop: (
        key: string,
        sources: readonly string[],
        options: LoopOptions = {},
      ) => this.startLoop(owner, key, sources, options),
      stopLoop: (key: string) => {
        const voice = this.loops.get(key);
        if (voice?.owner === owner) this.releaseLoop(voice);
      },
    };
  }
  private loopCurrent(voice: LoopVoice) {
    return this.loops.get(voice.key) === voice && this.available();
  }
  private releaseLoop(voice: LoopVoice) {
    if (this.loops.get(voice.key) === voice) this.loops.delete(voice.key);
    if (voice.frame !== undefined) cancelAnimationFrame(voice.frame);
    voice.frame = undefined;
    const media = voice.media;
    media.onended = null;
    media.onerror = null;
    media.muted = true;
    media.pause();
    media.removeAttribute("src");
    media.load();
    media.remove();
  }
  private tuneLoop(voice: LoopVoice, options: LoopOptions, immediate = false) {
    const bounded = (
      value: number | undefined,
      previous: number,
      min: number,
      max: number,
    ) =>
      value !== undefined && Number.isFinite(value)
        ? Math.max(min, Math.min(max, value))
        : previous;
    voice.volume = bounded(options.volume, voice.volume, 0, 1);
    voice.rate = bounded(options.rate, voice.rate, 0.25, 4);
    const media = voice.media;
    if (immediate || typeof requestAnimationFrame === "undefined") {
      media.volume = voice.volume;
      media.playbackRate = voice.rate;
      return;
    }
    if (voice.frame !== undefined) return;
    let previous: number | undefined;
    const ramp = (now: number) => {
      voice.frame = undefined;
      if (!this.loopCurrent(voice)) {
        this.releaseLoop(voice);
        return;
      }
      const elapsed = previous === undefined ? 16 : Math.max(1, now - previous);
      previous = now;
      const blend = 1 - Math.exp(-elapsed / 40);
      media.volume += (voice.volume - media.volume) * blend;
      media.playbackRate += (voice.rate - media.playbackRate) * blend;
      if (
        Math.abs(media.volume - voice.volume) < 0.001 &&
        Math.abs(media.playbackRate - voice.rate) < 0.001
      ) {
        media.volume = voice.volume;
        media.playbackRate = voice.rate;
      } else voice.frame = requestAnimationFrame(ramp);
    };
    voice.frame = requestAnimationFrame(ramp);
  }
  private startLoop(
    owner: string,
    key: string,
    requested: readonly string[],
    options: LoopOptions,
  ): Promise<boolean> {
    if (!this.available() || !key) return Promise.resolve(false);
    const sources = [...new Set(requested.filter(Boolean))];
    // A channel represents the current room. Do not let a previous room's
    // continuous sounds survive a new owner, even when it uses different keys.
    for (const voice of this.loops.values())
      if (voice.owner !== owner) this.releaseLoop(voice);
    const previous = this.loops.get(key);
    if (
      previous &&
      JSON.stringify(previous.sources) === JSON.stringify(sources)
    ) {
      this.tuneLoop(previous, options);
      return previous.pending;
    }
    if (previous) this.releaseLoop(previous);
    if (!sources.length) return Promise.resolve(false);
    const media = this.createElement();
    media.dataset.platformLoop = key;
    media.loop = true;
    // Engine rate controls pitch as well as speed, including older Safari and
    // Firefox implementations. No Web Audio route is created.
    const pitch = media as HTMLAudioElement & {
      mozPreservesPitch?: boolean;
      webkitPreservesPitch?: boolean;
    };
    pitch.preservesPitch = false;
    if ("mozPreservesPitch" in pitch) pitch.mozPreservesPitch = false;
    if ("webkitPreservesPitch" in pitch) pitch.webkitPreservesPitch = false;
    const voice: LoopVoice = {
      owner,
      key,
      sources,
      media,
      sourceIndex: 0,
      starting: false,
      pending: Promise.resolve(false),
      volume: 0.15,
      rate: 1,
    };
    this.loops.set(key, voice);
    this.tuneLoop(voice, options, true);
    this.playbackSession();
    voice.pending = this.playLoopSource(voice);
    return voice.pending;
  }
  private async playLoopSource(voice: LoopVoice): Promise<boolean> {
    const media = voice.media;
    while (voice.sourceIndex < voice.sources.length) {
      if (!this.loopCurrent(voice)) {
        this.releaseLoop(voice);
        return false;
      }
      const index = voice.sourceIndex;
      media.onerror = () => {
        if (
          !this.loopCurrent(voice) ||
          voice.starting ||
          voice.sourceIndex !== index
        )
          return;
        voice.sourceIndex++;
        voice.pending = this.playLoopSource(voice);
      };
      voice.starting = true;
      media.src = voice.sources[index];
      media.muted = false;
      try {
        await media.play();
        if (!this.loopCurrent(voice)) {
          // A cancelled play promise may resolve after the element was removed.
          // This voice has its own element, so pausing it cannot affect a successor.
          this.releaseLoop(voice);
          return false;
        }
        voice.starting = false;
        return true;
      } catch {
        voice.starting = false;
        if (!this.loopCurrent(voice)) {
          this.releaseLoop(voice);
          return false;
        }
        voice.sourceIndex++;
      }
    }
    this.releaseLoop(voice);
    return false;
  }
  private startMusic(
    sources: readonly string[],
    owner: string,
  ): Promise<boolean> {
    if (!this.available() || !sources.length) return Promise.resolve(false);
    const key = JSON.stringify(sources);
    if (
      this.backgroundOwner === owner &&
      this.backgroundSources === key &&
      this.background?.media &&
      !this.background.media.paused
    )
      return Promise.resolve(true);
    this.background ??= new GameAudio(this.factory);
    const media = this.background.element();
    media.dataset.platformMusic = "";
    media.loop = true;
    media.volume = 0.15;
    this.backgroundOwner = owner;
    this.backgroundSources = key;
    return this.background.channel(owner).playWithFallback(sources);
  }
  play(sound: Sound): Promise<boolean> {
    if (!this.available()) return Promise.resolve(false);
    const fallback = toneDataURL(sound);
    const src = this.assets
      ? `${this.assets}/platform/audio/v1/${sound}.wav`
      : fallback;
    return this.start(
      [{ src, fallback: src === fallback ? undefined : fallback }],
      "platform",
      sound === "preview"
        ? 4
        : sound === "finish"
          ? 3
          : sound === "turn"
            ? 2
            : 1,
    );
  }
  private async start(
    clips: Clip[],
    owner: string,
    priority: number,
  ): Promise<boolean> {
    if (!this.available() || !clips.length) return false;
    if (this.task && !this.media?.paused && this.task.priority > priority)
      return false;
    this.stopEffects();
    this.playbackSession();
    const task: Task = {
      owner,
      priority,
      clips: clips.slice(0, 4),
      starting: false,
    };
    this.task = task;
    return this.next(task);
  }
  private async next(task: Task): Promise<boolean> {
    if (this.task !== task || !this.available()) return false;
    const clip = task.clips.shift();
    if (!clip) {
      this.task = undefined;
      return false;
    }
    const media = this.element();
    media.onended = () => {
      if (this.task === task) void this.next(task);
    };
    media.onerror = () => {
      if (this.task === task && !task.starting) this.stopEffects(task.owner);
    };
    for (const src of [clip.src, clip.fallback].filter(
      (s): s is string => !!s,
    )) {
      if (this.task !== task || !this.available()) return false;
      task.starting = true;
      media.src = src;
      media.muted = false;
      try {
        await media.play();
        if (!this.allowed) {
          media.muted = true;
          media.pause();
          return false;
        }
        if (this.task !== task) return false;
        task.starting = false;
        this.primed = true;
        return true;
      } catch {
        if (this.task !== task) return false;
        this.primed = false;
      }
    }
    if (this.task === task) this.stopEffects(task.owner);
    return false;
  }
}
