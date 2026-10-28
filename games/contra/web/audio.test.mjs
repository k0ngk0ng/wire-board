// Failure inventory written before adapter implementation:
// old epoch/out-of-order/history snapshots replay effects; missed history
// resurrects transients; late media promises mutate replacements; paused starts
// burst; pulse override freezes music; reset leaves tracks; catalog accepts raw
// loops or ignores hashes; mute/hidden tombstones are bypassed with fabricated IDs.
import test from "node:test";
import assert from "node:assert/strict";
import { ContraAudioAdapter, createContraAudioResources } from "./audio.ts";
const voice = (overrides = {}) => ({
  key: "slot-0",
  id: 70,
  channel: "pulse1",
  playbackID: "epoch/0/1",
  positionTicks: 0,
  paused: false,
  muted: false,
  restore: true,
  continuous: true,
  ...overrides,
});
const view = (sequence, tick, voices = [], events = []) => ({
  epoch: "epoch",
  sequence,
  tick,
  paused: false,
  voices,
  events,
});
const event = (sequence, action, key, value = false, v) => ({
  sequence,
  tick: sequence,
  action,
  key,
  value,
  voice: v,
});
const setup = () => {
  const calls = [];
  let resolve;
  const port = {
    track: (...args) => {
      calls.push(["track", ...args]);
      return new Promise((r) => (resolve = r));
    },
    stopTrack: (...x) => calls.push(["stop", ...x]),
    muteTrack: (...x) => calls.push(["mute", ...x]),
    pauseTrack: (...x) => calls.push(["pause", ...x]),
    resumeTrack: (...x) => {
      calls.push(["resume", ...x]);
      return Promise.resolve(true);
    },
    seekTrack: (...x) => {
      calls.push(["seek", ...x]);
      return true;
    },
  };
  const resources = {
    resolve: (v) => ({
      sources: [`${v.id}-${v.channel}.wav`],
      loop: v.continuous,
      loopStartSeconds: v.continuous ? 2 : undefined,
      tickSeconds: 0.016639,
    }),
  };
  return {
    calls,
    adapter: new ContraAudioAdapter(port, resources, "epoch"),
    resolve: () => resolve?.(true),
  };
};
test("first view restores only music; explicit resync seeks same playback ID without restart", () => {
  const { adapter, calls } = setup();
  const music = voice({ positionTicks: 300 });
  const effect = voice({
    key: "slot-4",
    id: 26,
    restore: false,
    continuous: false,
  });
  adapter.update(
    view(
      10,
      300,
      [music, effect],
      [event(10, "start", "slot-4", false, effect)],
    ),
  );
  assert.equal(calls.filter((x) => x[0] === "track").length, 1);
  assert.equal(calls[0][3].playbackID, music.playbackID);
  assert.equal(calls[0][3].startOffset, 300 * 0.016639);
  adapter.update(view(10, 310, [voice({ positionTicks: 310 })]), {
    resync: true,
  });
  assert.deepEqual(calls.at(-1), ["seek", "contra/slot-0", 310 * 0.016639]);
  assert.equal(calls.filter((x) => x[0] === "track").length, 1);
});
test("ordered live events preserve mute/pause/resume and replace paused keys synchronously", () => {
  const { adapter, calls } = setup();
  adapter.update(view(0, 0));
  const old = voice({ paused: true, muted: true });
  adapter.update(view(1, 1, [old], [event(1, "start", "slot-0", false, old)]));
  assert.deepEqual(
    calls.map((x) => x[0]),
    ["track", "pause", "mute"],
  );
  assert.equal(calls[0][3].muted, true);
  const next = voice({ playbackID: "epoch/0/2" });
  adapter.update(
    view(
      5,
      5,
      [next],
      [
        event(2, "mute", "slot-0", true),
        event(3, "resume", "slot-0"),
        event(4, "stop", "slot-0"),
        event(5, "start", "slot-0", false, next),
      ],
    ),
  );
  assert.deepEqual(
    calls.slice(3).map((x) => x[0]),
    ["mute", "resume", "stop", "track"],
  );
  adapter.update(view(5, 6, [next]));
  assert.equal(calls.filter((x) => x[0] === "track").length, 2);
});
test("history gap drops missed effects and seeks persistent music; old epochs never bind implicitly", () => {
  const { adapter, calls } = setup();
  adapter.update(view(1, 1, [voice()]));
  const shot = voice({
    key: "slot-4",
    id: 26,
    playbackID: "epoch/4/2",
    restore: false,
    continuous: false,
  });
  adapter.update(
    view(
      90,
      90,
      [voice({ positionTicks: 89 }), shot],
      [
        event(89, "start", "slot-4", false, shot),
        event(90, "mute", "slot-0", true),
      ],
    ),
  );
  assert.equal(calls.filter((x) => x[0] === "track").length, 1);
  assert.ok(calls.some((x) => x[0] === "seek"));
  const count = calls.length;
  adapter.update({ ...view(100, 100), epoch: "retired" });
  adapter.update(view(80, 80));
  assert.equal(calls.length, count);
  adapter.bindEpoch("new");
  adapter.update(view(1000, 1000, [shot]));
  assert.equal(calls.filter((x) => x[0] === "track").length, 1);
});
test("stopped same-ID music cannot be revived by resync or late completion", async () => {
  const { adapter, calls, resolve } = setup();
  adapter.update(view(1, 1, [voice()]));
  adapter.stop();
  resolve();
  await Promise.resolve();
  adapter.update(view(1, 10, [voice({ positionTicks: 10 })]), { resync: true });
  assert.equal(calls.filter((x) => x[0] === "track").length, 1);
  adapter.dispose();
  const n = calls.length;
  adapter.update(view(2, 11, [voice()]));
  assert.equal(calls.length, n);
});
test("same tick final voices only; later identical effect uses a new original playback ID", () => {
  const { adapter, calls } = setup();
  adapter.update(view(0, 0));
  const a = voice({
    key: "slot-4",
    id: 26,
    playbackID: "epoch/4/1",
    restore: false,
    continuous: false,
  });
  adapter.update(view(1, 1, [a], [event(1, "start", "slot-4", false, a)]));
  adapter.update(view(1, 1, [a], [event(1, "start", "slot-4", false, a)]));
  const b = { ...a, playbackID: "epoch/4/2" };
  adapter.update(
    view(
      3,
      2,
      [b],
      [event(2, "stop", "slot-4"), event(3, "start", "slot-4", false, b)],
    ),
  );
  assert.deepEqual(
    calls.map((x) => x[0]),
    ["track", "stop", "track"],
  );
  assert.equal(calls[2][3].playbackID, "epoch/4/2");
});
test("resources require SHA-bound verified loop stems and never continuous noise", () => {
  const sha = "a".repeat(64);
  const resolved = [];
  const source = {
    sounds: [
      {
        id: 70,
        stems: [
          {
            channel: "pulse1",
            file: "raw.wav",
            researchDirectory: ".local/raw",
            sha256: sha,
          },
        ],
      },
      {
        id: 26,
        stems: [
          {
            channel: "pulse1",
            file: "shot.wav",
            researchDirectory: ".local/stems",
            sha256: sha,
          },
        ],
      },
    ],
  };
  const song = {
    id: 70,
    pcmSeamReady: true,
    nativeBoundaryReady: true,
    loopReady: true,
    loopStartSeconds: 2,
    periodTicks: 100,
    periodSamples: 73377,
    stems: [{ channel: "pulse1", file: ".local/loop.wav", sha256: sha }],
  };
  const bind = (path, digest) => {
    resolved.push([path, digest]);
    return ["/media/" + digest];
  };
  const good = createContraAudioResources(
    source,
    { sampleRate: 44100, songs: [song] },
    bind,
  );
  assert.equal(good.resolve(voice()).loop, true);
  assert.deepEqual(resolved[0], [".local/loop.wav", sha]);
  assert.equal(good.resolve(voice({ channel: "noise" })), undefined);
  const bad = createContraAudioResources(
    source,
    { sampleRate: 44100, songs: [{ ...song, nativeBoundaryReady: false }] },
    bind,
  );
  assert.equal(bad.resolve(voice()), undefined);
  assert.equal(good.resolve(voice({ id: 26, continuous: false })).loop, false);
});

test("terminal cue resolver uses only SHA-bound complete 6E, never a tonal stem or another effect", () => {
  const sha =
      "c5c16a2b174c64c4a2d2a2fef2ba5507311c8db66f09efd01ed62240731f3fe8",
    seen = [];
  const r = createContraAudioResources(
    {
      sounds: [
        {
          id: 110,
          file: "cue-6e.wav",
          researchDirectory: ".local/contra-audio/export",
          sha256: sha,
          stems: [],
        },
      ],
    },
    { sampleRate: 44100, songs: [] },
    (path, digest) => {
      seen.push([path, digest]);
      return ["verified/" + digest];
    },
  );
  const full = r.resolveTerminalCue(voice({ id: 110, continuous: false }));
  assert.equal(full.loop, false);
  assert.deepEqual(seen, [[".local/contra-audio/export/cue-6e.wav", sha]]);
  assert.equal(
    r.resolveTerminalCue(voice({ id: 26, continuous: false })),
    undefined,
  );
  assert.equal(
    r.resolveTerminalCue(voice({ id: 110, continuous: true })),
    undefined,
  );
});
