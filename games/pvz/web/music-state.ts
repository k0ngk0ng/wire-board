import type { GameAudioPort } from "../../../web/src/game-api/module";

export type MusicTrack = { key: string; src: string; role?: "main" | "drums" | "hihats"; initiallyMuted?: boolean; intentionalSilence?: boolean; loopStartSeconds?: number; bytes?: number; sha256?: string; required?: boolean };
export type MusicTheme = { tracks: MusicTrack[]; variants?: Record<string, MusicTheme> };
export type MusicFrame = { enabled: boolean; visible: boolean; ready: boolean; loading: boolean; paused: boolean; finished: boolean; elapsed: number; pressure: number; clockEpoch?: number };
type Port = Pick<GameAudioPort, "track" | "restoreTrack" | "stopTrack" | "seekTrack" | "muteTrack">;
type Lane = { id: string; started: boolean; muted: boolean };

// Public visible pressure is a PvP presentation adaptation. Night variants are
// preserved assets, but lack a reliable original bar-transition clock here;
// keep its verified base theme rather than layer two melodies.
export class SceneMusic {
  private port: Port;
  private base: string;
  private theme: MusicTheme;
  private scope: string;
  private prepare: (src: string) => Promise<void>;
  private frame?: MusicFrame;
  private lanes = new Map<string, Lane>();
  private pending = new Map<string, Promise<void>>();
  private disposed = false;
  private running = false;
  private failed = new Set<string>();
  constructor(port: Port, base: string, theme: MusicTheme, scope: string, prepare: (src: string) => Promise<void> = async () => {}) {
    this.port = port; this.base = base; this.theme = theme; this.scope = scope; this.prepare = prepare;
  }
  private active() {
    const f = this.frame;
    return !this.disposed && !!f && f.enabled && f.visible && f.ready && !f.loading && !f.paused && !f.finished;
  }
  private wanted(track: MusicTrack) {
    if (track.intentionalSilence) return false;
    if (track.required !== false || track.role === "main") return true;
    return !this.theme.variants && (this.frame?.pressure ?? 0) >= 10;
  }
  private start(track: MusicTrack): Promise<void> {
    const key = `pvz-music-${track.key}`, previous = this.pending.get(key);
    if (previous) return previous;
    const task = (async () => {
      await this.prepare(track.src);
      if (!this.active() || !this.wanted(track)) return;
      const elapsed = this.frame!.elapsed, id = `${this.scope}:${track.key}`;
      const muted = track.required === false ? false : track.initiallyMuted ?? false;
      this.lanes.set(key, { id, started: false, muted });
      let played = await this.port.restoreTrack(key, id, { startOffset: elapsed, muted, paused: false });
      if (!played && this.active() && this.wanted(track)) played = await this.port.track(key, [`${this.base.replace(/\/$/, "")}/${track.src.replace(/^\//, "")}`], { loop: true, playbackID: id, startOffset: this.frame!.elapsed, loopStartSeconds: track.loopStartSeconds, muted });
      const lane = this.lanes.get(key);
      if (lane && this.active()) lane.started = played;
      if (!played && this.active()) this.failed.add(key);
    })().catch(() => { this.failed.add(key); }).finally(() => this.pending.delete(key));
    this.pending.set(key, task); return task;
  }
  async sync(frame: MusicFrame): Promise<void> {
    if (this.disposed) return;
    const changedClock = this.frame?.clockEpoch !== undefined && frame.clockEpoch !== this.frame.clockEpoch;
    this.frame = frame;
    if (!this.active()) { this.stop(); return; }
    this.running = true;
    const pending: Promise<void>[] = [];
    for (const track of this.theme.tracks) {
      const key = `pvz-music-${track.key}`, lane = this.lanes.get(key);
      if (!this.wanted(track)) {
        if (lane?.started && !lane.muted) { this.port.muteTrack(key, true); lane.muted = true; }
        continue;
      }
      if (lane?.started) {
        const muted = track.required === false ? false : track.initiallyMuted ?? false;
        if (lane.muted !== muted) { this.port.seekTrack(key, frame.elapsed); this.port.muteTrack(key, muted); lane.muted = muted; }
        else if (changedClock) this.port.seekTrack(key, frame.elapsed);
      } else if (!this.failed.has(key)) pending.push(this.start(track));
    }
    await Promise.all(pending);
  }
  private stop() {
    this.failed.clear();
    if (!this.running) return;
    for (const [key, lane] of this.lanes) { this.port.stopTrack(key); lane.started = false; }
    this.running = false;
  }
  retry() { this.failed.clear(); }
  dispose() { this.stop(); this.disposed = true; }
}
