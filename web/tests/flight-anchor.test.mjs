import test from "node:test";
import assert from "node:assert/strict";
import { flightCenter } from "../src/flight-anchor.ts";

const viewport = { left: 0, top: 0, right: 390, bottom: 844 };
const rect = (left, top, width = 30, height = 30) => ({
  left,
  top,
  right: left + width,
  bottom: top + height,
});

test("flight anchors follow the real seat after horizontal and document scroll", () => {
  const strip = { left: 12, top: 60, right: 378, bottom: 190 };
  assert.deepEqual(flightCenter(rect(250, 85), [viewport, strip]), {
    x: 265,
    y: 100,
  });
  assert.deepEqual(flightCenter(rect(190, 85), [viewport, strip]), {
    x: 205,
    y: 100,
  });
  assert.deepEqual(
    flightCenter(rect(190, 65), [viewport, { ...strip, top: 40, bottom: 170 }]),
    { x: 205, y: 80 },
  );
});

test("clipped players never get projected onto another visible seat or screen edge", () => {
  const strip = { left: 30, top: 60, right: 340, bottom: 190 };
  // Both centers are inside the screen, but hidden by the nested scroll area.
  assert.equal(flightCenter(rect(0, 85), [viewport, strip]), null);
  assert.equal(flightCenter(rect(345, 85), [viewport, strip]), null);
  assert.equal(flightCenter(rect(500, 85), [viewport, strip]), null);
  assert.equal(flightCenter(rect(200, -40), [viewport, strip]), null);
  assert.deepEqual(flightCenter(rect(320, 85), [viewport, strip]), {
    x: 335,
    y: 100,
  });
});

test("mobile visual viewport and disappearing anchors cannot yield an incorrect endpoint", () => {
  const visual = { left: 40, top: 100, right: 300, bottom: 500 };
  assert.equal(flightCenter(rect(50, 30), [visual]), null);
  assert.equal(flightCenter(rect(50, 600), [visual]), null);
  assert.equal(flightCenter(rect(50, 150, 0), [visual]), null);
  assert.equal(flightCenter(rect(NaN, 150), [visual]), null);
  assert.deepEqual(flightCenter(rect(50, 150), [visual]), { x: 65, y: 165 });
});
