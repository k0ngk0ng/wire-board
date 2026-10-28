// Failure inventory before implementation:
// - finite same-slot replacements overlap, or different slots cancel each other;
// - identical keys in different owners collide or one owner pauses/stops another;
// - soft mute rewinds/pauses a voice instead of preserving its running timeline;
// - slot pause loses position; global resume unintentionally resumes a slot hold;
// - global mute/zero/hidden/stop leaves pending play/fallback able to resurrect;
// - stable event IDs replay after end/stop/mute or snapshot refresh; new IDs fail
//   to repeat the same URL; source/loop changes reuse an old ID;
// - offsets/suffix loops are ignored, applied late to a replacement, or invalid
//   media metadata throws into gameplay; failure leaves queued audio behind;
// - transient paused starts fall back or new pause prompt bypasses global mute;
// - unchanged play/music/loop calls lose compatibility or media-volume behavior.
import test from "node:test";
import assert from "node:assert/strict";
import { GameAudio } from "../src/audio.ts";

class Media {
  dataset = {};
  src = "";
  currentSrc = "";
  currentTime = 0;
  duration = 10;
  muted = false;
  volume = 1;
  loop = false;
  paused = true;
  removed = false;
  loads = 0;
  calls = [];
  onended = null;
  onerror = null;
  onloadedmetadata = null;
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
const setup = () => {
  let behavior = () => Promise.resolve();
  const media = [];
  const audio = new GameAudio(() => {
    const m = new Media();
    const run = behavior;
    m.behavior = () => run(m);
    media.push(m);
    return m;
  });
  return {
    audio,
    media,
    p: audio.channel("room:one"),
    q: audio.channel("room:two"),
    setBehavior: (value) => {
      behavior = value;
    },
  };
};
const drain = async () => {
  for (let n = 0; n < 8; n++) await Promise.resolve();
};

test("finite keyed voices replace only their own owner and slot", async () => {
  const { audio, media, p, q } = setup();
  await p.track("pulse1", ["shot.wav"], { playbackID: "one:1" });
  await p.track("noise", ["blast.wav"], { playbackID: "one:2" });
  await q.track("pulse1", ["other.wav"], { playbackID: "two:1" });
  assert.equal(media.length, 3);
  assert.ok(media.every((m) => !m.loop && !m.paused));
  await p.track("pulse1", ["shot.wav"], { playbackID: "one:3" });
  assert.equal(media[0].removed, true);
  assert.equal(media[0].paused, true);
  assert.equal(media[1].paused, false);
  assert.equal(media[2].paused, false);
  p.stopTrack("pulse1");
  assert.equal(media[3].paused, true);
  assert.equal(media[2].paused, false);
  p.stop();
  assert.equal(media[1].paused, true);
  assert.equal(media[2].paused, false);
  audio.dispose();
  assert.ok(media.every((m) => m.removed && m.paused));
});
test("slot mute preserves running position and master volume without restarting", async () => {
  const { audio, media, p } = setup();
  audio.setVolume(0.4);
  await p.track("bg-pulse1", ["music.wav"], {
    loop: true,
    playbackID: "round:music",
  });
  const m = media[0];
  m.currentTime = 3.5;
  const calls = m.calls.length,
    loads = m.loads;
  p.muteTrack("bg-pulse1", true);
  assert.equal(m.muted, true);
  assert.equal(m.paused, false);
  m.currentTime = 4;
  audio.setVolume(0.7);
  p.muteTrack("bg-pulse1", false);
  assert.equal(m.muted, false);
  assert.equal(m.currentTime, 4);
  assert.equal(m.volume, 0.7);
  assert.equal(m.calls.length, calls);
  assert.equal(m.loads, loads);
});
test("key pause allows a separate finite prompt but cannot bypass whole-owner pause", async () => {
  const { audio, media, p } = setup();
  await p.track("bg", ["music"], { loop: true });
  media[0].currentTime = 5;
  p.pauseTrack("bg");
  assert.equal(media[0].paused, true);
  await p.track("pause-prompt", ["prompt"]);
  assert.equal(media[1].paused, false);
  p.pause();
  assert.ok(media.every((m) => m.paused));
  assert.equal(await p.track("forbidden", ["no"]), false);
  await p.resume();
  assert.equal(media[0].paused, true);
  assert.equal(media[1].paused, false);
  assert.equal(await p.resumeTrack("bg"), true);
  assert.equal(media[0].currentTime, 5);
  assert.equal(await p.resumeTrack("bg"), false);
  audio.dispose();
});
test("owner pause/resume preserves slot mute and does not restart or seek", async () => {
  const { media, p, q } = setup();
  await p.track("bg", ["music"], { muted: true });
  await q.track("bg", ["other"]);
  media[0].currentTime = 6;
  const loads = media[0].loads;
  p.pause();
  assert.equal(media[0].paused, true);
  assert.equal(media[1].paused, false);
  await p.resume();
  assert.equal(media[0].paused, false);
  assert.equal(media[0].muted, true);
  assert.equal(media[0].currentTime, 6);
  assert.equal(media[0].loads, loads);
});
test("event IDs deduplicate snapshots, completed/cancelled slots and global mute", async () => {
  for (const cancel of ["end", "slot-stop", "owner-stop", "mute", "zero"]) {
    const { audio, media, p } = setup();
    const options = { playbackID: "round:accepted:7" };
    await p.track("slot", ["one"], options);
    media[0].currentTime = 2;
    await p.track("slot", ["one"], { ...options, startOffset: 0 });
    assert.equal(media[0].currentTime, 2);
    assert.equal(media[0].calls.length, 1);
    if (cancel === "end") {
      media[0].paused = true;
      media[0].onended();
      await drain();
    } else if (cancel === "slot-stop") p.stopTrack("slot");
    else if (cancel === "owner-stop") p.stop();
    else if (cancel === "mute") {
      audio.setEnabled(false);
      audio.setEnabled(true);
    } else {
      audio.setVolume(0);
      audio.setVolume(1);
    }
    assert.equal(await p.track("slot", ["one"], options), false, cancel);
    assert.equal(media.length, 1, cancel);
    assert.equal(
      await p.track("slot", ["one"], { playbackID: "round:accepted:8" }),
      true,
    );
    assert.equal(media.length, 2);
    audio.dispose();
  }
});
test("stable IDs cannot change sources/loop semantics and invalid options create no players", async () => {
  const { media, p } = setup();
  for (const options of [
    { startOffset: -1 },
    { startOffset: NaN },
    { loopStartSeconds: 1 },
    { loop: true, loopStartSeconds: -1 },
  ])
    assert.equal(await p.track("slot", ["a"], options), false);
  assert.equal(media.length, 0);
  await p.track("slot", ["a"], { playbackID: "a" });
  assert.equal(await p.track("slot", ["b"], { playbackID: "a" }), false);
  assert.equal(
    await p.track("slot", ["a"], { playbackID: "a", loop: true }),
    false,
  );
  assert.equal(media.length, 1);
  assert.equal(media[0].src, "a");
});
test("tracks share native offset and suffix-loop semantics; slots can loop independently", async () => {
  const { media, p } = setup();
  await p.track("bg", ["intro"], {
    loop: true,
    loopStartSeconds: 2,
    startOffset: 13,
    playbackID: "song",
  });
  const m = media[0];
  m.onloadedmetadata();
  assert.equal(m.currentTime, 5);
  assert.equal(m.loop, false);
  m.currentTime = 10;
  m.paused = true;
  m.onended();
  await drain();
  assert.equal(m.currentTime, 2);
  assert.equal(m.calls.length, 2);
  await p.track("finite", ["shot"], { startOffset: 3 });
  media[1].onloadedmetadata();
  assert.equal(media[1].currentTime, 3);
  media[1].currentTime = 10;
  media[1].onended();
  await drain();
  assert.equal(media[1].calls.length, 1);
  assert.equal(m.paused, false);
});
test("pending native promises cannot revive replaced/stopped/muted tracks", async () => {
  for (const cancel of [
    "replace",
    "slot-stop",
    "owner-stop",
    "mute",
    "pause",
  ]) {
    const { audio, media, p, setBehavior } = setup();
    let finish;
    setBehavior(
      (m) =>
        new Promise(
          (r) =>
            (finish = () => {
              m.paused = false;
              r();
            }),
        ),
    );
    const pending = p.track("slot", ["cdn", "fallback"], { playbackID: "old" });
    setBehavior(() => Promise.resolve());
    if (cancel === "replace")
      await p.track("slot", ["new"], { playbackID: "new" });
    else if (cancel === "slot-stop") p.stopTrack("slot");
    else if (cancel === "owner-stop") p.stop();
    else if (cancel === "mute") {
      audio.setEnabled(false);
      audio.setEnabled(true);
    } else p.pauseTrack("slot");
    finish();
    assert.equal(await pending, false);
    assert.equal(media[0].paused, true, cancel);
    if (cancel === "replace") assert.equal(media[1].paused, false);
    audio.dispose();
  }
});
test("denied or failed media do not replay fallback after cancellation, and slots fail independently", async () => {
  const { audio, media, p, setBehavior } = setup();
  let reject;
  setBehavior(() => new Promise((_, r) => (reject = r)));
  const pending = p.track("bad", ["cdn", "fallback"], { playbackID: "bad" });
  setBehavior(() => Promise.resolve());
  await p.track("good", ["good"]);
  p.stopTrack("bad");
  reject(Error("late"));
  assert.equal(await pending, false);
  assert.equal(media[0].calls.length, 1);
  assert.equal(media[1].paused, false);
  await p.track("broken", ["broken"]);
  const broken = media[2];
  broken.onerror();
  assert.equal(broken.paused, true);
  assert.equal(broken.src, "");
  assert.equal(media[1].paused, false);
  p.pauseTrack("broken");
  assert.equal(await p.resumeTrack("broken"), false);
  audio.dispose();
});
test("global mute blocks preview and new keyed requests; hidden availability blocks resume", async () => {
  const { audio, media, p } = setup();
  await p.track("one", ["a"]);
  audio.setEnabled(false);
  assert.equal(await audio.play("preview"), false);
  assert.equal(await p.track("two", ["b"]), false);
  assert.equal(media.length, 1);
  audio.setEnabled(true);
  assert.equal(media[0].paused, true);
  await p.track("three", ["c"]);
  p.pauseTrack("three");
  const old = globalThis.document;
  globalThis.document = { hidden: true };
  try {
    assert.equal(await p.resumeTrack("three"), false);
    assert.equal(await p.track("four", ["d"]), false);
  } finally {
    globalThis.document = old;
  }
  audio.dispose();
});
