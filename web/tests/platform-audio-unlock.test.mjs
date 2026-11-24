import test from "node:test";
import assert from "node:assert/strict";
import { GameAudio } from "../src/audio.ts";

class Media extends EventTarget {
  dataset = {};
  src = "";
  paused = true;
  muted = false;
  calls = 0;
  loads = 0;
  removed = false;
  behavior = () => Promise.resolve();
  play() {
    this.calls++;
    this.paused = false;
    return this.behavior();
  }
  pause() {
    this.paused = true;
  }
  load() {
    this.loads++;
  }
  removeAttribute(name) {
    if (name === "src") this.src = "";
  }
  remove() {
    this.removed = true;
  }
}
const setup = (t) => {
  const media = new Media();
  const audio = new GameAudio(() => media);
  t.after(() => audio.dispose());
  return { media, audio };
};

test("rapid touches and keys share one pending native unlock", async (t) => {
  const { media, audio } = setup(t);
  let finish;
  media.behavior = () =>
    new Promise((resolve) => {
      finish = resolve;
    });
  const requests = Array.from({ length: 40 }, () => audio.unlock());
  assert.equal(media.calls, 1);
  assert.equal(media.loads, 0);
  finish();
  assert.deepEqual(await Promise.all(requests), Array(40).fill(true));
  assert.equal(await audio.unlock(), true);
  assert.equal(media.calls, 1);
});

test("native decode error settles an otherwise hanging play and permits a new gesture", async (t) => {
  const { media, audio } = setup(t);
  media.behavior = () => new Promise(() => {});
  const request = audio.unlock();
  media.dispatchEvent(new Event("error"));
  const result = await Promise.race([
    request,
    new Promise((resolve) => setTimeout(() => resolve("hung"), 50)),
  ]);
  assert.equal(result, false);
  media.behavior = () => Promise.resolve();
  assert.equal(await audio.unlock(), true);
  assert.equal(media.calls, 2);
});

test("a denied unlock retries only on the next call", async (t) => {
  const { media, audio } = setup(t);
  media.behavior = () =>
    Promise.reject(new DOMException("gesture required", "NotAllowedError"));
  assert.equal(await audio.unlock(), false);
  assert.equal(media.calls, 1);
  await Promise.resolve();
  assert.equal(media.calls, 1);
  media.behavior = () => Promise.resolve();
  assert.equal(await audio.unlock(), true);
  assert.equal(media.calls, 2);
});

test("mute cancels a pending unlock without waiting for native play", async (t) => {
  const { media, audio } = setup(t);
  let finish;
  media.behavior = () =>
    new Promise((resolve) => {
      finish = resolve;
    });
  const stale = audio.unlock();
  audio.setEnabled(false);
  const result = await Promise.race([
    stale,
    new Promise((resolve) => setTimeout(() => resolve("hung"), 50)),
  ]);
  assert.equal(result, false);
  assert.equal(await audio.unlock(), false);
  audio.setEnabled(true);
  media.behavior = () => Promise.resolve();
  assert.equal(await audio.unlock(), true);
  const calls = media.calls;
  finish();
  await stale;
  assert.equal(await audio.unlock(), true);
  assert.equal(media.calls, calls);
});

test("cancelled unlock cannot clear a newer game source", async (t) => {
  const { media, audio } = setup(t);
  let finish;
  media.behavior = () =>
    new Promise((resolve) => {
      finish = resolve;
    });
  const stale = audio.unlock();
  media.behavior = () => Promise.resolve();
  assert.equal(await audio.channel("room").play(["game.wav"]), true);
  finish();
  assert.equal(await stale, false);
  assert.equal(media.src, "game.wav");
  assert.equal(media.paused, false);
});

test("a pending unlock has a bounded deadline", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const { media, audio } = setup(t);
  media.behavior = () => new Promise(() => {});
  const request = audio.unlock();
  t.mock.timers.tick(2000);
  let settled = false;
  void request.then((value) => {
    settled = value === false;
  });
  for (let at = 0; at < 5; at++) await Promise.resolve();
  assert.equal(settled, true);
});
