import test from "node:test";
import assert from "node:assert/strict";
import {
  relativeFlightBounds,
  projectFlightBounds,
} from "../src/card-flight-origin.ts";
import { flightCenter } from "../src/flight-anchor.ts";

const rect = (left, top, width, height) => ({
  left,
  top,
  right: left + width,
  bottom: top + height,
});
test("removed hand cards retain their original slot through page and nested scrolling", () => {
  const original = rect(280, 610, 64, 90);
  const saved = relativeFlightBounds(original, rect(100, 580, 260, 140), {
    scrollLeft: 40,
    scrollTop: 15,
  });
  assert.deepEqual(
    projectFlightBounds(saved, rect(100, 460, 260, 140), {
      scrollLeft: 100,
      scrollTop: 25,
    }),
    rect(220, 480, 64, 90),
  );
  // The slot still tracks its reference even when another card fills its old DOM position.
  assert.deepEqual(
    projectFlightBounds(saved, rect(110, 450, 260, 140), {
      scrollLeft: 100,
      scrollTop: 25,
    }),
    rect(230, 470, 64, 90),
  );
});
test("a removed noble departs from its previous slot, not the remaining noble's new center", () => {
  const reference = rect(10, 200, 370, 80);
  const origin = relativeFlightBounds(rect(275, 210, 50, 50), reference, {
    scrollLeft: 0,
    scrollTop: 0,
  });
  const moved = projectFlightBounds(origin, rect(10, 170, 370, 80), {
    scrollLeft: 0,
    scrollTop: 0,
  });
  assert.deepEqual(flightCenter(moved, [rect(0, 0, 390, 844)]), {
    x: 300,
    y: 205,
  });
  // Clipping hides this source rather than projecting it onto a visible card.
  assert.equal(
    flightCenter(moved, [rect(0, 0, 390, 844), rect(10, 170, 250, 80)]),
    null,
  );
});

test("a confirmed fixed-panel card keeps its screen origin after the panel disappears", async (t) => {
  const { captureFixedCardOrigin, resolveCardOrigin } =
    await import("../src/card-flight-origin.ts");
  const oldWindow = globalThis.window,
    oldDocument = globalThis.document;
  t.after(() => {
    if (oldWindow === undefined) delete globalThis.window;
    else globalThis.window = oldWindow;
    if (oldDocument === undefined) delete globalThis.document;
    else globalThis.document = oldDocument;
  });
  const viewport = { clientWidth: 390, clientHeight: 844, scrollTop: 0 };
  globalThis.document = { documentElement: viewport };
  globalThis.window = {};
  const element = { getBoundingClientRect: () => rect(145, 540, 100, 140) };
  const source = captureFixedCardOrigin(element);
  element.getBoundingClientRect = () => rect(0, 0, 0, 0);
  viewport.scrollTop = 400;
  assert.deepEqual(resolveCardOrigin(source), { x: 195, y: 610 });
  viewport.clientWidth = 430;
  assert.equal(resolveCardOrigin(source), null);
  viewport.clientWidth = 390;
  viewport.clientHeight = 500;
  assert.equal(resolveCardOrigin(source), null);
  viewport.clientHeight = 844;
  // Browser zoom/keyboard clipping must not invent a departure at the edge.
  globalThis.window.visualViewport = {
    offsetLeft: 0,
    offsetTop: 0,
    width: 390,
    height: 500,
  };
  assert.equal(resolveCardOrigin(source), null);
});
