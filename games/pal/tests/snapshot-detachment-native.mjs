// Test first: a verification snapshot must own its bytes. Buffer.from(the
// ArrayBuffer) aliases live Wasm memory and invalidates independent references.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { create, root, digest, wasm } from "./native-vm.mjs";
const vm = await create();
let event = vm.step("wb_run_campaign");
const state = () => Array.from(new Int32Array(vm.e.memory.buffer, vm.e.wb_view(), 32));
for (let i = 0; i < 4096; i++) {
  if (event.kind === 4 && state()[1] === 2 && state()[17] === 0 && state()[15] === 72) break;
  event = vm.step("wb_run_campaign", 0, event.kind === 2 ? 1 : 0);
  assert.notEqual(i, 4095);
}
const saved = vm.snapshot(), expected = Buffer.from(saved), identity = digest(saved);
expected.stackPointer = saved.stackPointer;
const fresh = await create(); fresh.restore(saved);
vm.e.wb_probe_field_config(3, 0, 3, 1); vm.e.wb_probe_field_move(0);
assert.equal(digest(saved), identity, "verification snapshot aliases live native memory");
const liveBefore = vm.snapshot(); saved[0] ^= 1;
assert.equal(vm.snapshot()[0], liveBefore[0], "caller snapshot mutation changes the native instance");
saved[0] ^= 1;
assert.deepEqual(saved, expected);
assert.equal(digest(fresh.snapshot()), identity, "one instance changed another restored instance");
await fs.writeFile(root + ".local/pal/verification/snapshot-detachment-js.json", JSON.stringify({ passed: true, authoritySHA256: digest(wasm), scope: "actual original inn snapshot, independent original field motor mutation and fresh instance; verification helper owns native bytes" }, null, 2) + "\n");
console.log("PASS detached JS native verification snapshots");
