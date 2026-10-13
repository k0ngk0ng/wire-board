import { toneDataURL, type Sound } from "./audio-tones.ts";
export type { Sound } from "./audio-tones.ts";
type Clip = { src: string; fallback?: string };
type Task = {
  owner: string;
  priority: number;
  clips: Clip[];
  starting: boolean;
};
const SILENCE =
  "data:audio/wav;base64,UklGRiYAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQIAAACAgA==";

// One native media element per page. All games share media-volume behavior,
// gesture unlock and mute cancellation; game rules only select clip URLs.
export class GameAudio {
  private media?: HTMLAudioElement;
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
  private element() {
    if (!this.media) {
      const media = this.factory();
      media.dataset.platformAudio = "";
      media.preload = "none";
      media.hidden = true;
      media.muted = !this.allowed;
      if (typeof document !== "undefined") document.body.append(media);
      this.media = media;
    }
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
      stop: () => this.stop(owner),
    };
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
    this.stop();
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
      if (this.task === task && !task.starting) this.stop(task.owner);
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
    if (this.task === task) this.stop(task.owner);
    return false;
  }
}
