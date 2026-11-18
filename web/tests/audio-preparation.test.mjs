import test from "node:test";
import assert from "node:assert/strict";
import { GameAudio } from "../src/audio.ts";

class Media extends EventTarget {
  dataset = {};
  src = "";
  duration = NaN;
  readyState = 0;
  muted = false;
  volume = 1;
  paused = true;
  removed = false;
  playCalls = 0;
  loads = 0;
  play() { this.playCalls++; return Promise.resolve(); }
  pause() { this.paused = true; }
  load() { this.loads++; }
  removeAttribute(name) { if (name === "src") this.src = ""; }
  remove() { this.removed = true; }
  decoded() { this.duration = 10; this.readyState = 3; this.dispatchEvent(new Event("canplay")); }
}
function setup() {
  const media = [], audio = new GameAudio(() => { const value = new Media(); media.push(value); return value; });
  return { audio, media, a: audio.channel("one"), b: audio.channel("two") };
}
const drain = async () => { for (let i=0;i<8;i++) await Promise.resolve(); };

test("media preparation waits for a decoded native source and never plays", async () => {
  const { audio, media, a } = setup();
  let done = false;
  const ready = a.prepare(["sound.flac"]).then(value => { done = true; return value; });
  assert.equal(media.length, 1);
  assert.equal(media[0].preload, "auto");
  assert.equal(media[0].muted, true);
  media[0].readyState = 1;
  media[0].duration = 10;
  media[0].dispatchEvent(new Event("loadedmetadata"));
  await drain();
  assert.equal(done, false);
  media[0].decoded();
  assert.equal(await ready, true);
  assert.equal(media[0].playCalls, 0);
  assert.equal(media[0].removed, true);
  audio.dispose();
});

test("global mute permits silent preparation while an owner stop cancels only its own work", async () => {
  const { audio, media, a, b } = setup();
  const first = a.prepare(["one.flac"]), second = b.prepare(["two.flac"]);
  audio.setEnabled(false);
  assert.equal(media[0].removed, false);
  a.stop();
  assert.equal(await first, false);
  assert.equal(media[0].removed, true);
  assert.equal(media[1].removed, false);
  media[1].decoded();
  assert.equal(await second, true);
  assert.ok(media.every(value=>value.playCalls===0));
  audio.dispose();
});

test("preparation fallback, abort and dispose release pending native elements without stale completion", async () => {
  const { audio, media, a } = setup();
  const fallback = a.prepare(["bad.flac", "good.ogg"]);
  media[0].dispatchEvent(new Event("error"));
  await drain();
  assert.equal(media.length, 2);
  media[1].decoded();
  assert.equal(await fallback, true);
  const abort = new AbortController(), pending = a.prepare(["slow.flac", "never.flac"], { signal: abort.signal });
  abort.abort();
  assert.equal(await pending, false);
  assert.equal(media.length, 3);
  media[2].decoded();
  const disposed = a.prepare(["last.flac"]);
  audio.dispose();
  assert.equal(await disposed, false);
  assert.ok(media.every(value=>value.removed&&value.playCalls===0));
});
