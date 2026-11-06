import type { GameAudioPort } from "../../../web/src/game-api/module";
import type { Event } from "./types";

type EffectsPort = Pick<GameAudioPort, "track" | "stopTrack">;

export class PvzEffects {
  private port: EffectsPort;
  private base: string;
  private sounds: Record<string, string | string[]>;
  private epoch = "";
  private cursor = -1;
  private slot = 0;
  private disposed = false;
  constructor(port: EffectsPort, base: string, sounds: Record<string, string | string[]>) {
    this.port = port; this.base = base; this.sounds = sounds;
  }
  silence(epoch: string, events: Event[]) {
    if (this.epoch !== epoch) this.cursor = -1;
    this.epoch = epoch;
    this.cursor = Math.max(this.cursor, ...events.map(event => event.id));
    for (let slot = 0; slot < 8; slot++) this.port.stopTrack(`pvz-fx-${slot}`);
  }
  consume(epoch: string, events: Event[], enabled: boolean, visible: boolean) {
    if (this.disposed) return;
    if (this.epoch !== epoch) {
      this.cursor = -1; this.slot = 0; this.silence(epoch, events); return;
    }
    if (!enabled || !visible) { this.silence(epoch, events); return; }
    for (const event of events) {
      if (event.id <= this.cursor) continue;
      this.cursor = Math.max(this.cursor, event.id);
      const source = this.sounds[`${event.type}:${event.kind}`] ?? this.sounds[event.type];
      if (!source) continue;
      const sources = (Array.isArray(source) ? source : [source]).map(path => /^(https?:|data:|blob:)/.test(path) ? path : `${this.base.replace(/\/$/, "")}/${path.replace(/^\//, "")}`);
      const slot = this.slot++ % 8;
      void this.port.track(`pvz-fx-${slot}`, sources, { loop: false, playbackID: `${epoch}:${event.id}` }).catch(() => false);
    }
  }
  dispose() {
    this.silence(this.epoch, []); this.disposed = true;
  }
}
