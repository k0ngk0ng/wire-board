import type { GameAudioPort } from "../../../web/src/game-api/module";
import playback from "../assets/audio-playback.json";
import { assetURL } from "./assets";
import { FPS, frameNumber, type NetworkView } from "./types";
import type { RuntimeView } from "./wasm";

type Clip = { file: string; frames: number; loop: boolean };
const music = playback.music as Record<string, Clip>;
const effects = playback.effects as Record<string, Clip>;
const arenaMusic = [0x82, 0x86, 0x83, 0x95, 0x85, 0x8a, 0x88, 0x8d];

/** Frame-based media selection. Playback and the uniform mute belong to the
 * platform. The original hardware's channel stealing/loop phase can differ. */
export class MatchAudio {
  private watermark = -1n;
  private track: number;
  private position = 0;
  private at = performance.now();
  private active = false;
  private disposed = false;
  private lane = 0;
  private generation = 0;
  private musicFailed = false;

  constructor(
    private port: GameAudioPort,
    private base: string,
    private epoch: string,
    arena: number,
  ) {
    this.track = arenaMusic[arena / 2] ?? 0x82;
    document.addEventListener("visibilitychange", this.visibility);
    window.addEventListener("pointerdown", this.gesture, true);
    window.addEventListener("keydown", this.gesture, true);
  }
  private sources(clip: Clip) {
    return [assetURL(this.base, "/yuyuhakusho/audio/" + clip.file)];
  }
  private settle() {
    const now = performance.now();
    if (this.active) this.position += Math.max(0, now - this.at) / 1000;
    this.at = now;
  }
  private playMusic() {
    const clip = music[this.track] ?? effects[this.track];
    if (!this.active || !clip) return;
    this.musicFailed = false;
    const generation = ++this.generation;
    void Promise.resolve(this.port.music(this.sources(clip), {
      loop: clip.loop,
      startOffset: this.position,
      playbackID: `${this.epoch}:music:${this.track}:${generation}`,
    })).then((ok) => {
      if (this.generation === generation) this.musicFailed = !ok;
    }).catch(() => {
      if (this.generation === generation) this.musicFailed = true;
    });
  }
  restore(view: NetworkView) {
    this.discardThrough(view.frame);
    this.settle();
    // Reconnect restores only current background state, never past effects.
    if (view.phase === "result") {
      this.track = 0x8f;
      this.position = 0;
    } else {
      this.track = arenaMusic[view.options.arena / 2] ?? 0x82;
      const clip = music[this.track];
      const period = Math.max(1, Math.round(clip.frames / 44100 * FPS));
      this.position = Number(frameNumber(view.frame) % BigInt(period)) / FPS;
    }
    this.suspend();
  }
  discardThrough(frame: string) {
    const value = frameNumber(frame);
    if (value > this.watermark) this.watermark = value;
  }
  sync(view: NetworkView, runtime: RuntimeView | undefined, sound: boolean, connected: boolean, paused: boolean) {
    if (this.disposed) return;
    const allowed = sound && connected && !document.hidden && !paused &&
      !runtime?.paused && view.phase !== "paused" && view.phase !== "loading";
    if (!allowed) {
      this.suspend();
      return;
    }
    if ((view.phase === "result" || runtime?.completed) && this.track !== 0x8f) {
      this.settle();
      this.track = 0x8f;
      this.position = 0;
      if (this.active) this.playMusic();
    }
    if (!this.active) {
      this.at = performance.now();
      this.active = true;
      this.playMusic();
    }
  }
  consume(view: RuntimeView, codes: readonly number[]) {
    if (this.disposed) return;
    const frame = frameNumber(view.frame);
    if (frame <= this.watermark) return;
    this.watermark = frame;
    if (view.paused) {
      this.suspend();
      return;
    }
    if (!this.active || document.hidden) return;
    codes.forEach((code, index) => {
      if (music[code] || code === 0x8f) {
        if (this.track !== code) {
          this.settle();
          this.track = code;
          this.position = 0;
          this.playMusic();
        }
        return;
      }
      const clip = effects[code];
      if (!clip) return;
      const lane = "yuyu-" + this.lane++ % 8;
      // A high-water mark survives all prediction forks. Corrections and
      // repeated input windows never replay a previously presented frame.
      void Promise.resolve(this.port.effect(lane, this.sources(clip), {
        playbackID: `${this.epoch}:${view.frame}:${index}:${code}`,
      })).catch(() => {});
    });
  }
  suspend() {
    if (!this.active) return;
    this.settle();
    this.active = false;
    this.generation++;
    this.port.stop();
  }
  private visibility = () => {
    if (document.hidden) this.suspend();
  };
  private gesture = () => {
    if (this.active && this.musicFailed) this.playMusic();
  };
  dispose() {
    if (this.disposed) return;
    this.suspend();
    this.port.stop();
    this.disposed = true;
    document.removeEventListener("visibilitychange", this.visibility);
    window.removeEventListener("pointerdown", this.gesture, true);
    window.removeEventListener("keydown", this.gesture, true);
  }
}
