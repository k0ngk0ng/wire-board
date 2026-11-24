import test from "node:test";
import assert from "node:assert/strict";
import playback from "../assets/audio-playback.json" with { type: "json" };
import { yuyuAudioResources } from "../web/audio-resources.ts";

test("a match preloads all effects and its actual arena music before play", () => {
  const files = yuyuAudioResources({ game: { data: { options: { arena: 0 } } }, options: { arena: 14 } });
  assert.ok(files.some(file => file.url.endsWith("/82.flac")));
  assert.ok(!files.some(file => file.url.endsWith("/8d.flac")));
  for (const clip of Object.values(playback.effects)) assert.ok(files.some(file => file.url.endsWith("/" + clip.file) && file.bytes === clip.bytes));
  assert.equal(new Set(files.map(file => file.url)).size, files.length);
});
test("local fixtures use the same exact paths as local playback", () => {
  const files = yuyuAudioResources({ game: { data: { options: { arena: 2 } } } }, "/bga");
  assert.ok(files.some(file => file.url === "/bga/yuyuhakusho/audio/86.flac"));
  assert.ok(files.every(file => file.bytes > 0));
});
