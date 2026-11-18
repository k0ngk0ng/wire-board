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
    this.calls.push({ src: this.src, volume: this.volume });
    return this.behavior();
  }
  pause() { this.paused = true; }
  load() { this.currentTime = 0; }
  removeAttribute(name) { if (name === "src") this.src = ""; }
  remove() {}
}

function setup() {
  const voices = [];
  const audio = new GameAudio(() => {
    const media = new Media();
    voices.push(media);
    return media;
  });
  return { audio, voices, port: audio.channel("room") };
}

function near(actual, expected) {
  assert.ok(Math.abs(actual - expected) < 1e-9, `${actual} !== ${expected}`);
}

test("optional per-call volume preserves defaults and composes with shared volume", async () => {
  const { audio, voices, port } = setup();
  audio.setVolume(0.5);
  assert.equal(await port.play(["default.wav"]), true);
  assert.equal(await port.music(["default.mp3"]), true);
  assert.deepEqual(voices.map(media => media.volume), [0.5, 0.5]);
  await port.play(["quiet.wav"], { volume: 0.25 });
  await port.music(["quiet.mp3"], { volume: 0.6 });
  near(voices[0].volume, 0.125);
  near(voices[1].volume, 0.3);
  const calls = voices.map(media => media.calls.length);
  audio.setVolume(0.7);
  near(voices[0].volume, 0.175);
  near(voices[1].volume, 0.42);
  assert.deepEqual(voices.map(media => media.calls.length), calls);
  audio.dispose();
});

test("playlist transitions, fallback and owner resume retain the requested volume", async () => {
  const { audio, voices, port } = setup();
  await port.play(["one.wav", "two.wav"], { volume: 0.25 });
  const media = voices[0];
  media.onended();
  await Promise.resolve();
  assert.deepEqual(media.calls.map(call => call.volume), [0.25, 0.25]);
  audio.setVolume(0.8);
  port.pause();
  await port.resume();
  near(media.volume, 0.2);
  assert.equal(media.calls.at(-1).volume, 0.2);
  await port.music(["cdn.mp3", "local.mp3"], { playbackID: "fallback", volume: 0.3 });
  const music = voices[1];
  music.behavior = () => music.src === "next-cdn.mp3" ? Promise.reject(Error("network")) : Promise.resolve();
  await port.music(["next-cdn.mp3", "next-local.mp3"], { playbackID: "next", volume: 0.6 });
  near(music.calls.at(-2).volume, 0.48);
  near(music.calls.at(-1).volume, 0.48);
  await port.play(["default-again.wav"]);
  assert.equal(media.volume, 0.8);
  audio.dispose();
});

test("a stable music identity updates gain without restarting or seeking", async () => {
  const { audio, voices, port } = setup();
  await port.music(["route.mp3"], { playbackID: "route-1", startOffset: 3, volume: 0.2 });
  const music = voices[0];
  music.currentTime = 7;
  assert.equal(await port.music(["route.mp3"], { playbackID: "route-1", startOffset: 0, volume: 0.8 }), true);
  assert.equal(music.volume, 0.8);
  assert.equal(music.currentTime, 7);
  assert.equal(music.calls.length, 1);
  await port.music(["route.mp3"], { playbackID: "route-1", volume: 0 });
  assert.equal(music.volume, 0);
  assert.equal(music.paused, false);
  await port.music(["route.mp3"], { playbackID: "route-1", volume: 0.6 });
  assert.equal(music.volume, 0.6);
  assert.equal(music.calls.length, 1);
  audio.dispose();
});

test("a superseded pending effect cannot replace the current owner's gain", async () => {
  const { audio, voices } = setup();
  audio.mount();
  const media = voices[0];
  let finish;
  media.behavior = () => media.src === "pending.wav" ? new Promise(resolve => { finish = resolve; }) : Promise.resolve();
  const previous = audio.channel("old").play(["pending.wav"], { volume: 0.2 });
  const active = audio.channel("new");
  assert.equal(await active.play(["current.wav"], { volume: 0.7 }), true);
  finish();
  assert.equal(await previous, false);
  assert.equal(media.src, "current.wav");
  assert.equal(media.volume, 0.7);
  audio.channel("old").stop();
  assert.equal(media.paused, false);
  audio.dispose();
});

test("global mute stops quiet requests and never replays them on unmute", async () => {
  const { audio, voices, port } = setup();
  await port.play(["one.wav", "queued.wav"], { volume: 0.2 });
  await port.music(["music.mp3"], { playbackID: "route-1", volume: 0.3 });
  const ended = voices[0].onended;
  audio.setEnabled(false);
  assert.ok(voices.every(media => media.paused && media.muted));
  const calls = voices.map(media => media.calls.length);
  ended();
  audio.setEnabled(true);
  assert.deepEqual(voices.map(media => media.calls.length), calls);
  assert.ok(voices.every(media => media.paused));
  await port.play(["fresh.wav"], { volume: 0.4 });
  assert.equal(voices[0].volume, 0.4);
  port.stop();
  assert.ok(voices.every(media => media.paused));
  audio.dispose();
});

test("finite gain is clamped and nonfinite requests cannot alter live output", async () => {
  const { audio, voices, port } = setup();
  await port.play(["current.wav"], { volume: 0.4 });
  await port.music(["current.mp3"], { playbackID: "current", volume: 0.3 });
  const calls = voices.map(media => media.calls.length);
  for (const volume of [NaN, Infinity, -Infinity]) {
    assert.equal(await port.play(["invalid.wav"], { volume }), false);
    assert.equal(await port.music(["invalid.mp3"], { volume }), false);
  }
  assert.deepEqual(voices.map(media => media.calls.length), calls);
  assert.deepEqual(voices.map(media => media.volume), [0.4, 0.3]);
  await port.play(["loud.wav"], { volume: 8 });
  assert.equal(voices[0].volume, 1);
  await port.music(["silent.mp3"], { volume: -8 });
  assert.equal(voices[1].volume, 0);
  assert.equal(voices[1].paused, false);
  audio.dispose();
});
