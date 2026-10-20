import test from "node:test";
import assert from "node:assert/strict";
import {
  spriteFrames,
  frameRect,
  spriteSourceCandidates,
  loadSprites,
  getSprite,
} from "../web/sprites.ts";

test("sprite sheets retain source frame geometry and clamp frames", () => {
  assert.deepEqual(frameRect("role1", 5, 3), {
    sx: 240,
    sy: 192,
    sw: 48,
    sh: 64,
  });
  assert.deepEqual(frameRect("role2", 99, -1), {
    sx: 280,
    sy: 0,
    sw: 56,
    sh: 67,
  });
  assert.equal(spriteFrames.bigBubble.columns, 9);
  assert.equal(spriteFrames.blast.rows, 5);
  assert.equal(spriteFrames.bubble.columns, 3);
});

test("versioned sources use a local fallback without duplicate local URLs", () => {
  assert.deepEqual(spriteSourceCandidates("", "Popo.png"), [
    "/bubbleparty/v2/sprites/Popo.png",
  ]);
  assert.deepEqual(
    spriteSourceCandidates("https://cdn.example/base/", "Popo.png"),
    [
      "https://cdn.example/base/bubbleparty/v2/sprites/Popo.png",
      "/bubbleparty/v2/sprites/Popo.png",
    ],
  );
});

test("failed CDN falls back locally and fully failed image stays unavailable", async () => {
  const requests = [];
  globalThis.Image = class {
    complete = false;
    naturalWidth = 0;
    set src(url) {
      requests.push(url);
      queueMicrotask(() => {
        if (url.startsWith("https:") || url.endsWith("/BigPopo.png"))
          this.onerror?.();
        else {
          this.complete = true;
          this.naturalWidth = 48;
          this.onload?.();
        }
      });
    }
  };
  const result = await loadSprites("https://cdn.example/test");
  assert.ok(result.loaded > 0);
  assert.ok(result.failed.includes("bigBubble"));
  assert.ok(getSprite("role1"));
  assert.equal(getSprite("bigBubble"), null);
  const count = requests.length;
  await loadSprites("https://cdn.example/test");
  assert.equal(requests.length, count);
});
