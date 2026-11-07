// Integration failures declared before changing audio implementation:
// 1. Delayed native play resolves after many draws but keeps the start frame's
//    offset; an ended/replaced/old-epoch promise must never seek a new voice.
// 2. Pause/resume and pulse1 reappearance retain wall-clock drift instead of the
//    currently painted authoritative position; no per-frame seek is acceptable.
// 3. A terminal revision can drain multiple audio ticks: its surviving voice
//    must start at the final visible age, not the historical start event age.
// 4. Background stop tombstones permanently silence same-ID BGM; restoring
//    finite effects or using invented IDs would instead replay missed sounds.
// 5. Queue buffering or fixed physics-frame pause revisions duplicate events;
//    hidden resync, explicit stop/final reset, and new epochs must stay distinct.
import test from "node:test";
import assert from "node:assert/strict";
import { ContraAudioAdapter } from "./audio.ts";
import { RuntimePresentationQueue } from "./runtime-presentation.ts";

const seconds = 0.016639;
const music = (positionTicks = 0, extra = {}) => ({
  key: "slot-0",
  id: 70,
  channel: "pulse1",
  playbackID: "e/0/1",
  positionTicks,
  paused: false,
  muted: false,
  restore: true,
  continuous: true,
  ...extra,
});
const shot = (positionTicks = 0, extra = {}) =>
  music(positionTicks, {
    key: "slot-4",
    id: 26,
    channel: "pulse1",
    playbackID: "e/4/2",
    restore: false,
    continuous: false,
    ...extra,
  });
const event = (sequence, tick, action, key = "slot-0", extra = {}) => ({
  sequence,
  tick,
  action,
  key,
  value: false,
  ...extra,
});
const snapshot = (tick, sequence, voices = [], events = []) => ({
  revision: String(tick),
  frame: String(tick),
  audio: { epoch: "e", tick, sequence, paused: false, voices, events },
});
const drain = async () => {
  for (let n = 0; n < 5; n++) await Promise.resolve();
};
function fixture() {
  const calls = [],
    pending = new Map();
  let drawn;
  const port = {
    track: (key, sources, options) => {
      calls.push(["track", key, options]);
      return new Promise((resolve) => pending.set(key, resolve));
    },
    restoreTrack: (key, id, options) => {
      calls.push(["restore", key, id, options]);
      return new Promise((resolve) => pending.set(key, resolve));
    },
    stopTrack: (key) => calls.push(["stop", key]),
    muteTrack: (key, value) => calls.push(["mute", key, value]),
    pauseTrack: (key) => calls.push(["pause", key]),
    resumeTrack: (key) => {
      calls.push(["resume", key]);
      return Promise.resolve(true);
    },
    seekTrack: (key, offset) => {
      calls.push(["seek", key, offset]);
      return true;
    },
  };
  const adapter = new ContraAudioAdapter(
    port,
    {
      resolve: (v) => ({
        sources: [`${v.id}-${v.channel}.wav`],
        loop: v.continuous,
        tickSeconds: seconds,
      }),
    },
    "e",
  );
  const q = new RuntimePresentationQueue((s, resync) => {
    drawn = s;
    adapter.update(s.audio, { resync });
  });
  return {
    adapter,
    q,
    calls,
    drawn: () => drawn,
    update: (s) => adapter.update(s.audio),
    resolve: (key = "slot-0", ok = true) => pending.get("contra/" + key)?.(ok),
  };
}
function start(f) {
  f.update(snapshot(0, 0));
  f.update(
    snapshot(
      1,
      1,
      [music()],
      [event(1, 1, "start", "slot-0", { voice: music() })],
    ),
  );
}

