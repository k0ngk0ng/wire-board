#!/usr/bin/env node
// Deterministic original WAV effects for bubbleparty. No external samples.
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const output = path.join(root, "games/bubbleparty/assets/public/bubbleparty/v1/audio");
const manifestPath = path.join(root, "games/bubbleparty/assets/manifest.json");
const rate = 44100;

function rng(seed) {
  let state = seed >>> 0;
  return () => {
    state = (state * 1664525 + 1013904223) >>> 0;
    return state / 0x100000000;
  };
}
function silence(seconds) {
  return new Float32Array(Math.round(rate * seconds));
}
function tone(buffer, start, frequency, seconds, volume, decay = 5, square = false) {
  const from = Math.round(start * rate);
  const length = Math.round(seconds * rate);
  for (let i = 0; i < length && from + i < buffer.length; i++) {
    const t = i / rate;
    const envelope = Math.exp(-decay * t) * Math.min(1, t / 0.004);
    const phase = Math.sin(2 * Math.PI * frequency * t);
    buffer[from + i] += (square ? Math.sign(phase) * 0.55 : phase) * volume * envelope;
  }
}
function glide(buffer, start, fromHz, toHz, seconds, volume, decay = 3) {
  const from = Math.round(start * rate);
  const length = Math.round(seconds * rate);
  let phase = 0;
  for (let i = 0; i < length && from + i < buffer.length; i++) {
    const t = i / rate;
    const f = fromHz + (toHz - fromHz) * (i / length);
    phase += (2 * Math.PI * f) / rate;
    const envelope = Math.exp(-decay * t) * Math.min(1, t / 0.01);
    buffer[from + i] += Math.sin(phase) * volume * envelope;
  }
}
function noise(buffer, start, seconds, volume, decay = 8, seed = 7) {
  const random = rng(seed);
  const from = Math.round(start * rate);
  const length = Math.round(seconds * rate);
  for (let i = 0; i < length && from + i < buffer.length; i++) {
    const t = i / rate;
    const envelope = Math.exp(-decay * t) * Math.min(1, t / 0.003);
    buffer[from + i] += (random() * 2 - 1) * volume * envelope;
  }
}
function normalize(buffer) {
  let peak = 0;
  for (const value of buffer) peak = Math.max(peak, Math.abs(value));
  if (peak > 0.82) for (let i = 0; i < buffer.length; i++) buffer[i] *= 0.82 / peak;
  for (let i = 0; i < buffer.length; i++) {
    const edge = Math.min(i, buffer.length - 1 - i) / (rate * 0.006);
    buffer[i] *= Math.min(1, edge);
  }
  return buffer;
}
function wav(buffer) {
  const data = Buffer.alloc(buffer.length * 2);
  for (let i = 0; i < buffer.length; i++) {
    data.writeInt16LE(Math.round(Math.max(-1, Math.min(1, buffer[i])) * 32767), i * 2);
  }
  const header = Buffer.alloc(44);
  header.write("RIFF", 0);
  header.writeUInt32LE(36 + data.length, 4);
  header.write("WAVE", 8);
  header.write("fmt ", 12);
  header.writeUInt32LE(16, 16);
  header.writeUInt16LE(1, 20);
  header.writeUInt16LE(1, 22);
  header.writeUInt32LE(rate, 24);
  header.writeUInt32LE(rate * 2, 28);
  header.writeUInt16LE(2, 32);
  header.writeUInt16LE(16, 34);
  header.write("data", 36);
  header.writeUInt32LE(data.length, 40);
  return Buffer.concat([header, data]);
}

const sounds = {
  countdown() {
    const b = silence(0.18);
    tone(b, 0, 660, 0.1, 0.25, 8, true);
    return b;
  },
  start() {
    const b = silence(0.55);
    [392, 523.25, 659.25, 783.99].forEach((f, i) => tone(b, i * 0.08, f, 0.22, 0.26, 5));
    return b;
  },
  place() {
    const b = silence(0.14);
    glide(b, 0, 260, 620, 0.1, 0.3, 8);
    noise(b, 0.02, 0.05, 0.06, 18, 3);
    return b;
  },
  explode() {
    const b = silence(0.5);
    noise(b, 0, 0.42, 0.5, 7, 11);
    tone(b, 0, 120, 0.3, 0.32, 7);
    glide(b, 0.02, 500, 90, 0.35, 0.18, 5);
    return b;
  },
  item() {
    const b = silence(0.28);
    [880, 1174.66, 1567.98].forEach((f, i) => tone(b, i * 0.055, f, 0.16, 0.22, 7));
    return b;
  },
  shield() {
    const b = silence(0.35);
    [523.25, 783.99, 1046.5].forEach((f, i) => tone(b, i * 0.07, f, 0.25, 0.2, 4));
    return b;
  },
  eliminate() {
    const b = silence(0.42);
    glide(b, 0, 520, 90, 0.32, 0.3, 4);
    noise(b, 0.08, 0.2, 0.12, 10, 19);
    return b;
  },
  sudden() {
    const b = silence(0.75);
    [0, 0.2, 0.4, 0.6].forEach((t, i) =>
      tone(b, t, i % 2 ? 440 : 330, 0.16, 0.3, 5, true),
    );
    return b;
  },
  win() {
    const b = silence(0.9);
    [523.25, 659.25, 783.99, 1046.5, 1318.5].forEach((f, i) =>
      tone(b, i * 0.09, f, 0.4, 0.24, 4),
    );
    return b;
  },
  lose() {
    const b = silence(0.8);
    [392, 349.23, 293.66, 220].forEach((f, i) => tone(b, i * 0.12, f, 0.32, 0.24, 4));
    return b;
  },
  battle() {
    const b = silence(8);
    const chords = [
      [261.63, 329.63, 392],
      [293.66, 349.23, 440],
      [220, 261.63, 329.63],
      [246.94, 293.66, 349.23],
    ];
    for (let bar = 0; bar < 8; bar++) {
      const chord = chords[bar % chords.length];
      for (let note = 0; note < 8; note++) {
        const f = chord[note % 3] * (note % 4 === 3 ? 2 : 1);
        tone(b, bar + note * 0.125, f, 0.11, 0.075, 8, note % 2 === 0);
      }
      tone(b, bar, chord[0] / 2, 0.4, 0.05, 3);
    }
    return b;
  },
};

mkdirSync(output, { recursive: true });
const entries = [];
for (const [name, make] of Object.entries(sounds)) {
  const data = wav(normalize(make()));
  const file = `${name}.wav`;
  writeFileSync(path.join(output, file), data);
  entries.push({
    file,
    urlPath: `/bubbleparty/v1/audio/${file}`,
    output: `games/bubbleparty/assets/public/bubbleparty/v1/audio/${file}`,
    sha256: createHash("sha256").update(data).digest("hex"),
    bytes: data.length,
  });
}
const generator = createHash("sha256").update(readFileSync(fileURLToPath(import.meta.url))).digest("hex");
writeFileSync(
  manifestPath,
  JSON.stringify(
    {
      version: "bubbleparty/v1",
      prefix: "bga/v20261002-1/bubbleparty/v1/",
      generator: "scripts/bubbleparty/prepare-audio.mjs",
      generatorSha256: generator,
      license: "项目原创，离线合成，无外部采样",
      entries,
    },
    null,
    2,
  ) + "\n",
);
console.log(`Generated ${entries.length} original WAV files in ${output}`);
