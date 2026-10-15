// Loop failure inventory, written before implementation:
// - speed updates restart an engine, introduce audible jumps or preserve pitch;
// - engine/skid/effect/music share a voice and interrupt each other;
// - mute, hidden-page/room cleanup or disposal leave a native loop or ramp alive;
// - unmute or stale async play/fallback resurrects cancelled/previous-room audio;
// - stopping an old owner/key interrupts another active owner/key;
// - CDN failure, late media error or exhausted alternatives throws/leaks a voice;
// - invalid control values reach native setters; pending repeated calls duplicate
//   media; replacing sources fails to release the previous voice;
// - lack of requestAnimationFrame or prefixed pitch controls breaks playback.
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
  preservesPitch = true;
  mozPreservesPitch = true;
  webkitPreservesPitch = true;
  preload = "";
  onended = null;
  onerror = null;
  calls = [];
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
    this.currentTime = 0;
  }
  removeAttribute(name) {
    if (name === "src") this.src = "";
  }
  remove() {
    this.removed = true;
  }
}
function setup(configure = () => {}) {
  const voices = [];
  const audio = new GameAudio(() => {
    const media = new Media();
    configure(media, voices.length);
    voices.push(media);
    return media;
  });
  audio.mount();
  return { audio, voices, port: audio.channel("race-a") };
}
const flush = async () => {
  for (let i = 0; i < 6; i++) await Promise.resolve();
};

test("adjustable effects use independent native looping voices and retain their position", async () => {
  const { audio, port, voices } = setup();
  assert.equal(
    await port.loop("engine", ["engine.mp3"], { volume: 0.2, rate: 0.8 }),
    true,
  );
  const engine = voices[1];
  assert.equal(engine.dataset.platformAudio, "");
  assert.equal(engine.dataset.platformLoop, "engine");
  assert.equal(engine.loop, true);
  assert.equal(engine.volume, 0.2);
  assert.equal(engine.playbackRate, 0.8);
  assert.equal(engine.preservesPitch, false);
  assert.equal(engine.mozPreservesPitch, false);
  assert.equal(engine.webkitPreservesPitch, false);
  engine.currentTime = 1.25;
  assert.equal(
    await port.loop("engine", ["engine.mp3"], { volume: 0.4, rate: 1.6 }),
    true,
  );
  assert.deepEqual(engine.calls, ["engine.mp3"]);
  assert.equal(engine.currentTime, 1.25);
  assert.equal(engine.volume, 0.4);
  assert.equal(engine.playbackRate, 1.6);
  await port.loop("tires", ["tires.mp3"], { volume: 0.1 });
  await port.music(["music.mp3"]);
  await port.play(["collision.mp3"]);
  assert.equal(voices.length, 4);
  assert.ok(voices.every((media) => !media.paused));
  port.stopLoop("tires");
  assert.equal(voices[2].paused, true);
  assert.equal(voices[2].removed, true);
  assert.equal(engine.paused, false);
  assert.equal(voices[0].paused, false);
  assert.equal(voices[3].paused, false);
  audio.dispose();
  assert.ok(voices.every((media) => media.paused && media.removed));
});

test("repeated pending loop calls share playback and use the latest controls", async () => {
  let finish;
  const { port, voices } = setup((media, i) => {
    if (i)
      media.behavior = () =>
        new Promise((resolve) => {
          finish = resolve;
        });
  });
  const first = port.loop("engine", ["engine.mp3"], { rate: 0.7 });
  const second = port.loop("engine", ["engine.mp3"], {
    rate: 1.8,
    volume: 0.3,
  });
  assert.equal(voices.length, 2);
  assert.deepEqual(voices[1].calls, ["engine.mp3"]);
  finish();
  assert.equal(await first, true);
  assert.equal(await second, true);
  assert.equal(voices[1].playbackRate, 1.8);
  assert.equal(voices[1].volume, 0.3);
});

