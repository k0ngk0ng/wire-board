import { toneDataURL, type Sound } from "./audio-tones.ts";
import { NativeAudioStream, normalizeStreamConfig } from "./audio-stream.ts";
import { NativePCMStream, normalizePCMConfig } from "./audio-pcm-stream.ts";
import { NativeContinuousLoops } from "./audio-continuous.ts";
import type {
  GameAudioPCMStreamConfig,
  GameAudioPCMStreamHandle,
} from "./game-api/audio-pcm-stream";
import type {
  GameAudioStreamConfig,
  GameAudioStreamHandle,
} from "./game-api/audio-stream";
export type { Sound } from "./audio-tones.ts";
type Clip = {
  src: string;
  fallback?: string;
  alternatives?: readonly string[];
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
  deferredStart?: boolean;
};
type MusicOptions = {
  loop?: boolean;
  startOffset?: number;
  playbackID?: string;
  loopStartSeconds?: number;
};
type TrackOptions = MusicOptions & { muted?: boolean; rate?: number };
type TrackRestoreOptions = {
  startOffset: number;
  muted?: boolean;
  paused?: boolean;
};
type TrackVoice = {
  owner: string;
  key: string;
  audio: GameAudio;
  signature: string;
  playbackID?: string;
  pending: Promise<boolean>;
  paused: boolean;
  stopped: boolean;
  clip: Clip;
  loop: boolean;
  rate: number;
};
type LoopOptions = { volume?: number; rate?: number; continuous?: boolean; seamOverlapMS?: number };
type EffectOptions = { playbackID: string; startOffset?: number };
type FiniteVoice = {
  owner: string;
  playbackID: string;
  sources: string;
  player: GameAudio;
  pending: Promise<boolean>;
};
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
  private continuous?: NativeContinuousLoops;
  // Stable keys name media lanes, not individual events. Keep cancelled event
  // identities so unmute or repeated snapshots cannot replay old one-shots.
  private tracks = new Map<string, TrackVoice>();
  private silent = false;
  private pausedOwners = new Set<string>();
  private factory: () => HTMLAudioElement;
  private allowed = true;
  private level = 1;
  private primed = false;
  private serial = 0;
  private task?: Task;
  private assets = "";
  constructor(factory: () => HTMLAudioElement = () => new Audio()) {
    this.factory = factory;
  }
  get enabled() {
    return this.allowed && this.level > 0;
  }
  setVolume(value: number) {
    if (!Number.isFinite(value)) return;
    this.level = Math.max(0, Math.min(1, value));
    if (this.level === 0) this.stop();
    if (this.media) this.media.volume = this.level;
    this.background?.setVolume(this.level);
    this.stream?.controller.setVolume(this.level);
    this.pcm?.controller.setVolume(this.level);
    this.continuous?.setVolume(this.level);
    for (const voice of this.finite.values())
      voice.player.setVolume(this.level);
    for (const voice of this.loops.values())
      voice.media.volume = voice.volume * this.level;
    for (const voice of this.tracks.values()) voice.audio.setVolume(this.level);
  }
  private createElement() {
    const media = this.factory();
    media.dataset.platformAudio = "";
    media.preload = "none";
    media.hidden = true;
    media.muted = !this.allowed || this.silent;
    media.volume = this.level;
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
      this.enabled && (typeof document === "undefined" || !document.hidden)
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
    void this.continuous?.unlock();
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
    this.continuous?.pause(owner);
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
    if (this.stream?.owner === owner) this.stream.controller.pause();
    if (this.pcm?.owner === owner) this.pcm.controller.pause();
    for (const voice of this.finite.values()) if (voice.owner === owner) voice.player.pause(owner);
    for (const voice of this.tracks.values())
      if (voice.owner === owner && !voice.stopped) voice.audio.pause(owner);
  }
  async resume(owner: string): Promise<boolean> {
    if (!this.pausedOwners.delete(owner) || !this.available()) return false;
    const pending: Promise<boolean>[] = [];
    if (this.continuous?.owner === owner) pending.push(this.continuous.resume(owner));
    const task = this.task;
    if (task?.owner === owner && task.deferredStart) {
      task.deferredStart = false;
      pending.push(this.next(task));
    } else if (task?.owner === owner && this.media?.src) {
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
    if (this.stream?.owner === owner) pending.push(this.stream.controller.resume());
    if (this.pcm?.owner === owner) pending.push(this.pcm.controller.resume());
    for (const voice of this.finite.values()) if (voice.owner === owner) pending.push(voice.player.resume(owner));
    for (const voice of this.tracks.values()) {
      if (voice.owner !== owner || voice.stopped || voice.paused) continue;
      voice.pending = voice.audio.resume(owner);
      pending.push(voice.pending);
    }
    return (await Promise.all(pending)).some(Boolean);
  }
  stop(owner?: string) {
    if (owner === undefined) this.pausedOwners.clear();
    else this.pausedOwners.delete(owner);
    this.stopEffects(owner);
    if (owner === undefined || this.continuous?.owner === owner) { this.continuous?.stop();this.continuous = undefined; }
    if (owner === undefined || this.stream?.owner === owner)
      this.stream?.controller.stop();
    if (owner === undefined || this.pcm?.owner === owner)
      this.pcm?.controller.stop();
    for (const voice of this.loops.values())
      if (owner === undefined || voice.owner === owner) this.releaseLoop(voice);
    for (const [key, voice] of this.finite)
      if (owner === undefined || voice.owner === owner) {
        this.finite.delete(key);
        voice.player.dispose();
      }
    this.background?.stop(owner);
    for (const voice of this.tracks.values())
      if (owner === undefined || voice.owner === owner)
        this.releaseTrack(voice);
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
    this.tracks.clear();
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
      effect: (
        lane: string,
        sources: readonly string[],
        options: EffectOptions,
      ) => this.startFinite(owner, lane, sources, options),
      stopEffect: (lane: string, expectedPlaybackID?: string) => {
        const key = JSON.stringify([owner, lane]);
        const voice = this.finite.get(key);
        if (
          !voice ||
          (expectedPlaybackID !== undefined &&
            voice.playbackID !== expectedPlaybackID)
        )
          return;
        this.finite.delete(key);
        voice.player.dispose();
      },
      openStream: (options: GameAudioStreamConfig) =>
        this.openStream(owner, options),
      openPCMStream: (options: GameAudioPCMStreamConfig) =>
        this.openPCMStream(owner, options),
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
        if (this.continuous?.owner === owner) this.continuous.stopKey(key);
        const voice = this.loops.get(key);
        if (voice?.owner === owner) this.releaseLoop(voice);
      },
      track: (
        key: string,
        sources: readonly string[],
        options: TrackOptions = {},
      ) => this.startTrack(owner, key, sources, options),
      stopTrack: (key: string) => {
        const voice = this.tracks.get(this.trackKey(owner, key));
        if (voice) this.releaseTrack(voice);
      },
      muteTrack: (key: string, muted: boolean) => {
        const voice = this.tracks.get(this.trackKey(owner, key));
        if (voice && !voice.stopped) this.silenceTrack(voice, muted);
      },
      pauseTrack: (key: string) => {
        const voice = this.tracks.get(this.trackKey(owner, key));
        if (!voice || voice.stopped) return;
        voice.paused = true;
        voice.audio.pause(owner);
      },
      resumeTrack: (key: string) => this.resumeTrack(owner, key),
      seekTrack: (key: string, offsetSeconds: number) =>
        this.seekTrack(owner, key, offsetSeconds),
      restoreTrack: (
        key: string,
        playbackID: string,
        options: TrackRestoreOptions,
      ) => this.restoreTrack(owner, key, playbackID, options),
    };
  }
  private trackKey(owner: string, key: string) {
    return JSON.stringify([owner, key]);
  }
  private releaseTrack(voice: TrackVoice) {
    voice.stopped = true;
    voice.paused = false;
    voice.audio.dispose();
  }
  private silenceTrack(voice: TrackVoice, muted: boolean) {
    voice.audio.silent = muted;
    if (voice.audio.media) voice.audio.media.muted = muted || !this.available();
  }
  private resumeTrack(owner: string, key: string): Promise<boolean> {
    const voice = this.tracks.get(this.trackKey(owner, key));
    if (!voice || voice.stopped || !voice.paused || !this.available())
      return Promise.resolve(false);
    voice.paused = false;
    if (this.pausedOwners.has(owner)) return Promise.resolve(false);
    voice.pending = voice.audio.resume(owner);
    return voice.pending;
  }
  private seekTrack(owner: string, key: string, offset: number): boolean {
    const voice = this.tracks.get(this.trackKey(owner, key));
    const media = voice?.audio.media;
    const task = voice?.audio.task;
    if (
      !this.available() ||
      !voice ||
      voice.stopped ||
      !task ||
      !Number.isFinite(offset) ||
      offset < 0
    )
      return false;
    if (task.deferredStart) {
      voice.clip.startOffset = offset;
      return true;
    }
    if (!media?.src) return false;
    const loopStart = voice.clip.loopStartSeconds;
    if (media.ended && !media.loop && loopStart === undefined) return false;
    if (media.readyState === 0) {
      // next() installed the source-bound metadata handler for every track.
      // Updating the shared Clip makes the latest seek win, including fallback.
      voice.clip.startOffset = offset;
      task.pendingLoopRestart = undefined;
      return true;
    }
    const duration = media.duration;
    if (
      !Number.isFinite(duration) ||
      duration <= 0 ||
      (loopStart !== undefined && loopStart >= duration) ||
      (!media.loop && loopStart === undefined && offset >= duration)
    )
      return false;
    const position =
      loopStart !== undefined && offset >= loopStart
        ? loopStart + ((offset - loopStart) % (duration - loopStart))
        : media.loop
          ? offset % duration
          : offset;
    try {
      media.currentTime = position;
    } catch {
      return false;
    }
    voice.clip.startOffset = offset;
    // A paused suffix may be waiting to restart at its loop head. An explicit
    // authority resync supersedes that pending position without resuming it.
    task.pendingLoopRestart = undefined;
    return true;
  }
  private restoreTrack(
    owner: string,
    key: string,
    playbackID: string,
    options: TrackRestoreOptions,
  ): Promise<boolean> {
    const id = this.trackKey(owner, key),
      previous = this.tracks.get(id);
    if (
      !this.available() ||
      this.pausedOwners.has(owner) ||
      !previous?.loop ||
      !playbackID ||
      previous.playbackID !== playbackID ||
      !Number.isFinite(options.startOffset) ||
      options.startOffset < 0
    )
      return Promise.resolve(false);
    const paused = options.paused ?? previous.paused;
    if (!previous.stopped && previous.audio.task) {
      if (!this.seekTrack(owner, key, options.startOffset))
        return Promise.resolve(false);
      if (options.muted !== undefined)
        this.silenceTrack(previous, options.muted);
      if (paused) {
        previous.paused = true;
        previous.audio.pause(owner);
        return Promise.resolve(true);
      }
      if (previous.paused) return this.resumeTrack(owner, key);
      return previous.pending;
    }
    const muted = options.muted ?? previous.audio.silent;
    this.releaseTrack(previous);
    const audio = new GameAudio(this.factory);
    audio.setVolume(this.level);
    audio.silent = muted;
    const media = audio.element();
    media.dataset.platformTrack = key;
    media.loop = previous.loop && previous.clip.loopStartSeconds === undefined;
    media.defaultPlaybackRate = previous.rate;
    media.playbackRate = previous.rate;
    const clip = { ...previous.clip, startOffset: options.startOffset };
    const voice: TrackVoice = {
      ...previous,
      audio,
      clip,
      paused,
      stopped: false,
      pending: Promise.resolve(false),
    };
    this.tracks.set(id, voice);
    voice.pending = audio.start([clip], owner, 1, paused);
    return voice.pending;
  }
  private startTrack(
    owner: string,
    key: string,
    requested: readonly string[],
    options: TrackOptions,
  ): Promise<boolean> {
    if (
      !this.available() ||
      this.pausedOwners.has(owner) ||
      !key ||
      key.length > 128
    )
      return Promise.resolve(false);
    const loop = options.loop ?? false;
    const loopStart = options.loopStartSeconds ?? 0;
    const rate = options.rate ?? 1;
    if (
      (options.startOffset !== undefined &&
        (!Number.isFinite(options.startOffset) || options.startOffset < 0)) ||
      !Number.isFinite(loopStart) ||
      loopStart < 0 ||
      (options.loopStartSeconds !== undefined && !loop) ||
      !Number.isFinite(rate) ||
      rate < 0.25 ||
      rate > 4
    )
      return Promise.resolve(false);
    const sources = [...new Set(requested.filter(Boolean))];
    if (!sources.length) return Promise.resolve(false);
    const id = this.trackKey(owner, key);
    const previous = this.tracks.get(id);
    const signature = JSON.stringify([sources, loop, loopStart, rate]);
    if (
      previous &&
      options.playbackID !== undefined &&
      previous.playbackID === options.playbackID
    ) {
      if (
        previous.signature !== signature ||
        previous.stopped ||
        !previous.audio.task
      )
        return Promise.resolve(false);
      if (options.muted !== undefined)
        this.silenceTrack(previous, options.muted);
      return previous.pending;
    }
    if (previous?.paused) return Promise.resolve(false);
    if (!previous && this.tracks.size >= 64) {
      // Bound retained lanes/tombstones. Never evict live media or this owner's
      // current event identities to make an arbitrary event-as-key request fit.
      const retired = [...this.tracks.entries()].find(
        ([, v]) => v.owner !== owner && v.stopped,
      );
      if (!retired) return Promise.resolve(false);
      this.tracks.delete(retired[0]);
    }
    if (previous) this.releaseTrack(previous);
    const audio = new GameAudio(this.factory);
    audio.setVolume(this.level);
    audio.silent = options.muted ?? false;
    const media = audio.element();
    media.dataset.platformTrack = key;
    media.loop = loop && loopStart === 0;
    media.defaultPlaybackRate = rate;
    media.playbackRate = rate;
    const clip: Clip = {
      src: sources[0],
      fallback: sources[1],
      startOffset: options.startOffset ?? 0,
      loopStartSeconds: loopStart > 0 ? loopStart : undefined,
    };
    const voice: TrackVoice = {
      owner,
      key,
      audio,
      signature,
      playbackID: options.playbackID,
      pending: Promise.resolve(false),
      paused: false,
      stopped: false,
      clip,
      loop,
      rate,
    };
    this.tracks.set(id, voice);
    // Reuse native one-shot playback, metadata seek, fallback, pause-generation
    // guards and intro/suffix looping. No game priority or timing rules live here.
    voice.pending = audio.start([clip], owner, 1);
    return voice.pending;
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
      media.volume = voice.volume * this.level;
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
      const targetVolume = voice.volume * this.level;
      media.volume += (targetVolume - media.volume) * blend;
      media.playbackRate += (voice.rate - media.playbackRate) * blend;
      if (
        Math.abs(media.volume - targetVolume) < 0.001 &&
        Math.abs(media.playbackRate - voice.rate) < 0.001
      ) {
        media.volume = targetVolume;
        media.playbackRate = voice.rate;
      } else voice.frame = requestAnimationFrame(ramp);
    };
    voice.frame = requestAnimationFrame(ramp);
  }
  private startLoop(owner: string, key: string, requested: readonly string[], options: LoopOptions): Promise<boolean> {
    if (!this.available() || this.pausedOwners.has(owner) || !key) return Promise.resolve(false);
    if (this.continuous && (this.continuous.owner !== owner || this.continuous.stopped)) { this.continuous.stop();this.continuous = undefined; }
    if (!options.continuous) {
      if (this.continuous?.owner === owner) this.continuous.stopKey(key);
      return this.startNativeLoop(owner, key, requested, options);
    }
    const sources = [...new Set(requested.filter(Boolean))];
    for (const voice of this.loops.values()) if (voice.owner !== owner) this.releaseLoop(voice);
    if (!sources.length) {
      this.continuous?.stopKey(key);
      const old = this.loops.get(key);if (old?.owner === owner) this.releaseLoop(old);
      return Promise.resolve(false);
    }
    if (!this.continuous) {
      this.playbackSession();this.continuous = NativeContinuousLoops.prepare(owner, () => this.createElement(), () => this.available());
    }
    const backend = this.continuous;
    if (!backend) return this.startNativeLoop(owner, key, sources, options, "unsupported-capability");
    backend.setVolume(this.level);this.playbackSession();
    if (backend.failed(key, sources)) return this.startNativeLoop(owner, key, sources, options, "decode-failed");
    const previous = this.loops.get(key);if (previous) this.releaseLoop(previous);
    return backend.loop(key, sources, options).then(ok => {
      if (!ok && this.continuous === backend && backend.failed(key, sources) && this.available() && !this.pausedOwners.has(owner))
        return this.startNativeLoop(owner, key, sources, options, "decode-failed");
      return ok;
    });
  }
  private startNativeLoop(
    owner: string,
    key: string,
    requested: readonly string[],
    options: LoopOptions,
    fallback?: string,
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
    if (fallback) media.dataset.continuousFallback = fallback;
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
      // The native load algorithm restores playbackRate from this default.
      // Configure it before src so an idle loop keeps its requested pitch even
      // when no later RPM/control update arrives to tune it again.
      media.defaultPlaybackRate = voice.rate;
      media.src = voice.sources[index];
      this.tuneLoop(voice, {}, true);
      media.muted = this.silent;
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
    this.background.setVolume(this.level);
    const media = this.background.element();
    media.dataset.platformMusic = "";
    media.loop = (options.loop ?? true) && loopStart === 0;
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
    initiallyPaused = false,
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
    if (initiallyPaused) {
      task.deferredStart = true;
      this.pausedOwners.add(owner);
      return true;
    }
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
    for (const src of [
      clip.src,
      clip.fallback,
      ...(clip.alternatives ?? []),
    ].filter((s): s is string => !!s)) {
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
      // A cached finite duration can expire before play() is requested.
      if (media.readyState >= 1) media.onloadedmetadata?.(new Event("loadedmetadata"));
      if (this.task !== task) return false;
      media.muted = this.silent;
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

private finite = new Map<string, FiniteVoice>();
private stream?: { owner: string; controller: NativeAudioStream };
private pcm?: { owner: string; controller: NativePCMStream };
private openStream(
    owner: string,
    options: GameAudioStreamConfig,
  ): GameAudioStreamHandle | undefined {
    if (!this.available() || this.pausedOwners.has(owner)) return;
    const config = normalizeStreamConfig(options);
    if (!config) return;
    const old = this.stream;
    if (
      old?.owner === owner &&
      old.controller.status().playbackID === config.playbackID
    )
      return old.controller.matches(config) ? old.controller : undefined;
    let controller: NativeAudioStream | undefined;
    controller = NativeAudioStream.prepare(
      config,
      () => this.createElement(),
      () => this.available(),
      () => {
        if (this.stream?.controller === controller) this.stream = undefined;
      },
    );
    if (!controller) return;
    this.stream = { owner, controller };
    old?.controller.stop();
    this.pcm?.controller.stop();
    controller.setVolume(this.level);
    this.playbackSession();
    controller.begin();
    return controller;
  }
private openPCMStream(
    owner: string,
    options: GameAudioPCMStreamConfig,
  ): GameAudioPCMStreamHandle | undefined {
    if (!this.available() || this.pausedOwners.has(owner)) return;
    const config = normalizePCMConfig(options);
    if (!config) return;
    const old = this.pcm;
    if (
      old?.owner === owner &&
      old.controller.status().playbackID === config.playbackID
    )
      return old.controller.matches(config) ? old.controller : undefined;
    let controller: NativePCMStream | undefined;
    controller = NativePCMStream.prepare(
      config,
      () => this.createElement(),
      () => this.available(),
      () => {
        if (this.pcm?.controller === controller) this.pcm = undefined;
      },
    );
    if (!controller) return;
    this.pcm = { owner, controller };
    old?.controller.stop();
    this.stream?.controller.stop();
    controller.setVolume(this.level);
    this.playbackSession();
    controller.begin();
    return controller;
  }
private startFinite(
    owner: string,
    lane: string,
    sources: readonly string[],
    options: EffectOptions,
  ): Promise<boolean> {
    if (
      !this.available() ||
      this.pausedOwners.has(owner) ||
      typeof lane !== "string" ||
      !lane.trim() ||
      lane.length > 128 ||
      !Array.isArray(sources) ||
      !sources.length ||
      sources.length > 4 ||
      sources.some((source) => typeof source !== "string" || !source) ||
      !options ||
      typeof options.playbackID !== "string" ||
      !options.playbackID.trim() ||
      options.playbackID.length > 256 ||
      (options.startOffset !== undefined &&
        (!Number.isFinite(options.startOffset) || options.startOffset < 0))
    )
      return Promise.resolve(false);
    const key = JSON.stringify([owner, lane]);
    const identity = JSON.stringify(sources);
    const previous = this.finite.get(key);
    if (previous?.playbackID === options.playbackID) {
      if (previous.sources !== identity || !previous.player.task)
        return Promise.resolve(false);
      if (
        previous.player.media &&
        !previous.player.media.paused &&
        !previous.player.task.starting
      )
        return Promise.resolve(true);
      return previous.pending;
    }
    // Stable lanes retain only their latest instance, including an ended token.
    // Instance IDs must not become lane names or create an unbounded media pool.
    if (
      !previous &&
      (this.finite.size >= 32 ||
        [...this.finite.values()].filter((voice) => voice.owner === owner)
          .length >= 16)
    )
      return Promise.resolve(false);
    previous?.player.dispose();
    const player = new GameAudio(this.factory);
    player.setVolume(this.level);
    const media = player.element();
    media.dataset.platformEffectLane = lane;
    media.loop = false;
    const voice: FiniteVoice = {
      owner,
      playbackID: options.playbackID,
      sources: identity,
      player,
      pending: Promise.resolve(false),
    };
    this.finite.set(key, voice);
    voice.pending = player.start(
      [
        {
          src: sources[0],
          fallback: sources[1],
          alternatives: sources.slice(2),
          startOffset: options.startOffset,
        },
      ],
      owner,
      1,
    );
    return voice.pending;
  }
}