test("late media start seeks the latest painted position, never a queued future state", async () => {
  const f = fixture();
  f.q.reset(snapshot(0, 0), 0);
  f.q.push(
    snapshot(
      1,
      1,
      [music()],
      [event(1, 1, "start", "slot-0", { voice: music() })],
    ),
  );
  f.q.sample(0);
  f.q.sample(34);
  assert.equal(f.calls.filter((c) => c[0] === "track").length, 1);
  f.q.push(snapshot(2, 1, [music(1)]));
  f.q.push(snapshot(3, 1, [music(2)]));
  f.q.sample(51);
  assert.equal(f.drawn().revision, "2");
  f.resolve();
  await drain();
  assert.deepEqual(f.calls.at(-1), ["seek", "contra/slot-0", seconds]);
  const before = f.calls.length;
  f.q.sample(67);
  assert.equal(
    f.calls.length,
    before,
    "steady frame must not continually seek",
  );
});
test("pause revision and unmuted pulse align at authoritative boundaries before becoming audible", async () => {
  const f = fixture();
  start(f);
  f.resolve();
  await drain();
  f.calls.length = 0;
  const paused = music(10, { paused: true, muted: true });
  f.update({
    ...snapshot(
      11,
      3,
      [paused],
      [event(2, 11, "mute", "slot-0", { value: true }), event(3, 11, "pause")],
    ),
    frame: "10",
  });
  assert.deepEqual(f.calls, [
    ["mute", "contra/slot-0", true],
    ["pause", "contra/slot-0"],
    ["seek", "contra/slot-0", 10 * seconds],
  ]);
  f.calls.length = 0;
  f.update({ ...snapshot(50, 3, [paused]), frame: "10" });
  assert.deepEqual(f.calls, []);
  f.update(
    snapshot(51, 4, [music(10, { muted: true })], [event(4, 51, "resume")]),
  );
  assert.deepEqual(f.calls, [
    ["seek", "contra/slot-0", 10 * seconds],
    ["resume", "contra/slot-0"],
  ]);
  f.calls.length = 0;
  f.update(snapshot(52, 5, [music(11)], [event(5, 52, "mute")]));
  assert.deepEqual(f.calls, [
    ["seek", "contra/slot-0", 11 * seconds],
    ["mute", "contra/slot-0", false],
  ]);
});
test("terminal multi-tick flush starts its surviving cue at its final visible age; retries do not repeat", () => {
  const f = fixture();
  f.update(snapshot(0, 0));
  const cue = music(4, {
    id: 110,
    continuous: false,
    restore: false,
    playbackID: "e/0/8",
  });
  const final = {
    ...snapshot(
      5,
      2,
      [cue],
      [
        event(1, 1, "reset"),
        event(2, 1, "start", "slot-0", { voice: { ...cue, positionTicks: 0 } }),
      ],
    ),
    finalized: true,
  };
  f.update(final);
  assert.equal(
    f.calls.find((c) => c[0] === "track")[2].startOffset,
    4 * seconds,
  );
  const count = f.calls.length;
  for (let n = 0; n < 4; n++) f.update(final);
  assert.equal(f.calls.length, count);
  f.update({ ...snapshot(6, 3, [], [event(3, 6, "reset")]), finalized: true });
  assert.deepEqual(f.calls.at(-1), ["stop", "contra/slot-0"]);
});
test("background suspension restores only current persistent loops with identical authoritative IDs", async () => {
  const f = fixture();
  start(f);
  f.resolve();
  await drain();
  f.update(
    snapshot(
      2,
      2,
      [music(1), shot()],
      [event(2, 2, "start", "slot-4", { voice: shot() })],
    ),
  );
  f.adapter.suspend();
  const from = f.calls.length;
  const current = music(300, { paused: true, muted: true });
  f.adapter.update(snapshot(400, 30, [current, shot(20)]).audio, {
    resync: true,
  });
  assert.deepEqual(f.calls.slice(from), [
    [
      "restore",
      "contra/slot-0",
      "e/0/1",
      { startOffset: 300 * seconds, muted: true, paused: true },
    ],
  ]);
  f.adapter.update(snapshot(401, 30, [current, shot(20)]).audio, {
    resync: true,
  });
  assert.equal(f.calls.filter((c) => c[0] === "restore").length, 1);
  f.update(
    snapshot(
      402,
      31,
      [music(300, { muted: true })],
      [event(31, 402, "resume")],
    ),
  );
  f.resolve();
  await drain();
  assert.equal(
    f.calls.filter((c) => c[0] === "track").length,
    2,
    "old SFX cannot replay",
  );
});
test("suspension while a new stage arrives starts only its current song; explicit stop stays terminal", async () => {
  const f = fixture();
  start(f);
  f.adapter.suspend();
  const from = f.calls.length;
  const next = music(80, { id: 90, playbackID: "e/0/5" });
  f.adapter.update(snapshot(200, 20, [next, shot()]).audio, { resync: true });
  assert.equal(f.calls.slice(from).filter((c) => c[0] === "track").length, 1);
  assert.equal(f.calls.at(-1)[2].playbackID, "e/0/5");
  assert.equal(f.calls.slice(from).filter((c) => c[0] === "restore").length, 0);
  f.adapter.stop();
  f.adapter.update(snapshot(201, 20, [next]).audio, { resync: true });
  assert.equal(f.calls.filter((c) => c[0] === "track").length, 2);
  f.resolve();
  await drain();
  assert.equal(f.calls.at(-1)[0], "stop");
});
test("late completions from replaced or old-epoch voices cannot seek new lanes", async () => {
  const f = fixture();
  start(f);
  const old = f.resolve;
  f.update(snapshot(10, 1, [music(9)]));
  f.adapter.bindEpoch("new");
  const count = f.calls.length;
  old();
  await drain();
  assert.equal(f.calls.length, count);
  const g = fixture();
  start(g);
  g.update(snapshot(2, 2, [], [event(2, 2, "reset")]));
  g.resolve();
  await drain();
  assert.deepEqual(g.calls.at(-1), ["stop", "contra/slot-0"]);
});

