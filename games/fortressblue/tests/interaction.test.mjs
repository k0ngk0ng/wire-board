// Failure scenarios specified before implementation: duplicate keydown/release;
// pointer cancellation, blur/turn change cancelling charge; zoom anchor drift;
// dragging outside bounds; refresh mid-flight restarting the arc; missing times;
// falling before server start; forbidden input from editors/dialogs.
import test from "node:test";
import assert from "node:assert/strict";
import {
  beginCharge,
  releaseCharge,
  finishFullCharge,
  cancelCharge,
  chargePower,
  acceptsGameKeys,
} from "../web/controls.ts";
import { zoomAt, panCamera, centeredCamera } from "../web/camera.ts";
import { arcAt, fallAt, renderableShot } from "../web/timeline.ts";
test("a hold sends exactly once and cancellation never fires", () => {
  const state = { start: null, token: "", source: "" };
  assert.equal(beginCharge(state, 100, "turn-1", "key"), true);
  assert.equal(beginCharge(state, 200, "turn-1", "key"), false);
  assert.equal(releaseCharge(state, 1000, "turn-1", "pointer"), null);
  assert.equal(releaseCharge(state, 1000, "turn-1", "key"), 60);
  assert.equal(releaseCharge(state, 1100, "turn-1", "key"), null);
  beginCharge(state, 1200, "turn-1", "pointer");
  cancelCharge(state);
  assert.equal(releaseCharge(state, 2000, "turn-1", "pointer"), null);
  beginCharge(state, 2100, "turn-1", "key");
  assert.equal(releaseCharge(state, 2300, "turn-2", "key"), null);
  assert.equal(chargePower(100, 10000), 100);
});
test("camera zoom keeps the pointer world anchor, panning is bounded", () => {
  const c = { x: 200, y: 100, worldViewW: 800, worldViewH: 450 };
  const n = zoomAt(c, 0.8, { x: 0.25, y: 0.75 }, 1500, 960);
  assert.equal(n.x + n.worldViewW * 0.25, c.x + c.worldViewW * 0.25);
  assert.equal(n.y + n.worldViewH * 0.75, c.y + c.worldViewH * 0.75);
  const end = panCamera(c, 10000, -10000, 1500, 960);
  assert.equal(end.x, 700);
  assert.equal(end.y, 0);
  assert.equal(
    centeredCamera(800, 450, 800, { x: 50, y: 800 }, 1500, 960).worldViewW,
    800,
  );
  // The server permits shots up to y=-240; automatic tracking must show them.
  const sky = centeredCamera(800, 450, 800, { x: 500, y: -220 }, 1500, 960);
  assert.ok(sky.y <= -220 && sky.y + sky.worldViewH >= -220);
});
test("absolute timeline resumes at the actual flight and fall position", () => {
  const arc = {
    path: [
      { x: 0, y: 0 },
      { x: 100, y: 100 },
      { x: 200, y: 0 },
    ],
    pathTimes: [0, 1000, 2000],
    impact: { x: 200, y: 0 },
    hit: -1,
    impactAt: 7000,
  };
  assert.equal(arcAt(arc, 5000, 6500).x, 150);
  assert.equal(arcAt(arc, 5000, 7000), null);
  assert.equal(arcAt(arc, 5000, 4999), null);
  const f = {
    seat: 0,
    fromX: 20,
    fromY: 40,
    toX: 20,
    toY: 140,
    at: 7000,
    endAt: 8000,
  };
  assert.deepEqual(fallAt(f, 6500), { x: 20, y: 40 });
  assert.deepEqual(fallAt(f, 8000), { x: 20, y: 140 });
  assert.equal(fallAt(f, 7500).y, 65);
});
test("keyboard never steals editor, select or dialog input", () => {
  for (const target of [
    { tagName: "INPUT" },
    { tagName: "TEXTAREA" },
    { tagName: "SELECT" },
    { isContentEditable: true },
    { closest: () => ({}) },
  ])
    assert.equal(acceptsGameKeys(target), false);
  assert.equal(
    acceptsGameKeys({ tagName: "CANVAS", closest: () => null }),
    true,
  );
});

// Native evidence follow-up, specified before changing charging: normal holds
// begin at zero; max ticks vary by tank Range; reaching max consumes the same
// one-shot latch as keyup. Early ticks, cancellation, changed turn, repeated
// max callbacks and a later release must never add another shot.
test("native charge starts at zero and fills by each tank's Range ticks", () => {
  assert.equal(chargePower(100, 100, 45), 0);
  assert.equal(chargePower(100, 1100, 45), 66);
  assert.equal(chargePower(100, 1100, 60), 50);
  assert.equal(chargePower(100, 1600, 45), 100);
  assert.equal(chargePower(100, 1600, 60), 75);
});
test("full power auto-fires once and the eventual keyup cannot fire", () => {
  const state = { start: null, token: "", source: "" };
  beginCharge(state, 100, "turn-1", "key", 45);
  assert.equal(finishFullCharge(state, 1599, "turn-1"), null);
  assert.equal(finishFullCharge(state, 1600, "turn-1"), 100);
  assert.equal(finishFullCharge(state, 1650, "turn-1"), null);
  assert.equal(releaseCharge(state, 1700, "turn-1", "key"), null);
  beginCharge(state, 2000, "turn-2", "pointer", 60);
  assert.equal(finishFullCharge(state, 3500, "turn-2"), null);
  cancelCharge(state);
  assert.equal(finishFullCharge(state, 4000, "turn-2"), null);
  beginCharge(state, 5000, "turn-2", "pointer", 45);
  assert.equal(finishFullCharge(state, 6500, "turn-3"), null);
  assert.equal(releaseCharge(state, 6600, "turn-3", "pointer"), null);
});
// Native Fire frame 5 launches the projectile. The server supplies launchAt
// and offsets pathTimes from shot.at; windup must not show/track the muzzle.
test("server windup hides the projectile until launch and reload keeps progress", () => {
  const arc = {
    path: [
      { x: 10, y: 20 },
      { x: 110, y: 120 },
    ],
    pathTimes: [334, 1334],
    impact: { x: 110, y: 120 },
    hit: -1,
    impactAt: 6334,
  };
  assert.equal(arcAt(arc, 5000, 5200, 5334), null);
  assert.equal(arcAt(arc, 5000, 5333, 5334), null);
  assert.equal(arcAt(arc, 5000, 5334, 5334).x, 10);
  assert.equal(arcAt({ ...arc, pathTimes: undefined }, 5000, 5334, 5334).x, 10);
  assert.equal(arcAt(arc, 5000, 5834, 5334).x, 60);
  assert.equal(arcAt(arc, 5000, 6334, 5334), null);
});

// A cancelled pre-launch LastShot remains diagnostic data. Neither the camera
// nor the renderer may reuse its future path or blast after the shooter dies.
test("cancelled shots are not available to projectile, blast or camera rendering", () => {
  const shot = { id: 2, at: 5000, launchAt: 5334, cancelledAt: 5100, arcs: [] };
  assert.equal(renderableShot(shot), undefined);
  const active = { ...shot, cancelledAt: undefined };
  assert.equal(renderableShot(active), active);
  assert.equal(renderableShot(undefined), undefined);
});
