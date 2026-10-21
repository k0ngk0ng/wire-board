import { toneDataURL, type Sound } from "./audio-tones.ts";
export type { Sound } from "./audio-tones.ts";
type Clip = {
  src: string;
  fallback?: string;
  startOffset?: number;
  loopStartSeconds?: number;
};
type Task = {
  owner: string;
  priority: number;
  clips: Clip[];
  starting: boolean;
  generation: number;
  pendingLoopRestart?: number;
};
type MusicOptions = {
  loop?: boolean;
  startOffset?: number;
  playbackID?: string;
  loopStartSeconds?: number;
};
type LoopOptions = { volume?: number; rate?: number };
type LoopVoice = {
  owner: string;
  key: string;
  sources: string[];
  media: HTMLAudioElement;
  sourceIndex: number;
  starting: boolean;
  generation: number;
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
  // A finite clip can finish and release its Task while its source remains
  // loaded. Retain that source's owner for later room/dispose cleanup.
  private mediaOwner?: string;
  private background?: GameAudio;
  private backgroundOwner?: string;
  private backgroundSources = "";
  private loops = new Map<string, LoopVoice>();
  private pausedOwners = new Set<string>();
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
    this.mediaOwner = undefined;
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
  pause(owner: string) {
    this.pausedOwners.add(owner);
    if (this.task?.owner === owner) {
      this.task.generation++;
      this.media?.pause();
    }
    for (const voice of this.loops.values()) {
      if (voice.owner !== owner) continue;
      voice.generation++;
      voice.media.pause();
      if (voice.frame !== undefined) cancelAnimationFrame(voice.frame);
      voice.frame = undefined;
    }
    this.background?.pause(owner);
  }
  async resume(owner: string): Promise<boolean> {
    if (!this.pausedOwners.delete(owner) || !this.available()) return false;
    const pending: Promise<boolean>[] = [];
    const task = this.task;
    if (task?.owner === owner && this.media?.src) {
      const media = this.media;
      const generation = ++task.generation;
      task.starting = false;
      pending.push(
        (async () => {
          try {
            this.seekPendingLoopRestart(task, media);
            await media.play();
            if (
              this.task !== task ||
              task.generation !== generation ||
              !this.available() ||
              this.pausedOwners.has(owner)
            ) {
              if (!this.task || this.pausedOwners.has(this.task.owner))
                media.pause();
              return false;
            }
            return true;
          } catch {
            if (this.task === task && task.generation === generation)
              this.stopEffects(owner);
            return false;
          }
        })(),
      );
    }
    for (const voice of this.loops.values()) {
      if (voice.owner !== owner) continue;
      const generation = ++voice.generation;
      voice.starting = false;
      pending.push(
        (async () => {
          try {
            await voice.media.play();
            if (
              !this.loopCurrent(voice) ||
              voice.generation !== generation ||
              this.pausedOwners.has(owner)
            ) {
              if (!this.loopCurrent(voice) || this.pausedOwners.has(owner))
                voice.media.pause();
              return false;
            }
            this.tuneLoop(voice, {});
            return true;
          } catch {
            if (voice.generation === generation) this.releaseLoop(voice);
            return false;
          }
        })(),
      );
    }
    if (this.background) pending.push(this.background.resume(owner));
    return (await Promise.all(pending)).some(Boolean);
  }
  stop(owner?: string) {
    if (owner === undefined) this.pausedOwners.clear();
    else this.pausedOwners.delete(owner);
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
    if (owner !== undefined && (this.task?.owner ?? this.mediaOwner) !== owner)
      return;
    this.serial++;
    this.task = undefined;
    this.mediaOwner = undefined;
    if (this.media) {
      this.media.onended = null;
      this.media.onerror = null;
      this.media.onloadedmetadata = null;
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
      pause: () => this.pause(owner),
      resume: () => this.resume(owner),
      music: (sources: readonly string[], options: MusicOptions = {}) =>
        this.startMusic(sources, owner, options),
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
    if (!this.available() || this.pausedOwners.has(owner) || !key)
      return Promise.resolve(false);
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
      generation: 0,
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
      if (this.pausedOwners.has(voice.owner)) return false;
      const index = voice.sourceIndex;
      media.onerror = () => {
        if (
          !this.loopCurrent(voice) ||
          this.pausedOwners.has(voice.owner) ||
          voice.starting ||
          voice.sourceIndex !== index
        )
          return;
        voice.sourceIndex++;
        voice.pending = this.playLoopSource(voice);
      };
      voice.starting = true;
      const generation = voice.generation;
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
        if (voice.generation !== generation) {
          if (this.pausedOwners.has(voice.owner)) media.pause();
          return false;
        }
        voice.starting = false;
        if (this.pausedOwners.has(voice.owner)) {
          media.pause();
          return false;
        }
        return true;
      } catch {
        if (voice.generation !== generation) return false;
        voice.starting = false;
        if (!this.loopCurrent(voice)) {
          this.releaseLoop(voice);
          return false;
        }
        if (this.pausedOwners.has(voice.owner)) return false;
        voice.sourceIndex++;
      }
    }
    this.releaseLoop(voice);
    return false;
  }
  private startMusic(
    sources: readonly string[],
    owner: string,
    options: MusicOptions,
  ): Promise<boolean> {
    if (
      options.startOffset !== undefined &&
      (!Number.isFinite(options.startOffset) || options.startOffset < 0)
    )
      return Promise.resolve(false);
    if (
      options.loopStartSeconds !== undefined &&
      (!Number.isFinite(options.loopStartSeconds) ||
        options.loopStartSeconds < 0 ||
        options.loop === false)
    )
      return Promise.resolve(false);
    if (!this.available() || this.pausedOwners.has(owner) || !sources.length)
      return Promise.resolve(false);
    const loopStart = options.loopStartSeconds ?? 0;
    const key = JSON.stringify([
      sources,
      options.loop ?? true,
      options.playbackID ?? null,
      loopStart,
    ]);
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
    media.loop = (options.loop ?? true) && loopStart === 0;
    media.volume = 0.15;
    this.backgroundOwner = owner;
    this.backgroundSources = key;
    return this.background.start(
      [
        {
          src: sources[0],
          fallback: sources[1],
          startOffset: options.startOffset,
          loopStartSeconds: loopStart > 0 ? loopStart : undefined,
        },
      ],
      owner,
      1,
    );
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
    if (!this.available() || this.pausedOwners.has(owner) || !clips.length)
      return false;
    if (this.task && !this.media?.paused && this.task.priority > priority)
      return false;
    this.stopEffects();
    this.playbackSession();
    const task: Task = {
      owner,
      priority,
      clips: clips.slice(0, 4),
      starting: false,
      generation: 0,
    };
    this.task = task;
    return this.next(task);
  }
  private seekPendingLoopRestart(task: Task, media: HTMLAudioElement) {
    if (task.pendingLoopRestart === undefined) return;
    media.currentTime = task.pendingLoopRestart;
    task.pendingLoopRestart = undefined;
  }
  private async restartMusicSuffix(task: Task, media: HTMLAudioElement) {
    if (
      this.task !== task ||
      task.pendingLoopRestart === undefined ||
      !this.available() ||
      this.pausedOwners.has(task.owner)
    )
      return false;
    const generation = ++task.generation;
    task.starting = true;
    try {
      this.seekPendingLoopRestart(task, media);
      await media.play();
      if (this.task !== task) {
        if (!this.task) media.pause();
        return false;
      }
      if (task.generation !== generation) {
        if (this.pausedOwners.has(task.owner)) media.pause();
        return false;
      }
      task.starting = false;
      if (!this.available() || this.pausedOwners.has(task.owner)) {
        media.pause();
        return false;
      }
      return true;
    } catch {
      if (this.task === task && task.generation === generation)
        this.stopEffects(task.owner);
      return false;
    }
  }
  private async next(task: Task): Promise<boolean> {
    if (
      this.task !== task ||
      !this.available() ||
      this.pausedOwners.has(task.owner)
    )
      return false;
    const clip = task.clips.shift();
    if (!clip) {
      this.task = undefined;
      return false;
    }
    const media = this.element();
    media.onended = () => {
      if (this.task !== task || task.starting) return;
      if (clip.loopStartSeconds !== undefined) {
        // Native loop would repeat the intro. Keep this source/owner loaded and
        // return only to its verified suffix; pause retains a pending boundary.
        task.pendingLoopRestart = clip.loopStartSeconds;
        void this.restartMusicSuffix(task, media);
      } else void this.next(task);
    };
    media.onerror = () => {
      if (this.task === task && !task.starting) this.stopEffects(task.owner);
    };
    for (const src of [clip.src, clip.fallback].filter(
      (s): s is string => !!s,
    )) {
      if (this.task !== task || !this.available()) return false;
      task.starting = true;
      const generation = task.generation;
      this.mediaOwner = task.owner;
      media.src = src;
      const loadedSource = media.src;
      media.onloadedmetadata =
        clip.startOffset === undefined && clip.loopStartSeconds === undefined
          ? null
          : () => {
              // A cancelled source's metadata must never seek a newer owner's clip.
              // Owner pause deliberately keeps this handler: resume must retain the
              // intended position even when metadata arrived while playback paused.
              if (
                this.task !== task ||
                !this.available() ||
                media.src !== loadedSource
              )
                return;
              if (media.currentSrc && media.currentSrc !== loadedSource) return;
              const duration = media.duration;
              const loopStart = clip.loopStartSeconds;
              if (
                !Number.isFinite(duration) ||
                duration <= 0 ||
                (loopStart !== undefined && loopStart >= duration)
              ) {
                this.stopEffects(task.owner);
                return;
              }
              const offset = clip.startOffset ?? 0;
              if (
                !media.loop &&
                loopStart === undefined &&
                offset >= duration
              ) {
                this.stopEffects(task.owner);
                return;
              }
              try {
                media.currentTime =
                  loopStart !== undefined && offset >= loopStart
                    ? loopStart +
                      ((offset - loopStart) % (duration - loopStart))
                    : media.loop
                      ? offset % duration
                      : offset;
              } catch {
                this.stopEffects(task.owner);
              }
            };
      if (clip.startOffset !== undefined) {
        // HTML media permits a default start position before metadata. The
        // metadata handler performs duration-aware wrapping before audible play.
        try {
          media.currentTime = clip.startOffset;
        } catch {
          /* Seek on metadata. */
        }
      }
      media.muted = false;
      try {
        await media.play();
        if (!this.allowed) {
          media.muted = true;
          media.pause();
          return false;
        }
        if (this.task !== task) {
          if (!this.task) media.pause();
          return false;
        }
        if (task.generation !== generation) {
          if (this.pausedOwners.has(task.owner)) media.pause();
          return false;
        }
        task.starting = false;
        if (this.pausedOwners.has(task.owner)) {
          media.pause();
          return false;
        }
        this.primed = true;
        return true;
      } catch {
        if (this.task !== task || task.generation !== generation) return false;
        task.starting = false;
        if (this.pausedOwners.has(task.owner)) return false;
        this.primed = false;
      }
    }
    if (this.task === task) this.stopEffects(task.owner);
    return false;
  }
}
