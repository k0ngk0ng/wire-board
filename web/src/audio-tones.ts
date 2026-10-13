// Original platform cues rendered as PCM media, not an AudioContext output.
const notes = {
  preview: [523.25, 659.25, 783.99],
  take: [880, 1174.66, 1318.51],
  buy: [523.25, 659.25, 783.99, 1046.5],
  reserve: [440, 659.25],
  turn: [659.25, 880, 880],
  finish: [523.25, 659.25, 783.99, 1046.5, 1318.51],
  move: [587.33, 783.99],
};
export type Sound = keyof typeof notes;
export const soundNames = Object.keys(notes) as Sound[];
export function soundWav(sound: Sound): Uint8Array {
  const frequencies = notes[sound],
    rate = 44100;
  const step = sound === "turn" || sound === "finish" ? 0.18 : 0.09;
  const duration = sound === "finish" ? 0.36 : 0.22;
  const samples = new Float64Array(
    Math.ceil(
      (0.02 + (frequencies.length - 1) * step + duration + 0.04) * rate,
    ),
  );
  frequencies.forEach((frequency, i) => {
    const start = Math.round((0.02 + i * step) * rate);
    for (let j = 0; j < Math.floor(duration * rate); j++) {
      const t = j / rate;
      const gain =
        t < 0.008
          ? (0.18 * t) / 0.008
          : 0.18 * Math.pow(0.001 / 0.18, (t - 0.008) / (duration - 0.008));
      samples[start + j] +=
        (2 / Math.PI) * Math.asin(Math.sin(2 * Math.PI * frequency * t)) * gain;
    }
  });
  const bytes = new Uint8Array(44 + samples.length * 2),
    view = new DataView(bytes.buffer);
  const text = (at: number, value: string) => {
    for (let i = 0; i < value.length; i++) bytes[at + i] = value.charCodeAt(i);
  };
  text(0, "RIFF");
  view.setUint32(4, bytes.length - 8, true);
  text(8, "WAVE");
  text(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, rate, true);
  view.setUint32(28, rate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  text(36, "data");
  view.setUint32(40, samples.length * 2, true);
  samples.forEach((sample, i) =>
    view.setInt16(
      44 + i * 2,
      Math.round(Math.max(-1, Math.min(1, sample)) * 32767),
      true,
    ),
  );
  return bytes;
}
const cached = new Map<Sound, string>();
export function toneDataURL(sound: Sound): string {
  if (!cached.has(sound)) {
    const bytes = soundWav(sound);
    let text = "";
    for (let i = 0; i < bytes.length; i += 8192)
      text += String.fromCharCode(...bytes.subarray(i, i + 8192));
    cached.set(sound, "data:audio/wav;base64," + btoa(text));
  }
  return cached.get(sound)!;
}
