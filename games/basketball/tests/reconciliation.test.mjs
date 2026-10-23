// Before implementation: a correction teleports walking players; linked balls
// separate; correction leaks into authority, survives reset/hidden/result or
// warps, accumulates forever, or produces invalid camera page coordinates.
import test from "node:test";
import assert from "node:assert/strict";
import { DisplayReconciliation } from "../web/reconciliation.ts";
function frame(x = 100, ball = x + 4) {
  return {
    frame: 100,
    display: {
      HasVisible: true,
      CourtEpoch: 1,
      Visible: {
        court: 1,
        phase: 1,
        camera: { x: 0, y: 48 },
        drawOrder: [0, 6],
        objects: [
          {
            id: 0,
            kind: 0,
            character: 0,
            pose: 1,
            action: 1,
            airborne: false,
            position: { x: x * 256, depth: 64 * 256, height: 0 },
            projected: { x, y: 100 },
          },
          {
            id: 6,
            kind: 8,
            character: 0,
            pose: 1,
            action: 2,
            airborne: false,
            position: { x: ball * 256, depth: 64 * 256, height: 4 * 256 },
            projected: { x: ball, y: 96 },
          },
        ],
      },
    },
  };
}
test("ordinary correction is continuous, bounded and does not mutate simulation", () => {
  const smoother = new DisplayReconciliation(),
    shown = frame(110),
    next = frame(100),
    before = structuredClone(next);
  smoother.correct(shown, next, 0);
  assert.equal(
    smoother.frame(next, 0).display.Visible.objects[0].position.x,
    110 * 256,
  );
  const half = smoother.frame(next, 80).display.Visible.objects[0].position.x;
  assert.ok(half > 100 * 256 && half < 110 * 256);
  assert.equal(
    smoother.frame(next, 200).display.Visible.objects[0].position.x,
    100 * 256,
  );
  assert.deepEqual(next, before);
});
test("held object keeps the same translation; unrelated ball is not dragged", () => {
  const smoother = new DisplayReconciliation(),
    next = frame(100);
  smoother.correct(frame(110), next, 0);
  const visible = smoother.frame(next, 40).display.Visible.objects;
  assert.equal(visible[1].position.x - visible[0].position.x, 4 * 256);
  smoother.correct(frame(110, 104), next, 50);
  assert.equal(
    smoother.frame(next, 50).display.Visible.objects[1].position.x,
    104 * 256,
  );
});
test("warps, hits, hidden frames, scene transitions and reset snap to authority", () => {
  for (const edit of [
    (v) => v.display.CourtEpoch++,
    (v) => (v.display.Visible.court = 2),
    (v) => (v.display.Visible.phase = 6),
    (v) => (v.display.Visible.objects[0].action = 96),
    (v) => (v.display.HasVisible = false),
    (v) => (v.display.Visible.objects[0].pose = 127),
  ]) {
    const smoother = new DisplayReconciliation(),
      next = frame(100);
    edit(next);
    smoother.correct(frame(110), next, 0);
    assert.deepEqual(smoother.frame(next, 0), next);
  }
  const smoother = new DisplayReconciliation(),
    next = frame(100);
  smoother.correct(frame(180), next, 0);
  assert.deepEqual(smoother.frame(next, 0), next);
  smoother.correct(frame(110), next, 0);
  smoother.reset();
  assert.deepEqual(smoother.frame(next, 0), next);
});
test("repeated corrections start from last shown position and stop after their deadline", () => {
  const smoother = new DisplayReconciliation(),
    raw = frame(100);
  smoother.correct(frame(110), raw, 0);
  const shown = smoother.frame(raw, 60),
    next = frame(98);
  smoother.correct(shown, next, 60);
  assert.equal(
    smoother.frame(next, 60).display.Visible.objects[0].position.x,
    shown.display.Visible.objects[0].position.x,
  );
  assert.deepEqual(smoother.frame(next, 300), next);
});
test("small camera corrections remain coherent with object projection", () => {
  const smoother = new DisplayReconciliation(),
    shown = frame(110),
    next = frame(100);
  shown.display.Visible.camera.x = 10;
  shown.display.Visible.objects.forEach((o) => (o.projected.x -= 10));
  smoother.correct(shown, next, 0);
  const visible = smoother.frame(next, 0).display.Visible;
  assert.equal(visible.camera.x, 10);
  assert.equal(
    visible.objects[0].projected.x,
    shown.display.Visible.objects[0].projected.x,
  );
  const across = frame(100);
  across.display.Visible.camera.y = 256;
  shown.display.Visible.camera.y = 239;
  smoother.correct(shown, across, 0);
  assert.deepEqual(smoother.frame(across, 0), across);
});
test("a hit or disappearance during decay clears stale display offsets", () => {
  for (const edit of [
    (v) => (v.display.Visible.objects[0].action = 96),
    (v) => (v.display.Visible.objects[0].airborne = true),
    (v) => (v.display.Visible.drawOrder = [6]),
  ]) {
    const smoother = new DisplayReconciliation(),
      next = frame(100);
    smoother.correct(frame(110), next, 0);
    edit(next);
    assert.deepEqual(smoother.frame(next, 30), next);
  }
});
