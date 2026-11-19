


import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolveBasketballAssetsBase } from "../web/asset-base.ts";
import release from "../assets/release-manifest.json" with { type: "json" };
import { BasketballAudio } from "../web/audio.ts";
test("default production assets use pinned OSS without local fallback", async () => {
  assert.equal(resolveBasketballAssetsBase(""), "/bga");
  const calls = [];
  const port = {
    music: async (s) => {
      calls.push(s);
      return true;
    },
    stop: () => {},
    stopMusic: () => {},
  };
  const sound = new BasketballAudio(port);
  assert.equal(calls.length, 0);
  await sound.consume([66], 1);
  assert.deepEqual(calls, [
    [release.files["/basketball/v1/audio/music-066-loop.mp3"].url],
  ]);
});
test("production collector never materializes basketball media or cover", () => {
  const vite = readFileSync(
    new URL("../../../web/vite.config.ts", import.meta.url),
    "utf8",
  );
  assert.doesNotMatch(
    vite,
    /prepareBasketballAssets\(|prepareBasketballCover\(/,
  );
});