test("animation-frame controls ramp smoothly without restarting and cleanup cancels the ramp", async () => {
  const oldRAF = globalThis.requestAnimationFrame,
    oldCancel = globalThis.cancelAnimationFrame;
  const frames = new Map();
  let sequence = 0;
  globalThis.requestAnimationFrame = (callback) => {
    frames.set(++sequence, callback);
    return sequence;
  };
  globalThis.cancelAnimationFrame = (id) => frames.delete(id);
  const frame = (time) => {
    const batch = [...frames.values()];
    frames.clear();
    for (const callback of batch) callback(time);
  };
  try {
    const { port, voices } = setup();
    await port.loop("engine", ["engine.mp3"], { volume: 0.1, rate: 0.8 });
    await port.loop("engine", ["engine.mp3"], { volume: 0.5, rate: 1.8 });
    assert.equal(frames.size, 1);
    frame(16);
    assert.ok(voices[1].volume > 0.1 && voices[1].volume < 0.5);
    assert.ok(voices[1].playbackRate > 0.8 && voices[1].playbackRate < 1.8);
    for (let t = 32; t < 1000; t += 16) frame(t);
    assert.equal(voices[1].volume, 0.5);
    assert.equal(voices[1].playbackRate, 1.8);
    assert.deepEqual(voices[1].calls, ["engine.mp3"]);
    await port.loop("engine", ["engine.mp3"], { volume: 0.2 });
    port.stopLoop("engine");
    assert.equal(frames.size, 0);
    assert.equal(voices[1].paused, true);
  } finally {
    if (oldRAF === undefined) delete globalThis.requestAnimationFrame;
    else globalThis.requestAnimationFrame = oldRAF;
    if (oldCancel === undefined) delete globalThis.cancelAnimationFrame;
    else globalThis.cancelAnimationFrame = oldCancel;
  }
});

test("global mute drops loops and pending play cannot revive after unmute", async () => {
  const finishes = [];
  const { audio, port, voices } = setup((media, i) => {
    if (i)
      media.behavior = () =>
        new Promise((resolve) =>
          finishes.push(() => {
            media.paused = false;
            resolve();
          }),
        );
  });
  const engine = port.loop("engine", ["cdn.mp3", "local.mp3"]);
  const tires = port.loop("tires", ["tires.mp3"]);
  audio.setEnabled(false);
  assert.ok(voices.every((media) => media.paused && media.muted));
  assert.equal(await port.loop("ignored", ["ignored.mp3"]), false);
  audio.setEnabled(true);
  for (const finish of finishes) finish();
  assert.equal(await engine, false);
  assert.equal(await tires, false);
  assert.ok(voices.every((media) => media.paused));
  assert.equal(voices.length, 3);
  assert.deepEqual(voices[1].calls, ["cdn.mp3"]);
  assert.deepEqual(voices[2].calls, ["tires.mp3"]);
});

test("owner changes release all prior loops and late old cleanup cannot stop the new owner", async () => {
  const { audio, port, voices } = setup();
  await port.loop("engine", ["old-engine.mp3"]);
  await port.loop("tires", ["old-tires.mp3"]);
  const next = audio.channel("race-b");
  await next.loop("engine", ["next-engine.mp3"]);
  assert.equal(voices[1].removed, true);
  assert.equal(voices[2].removed, true);
  port.stopLoop("engine");
  port.stop();
  assert.equal(voices[3].paused, false);
  assert.equal(voices[3].src, "next-engine.mp3");
  next.stop();
  assert.equal(voices[3].paused, true);
  assert.equal(voices[3].removed, true);
});

