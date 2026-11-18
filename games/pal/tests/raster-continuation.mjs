// Original battle E2E: repeated real UI redraws must not exhaust the graph
// arena, collection must retain live backups, and fresh saved continuations
// must preserve dependencies without advancing combat or spending resources.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { create, root } from "./native-vm.mjs";
const vm = await create();
vm.e.wb_platform_players(3);
const battle = (v) =>
  Array.from(new Int32Array(v.e.memory.buffer, v.e.wb_battle_info(), 87));
const graph = (v) => {
  const p = v.e.wb_raster_graph(),
    h = Array.from(new Int32Array(v.e.memory.buffer, p, 3));
  return {
    version: h[0],
    root: h[2],
    nodes: Array.from(new Int32Array(v.e.memory.buffer, p + 12, h[1] * 16)),
  };
};
let event = vm.step("wb_run_campaign");
for (let i = 0; battle(vm)[3] !== 1; i++) {
  assert.ok(i < 512);
  event = vm.step("wb_run_campaign");
}
const hp = battle(vm).slice(16, 52);
let largest = 0;
for (let i = 0; i < 2000; i++) {
  event = vm.step("wb_run_campaign");
  if (i % 100 === 0) {
    const g = graph(vm);
    largest = Math.max(largest, g.nodes.length / 16);
    assert.ok(
      g.root > 0 &&
        g.root <= g.nodes.length / 16 &&
        g.nodes.length / 16 <= 16384,
    );
    for (let j = 0; j < g.nodes.length / 16; j++) {
      assert.ok(
        g.nodes[j * 16 + 1] <= j && g.nodes[j * 16 + 2] <= j,
        "collection corrupted an original surface dependency",
      );
    }
  }
}
assert.deepEqual(
  battle(vm).slice(16, 52),
  hp,
  "idle drawing altered original characters",
);
const save = vm.snapshot(),
  fresh = await create();
fresh.restore(save);
fresh.e.wb_release();
assert.deepEqual(graph(vm), graph(fresh));
for (let i = 0; i < 24; i++) {
  const a = vm.step("wb_run_campaign"),
    b = fresh.step("wb_run_campaign");
  assert.deepEqual(a, b);
  assert.deepEqual(graph(vm), graph(fresh));
  assert.deepEqual(battle(vm), battle(fresh));
}
await fs.writeFile(
  root + ".local/pal/verification/raster-continuation-js.json",
  JSON.stringify(
    {
      passed: true,
      scope:
        "2000 original battle UI redraws, bounded immutable surface dependencies, independent-instance saved continuation",
      largestExportedGraph: largest,
      memoryBytes: save.length,
    },
    null,
    2,
  ) + "\n",
);
console.log("PASS original offscreen graph collection and saved continuation");
