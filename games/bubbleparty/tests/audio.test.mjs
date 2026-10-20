// Failure cases: logical events point at missing files; trap/rescue use unrelated
// sounds; switching to the new asset version loses same-origin fallback.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { sources } from "../web/types.ts";
test("every gameplay sound resolves to shipped classic audio with local fallback", () => {
  for (const event of [
    "place",
    "explode",
    "trap",
    "rescue",
    "eliminate",
    "item",
    "shield",
    "sudden",
    "win",
    "lose",
    "draw",
    "start",
    "countdown",
    "battle",
  ]) {
    const list = sources("https://assets.example.test", event + ".wav");
    assert.equal(list.length, 2);
    assert.ok(list[1].startsWith("/bubbleparty/v2/audio/"));
    const bytes = readFileSync(
      new URL("../assets/public" + list[1], import.meta.url),
    );
    assert.ok(bytes.length > 100);
    assert.equal(list[0], "https://assets.example.test" + list[1]);
    assert.deepEqual(sources("", event + ".wav"), [list[1]]);
  }
  assert.ok(sources("", "trap.wav")[0].endsWith("/LockedIn.wav"));
  assert.ok(sources("", "rescue.wav")[0].endsWith("/Release.wav"));
  assert.ok(sources("", "battle.wav")[0].endsWith("/Village.mp3"));
});
