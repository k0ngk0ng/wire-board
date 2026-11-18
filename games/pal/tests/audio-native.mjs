import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { create, root, wasm, digest } from "./native-vm.mjs";

function service(vm, name, ...args) {
  const e = vm.e, previous = e.__stack_pointer.value;
  e.__stack_pointer.value = e.wb_service_stack.value + 262144;
  try { return e[name](...args); }
  finally { e.__stack_pointer.value = previous; }
}
function journal(vm) {
  assert.equal(typeof vm.e.wb_audio_info, "function", "native ordered audio journal is missing");
  const address = service(vm, "wb_audio_info"), header = Array.from(new Uint32Array(vm.e.memory.buffer, address, 3));
  assert.equal(header[0], 1);
  assert.ok(header[2] <= 128);
  const commands = Array.from({ length: header[2] }, (_, i) => {
    const [seq, clock, kind, id, loop, fadeMS] = new Int32Array(vm.e.memory.buffer, address + 12 + i * 24, 6);
    return { seq: seq >>> 0, clock: clock >>> 0, kind, id, loop: !!loop, fadeMS };
  });
  assert.equal(commands.at(-1)?.seq ?? 0, header[1]);
  commands.forEach((command, i) => assert.equal(command.seq, header[1] - commands.length + i + 1));
  return { serial: header[1], commands };
}
const vm = await create();
const initial = journal(vm).serial;
const results = [];
for (const [name, args, expected] of [
  ["AUDIO_PlayMusic", [9, 1, 3]],
  ["AUDIO_PlaySound", [45]],
  ["AUDIO_PlaySound", [-28]],
  ["AUDIO_PlayCDTrack", [2], 1],
  ["AUDIO_PlayCDTrack", [-2], 1],
  ["AUDIO_PlayMusic", [0, 0, 2]],
  ["AUDIO_PlayCDTrack", [-2], 1],
  ["AUDIO_PlayMusic", [9, 0, 0]],
  ["AUDIO_PlayCDTrack", [-2], 0],
  ["AUDIO_PlayCDTrack", [12], 0],
  ["AUDIO_PlayMusic", [26, 1, 0]],
]) {
  const before = journal(vm).serial;
  const result = service(vm, name, ...args);
  if (expected !== undefined) assert.equal(result, expected, name + "/" + args[0]);
  results.push({ name, args, result: expected, before, after: journal(vm).serial });
}
const commands = journal(vm).commands.filter(c => c.seq > initial);
assert.deepEqual(commands.map(({ kind, id, loop, fadeMS }) => [kind, id, loop, fadeMS]), [
  [3, -1, false, 0], [1, 9, true, 3000], [2, 45, false, 0], [2, 28, false, 0],
  [1, -1, false, 0], [3, 2, true, 0], [1, 0, false, 2000],
  [3, -1, false, 0], [1, 9, false, 0], [3, -2, false, 0],
  [1, -1, false, 0], [3, 12, true, 0], [3, -1, false, 0], [1, 26, true, 0],
]);
for (let i = 0; i < 200; i++) service(vm, "AUDIO_PlaySound", i % 2 ? 45 : 28);
assert.equal(journal(vm).commands.length, 128);
const saved = vm.snapshot(), fresh = await create();
fresh.restore(saved);
assert.deepEqual(journal(fresh), journal(vm));
service(vm, "AUDIO_PlayCDTrack", 3);
service(fresh, "AUDIO_PlayCDTrack", 3);
assert.deepEqual(journal(fresh), journal(vm));
const campaign = await create(), pages = [], events = [];
let event = campaign.step("wb_run_campaign"), previous = 0;
for (let step = 0; step < 4096; step++) {
  const current = journal(campaign);
  const next = current.commands.filter(c => c.seq > previous);
  if (next.length) assert.equal(next[0].seq, previous + 1);
  events.push(...next);
  previous = current.serial;
  const view = Array.from(new Int32Array(campaign.e.memory.buffer, campaign.e.wb_view(), 32));
  if (event.kind === 2) pages.push({ message: event.value, journal: current });
  if ([1, 8, 23].includes(pages.length) && event.kind === 2) {
    const state = campaign.snapshot(), restored = await create();
    restored.restore(state);
    assert.deepEqual(journal(restored), current);
    const expected = campaign.step("wb_run_campaign", 0, 1), actual = restored.step("wb_run_campaign", 0, 1);
    assert.deepEqual(actual, expected);
    assert.deepEqual(journal(restored), journal(campaign));
    campaign.restore(state);
  }
  if (pages.length && event.kind === 4 && view[17] === 0 && !view[18]) break;
  assert.notEqual(step, 4095, "new original campaign did not reach its first field");
  event = campaign.step("wb_run_campaign", 0, event.kind === 2 ? 1 : 0);
}
assert.equal(pages.length, 23);
assert.ok(events.some(c => (c.kind === 1 && c.id > 0) || (c.kind === 3 && c.id >= 2 && c.id <= 9)));
const report = { passed: true, scope: "ordered original audio API bridge, CD no-op/fallback/stop behavior, 128-entry journal rollover and fresh full-memory continuation; actual new-game original23 dialogue pages and native audio requests; no browser or full campaign acceptance", runtimeSHA256: digest(wasm), api: results, commands, pages, originalEvents: events };
await fs.writeFile(root + ".local/pal/verification/audio-native.json", JSON.stringify(report, null, 2) + "\n");
console.log(JSON.stringify({ passed: true, originalPages: pages.length, originalEvents: events.length }));
