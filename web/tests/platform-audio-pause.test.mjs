// Failure inventory before implementation: pause resets media position; resume
// restarts a URL or loops finite music; owner leakage; queue advances while
// paused; in-flight play resurrects sound; pause-aborted play triggers fallback;
// stop/mute/hidden/leave/error allow old resume; repeated resume duplicates play;
// an old resume cancels a new owner's voice; tuning loops resumes paused media.
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
  loop = false;
  onended = null;
  onerror = null;
  calls = [];
  loads = 0;
  removed = false;
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
    this.loads++;
    this.currentTime = 0;
  }
  removeAttribute(name) {
    if (name === "src") this.src = "";
  }
  remove() {
    this.removed = true;
  }
}
function setup() {
  const voices = [];
  const audio = new GameAudio(() => {
    const m = new Media();
    voices.push(m);
    return m;
  });
  audio.mount();
  return { audio, voices, port: audio.channel("basketball") };
}
const flush = async () => {
  for (let i = 0; i < 8; i++) await Promise.resolve();
};
test("owner pause resumes effects, finite music and loops at the same media positions", async () => {
  const { audio, voices, port } = setup();
  await port.play(["effect", "next"]);
  await port.music(["music"], { loop: false });
  await port.loop("engine", ["engine"]);
  voices.forEach((m, i) => (m.currentTime = 4 + i));
  const loads = voices.map((m) => m.loads);
  port.pause();
  port.pause();
  assert.ok(voices.every((m) => m.paused));
  assert.deepEqual(
    voices.map((m) => m.currentTime),
    [4, 5, 6],
  );
  assert.equal(voices[1].loop, false);
  assert.equal(await port.resume(), true);
  assert.deepEqual(
    voices.map((m) => m.loads),
    loads,
  );
  assert.deepEqual(
    voices.map((m) => m.currentTime),
    [4, 5, 6],
  );
  assert.ok(voices.every((m) => !m.paused));
  assert.equal(await port.resume(), false);
  assert.ok(voices.every((m) => m.calls.length === 2));
  audio.dispose();
});
test("paused queues and repeated music/loop updates never restart media", async () => {
  const { port, voices } = setup();
  await port.play(["first", "second"]);
  await port.music(["music"]);
  await port.loop("engine", ["engine"]);
  port.pause();
  const calls = voices.map((m) => m.calls.length);
  voices[0].onended();
  await flush();
  assert.equal(await port.music(["music"]), false);
  assert.equal(await port.loop("engine", ["engine"], { rate: 2 }), false);
  assert.equal(await port.play(["new"]), false);
  assert.deepEqual(
    voices.map((m) => m.calls.length),
    calls,
  );
  assert.equal(voices[1].loop, true);
  await port.resume();
  voices[0].onended();
  await flush();
  assert.equal(voices[0].src, "second");
});
test("pause is owner scoped and an old owner cannot resume newer media", async () => {
  const { audio, port, voices } = setup();
  await port.play(["old"]);
  await port.music(["old-music"]);
  port.pause();
  const newer = audio.channel("new");
  await newer.play(["new"]);
  await newer.music(["new-music"]);
  port.pause();
  assert.ok(voices.every((m) => !m.paused));
  assert.equal(await port.resume(), false);
  assert.equal(voices[0].src, "new");
  assert.equal(voices[1].src, "new-music");
});
for (const cancel of [
  "stop",
  "mute",
  "hidden",
  "dispose",
  "music-stop",
  "loop-stop",
])
  test(`${cancel} prevents stale resume`, async () => {
    const { audio, port, voices } = setup();
    if (cancel === "music-stop") await port.music(["music"]);
    else if (cancel === "loop-stop") await port.loop("engine", ["engine"]);
    else {
      await port.play(["effect"]);
      await port.music(["music"]);
      await port.loop("engine", ["engine"]);
    }
    port.pause();
    if (cancel === "mute") {
      audio.setEnabled(false);
      audio.setEnabled(true);
    } else if (cancel === "music-stop") port.stopMusic();
    else if (cancel === "loop-stop") port.stopLoop("engine");
    else if (cancel === "dispose") audio.dispose();
    else audio.stop();
    const count = voices.map((m) => m.calls.length);
    assert.equal(await port.resume(), false);
    assert.deepEqual(
      voices.map((m) => m.calls.length),
      count,
    );
    assert.ok(voices.every((m) => m.paused));
  });
test("late start resolution stays paused and resume does not replace its source", async () => {
  const { port, voices } = setup();
  let finish;
  voices[0].behavior = () =>
    new Promise(
      (r) =>
        (finish = () => {
          voices[0].paused = false;
          r();
        }),
    );
  const pending = port.playWithFallback(["cdn", "local"]);
  port.pause();
  finish();
  assert.equal(await pending, false);
  assert.equal(voices[0].paused, true);
  assert.equal(voices[0].src, "cdn");
  voices[0].behavior = () => Promise.resolve();
  assert.equal(await port.resume(), true);
  assert.deepEqual(voices[0].calls, ["cdn", "cdn"]);
});
test("pause rejection preserves clip instead of trying its fallback", async () => {
  const { port, voices } = setup();
  let fail;
  voices[0].behavior = () => new Promise((_, r) => (fail = r));
  const pending = port.playWithFallback(["cdn", "local"]);
  port.pause();
  fail(Error("AbortError"));
  assert.equal(await pending, false);
  assert.deepEqual(voices[0].calls, ["cdn"]);
  voices[0].behavior = () => Promise.resolve();
  assert.equal(await port.resume(), true);
});
test("late resume after cancellation or a second pause cannot revive playback", async () => {
  for (const cancel of [false, true]) {
    const { port, voices } = setup();
    await port.play(["effect"]);
    port.pause();
    let finish;
    voices[0].behavior = () =>
      new Promise(
        (r) =>
          (finish = () => {
            voices[0].paused = false;
            r();
          }),
      );
    const pending = port.resume();
    if (cancel) port.stop();
    else port.pause();
    finish();
    assert.equal(await pending, false);
    assert.equal(voices[0].paused, true);
  }
});
test("a denied resume cancels old audio and returns false", async () => {
  const { port, voices } = setup();
  await port.play(["effect"]);
  port.pause();
  voices[0].behavior = () => Promise.reject(Error("denied"));
  assert.equal(await port.resume(), false);
  assert.equal(voices[0].paused, true);
  voices[0].behavior = () => Promise.resolve();
  assert.equal(await port.resume(), false);
});
test("an aborted initial play cannot fall back after immediate pause/resume", async () => {
  const { port, voices } = setup();
  let rejectFirst;
  let calls = 0;
  voices[0].behavior = () =>
    ++calls === 1
      ? new Promise((_, r) => (rejectFirst = r))
      : Promise.resolve();
  const initial = port.playWithFallback(["cdn", "local"]);
  port.pause();
  assert.equal(await port.resume(), true);
  rejectFirst(Error("AbortError"));
  assert.equal(await initial, false);
  assert.deepEqual(voices[0].calls, ["cdn", "cdn"]);
  assert.equal(voices[0].paused, false);
  assert.equal(voices[0].src, "cdn");
});
