// Failure inventory before implementation: resync restarts/replaces a lane or
// changes its playbackID; a foreign owner seeks the same key; seek resumes a
// paused/muted voice or resurrects a completed/cancelled one-shot; prefix/suffix
// offsets normalize incorrectly; initial/late metadata overwrites a newer seek;
// replaced, muted, failed or disposed sources apply an old queued seek; rejected
// offsets mutate state; a paused suffix boundary later overwrites resync.
import test from "node:test";
import assert from "node:assert/strict";
import { GameAudio } from "../src/audio.ts";

class Media {
  dataset = {};
  src = "";
  currentSrc = "";
  currentTime = 0;
  duration = 10;
  readyState = 1;
  paused = true;
  muted = false;
  loop = false;
  ended = false;
  volume = 1;
  calls = [];
  loads = 0;
  removed = false;
  onloadedmetadata = null;
  onended = null;
  onerror = null;
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
  removeAttribute(k) {
    if (k === "src") this.src = "";
  }
  remove() {
    this.removed = true;
  }
}
function setup(configure = () => {}) {
  const media = [];
  const audio = new GameAudio(() => {
    const m = new Media();
    configure(m, media.length);
    media.push(m);
    return m;
  });
  return { audio, media, p: audio.channel("room"), q: audio.channel("other") };
}
const flush = async () => {
  for (let i = 0; i < 8; i++) await Promise.resolve();
};

