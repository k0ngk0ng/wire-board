// Before implementation: prediction must match server launch arcs, stop at a
// bounded horizon, never invent a collision/outcome, and clear on scope changes.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { predictPath, BallDisplay } from "../web/prediction.ts";
import { sampleFlight } from "../web/timeline.ts";
const rows = JSON.parse(
  readFileSync(new URL("./testdata/prediction-oracle.json", import.meta.url)),
);
test("72 launch variants agree with server physics at 288 sample points", () => {
  assert.equal(rows.length, 72);
  for (const r of rows) {
    const path = predictPath(r.view, r.seat, r.stroke, r.at);
    for (const want of r.points) {
      const p = sampleFlight(path, want.at);
      for (const axis of ["x", "y", "z"])
        assert.ok(
          Math.abs(p[axis] - want.p[axis]) < 1e-8,
          `${r.name} ${axis}: ${p[axis]} != ${want.p[axis]}`,
        );
    }
  }
});
test("before contact, display stays on the incoming authority; prediction is bounded", () => {
  const r = rows.find((r) => r.view.phase === "rally");
  const q = new BallDisplay();
  q.start(r.view, r.seat, r.stroke, r.at, "same");
  const early = q.sample(r.view, r.at - 1, "same");
  assert.deepEqual(early, sampleFlight(r.view.flight, r.at - 1));
  const cap = q.sample(r.view, r.at + 250, "same"),
    late = q.sample(r.view, r.at + 1500, "same");
  assert.deepEqual(cap, late);
});
test("a new scope discards all predicted movement", () => {
  const r = rows.find((r) => r.view.phase === "rally");
  const q = new BallDisplay();
  q.start(r.view, r.seat, r.stroke, r.at, "old");
  assert.deepEqual(
    q.sample(r.view, r.at + 150, "new"),
    sampleFlight(r.view.flight, r.at + 150),
  );
});
test("authoritative new point cancels prediction instead of manufacturing score", () => {
  const r = rows.find((r) => r.view.phase === "rally");
  const q = new BallDisplay();
  q.start(r.view, r.seat, r.stroke, r.at, "same");
  const next = { ...r.view, rally: r.view.rally + 1, phase: "point_break" };
  assert.deepEqual(
    q.sample(next, r.at + 100, "same"),
    sampleFlight(next.flight, r.at + 100),
  );
  assert.deepEqual(next.score, r.view.score);
});