test("media tick rate matches the fixed 60Hz authority without changing media offset units", () => {
  const f = fixture();
  start(f);
  const options = f.calls.find((c) => c[0] === "track")[2];
  assert.equal(options.rate, seconds * 60);
  const g = fixture();
  g.update(snapshot(0, 0));
  const cue = shot(12);
  g.update(
    snapshot(20, 1, [cue], [event(1, 8, "start", "slot-4", { voice: shot() })]),
  );
  const finite = g.calls.find((c) => c[0] === "track")[2];
  assert.equal(finite.rate, seconds * 60);
  assert.equal(finite.startOffset, 12 * seconds);
});

// Terminal failure inventory before full-mix handoff: finalized stops authority
// updates before $6E's first age7 percussion; normal $6E must stay per-slot;
// handoff must stop the three tonal stems and use their original identity/age;
// initial/reconnect/resync/hidden/stop/repeated-final must not replay finite audio.
test("live finalized 6E handoff uses the original ID and current offset once, stopping stems first", () => {
  const calls = [];
  const port = {
    track: (key, src, options) => {
      calls.push(["track", key, src, options]);
      return Promise.resolve(true);
    },
    stopTrack: (key) => calls.push(["stop", key]),
    muteTrack() {},
    pauseTrack() {},
    resumeTrack: () => Promise.resolve(true),
    seekTrack: () => true,
    restoreTrack: () => {
      throw Error("finite restore forbidden");
    },
  };
  const resources = {
    resolve: (v) => ({
      sources: [v.channel + ".wav"],
      loop: false,
      tickSeconds: seconds,
    }),
    resolveTerminalCue: () => ({
      sources: ["verified-full-6e.wav"],
      loop: false,
      tickSeconds: seconds,
    }),
  };
  const f = new ContraAudioAdapter(port, resources, "e");
  const voices = [
    music(0, { id: 110, continuous: false, restore: false }),
    music(0, {
      key: "slot-1",
      channel: "pulse2",
      id: 110,
      continuous: false,
      restore: false,
      playbackID: "e/1/2",
    }),
    music(0, {
      key: "slot-2",
      channel: "triangle",
      id: 110,
      continuous: false,
      restore: false,
      playbackID: "e/2/3",
    }),
  ];
  f.update(snapshot(0, 0).audio);
  f.update(
    snapshot(
      1,
      3,
      voices,
      voices.map((v, i) => event(i + 1, 1, "start", v.key, { voice: v })),
    ).audio,
  );
  assert.equal(
    calls.filter((c) => c[0] === "track").length,
    3,
    "non-final cue stays per-slot",
  );
  const end = snapshot(
    5,
    3,
    voices.map((v) => ({ ...v, positionTicks: 4 })),
  ).audio;
  const from = calls.length;
  f.update(end, { finalized: true });
  assert.deepEqual(
    calls.slice(from).map((c) => c.slice(0, 2)),
    [
      ["stop", "contra/slot-0"],
      ["stop", "contra/slot-1"],
      ["stop", "contra/slot-2"],
      ["track", "contra/terminal-cue"],
    ],
  );
  const mixed = calls.at(-1);
  assert.deepEqual(mixed[2], ["verified-full-6e.wav"]);
  assert.equal(mixed[3].playbackID, voices[0].playbackID);
  assert.equal(mixed[3].startOffset, 4 * seconds);
  assert.equal(mixed[3].loop, false);
  f.update(end, { finalized: true });
  assert.equal(
    calls.filter((c) => c[1] === "contra/terminal-cue" && c[0] === "track")
      .length,
    1,
  );
  f.suspend();
  f.update(end, { finalized: true, resync: true });
  assert.equal(
    calls.filter((c) => c[1] === "contra/terminal-cue" && c[0] === "track")
      .length,
    1,
  );
  f.stop();
  f.update(end, { finalized: true });
  assert.equal(calls.at(-1)[0], "stop");
  for (const resync of [false, true]) {
    const before = calls.length,
      g = new ContraAudioAdapter(port, resources, "e");
    if (resync) g.update(snapshot(0, 0).audio);
    g.update(end, { finalized: true, resync });
    assert.equal(
      calls.length,
      before,
      "initial/recovery final must not play an old finite cue",
    );
  }
});
