#!/usr/bin/env node
// Deterministic original WAV sound effects for the tetris module. No external
// samples are used. Run from the repository root:
//   node scripts/tetris/prepare-audio.mjs
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const output = path.join(root, "games/tetris/assets/public/tetris/v1/audio");
const manifestPath = path.join(root, "games/tetris/assets/manifest.json");
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
    const envelope = Math.exp(-decay * t) * Math.min(1, t / 0.004);
    const phase = Math.sin(2 * Math.PI * frequency * t);
    const wave = square ? Math.sign(phase) * 0.55 : phase;
    buffer[from + i] += wave * volume * envelope;
  }
}

function addNoise(buffer, start, seconds, volume, decay = 8, seed = 7) {
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
  if (peak > 0.82) {
    for (let i = 0; i < buffer.length; i++) buffer[i] *= 0.82 / peak;
  }
  for (let i = 0; i < buffer.length; i++) {
    const edge = Math.min(i, buffer.length - 1 - i) / (rate * 0.004);
    buffer[i] *= Math.min(1, edge);
  }
  return buffer;
}

function wav(buffer) {
  const data = Buffer.alloc(buffer.length * 2);
  for (let i = 0; i < buffer.length; i++) {
    const value = Math.max(-1, Math.min(1, buffer[i]));
    data.writeInt16LE(Math.round(value * 32767), i * 2);
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
  start() {
    const b = silence(0.46);
    addTone(b, 0.0, 392, 0.14, 0.34, 5);
    addTone(b, 0.1, 523.25, 0.14, 0.34, 5);
    addTone(b, 0.2, 659.25, 0.24, 0.38, 4);
    return b;
  },
  lock() {
    const b = silence(0.1);
    addTone(b, 0, 128, 0.09, 0.52, 18);
    addNoise(b, 0, 0.05, 0.16, 22, 3);
    return b;
  },
  clear() {
    const b = silence(0.22);
    addTone(b, 0, 660, 0.1, 0.3, 8);
    addTone(b, 0.07, 880, 0.13, 0.32, 7);
    return b;
  },
  tetris() {
    const b = silence(0.46);
    [523.25, 659.25, 783.99, 1046.5].forEach((f, i) =>
      addTone(b, i * 0.075, f, 0.2, 0.3, 6, true),
    );
    return b;
  },
  garbage() {
    const b = silence(0.3);
    addTone(b, 0, 92, 0.28, 0.36, 7);
    addNoise(b, 0.02, 0.22, 0.18, 9, 11);
    return b;
  },
  level() {
    const b = silence(0.28);
    addTone(b, 0, 587.33, 0.12, 0.3, 7);
    addTone(b, 0.11, 880, 0.16, 0.32, 6);
    return b;
  },
  eliminate() {
    const b = silence(0.52);
    [392, 329.63, 261.63, 196].forEach((f, i) =>
      addTone(b, i * 0.1, f, 0.22, 0.3, 6),
    );
    return b;
  },
  win() {
    const b = silence(0.72);
    [523.25, 659.25, 783.99, 1046.5, 1318.5].forEach((f, i) =>
      addTone(b, i * 0.1, f, 0.3, 0.28, 5, i % 2 === 0),
    );
    return b;
  },
  finish() {
    const b = silence(0.5);
    addTone(b, 0, 392, 0.4, 0.25, 4);
    addTone(b, 0, 493.88, 0.4, 0.22, 4);
    addTone(b, 0, 587.33, 0.4, 0.2, 4);
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
    urlPath: `/tetris/v1/audio/${file}`,
    output: `games/tetris/assets/public/tetris/v1/audio/${file}`,
    sha256: createHash("sha256").update(data).digest("hex"),
    bytes: data.length,
  });
}
const generator = "scripts/tetris/prepare-audio.mjs";
const generatorSha256 = createHash("sha256")
  .update(readFileSync(path.join(root, generator)))
  .digest("hex");
const manifest = {
  version: "tetris/v1",
  prefix: "bga/v20261002-1/tetris/v1/",
  generator,
  generatorSha256,
  license: "项目原创，离线合成，无外部采样",
  entries,
};
writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");
console.log(`wrote ${entries.length} WAV files and ${path.relative(root, manifestPath)}`);