test("replacing sources or explicitly stopping a key invalidates old async work", async () => {
  let finish;
  const { port, voices } = setup((media, i) => {
    if (i === 1)
      media.behavior = () =>
        new Promise((resolve) => {
          finish = () => {
            media.paused = false;
            resolve();
          };
        });
  });
  const old = port.loop("engine", ["old.mp3"]);
  assert.equal(await port.loop("engine", ["new.mp3"]), true);
  finish();
  assert.equal(await old, false);
  assert.equal(voices[1].paused, true);
  assert.equal(voices[2].paused, false);
  port.stopLoop("engine");
  assert.equal(voices[2].removed, true);
  assert.equal(await port.loop("engine", []), false);
  assert.equal(voices.length, 3);
});

test("loop alternatives fall back once, retain looping controls and clean up after late final failure", async () => {
  const { port, voices } = setup((media) => {
    media.behavior = () =>
      media.src.startsWith("cdn")
        ? Promise.reject(Error("cdn"))
        : Promise.resolve();
  });
  assert.equal(
    await port.loop("engine", ["cdn.mp3", "local.mp3"], {
      volume: 0.23,
      rate: 1.4,
    }),
    true,
  );
  const media = voices[1];
  assert.deepEqual(media.calls, ["cdn.mp3", "local.mp3"]);
  assert.equal(media.loop, true);
  assert.equal(media.volume, 0.23);
  assert.equal(media.playbackRate, 1.4);
  const late = media.onerror;
  late();
  await flush();
  assert.equal(media.paused, true);
  assert.equal(media.removed, true);
  assert.equal(media.src, "");
  late();
  await flush();
  assert.deepEqual(media.calls, ["cdn.mp3", "local.mp3"]);
});

test("an error after successful CDN playback falls back, but never after mute or disposal", async () => {
  const { audio, port, voices } = setup();
  await port.loop("engine", ["cdn.mp3", "local.mp3"]);
  voices[1].onerror();
  await flush();
  assert.deepEqual(voices[1].calls, ["cdn.mp3", "local.mp3"]);
  await port.loop("tires", ["cdn-tires.mp3", "local-tires.mp3"]);
  const late = voices[2].onerror;
  audio.dispose();
  late();
  await flush();
  assert.deepEqual(voices[2].calls, ["cdn-tires.mp3"]);
  assert.ok(voices.every((media) => media.paused && media.removed));
});

test("denied playback and invalid controls are contained without leaking a voice", async () => {
  const { port, voices } = setup((media) => {
    media.behavior = () => Promise.reject(Error("denied"));
  });
  assert.equal(
    await port.loop("engine", ["one.mp3", "two.mp3"], {
      volume: Infinity,
      rate: NaN,
    }),
    false,
  );
  assert.deepEqual(voices[1].calls, ["one.mp3", "two.mp3"]);
  assert.equal(voices[1].removed, true);
  const accepted = setup();
  await accepted.port.loop("engine", ["one.mp3"], { volume: 12, rate: -1 });
  assert.equal(accepted.voices[1].volume, 1);
  assert.equal(accepted.voices[1].playbackRate, 0.25);
  await accepted.port.loop("engine", ["one.mp3"], {
    volume: NaN,
    rate: Infinity,
  });
  assert.equal(accepted.voices[1].volume, 1);
  assert.equal(accepted.voices[1].playbackRate, 0.25);
});

test("hidden-page and room cleanup stop loops and hidden requests never create media", async () => {
  const oldDocument = globalThis.document;
  const { audio, port, voices } = setup();
  try {
    await port.loop("engine", ["engine.mp3"]);
    globalThis.document = { hidden: true };
    // The platform visibilitychange/room cleanup handlers call audio.stop().
    audio.stop();
    assert.equal(voices[1].paused, true);
    assert.equal(voices[1].removed, true);
    assert.equal(await port.loop("engine", ["engine.mp3"]), false);
    assert.equal(voices.length, 2);
    globalThis.document.hidden = false;
    assert.equal(voices[1].paused, true);
    assert.deepEqual(voices[1].calls, ["engine.mp3"]);
  } finally {
    if (oldDocument === undefined) delete globalThis.document;
    else globalThis.document = oldDocument;
  }
});
