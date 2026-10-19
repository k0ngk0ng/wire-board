#!/usr/bin/env node
// Deterministic, original WAV sound bank for the fortressblue module.
// No external samples. Run from the repository root:
//   node scripts/fortressblue/prepare-audio.mjs
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const output = path.join(
  root,
  "games/fortressblue/assets/public/fortressblue/v1/audio",
);
const manifestPath = path.join(
  root,
  "games/fortressblue/assets/manifest.json",
);
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
function addTone(buffer, start, frequency, seconds, volume, decay = 4, square = false) {
  const from = Math.round(start * rate);
  const length = Math.round(seconds * rate);
  for (let i = 0; i < length && from + i < buffer.length; i++) {
    const t = i / rate;
    const envelope = Math.exp(-decay * t) * Math.min(1, t / 0.005);
    const phase = Math.sin(2 * Math.PI * frequency * t);
    buffer[from + i] += (square ? Math.sign(phase) * 0.55 : phase) * volume * envelope;
  }
}
function addNoise(buffer, start, seconds, volume, decay = 8, seed = 7) {
  const random = rng(seed);
  const from = Math.round(start * rate);
  const length = Math.round(seconds * rate);
  for (let i = 0; i < length && from + i < buffer.length; i++) {
    const t = i / rate;
    const envelope = Math.exp(-decay * t) * Math.min(1, t / 0.004);
    buffer[from + i] += (random() * 2 - 1) * volume * envelope;
  }
}
function normalize(buffer) {
  let peak = 0;
  for (const value of buffer) peak = Math.max(peak, Math.abs(value));
  if (peak > 0.8) for (let i = 0; i < buffer.length; i++) buffer[i] *= 0.8 / peak;
  const edge = rate * 0.005;
  for (let i = 0; i < buffer.length; i++) {
    buffer[i] *= Math.min(1, Math.min(i, buffer.length - 1 - i) / edge);
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
  battle() {
    const b = silence(8);
    const bass = [110, 110, 146.83, 130.81, 98, 98, 123.47, 130.81];
    const lead = [329.63, 392, 440, 392, 349.23, 392, 293.66, 261.63];
    for (let bar = 0; bar < 4; bar++) {
      for (let i = 0; i < 8; i++) {
        const t = bar * 2 + i * 0.25;
        addTone(b, t, bass[i], 0.3, 0.10, 3, true);
        if (i % 2 === 0) addTone(b, t, lead[i], 0.22, 0.06, 4);
      }
    }
    addNoise(b, 0, 8, 0.006, 0, 21);
    return normalize(b);
  },
  shot() {
    const b = silence(0.2);
    addTone(b, 0, 170, 0.16, 0.5, 8);
    addNoise(b, 0, 0.12, 0.28, 12, 4);
    return normalize(b);
  },
  explode() {
    const b = silence(0.55);
    addNoise(b, 0, 0.45, 0.55, 7, 11);
    addTone(b, 0, 70, 0.4, 0.4, 6);
    addTone(b, 0.03, 120, 0.25, 0.2, 9);
    return normalize(b);
  },
  hit() {
    const b = silence(0.18);
    addTone(b, 0, 330, 0.13, 0.35, 12);
    addNoise(b, 0, 0.08, 0.18, 18, 8);
    return normalize(b);
  },
  death() {
    const b = silence(0.6);
    [220, 174.61, 130.81, 98].forEach((f, i) =>
      addTone(b, i * 0.1, f, 0.22, 0.3, 7),
    );
    addNoise(b, 0.05, 0.4, 0.16, 5, 17);
    return normalize(b);
  },
  select() {
    const b = silence(0.18);
    addTone(b, 0, 523.25, 0.12, 0.3, 8);
    addTone(b, 0.06, 659.25, 0.1, 0.24, 9);
    return normalize(b);
  },
  turn() {
    const b = silence(0.28);
    addTone(b, 0, 392, 0.12, 0.28, 7);
    addTone(b, 0.1, 587.33, 0.16, 0.3, 7);
    return normalize(b);
  },
  sudden() {
    const b = silence(0.55);
    addTone(b, 0, 110, 0.5, 0.35, 3, true);
    addTone(b, 0.12, 220, 0.35, 0.22, 5, true);
    addNoise(b, 0.1, 0.35, 0.18, 5, 29);
    return normalize(b);
  },
  win() {
    const b = silence(0.9);
    [392, 523.25, 659.25, 783.99].forEach((f, i) =>
      addTone(b, i * 0.13, f, 0.35, 0.3, 4),
    );
    return normalize(b);
  },
  lose() {
    const b = silence(0.9);
    [329.63, 293.66, 246.94, 196].forEach((f, i) =>
      addTone(b, i * 0.14, f, 0.34, 0.28, 4),
    );
    return normalize(b);
  },
};

mkdirSync(output, { recursive: true });
const entries = [];
for (const [name, build] of Object.entries(sounds)) {
  const data = wav(build());
  const file = path.join(output, name + ".wav");
  writeFileSync(file, data);
  entries.push({
    file: name + ".wav",
    urlPath: "/fortressblue/v1/audio/" + name + ".wav",
    output: "games/fortressblue/assets/public/fortressblue/v1/audio/" + name + ".wav",
    sha256: createHash("sha256").update(data).digest("hex"),
    bytes: data.length,
  });
}
writeFileSync(
  manifestPath,
  JSON.stringify(
    {
      version: "fortressblue/v1",
      prefix: "bga/v20261002-1/fortressblue/v1/",
      generator: "scripts/fortressblue/prepare-audio.mjs",
      generatorSha256: createHash("sha256")
        .update(readFileSync(fileURLToPath(import.meta.url)))
        .digest("hex"),
      license: "项目原创，离线合成，无外部采样",
      entries,
    },
    null,
    2,
  ) + "\n",
);
console.log(`wrote ${entries.length} original sounds to ${output}`);
