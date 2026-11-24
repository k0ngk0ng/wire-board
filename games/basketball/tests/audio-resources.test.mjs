import test from "node:test";
import assert from "node:assert/strict";
import { basketballAudioResources } from "../web/audio-resources.ts";
import catalog from "../assets/audio-catalog.json" with { type: "json" };
import playback from "../assets/audio-playback.json" with { type: "json" };

test("a single court prepares its music, results and all possible effects", () => {
  const files = basketballAudioResources({ options: { court: "new-york" } });
  assert.ok(files.some(file => file.url.endsWith("/music-066-loop.mp3")));
  assert.ok(!files.some(file => file.url.endsWith("/music-059-loop.mp3")));
  for (const clip of catalog.effects) assert.ok(files.some(file => file.url.endsWith("/" + (playback.effects[clip.id]?.wav ?? clip.wav))));
  for (const id of [75, 76]) assert.ok(files.some(file => file.url.endsWith("/" + playback.music[id].file)));
  assert.ok(files.every(file => file.bytes > 0));
});
test("a campaign prepares every court it can enter without downloading between matches", () => {
  const files = basketballAudioResources({ options: { court: "court-0", campaign: { entry: "new" } } });
  for (const clip of Object.values(playback.music)) assert.ok(files.some(file => file.url.endsWith("/" + clip.file)));
  assert.equal(files.length, new Set(files.map(file => file.url)).size);
});
