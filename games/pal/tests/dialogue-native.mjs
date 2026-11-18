import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { root, wasm, digest, create } from "./native-vm.mjs";
const results = [];
for (const testCase of [0, 1, 2]) {
  const vm = await create();
  let ev = vm.step("wb_probe_dialogue", testCase),
    waits = [];
  let firstGlyph;
  for (let i = 0; i < 4096; i++) {
    firstGlyph ??= ev.ops.find((op) => op[0] === 0 && op[6] !== 0);
    if (ev.kind === 2) {
      assert.equal(ev.info[0], 1);
      const frozen = vm.snapshot();
      const stable = vm.step("wb_probe_dialogue", testCase, 0);
      assert.equal(stable.kind, 2);
      assert.deepEqual(
        stable.ops,
        ev.ops,
        "page changed before all readers confirmed",
      );
      const fresh = await create();
      fresh.restore(frozen);
      const continued = fresh.step("wb_probe_dialogue", testCase, 1);
      const original = vm.step("wb_probe_dialogue", testCase, 1);
      assert.deepEqual(
        continued,
        original,
        "fresh-instance page continuation diverged",
      );
      const acknowledged = vm.snapshot();
      fresh.restore(acknowledged);
      fresh.e.wb_release();
      const released = fresh.step("wb_probe_dialogue", testCase),
        next = vm.step("wb_probe_dialogue", testCase);
      assert.deepEqual(
        released,
        next,
        "saved resume or reclaim erased the logical page acknowledgement",
      );
      waits.push(ev);
      ev = next;
      continue;
    }
    if (ev.kind === 3) break;
    ev = vm.step("wb_probe_dialogue", testCase);
  }
  assert.equal(ev.kind, 3, "dialogue did not finish");
  const glyph = (op) => op[0] === 0 && op[6] !== 0;
  if (testCase === 0) {
    assert.equal(
      waits.length,
      2,
      "five original message lines must form two pages",
    );
    const lineYs = new Set(waits[0].ops.filter(glyph).map((op) => op[5]));
    assert.deepEqual([...lineYs], [26, 44, 62, 80]);
    assert.deepEqual(
      [...new Set(waits[1].ops.filter(glyph).map((op) => op[5]))],
      [26],
      "restored page retained old lines",
    );
  } else if (testCase === 1) {
    assert.equal(waits.length, 1);
    const operations = waits[0].ops;
    assert.ok(
      operations.some(
        (op) =>
          op[0] === 0 &&
          op[2] === 0x674e &&
          op[4] === 4 &&
          op[5] === 108 &&
          op[6] === 0x8c,
      ),
      "original lower title is misplaced",
    );
    assert.ok(
      operations.some(
        (op) =>
          op[0] === 0 &&
          op[2] === 0x674e &&
          op[4] === 20 &&
          op[5] === 126 &&
          op[6] === 0x4f,
      ),
      "title consumed a body line",
    );
    assert.ok(
      operations.some((op) => op[0] === 1 && op[1] === 1 && op[2] === 8),
      "original face missing",
    );
  } else {
    assert.equal(
      waits.length,
      0,
      "the original timed dream line gained an extra confirmation",
    );
    assert.equal(firstGlyph[2], 0x674e);
    assert.equal(firstGlyph[4], 80);
    assert.equal(firstGlyph[5], 40);
    assert.ok(
      !ev.ops.filter(glyph).some((op) => op[2] === 36 || op[2] === 126),
      "control markup was rendered as a glyph",
    );
  }
  results.push({
    case: testCase,
    waits: waits.length,
    firstGlyph,
    final: ev.info,
    waitPages: waits.map((wait) => ({ info: wait.info, ops: wait.ops })),
  });
}
const menu = await create();
let event = menu.step("wb_probe_menu");
for (let i = 0; i < 10 && event.kind !== 3; i++)
  event = menu.step("wb_probe_menu", 0, 0, i === 0 ? 4 : 2);
assert.equal(event.kind, 3);
assert.equal(
  event.value,
  22,
  "original menu did not navigate and confirm its second item",
);
const report = {
  passed: true,
  scope:
    "actual pinned C dialogue/UI functions, original message resources, page grouping, timed text, title geometry, portrait, original menu and independent native memory restore",
  runtimeSHA256: digest(wasm),
  cases: results,
};
await fs.writeFile(
  root + ".local/pal/verification/native-dialogue.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log(
  "PASS original dialogue pages, timed text, title, portrait, menu and saved continuation",
);
