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
  const index = Math.max(0, Math.min(511, Math.round((rpm - meta.mapping.index0RPM) / meta.mapping.rpmPerIndex)));
  const voices = meta.samples.map((sample) => {
    let gain = 0, rate = 0;
    for (const table of sample.tables) {
      const weight = table.kind === "load" ? throttle : 1 - throttle;
      const volume = Math.max(0, (table.volume[index] ?? 0) / 127) * weight;
      gain += volume;
      rate += volume * (table.rate[index] ?? 1);
    }
    rate = gain > 0 ? rate / gain : 1;
    gain *= Math.max(0, Math.min(1, sample.timbreVolume / 127));
    return { key: `racing-engine-${meta.id}-${sample.patch}`, path: sample.path, volume: gain, rate: Math.max(.25, Math.min(4, rate)) };
  });
  const energy = Math.sqrt(voices.reduce((sum, voice) => sum + voice.volume * voice.volume, 0));
  const scale = .23 / Math.max(1, energy);
  for (const voice of voices) voice.volume *= scale;
  return voices;
}
