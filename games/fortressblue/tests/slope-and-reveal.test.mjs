// Failure scenarios before implementation: flat/up/down supports; reversed facing;
// a crater removes one foot; a distant lower platform must not be used as support;
// invalid frame metadata or atlas ordering must never cross native groups;
// non-slope actions stay unchanged. One-time reveal must skip selection, already
// visible boards, boards taller than the viewport, and ordinary snapshots.
import test from "node:test";
import assert from "node:assert/strict";
import { supportSlope, selectSlopeFrame } from "../web/slope.ts";
import { shouldRevealBoard } from "../web/reveal.ts";

test("slope follows nearby solid support, including current crater holes", () => {
  const flat = (x, y) => y >= 100;
  assert.equal(supportSlope({ x: 100, y: 100 }, flat).slope, 0);
  const down = (x, y) => y >= 100 + (x - 100) * 0.5;
  assert.equal(supportSlope({ x: 100, y: 100 }, down).slope, 1);
  const up = (x, y) => y >= 100 - (x - 100) * 0.5;
  assert.equal(supportSlope({ x: 100, y: 100 }, up).slope, -1);
  assert.equal(
    supportSlope({ x: 100, y: 100 }, (x, y) => y >= 100 + x - 100).slope,
    2,
  );
  const crater = (x, y) => flat(x, y) && Math.hypot(x - 112, y - 100) > 30;
  assert.equal(supportSlope({ x: 100, y: 100 }, crater).slope, 0);
  const lowerPlatform = (x, y) => y >= 200;
  assert.equal(supportSlope({ x: 100, y: 100 }, lowerPlatform).slope, 0);
});
test("native slope frame keeps direction, group, and original anchor", () => {
  const frames = Array.from({ length: 10 }, (_, i) => ({
    sourceIndex: 25 + Math.floor(i / 2),
    facing: i % 2 ? "left" : "right",
    ax: i,
    ay: 20 + i,
  }));
  assert.equal(selectSlopeFrame(frames, 4, 2, 1, 5), frames[0]);
  assert.equal(selectSlopeFrame(frames, 5, 2, -1, 5), frames[9]);
  assert.equal(selectSlopeFrame(frames, 4, -2, 1, 5), frames[8]);
  assert.equal(selectSlopeFrame(frames, 4, 2, 1, 1), frames[4]);
  const broken = frames.map((f) => ({ ...f }));
  broken[0].sourceIndex = 30;
  assert.equal(selectSlopeFrame(broken, 4, 2, 1, 5), broken[4]);
  broken[0] = { ...frames[0], facing: "left" };
  assert.equal(selectSlopeFrame(broken, 4, 2, 1, 5), broken[4]);
});
test("entering play reveals a fitting board at most once, never the select overlay", () => {
  const rect = { top: 400, bottom: 1080, height: 680 };
  assert.equal(shouldRevealBoard(rect, 844, "playing", false), true);
  assert.equal(shouldRevealBoard(rect, 844, "select", false), false);
  assert.equal(shouldRevealBoard(rect, 844, "playing", true), false);
  assert.equal(
    shouldRevealBoard(
      { top: 0, bottom: 680, height: 680 },
      844,
      "playing",
      false,
    ),
    false,
  );
  assert.equal(
    shouldRevealBoard(
      { top: -30, bottom: 650, height: 680 },
      844,
      "playing",
      false,
    ),
    true,
  );
  assert.equal(
    shouldRevealBoard(
      { top: 200, bottom: 1100, height: 900 },
      844,
      "playing",
      false,
    ),
    false,
  );
});
