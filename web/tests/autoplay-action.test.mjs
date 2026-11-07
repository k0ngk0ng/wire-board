// Failure inventory: autoplay blocks a final held-input release; ordinary input
// bypasses autoplay; non-realtime or undeclared modules acquire an exemption.
import test from "node:test";
import assert from "node:assert/strict";
import { blocksAutoplayAction } from "../src/game-api/autoplay-action.ts";

test("autoplay permits only explicitly declared realtime input releases", () => {
  const module = { realtime: true, isInputRelease: (a) => a.type === "release" };
  assert.equal(blocksAutoplayAction(true, module, { type: "release" }), false);
  for (const action of [{ type: "input", held: 64 }, { type: "stage-ready" }, {}])
    assert.equal(blocksAutoplayAction(true, module, action), true);
  for (const unsupported of [undefined, {}, { realtime: true }, { ...module, realtime: false }])
    assert.equal(blocksAutoplayAction(true, unsupported, { type: "release" }), true);
});

test("manual control preserves the ordinary action path", () => {
  assert.equal(blocksAutoplayAction(false, undefined, { type: "input" }), false);
  assert.equal(blocksAutoplayAction(undefined, undefined, {}), false);
});
