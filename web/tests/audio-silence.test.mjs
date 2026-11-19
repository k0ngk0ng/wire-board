import test from "node:test";
import assert from "node:assert/strict";
import { silenceDataURL } from "../src/audio-silence.ts";

test("gesture unlock generates the original short PCM silence without a stored media payload", () => {
  const url = silenceDataURL();
  assert.ok(url.startsWith("data:audio/wav;base64,"));
  const bytes = Buffer.from(url.split(",")[1], "base64");
  assert.equal(bytes.length, 46);
  assert.equal(bytes.toString("ascii", 0, 4), "RIFF");
  assert.equal(bytes.readUInt32LE(4), 38);
  assert.equal(bytes.toString("ascii", 8, 12), "WAVE");
  assert.equal(bytes.readUInt16LE(20), 1);
  assert.equal(bytes.readUInt16LE(22), 1);
  assert.equal(bytes.readUInt32LE(24), 8000);
  assert.equal(bytes.readUInt16LE(34), 8);
  assert.equal(bytes.readUInt32LE(40), 2);
  assert.deepEqual([...bytes.subarray(44)], [128, 128]);
});
