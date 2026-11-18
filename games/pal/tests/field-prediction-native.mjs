// Test first against original C movement, gestures, collision and draw order.
// Failures: prediction crosses walls, turns in the wrong isometric direction,
// loses idle/walk phase, changes party/follower trail, ignores 0/3/4 walk
// frames, aliases authority state, or omits/reorders covering map tiles.
// Extra party/follower configurations are explicit motor fixtures at the
// real original inn, never evidence that the campaign reached those parties.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createRequire } from "node:module";
import { gunzipSync } from "node:zlib";
import { create, root, wasm, digest } from "./native-vm.mjs";
const { transformSync } = createRequire(new URL("../../../web/package.json", import.meta.url))("esbuild");
const vm = await create();
assert.equal(typeof vm.e.wb_field_info, "function", "missing original field prediction observer");
const source = await fs.readFile(root + "games/pal/web/field-rules.ts", "utf8");
const { code } = transformSync(source, { loader: "ts", format: "esm", target: "es2022" });
const { stepField, drawField } = await import("data:text/javascript;base64," + Buffer.from(code).toString("base64"));
let event = vm.step("wb_run_campaign");
const view = () => Array.from(new Int32Array(vm.e.memory.buffer, vm.e.wb_view(), 32));
for (let i = 0; i < 4096; i++) {
  if (event.kind === 4 && view()[1] === 2 && view()[17] === 0 && view()[15] === 72) break;
  event = vm.step("wb_run_campaign", 0, event.kind === 2 ? 1 : 0);
  assert.notEqual(i, 4095, "original inn did not finish");
}
const originalInn = vm.snapshot();
function field() {
  const pointer = vm.e.wb_field_info();
  const head = Array.from(new Int32Array(vm.e.memory.buffer, pointer, 16));
  assert.equal(head[0], 1);
  assert.equal(head[14], 8); assert.equal(head[15], 334);
  assert.ok(head[13] >= head[11] + head[12] && head[13] <= 517);
  const words = Array.from(new Int32Array(vm.e.memory.buffer, pointer, 365 + head[13] * 8));
  return {
    frame: head[1] >>> 0, anchor: head.slice(2, 4), camera: head.slice(4, 6),
    offset: head.slice(6, 8), layer: head[8], direction: head[9], step: head[10],
    party: head[11], followers: head[12], radius: head[14],
    trail: Array.from({ length: 5 }, (_, i) => words.slice(16 + i * 3, 19 + i * 3)),
    checks: Array.from({ length: 5 }, (_, i) => words.slice(16 + i * 3, 18 + i * 3)),
    cells: Buffer.from(words.slice(31, 365)).toString("base64"),
    actors: Array.from({ length: head[13] }, (_, i) => words.slice(365 + i * 8, 373 + i * 8)),
  };
}
const map = view()[25], base = root + ".local/pal/visual/pal/v1/";
const info = JSON.parse(await fs.readFile(base + `maps/${map}.json`, "utf8"));
const rawMap = gunzipSync(await fs.readFile(base + info.tiles));
const atlases = new Map();
async function loadSprites(f) {
  for (const sprite of new Set(f.actors.map(a => a[2])))
    if (!atlases.has(sprite)) atlases.set(sprite, JSON.parse(await fs.readFile(base + `atlases/mgo/${sprite}.json`, "utf8")));
}
const geometry = {
  sprite: (asset, frame) => atlases.get(asset)?.frames[frame],
  height(x, y, half, layer) {
    if (x < 0 || x >= 64 || y < 0 || y >= 128 || half < 0 || half > 1) return 0;
    return (rawMap.readUInt32LE(((y * 64 + x) * 2 + half) * 4) >>> (layer ? 24 : 8)) & 15;
  },
};
function draws() {
  const pointer = vm.e.wb_scene_ops(), count = new Int32Array(vm.e.memory.buffer, pointer, 1)[0];
  const data = new Int32Array(vm.e.memory.buffer, pointer + 4, count * 8);
  return Array.from({ length: count }, (_, i) => {
    const [kind, a, b, c, d, x, baseY, sortY] = data.slice(i * 8, i * 8 + 8);
    return { kind, a, b, c, d, x, baseY, sortY };
  }).sort((a, b) => a.sortY - b.sortY);
}
const samples = []; let blocked = 0, moved = 0;
for (const [party, followers, walk, odd = 0] of [[1, 0, 3], [2, 0, 3], [3, 0, 3], [3, 1, 4], [3, 2, 0], [3, 0, 3, 1]]) {
  vm.restore(originalInn);
  vm.e.wb_probe_field_config(party, followers, walk, odd);
  const directions = [4, 4, 0, 2, 1, 3, ...Array(20).fill(0), ...Array(20).fill(1), ...Array(20).fill(2), ...Array(20).fill(3), 4, 4, 0, 2];
  for (const direction of directions) {
    const before = field(), original = JSON.stringify(before);
    await loadSprites(before);
    const predicted = stepField(before, direction);
    assert.equal(JSON.stringify(before), original, "local field rule mutated its authority source");
    vm.e.wb_probe_field_move(direction);
    const after = field();
    assert.deepEqual(
      Object.fromEntries(["frame", "camera", "offset", "layer", "direction", "step", "party", "followers", "trail", "actors"].map(key => [key, predicted[key]])),
      Object.fromEntries(["frame", "camera", "offset", "layer", "direction", "step", "party", "followers", "trail", "actors"].map(key => [key, after[key]])),
      `original party/follower movement mismatch: party=${party} followers=${followers} walk=${walk} direction=${direction}`,
    );
    assert.deepEqual(drawField(predicted, geometry), draws(), "predicted original sprite/cover order differs from C");
    const walking = before.camera[0] !== after.camera[0] || before.camera[1] !== after.camera[1];
    if (direction !== 4) walking ? moved++ : blocked++;
    samples.push({ party, followers, walk, oddTrail: !!odd, direction, walking, camera: after.camera, step: after.step });
  }
}
assert.ok(moved > 50 && blocked > 50, "original walk and obstacle branches were not both exercised");
await fs.writeFile(root + ".local/pal/verification/field-prediction-native.json", JSON.stringify({ passed: true, authoritySHA256: digest(wasm), scope: "original GameMain reaches the inn, then independent original-C motor fixtures including script-style off-lattice trails; exact party/follower motion, idle/walk phase and sprite/cover draw sequences; not story/latency acceptance", moved, blocked, samples }, null, 2) + "\n");
console.log(`PASS original field prediction: ${samples.length} steps, ${moved} moves, ${blocked} blocked`);
