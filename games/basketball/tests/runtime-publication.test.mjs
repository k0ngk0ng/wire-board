// Failure inventory: source/runtime mismatch, mutable URL, checksum/size
// mismatch, accidental inclusion of Wasm in the container after a warm build.
import test from "node:test";
import assert from "node:assert/strict";
import { verifyRuntimePin } from "../build/runtime-delivery.mjs";
const m = {
  sourceSHA256: "a".repeat(64),
  files: { "basketball.wasm": { sha256: "b".repeat(64), bytes: 5000 } },
};
const p = {
  sourceSHA256: m.sourceSHA256,
  ...m.files["basketball.wasm"],
  url:
    "https://cdn.ichenj.com/bga/basketball/runtime-" +
    "b".repeat(64) +
    "/basketball.wasm",
};
test("only exact compiled runtime and immutable URL can ship", () => {
  assert.doesNotThrow(() => verifyRuntimePin(m, p));
  for (const change of [
    { sourceSHA256: "c".repeat(64) },
    { sha256: "c".repeat(64) },
    { bytes: 4999 },
    { url: "https://cdn.ichenj.com/latest.wasm" },
  ])
    assert.throws(() => verifyRuntimePin(m, { ...p, ...change }));
});