test("active seek is owner scoped and changes neither source, identity nor playback state", async () => {
  const { media, p, q } = setup();
  await p.track("bg", ["music"], { loop: true, playbackID: "round:music" });
  await q.track("bg", ["other"], { loop: true, playbackID: "other:music" });
  const m = media[0],
    source = m.src,
    calls = m.calls.length,
    loads = m.loads;
  assert.equal(p.seekTrack("bg", 4.5), true);
  assert.equal(m.currentTime, 4.5);
  assert.equal(m.paused, false);
  assert.equal(media[1].currentTime, 0);
  assert.equal(q.seekTrack("missing", 3), false);
  await p.track("bg", ["music"], {
    loop: true,
    playbackID: "round:music",
    startOffset: 0,
  });
  assert.equal(media.length, 2);
  assert.equal(m.currentTime, 4.5);
  assert.equal(m.src, source);
  assert.equal(m.calls.length, calls);
  assert.equal(m.loads, loads);
});
test("elapsed offsets normalize full loops and intro/suffix loops without replay", async () => {
  const { media, p } = setup();
  await p.track("suffix", ["intro"], { loop: true, loopStartSeconds: 2 });
  await p.track("full", ["loop"], { loop: true });
  await p.track("finite", ["shot"]);
  for (const [elapsed, position] of [
    [0, 0],
    [1.5, 1.5],
    [2, 2],
    [10, 2],
    [13, 5],
    [26, 2],
  ]) {
    assert.equal(p.seekTrack("suffix", elapsed), true);
    assert.equal(media[0].currentTime, position);
  }
  assert.equal(p.seekTrack("full", 13), true);
  assert.equal(media[1].currentTime, 3);
  assert.equal(p.seekTrack("finite", 9), true);
  assert.equal(media[2].currentTime, 9);
  for (const value of [-1, NaN, Infinity, 10, 11])
    assert.equal(p.seekTrack("finite", value), false);
  assert.equal(media[2].currentTime, 9);
  assert.ok(media.every((m) => m.calls.length === 1 && !m.paused));
});
test("paused/soft-muted seek remains paused/muted and supersedes a pending suffix restart", async () => {
  const { media, p } = setup();
  await p.track("bg", ["intro"], {
    loop: true,
    loopStartSeconds: 2,
    muted: true,
  });
  p.pauseTrack("bg");
  const m = media[0],
    calls = m.calls.length;
  m.ended = true;
  m.currentTime = 10;
  m.onended();
  await flush();
  assert.equal(p.seekTrack("bg", 5), true);
  assert.equal(m.currentTime, 5);
  assert.equal(m.paused, true);
  assert.equal(m.muted, true);
  assert.equal(m.calls.length, calls);
  assert.equal(await p.resumeTrack("bg"), true);
  assert.equal(m.currentTime, 5);
  assert.equal(m.muted, true);
  p.pause();
  assert.equal(p.seekTrack("bg", 6), true);
  assert.equal(m.paused, true);
  assert.equal(m.currentTime, 6);
});
test("pending metadata applies only the latest accepted seek, including default-offset tracks", async () => {
  let finish;
  const { media, p } = setup((m) => {
    m.readyState = 0;
    m.duration = NaN;
    m.behavior = () => new Promise((r) => (finish = r));
  });
  const pending = p.track("bg", ["music"], { loop: true, playbackID: "song" });
  const m = media[0];
  assert.equal(p.seekTrack("bg", 14), true);
  assert.equal(p.seekTrack("bg", 17), true);
  m.duration = 10;
  m.readyState = 1;
  m.onloadedmetadata();
  assert.equal(m.currentTime, 7);
  finish();
  assert.equal(await pending, true);
  assert.equal(m.currentTime, 7);
  assert.equal(m.calls.length, 1);
});
test("late metadata after replacement or mute cannot apply a cancelled seek", async () => {
  for (const cancel of ["replace", "stop", "mute", "dispose"]) {
    const { audio, media, p } = setup((m) => {
      m.readyState = 0;
      m.duration = NaN;
    });
    await p.track("bg", ["old"], { loop: true, playbackID: "old" });
    const old = media[0],
      metadata = old.onloadedmetadata;
    assert.equal(p.seekTrack("bg", 17), true);
    if (cancel === "replace")
      await p.track("bg", ["new"], { loop: true, playbackID: "new" });
    else if (cancel === "stop") p.stopTrack("bg");
    else if (cancel === "mute") {
      audio.setEnabled(false);
      audio.setEnabled(true);
    } else audio.dispose();
    old.duration = 10;
    old.readyState = 1;
    metadata?.();
    assert.equal(old.currentTime, 0);
    assert.equal(old.paused, true);
    if (cancel === "replace") {
      assert.equal(media[1].currentTime, 0);
      assert.equal(media[1].calls.length, 1);
    } else assert.equal(p.seekTrack("bg", 2), false);
  }
});
test("seek cannot revive completed, failed, unavailable or nonexistent lanes", async () => {
  for (const cancel of [
    "end",
    "failure",
    "slot-stop",
    "owner-stop",
    "mute",
    "zero",
    "dispose",
    "hidden",
  ]) {
    const { audio, media, p } = setup();
    await p.track("shot", ["shot"], { playbackID: "one" });
    const m = media[0];
    const oldDocument = globalThis.document;
    if (cancel === "end") {
      m.ended = true;
      m.paused = true;
      m.onended();
      await flush();
    } else if (cancel === "failure") m.onerror();
    else if (cancel === "slot-stop") p.stopTrack("shot");
    else if (cancel === "owner-stop") p.stop();
    else if (cancel === "mute") audio.setEnabled(false);
    else if (cancel === "zero") audio.setVolume(0);
    else if (cancel === "dispose") audio.dispose();
    else globalThis.document = { hidden: true };
    try {
      const before = m.currentTime,
        calls = m.calls.length;
      assert.equal(p.seekTrack("shot", 2), false, cancel);
      assert.equal(p.seekTrack("new", 1), false);
      assert.equal(m.currentTime, before);
      assert.equal(m.calls.length, calls);
      assert.equal(media.length, 1);
    } finally {
      globalThis.document = oldDocument;
      audio.dispose();
    }
  }
});
test("a native seek failure leaves an active source usable without retaining the rejected offset", async () => {
  const { media, p } = setup();
  await p.track("bg", ["music"], { loop: true });
  const m = media[0];
  let at = 1;
  Object.defineProperty(m, "currentTime", {
    get: () => at,
    set: (v) => {
      if (v === 3) throw Error("unseekable");
      at = v;
    },
    configurable: true,
  });
  assert.equal(p.seekTrack("bg", 3), false);
  assert.equal(at, 1);
  assert.equal(m.paused, false);
  assert.equal(p.seekTrack("bg", 4), true);
  assert.equal(at, 4);
  m.onloadedmetadata();
  assert.equal(at, 4);
  assert.equal(m.calls.length, 1);
});
test("pending seek follows the successful fallback, not stale primary metadata", async () => {
  let reject;
  const { media, p } = setup((m) => {
    m.readyState = 0;
    m.duration = NaN;
    m.behavior = () =>
      m.src === "cdn" ? new Promise((_, r) => (reject = r)) : Promise.resolve();
  });
  const pending = p.track("bg", ["cdn", "fallback"], { loop: true });
  const m = media[0],
    oldMetadata = m.onloadedmetadata;
  assert.equal(p.seekTrack("bg", 16), true);
  reject(Error("cdn"));
  assert.equal(await pending, true);
  m.duration = 10;
  m.readyState = 1;
  m.currentSrc = "fallback";
  oldMetadata();
  m.onloadedmetadata();
  assert.equal(m.currentTime, 6);
  assert.equal(m.src, "fallback");
  assert.deepEqual(m.calls, ["cdn", "fallback"]);
});
