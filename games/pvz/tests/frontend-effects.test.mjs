import assert from "node:assert/strict";
import test from "node:test";
import { PvzEffects } from "../web/effects.ts";

function fixture() {
  const live = new Map(), calls = [];
  const port = {
    playWithFallback() { throw Error("the shared single effect lane must not be used"); },
    track(key, sources, options) { live.set(key, { sources, options }); calls.push({ key, sources, options }); return Promise.resolve(true); },
    stopTrack(key) { live.delete(key); },
  };
  const effects = new PvzEffects(port, "https://example.test/pvz/classic-v2", { shoot: "audio/throw.ogg", hit: "audio/splat.ogg", freeze: "audio/frozen.ogg" });
  effects.consume("round-1", [], true, true);
  return { effects, live, calls };
}
const event = (id, type, tick = 10) => ({ id, type, tick, row: 0, x: 1000 });

test("same-frame different effects remain alive in separate native tracks", () => {
  const { effects, live, calls } = fixture();
  effects.consume("round-1", [event(1, "shoot"), event(2, "hit"), event(3, "freeze")], true, true);
  assert.equal(live.size, 3);
  assert.equal(new Set(calls.map(call => call.key)).size, 3);
  assert.ok(calls.every(call => call.options.loop === false));
  assert.deepEqual(calls.map(call => call.options.playbackID), ["round-1:1", "round-1:2", "round-1:3"]);
  effects.consume("round-1", [event(1, "shoot"), event(2, "hit"), event(3, "freeze")], true, true);
  assert.equal(calls.length, 3, "snapshot replay must not restart audio");
  effects.consume("round-1", [event(4, "shoot"), event(4, "shoot")], true, true);
  assert.equal(calls.length, 4, "one event ID must have one voice even inside a duplicate batch");
});
test("mute and hidden cleanup advance watermark and never replay on restore", () => {
  const { effects, live, calls } = fixture();
  effects.consume("round-1", [event(1, "shoot")], true, true);
  effects.consume("round-1", [event(2, "hit")], false, true);
  assert.equal(live.size, 0);
  effects.consume("round-1", [event(1, "shoot"), event(2, "hit")], true, true);
  assert.equal(calls.length, 1);
  effects.consume("round-1", [event(3, "freeze")], true, false);
  effects.consume("round-1", [event(3, "freeze")], true, true);
  assert.equal(calls.length, 1);
  effects.consume("round-1", [event(4, "hit")], true, true);
  assert.equal(live.size, 1);
});
test("restored rounds skip history and the fixed lane set stays bounded", () => {
  const { effects, live, calls } = fixture();
  effects.consume("round-1", Array.from({ length: 20 }, (_, i) => event(i + 1, i % 2 ? "hit" : "shoot")), true, true);
  assert.ok(live.size <= 8);
  effects.consume("round-2", [event(50, "freeze")], true, true);
  assert.equal(live.size, 0);
  const count = calls.length;
  effects.consume("round-2", [event(51, "hit")], true, true);
  assert.equal(calls.length, count + 1);
  effects.dispose(); assert.equal(live.size, 0);
  effects.consume("round-2", [event(52, "shoot")], true, true);
  assert.equal(live.size, 0);
});
test("visibility cleanup at a new epoch accepts its own new event sequence", () => {
  const { effects, calls } = fixture();
  effects.consume("round-1", [event(100, "shoot")], true, true);
  effects.silence("round-2", [event(1, "hit")]);
  effects.consume("round-2", [event(1, "hit"), event(2, "freeze")], true, true);
  assert.equal(calls.length, 2);
  assert.equal(calls[1].options.playbackID, "round-2:2");
});
