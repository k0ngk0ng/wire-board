export type ClassicEngineAudio = {
  id: string;
  idleRPM: number;
  redlineRPM: number;
  mapping: { index0RPM: number; rpmPerIndex: number };
  samples: Array<{
    patch: number;
    path: string;
    timbreVolume: number;
    tables: Array<{ kind: string; volume: number[]; rate: number[] }>;
  }>;
};
export type ClassicEngineVoice = { key: string; path: string; volume: number; rate: number };

// Source samples and table values are preserved. This bounded linear crossfade
// is an independent approximation, not a claim about the original fixed-point mixer.
export function classicEngineVoices(meta: ClassicEngineAudio, rpm: number, throttle: number): ClassicEngineVoice[] {
  rpm = Number.isFinite(rpm) ? rpm : meta.idleRPM;
  throttle = Math.max(0, Math.min(1, Number.isFinite(throttle) ? throttle : 0));
  const index = Math.max(0, Math.min(511, (rpm - meta.mapping.index0RPM) / meta.mapping.rpmPerIndex));
  const first = Math.floor(index), second = Math.min(511, first + 1), fraction = index - first;
  const control = (values: number[], fallback: number) => {
    const a = values[first] ?? fallback, b = values[second] ?? fallback;
    return a + (b - a) * fraction;
  };
  const voices = meta.samples.map((sample) => {
    let gain = 0, rate = 0, quietRate = 0, quietWeight = 0;
    for (const table of sample.tables) {
      const weight = table.kind === "load" ? throttle : 1 - throttle;
      const pitch = control(table.rate, 1);
      const volume = Math.max(0, control(table.volume, 0) / 127) * weight;
      gain += volume;
      rate += volume * pitch;
      quietRate += weight * pitch;quietWeight += weight;
    }
    rate = gain > 0 ? rate / gain : quietWeight > 0 ? quietRate / quietWeight : control(sample.tables[0]?.rate ?? [], 1);
    gain *= Math.max(0, Math.min(1, sample.timbreVolume / 127));
    return { key: `racing-engine-${meta.id}-${sample.patch}`, path: sample.path, volume: gain, rate: Math.max(.25, Math.min(4, rate)) };
  });
  const energy = Math.sqrt(voices.reduce((sum, voice) => sum + voice.volume * voice.volume, 0));
  const scale = .23 / Math.max(1, energy);
  for (const voice of voices) voice.volume *= scale;
  return voices;
}
import type { GameAudioPort } from "../../../web/src/game-api/module";

export type ClassicLoadedEngine = { metadata: ClassicEngineAudio; sources: ReadonlyMap<string, string> };
type EnginePort = Pick<GameAudioPort, "loop" | "stopLoop">;

/** A quiet RPM layer keeps its native phase. Only lifecycle/source changes
 * release it; stepping across a control-table boundary must not decode/restart.
 */
export class ClassicEnginePlayback {
  private readonly audio: EnginePort;
  private readonly active = new Set<string>();
  private engine?: ClassicLoadedEngine;
  constructor(audio: EnginePort) { this.audio = audio; }
  readonly stop = () => {
    for (const key of this.active) this.audio.stopLoop(key);
    this.active.clear();
  };
  update(engine: ClassicLoadedEngine | undefined, rpm: number, throttle: number, running: boolean) {
    if (engine !== this.engine) { this.stop();this.engine = engine; }
    if (!running || !engine) { this.stop();return; }
    const desired = new Set<string>();
    for (const voice of classicEngineVoices(engine.metadata, rpm, throttle)) {
      const source = engine.sources.get(voice.path);
      if (!source) continue;
      desired.add(voice.key);this.active.add(voice.key);
      const options = { volume: voice.volume, rate: voice.rate, continuous: true, seamOverlapMS: 5 };
      void this.audio.loop(voice.key, [source], options).catch(() => { /* Audio never interrupts driving. */ });
    }
    for (const key of this.active) if (!desired.has(key)) { this.audio.stopLoop(key);this.active.delete(key); }
  }
}
