import type { GameAudioPort } from "../../../web/src/game-api/module";
import { PalAudioAssets } from "./audio-assets";
import type { PalAudioState, PalAudioTrack, PalAudioSound } from "./audio-types";

export type PalAudioFrame = { audio?: PalAudioState; now: number; ready: boolean; paused: boolean; quiet: boolean };
export type PalAudioMemory = { floor: number };
type Attempt = { pending: boolean; tried: boolean; playing: boolean; duration?: number; key?: string; error?: string };
type MusicAttempt = Attempt & { descriptor: PalAudioTrack; signature: string };
type SoundAttempt = Attempt & { descriptor: PalAudioSound };

export class PalAudioDirector {
  private bank: PalAudioAssets;
  private audio: GameAudioPort;
  private scope: string;
  private frame: () => PalAudioFrame;
  private music?: MusicAttempt;
  private sounds = new Map<number, SoundAttempt>();
  private memory: PalAudioMemory;
  private serial = 0;
  private blocked = true;
  private disposed = false;

  constructor(bank: PalAudioAssets, audio: GameAudioPort, scope: string, frame: () => PalAudioFrame, memory: PalAudioMemory) {
    this.bank = bank;
    this.audio = audio;
    this.scope = scope;
    this.frame = frame;
    this.memory = memory;
    const initial = frame();
    if (initial.quiet && initial.audio) this.memory.floor = Math.max(this.memory.floor, initial.audio.serial);
    document.addEventListener("pointerdown", this.gesture, true);
    document.addEventListener("keydown", this.gesture, true);
    document.addEventListener("visibilitychange", this.visibility);
  }

  private visibility = () => this.update();

  private gesture = () => {
    this.bank.retry();
    if (this.music && !this.music.pending && !this.music.playing) this.music.tried = false;
    for (const sound of this.sounds.values()) if (!sound.pending && !sound.playing) sound.tried = false;
    this.update();
  };

  private allowed() {
    const frame = this.frame();
    return !this.disposed && frame.ready && !frame.paused && !frame.quiet;
  }

  private stopMusic() { this.audio.stopTrack("pal-music"); this.music = undefined; }

  private stopSounds() {
    for (const sound of this.sounds.values()) if (sound.key) this.audio.stopTrack(sound.key);
    this.sounds.clear();
  }

  private async playMusic(attempt: MusicAttempt) {
    attempt.pending = attempt.tried = true;
    const descriptor = attempt.descriptor;
    try {
      const source = await this.bank.source(descriptor.kind, descriptor.id);
      if (this.music !== attempt || !this.allowed()) return;
      attempt.duration = source.duration;
      const offset = Math.max(0, this.frame().now - descriptor.startedAt) / 1000;
      if (!descriptor.loop && offset >= source.duration) return;
      attempt.playing = await this.audio.track("pal-music", [source.url], { loop: descriptor.loop, startOffset: offset, playbackID: `${this.scope}:music:${descriptor.seq}:${++this.serial}` });
    } catch (error) {
      attempt.error = error instanceof Error ? error.message : String(error);
      attempt.playing = false;
    } finally {
      attempt.pending = false;
    }
  }

  private lane() {
    const used = new Set([...this.sounds.values()].map(sound => sound.key));
    for (let i = 0; i < 63; i++) if (!used.has(`pal-sfx-${i}`)) return `pal-sfx-${i}`;
    return undefined;
  }

  private async playSound(attempt: SoundAttempt) {
    const key = this.lane();
    if (!key) return;
    attempt.key = key;
    attempt.pending = attempt.tried = true;
    const descriptor = attempt.descriptor;
    try {
      const source = await this.bank.source(2, descriptor.id);
      if (this.sounds.get(descriptor.seq) !== attempt || !this.allowed() || descriptor.seq <= this.memory.floor) return;
      attempt.duration = source.duration;
      const offset = Math.max(0, this.frame().now - descriptor.startedAt) / 1000;
      if (offset >= source.duration) return;
      attempt.playing = await this.audio.track(key, [source.url], { loop: false, startOffset: offset, playbackID: `${this.scope}:sound:${descriptor.seq}:${++this.serial}` });
    } catch (error) {
      attempt.error = error instanceof Error ? error.message : String(error);
      attempt.playing = false;
    } finally {
      attempt.pending = false;
      if (!attempt.playing) attempt.key = undefined;
    }
  }

  update() {
    if (this.disposed) return;
    const frame = this.frame(), view = frame.audio;
    if (view) void this.bank.prepare(view).catch(() => {});
    if (!view || !this.allowed()) {
      if (frame.quiet && view) this.memory.floor = Math.max(this.memory.floor, view.serial);
      if (!this.blocked) { this.stopMusic(); this.stopSounds(); }
      this.blocked = true;
      return;
    }
    this.blocked = false;
    const signature = JSON.stringify(view.music);
    if (!view.music) this.stopMusic();
    else {
      if (this.music?.signature !== signature) {
        this.stopMusic();
        this.music = { descriptor: view.music, signature, pending: false, tried: false, playing: false };
      }
      const music = this.music!;
      if (music.playing && !music.descriptor.loop && music.duration !== undefined && frame.now - music.descriptor.startedAt >= music.duration * 1000) {
        this.audio.stopTrack("pal-music");
        music.playing = false;
      }
      if (!music.pending && !music.tried) void this.playMusic(music);
    }
    const present = new Set(view.sounds.map(sound => sound.seq));
    for (const [seq, sound] of this.sounds) {
      if (!present.has(seq) || seq <= this.memory.floor) {
        if (sound.key) this.audio.stopTrack(sound.key);
        this.sounds.delete(seq);
      } else if (sound.key && sound.duration !== undefined && frame.now - sound.descriptor.startedAt >= sound.duration * 1000) {
        this.audio.stopTrack(sound.key);
        sound.key = undefined;
        sound.playing = false;
      }
    }
    for (const descriptor of view.sounds) {
      if (descriptor.seq <= this.memory.floor) continue;
      let sound = this.sounds.get(descriptor.seq);
      if (!sound) {
        sound = { descriptor, pending: false, tried: false, playing: false };
        this.sounds.set(descriptor.seq, sound);
      }
      if (!sound.tried && !sound.pending) void this.playSound(sound);
    }
  }

  resync() {
    const view = this.frame().audio;
    if (view) this.memory.floor = Math.max(this.memory.floor, view.serial);
    this.stopMusic();
    this.stopSounds();
    this.blocked = true;
  }

  dispose() {
    this.disposed = true;
    this.stopMusic();
    this.stopSounds();
    document.removeEventListener("pointerdown", this.gesture, true);
    document.removeEventListener("keydown", this.gesture, true);
    document.removeEventListener("visibilitychange", this.visibility);
  }
}
