// Failure inventory before implementation:
// - music stays fixed at 15%, or effects/loops ignore the shared volume;
// - live changes restart playback, seek, or lose a loop's relative volume;
// - new/fallback/resumed media use stale volume, or a ramp restores old gain;
// - zero volume leaves queued/pending sound able to revive; raising volume or
//   unmuting replays cancelled effects; changing volume bypasses explicit mute;
// - nonfinite/out-of-range values reach native setters;
// - missing, corrupt or unavailable browser storage breaks initial rendering.
import { test } from "node:test";
import assert from "node:assert/strict";
import { GameAudio } from "../src/audio.ts";
class Media {
  dataset = {};
  src = "";
  muted = false;
  paused = true;
  currentTime = 0;
  volume = 1;
  playbackRate = 1;
  calls = [];
  behavior = () => Promise.resolve();
  play() {
    this.paused = false;
    this.calls.push(this.src);
    return this.behavior();
  }
  pause() {
    this.paused = true;
  }
  load() {
    this.currentTime = 0;
  }
  removeAttribute(name) {
    if (name === "src") this.src = "";
  }
  remove() {}
}
function setup() {
  const voices = [];
  const audio = new GameAudio(() => {
    const m = new Media();
    voices.push(m);
    return m;
  });
  return { audio, voices, port: audio.channel("room") };
}
test("one volume controls effects, music and relative loop levels without restarting", async () => {
  const { audio, voices, port } = setup();
  audio.setVolume(0.5);
  await port.play(["effect.wav"]);
  await port.music(["music.wav"]);
  await port.loop("engine", ["engine.wav"], { volume: 0.4 });
  assert.deepEqual(
    voices.map((m) => m.volume),
    [0.5, 0.5, 0.2],
  );
  for (const m of voices) m.currentTime = 4;
  const calls = voices.map((m) => m.calls.length);
  audio.setVolume(0.8);
  assert.deepEqual(
    voices.map((m) => m.volume),
    [0.8, 0.8, 0.4 * 0.8],
  );
  assert.deepEqual(
    voices.map((m) => m.currentTime),
    [4, 4, 4],
  );
  assert.deepEqual(
    voices.map((m) => m.calls.length),
    calls,
  );
  await port.loop("engine", ["engine.wav"], { volume: 0.25 });
  assert.equal(voices[2].volume, 0.2);
  await port.music(["next.wav"]);
  assert.equal(voices[1].volume, 0.8);
  audio.dispose();
});
test("zero volume cancels playback and queued clips without later replay", async () => {
  const { audio, voices, port } = setup();
  await port.play(["one.wav", "two.wav"]);
  await port.music(["music.wav"]);
  await port.loop("engine", ["engine.wav"]);
  const ended = voices[0].onended;
  audio.setVolume(0);
  ended?.();
  assert.ok(voices.every((m) => m.paused));
  assert.equal(await audio.play("preview"), false);
  const calls = voices.map((m) => m.calls.length);
  audio.setVolume(0.6);
  assert.deepEqual(
    voices.map((m) => m.calls.length),
    calls,
  );
  assert.ok(voices.every((m) => m.paused));
  await port.play(["fresh.wav"]);
  assert.equal(voices[0].volume, 0.6);
  audio.dispose();
});
test("volume changes cannot bypass explicit mute or revive pending requests", async () => {
  const { audio, voices, port } = setup();
  audio.mount();
  let finish;
  voices[0].behavior = () => new Promise((resolve) => (finish = resolve));
  const playing = port.play(["pending.wav"]);
  audio.setVolume(0);
  audio.setVolume(0.7);
  finish();
  assert.equal(await playing, false);
  assert.equal(voices[0].paused, true);
  audio.setEnabled(false);
  audio.setVolume(1);
  assert.equal(await audio.play("preview"), false);
  assert.equal(audio.enabled, false);
  audio.dispose();
});
test("invalid volume is ignored and finite values are clamped", () => {
  const { audio, voices } = setup();
  audio.mount();
  audio.setVolume(0.3);
  for (const v of [NaN, Infinity, -Infinity]) {
    audio.setVolume(v);
    assert.equal(voices[0].volume, 0.3);
  }
  audio.setVolume(2);
  assert.equal(voices[0].volume, 1);
  audio.setVolume(-1);
  assert.equal(voices[0].volume, 0);
  audio.dispose();
});
test("an in-flight loop ramp follows the new master volume", async () => {
  const old = globalThis.requestAnimationFrame,
    oldCancel = globalThis.cancelAnimationFrame;
  let id = 0,
    queue = new Map();
  globalThis.requestAnimationFrame = (fn) => {
    queue.set(++id, fn);
    return id;
  };
  globalThis.cancelAnimationFrame = (n) => queue.delete(n);
  try {
    const { audio, voices, port } = setup();
    await port.loop("engine", ["engine.wav"], { volume: 0.2 });
    await port.loop("engine", ["engine.wav"], { volume: 0.8 });
    audio.setVolume(0.25);
    for (let t = 0; t < 1000; t += 16) {
      const callbacks = [...queue.values()];
      queue.clear();
      callbacks.forEach((fn) => fn(t));
    }
    assert.equal(voices[0].volume, 0.2);
    audio.dispose();
    assert.equal(queue.size, 0);
  } finally {
    globalThis.requestAnimationFrame = old;
    globalThis.cancelAnimationFrame = oldCancel;
  }
});
