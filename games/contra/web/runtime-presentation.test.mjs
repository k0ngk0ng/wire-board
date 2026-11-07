// Failure cases before buffering: three accepted frames all paint in one RAF;
// audio runs ahead of visible hits/lives; paused revisions stall on fixed frame;
// background backlog replays obsolete sounds; catch-up paints several invisible
// frames at once; a restore keeps an old queue or rewinds a newer revision.
import test from "node:test";
import assert from "node:assert/strict";
import { RuntimePresentationQueue } from "./runtime-presentation.ts";
const state = (n) => ({
  revision: String(n),
  frame: String(n),
  audioTick: String(n),
});
test("50ms authority batches paint one accepted state per display frame with audio at that same state", () => {
  const shown = [],
    q = new RuntimePresentationQueue((s, resync) =>
      shown.push({ revision: s.revision, audioTick: s.audioTick, resync }),
    );
  q.reset(state(0), 0);
  for (let n = 1; n <= 3; n++) q.push(state(n));
  q.sample(0);
  q.sample(16.667);
  assert.equal(shown.length, 1);
  q.sample(33.334);
  assert.equal(shown.at(-1).revision, "1");
  for (let n = 4; n <= 6; n++) q.push(state(n));
  for (let t = 50; t <= 117; t += 1000 / 60) q.sample(t);
  assert.deepEqual(
    shown.map((s) => s.revision),
    ["0", "1", "2", "3", "4", "5", "6"],
  );
  assert.ok(shown.every((s) => s.audioTick === s.revision));
  assert.equal(shown.filter((s) => s.resync).length, 1);
});
test("paused presentation advances revisions while the game frame is fixed", () => {
  const shown = [],
    q = new RuntimePresentationQueue((s) => shown.push(s));
  q.reset(state(4), 0);
  q.push({ ...state(5), frame: "4" });
  q.push({ ...state(6), frame: "4" });
  q.sample(0);
  q.sample(34);
  q.sample(51);
  assert.deepEqual(
    shown.map((s) => [s.revision, s.frame]),
    [
      ["4", "4"],
      ["5", "4"],
      ["6", "4"],
    ],
  );
});
test("background or oversized backlog skips to the newest accepted state without replaying obsolete audio", () => {
  const shown = [],
    q = new RuntimePresentationQueue((s, resync) =>
      shown.push([s.revision, resync]),
    );
  q.reset(state(0), 0);
  for (let n = 1; n <= 20; n++) q.push(state(n));
  q.sample(1000);
  assert.deepEqual(shown, [
    ["0", true],
    ["20", true],
  ]);
  q.push(state(21));
  q.clear();
  q.sample(2000);
  assert.equal(shown.length, 2);
  q.reset(state(30), 2001);
  q.sample(2050);
  assert.deepEqual(shown.at(-1), ["30", true]);
});
test("a running RAF with no authority frames resynchronizes audio when delivery resumes", () => {
  const shown = [],
    q = new RuntimePresentationQueue((s, resync) =>
      shown.push([s.revision, resync]),
    );
  q.reset(state(0), 0);
  q.push(state(1));
  q.sample(0);
  q.sample(34);
  for (let at = 50; at < 600; at += 1000 / 60) q.sample(at);
  q.push(state(2));
  q.push(state(3));
  q.sample(600);
  assert.deepEqual(shown.at(-1), ["3", true]);
  assert.equal(shown.length, 3);
});
// Local players consume confirmed public state/audio immediately while their
// separate prediction draws; old queued states must not overwrite correction.
test("immediate authority delivery clears backlog without flagging a checkpoint resync", () => {
  const shown = [],
    q = new RuntimePresentationQueue((s, resync) =>
      shown.push([s.revision, resync]),
    );
  q.reset(state(0), 0);
  q.push(state(1));
  q.push(state(2));
  q.presentNow(state(3), 10);
  q.sample(50);
  assert.deepEqual(shown, [
    ["0", true],
    ["3", false],
  ]);
});
