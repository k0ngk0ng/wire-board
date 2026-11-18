// Real GameMain before JS continuation-stack support: the same-instance menu
// appears valid while a copied memory image loses its C stack pointer and
// corrupts a nested inventory menu; observers overwrite suspended local arrays.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { create, root } from "./native-vm.mjs";
const vm = await create();
let ev = vm.step("wb_run_campaign");
const state = () =>
  Array.from(new Uint32Array(vm.e.memory.buffer, vm.e.wb_view(), 32));
for (let i = 0; i < 4096; i++) {
  const v = state();
  if (ev.kind === 4 && v[1] === 2 && v[17] === 0 && v[15] === 72) break;
  ev = vm.step("wb_run_campaign", 0, ev.kind === 2 ? 1 : 0);
  if (i === 4095) throw Error("original inn unavailable");
}
function menu(v) {
  const p = v.e.wb_menu_info(),
    head = Array.from(new Int32Array(v.e.memory.buffer, p, 6));
  return {
    depth: head[0],
    kind: head[1],
    count: head[2],
    selected: head[3],
    items: Array.from(new Int32Array(v.e.memory.buffer, p + 24, head[2] * 5)),
  };
}
for (let i = 0; i < 8 && menu(vm).kind !== 1; i++)
  ev = vm.step("wb_run_campaign", 0, 0, i === 0 ? 1 : 0);
assert.equal(menu(vm).kind, 1);
assert.notEqual(
  vm.e.__stack_pointer.value,
  vm.e.__stack_high.value,
  "fixture must exercise a live C stack frame",
);
const saved = vm.snapshot(),
  fresh = await create();
fresh.restore(saved);
for (const key of [4, 4, 2]) {
  const a = vm.step("wb_run_campaign", 0, 0, key),
    b = fresh.step("wb_run_campaign", 0, 0, key);
  assert.deepEqual(
    a,
    b,
    "fresh native continuation differs from the original instance",
  );
  assert.deepEqual(menu(vm), menu(fresh), "fresh native menu lost local items");
}
assert.equal(menu(fresh).kind, 4);
assert.deepEqual(menu(fresh).items, [1, 22, 1, 43, 73, 2, 23, 1, 43, 91]);
const report = {
  passed: true,
  scope:
    "actual original GameMain -> main/inventory menus; native suspended C stack pointer and independent-instance nested-menu continuation",
  menu: menu(fresh),
  stackPointer: fresh.e.__stack_pointer.value,
};
await fs.writeFile(
  root + ".local/pal/verification/native-field-menus-js.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log("PASS original nested menus and suspended C stack continuation");
