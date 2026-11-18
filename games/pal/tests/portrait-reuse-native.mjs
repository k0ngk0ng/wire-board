// Test first with real original archive reads and original C portrait blitting.
// Failures: reusing a small portrait buffer inside an older large FBP buffer
// selects the stale background binding; a repeated exact binding loses recency;
// a valid portrait is silently skipped or interpreted as a sprite atlas.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { root, wasm, digest, create } from "./native-vm.mjs";

const results = [];
for (const mode of [0, 1, 2]) {
  const vm = await create();
  assert.equal(typeof vm.e.wb_probe_portrait_reuse, "function");
  const event = vm.step("wb_probe_portrait_reuse", mode);
  assert.equal(event.kind, 3);
  const images = event.ops.filter((op) => op[0] === 1);
  const expected = mode === 1 ? [] : [[1, mode === 2 ? 5 : 1, mode === 2 ? 20 : 4, 0, 9, 10, 0, 0]];
  assert.deepEqual(
    images,
    expected,
    "original reloaded portrait/item or decompressed actor inherited an overlapping old UI binding",
  );
  results.push({ mode, images });
}
await fs.writeFile(
  root + ".local/pal/verification/portrait-reuse-native.json",
  JSON.stringify({
    passed: true,
    scope: "actual original RGM4 read, overlapping FBP24 read, RGM4 reload and original C RLE blit; no campaign/stat modification",
    authoritySHA256: digest(wasm),
    results,
  }, null, 2) + "\n",
);
console.log("PASS original portrait reload after overlapping background buffer");
