// Test first. Original combat presentation failures: offscreen draws/copies
// disappear; old UI or hurt numbers accumulate; target/poison color modes lose
// palette ramps; physical/magic effects use the wrong atlas/frame; background
// changes and wave/fade writes are omitted; saved draw dependencies are mutable;
// stripped authority assets generate a different draw sequence from original C.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createHash } from "node:crypto";
import { create, root, wasm } from "./native-vm.mjs";

const mode = process.env.PAL_BATTLE_CASE ?? "attack";
assert.ok(["attack", "coop", "wave"].includes(mode));
const reference =
  process.env.PAL_REFERENCE_FONT === "1" &&
  process.env.PAL_ORIGINAL_PIXELS === "1";
const v = await create();
v.e.wb_platform_players(3);
let ev = v.step("wb_run_campaign");
const battle = () =>
  Array.from(new Int32Array(v.e.memory.buffer, v.e.wb_battle_info(), 87));
const step = (keys = 0, dir = 4, response = 0) =>
  (ev = v.step("wb_run_campaign", 0, response, keys, dir));
for (let i = 0; battle()[3] !== 1; i++) {
  assert.ok(i < 512);
  step();
}
function state() {
  const raw = Array.from(new Uint32Array(v.e.memory.buffer, v.e.wb_view(), 32));
  let raster;
  if (v.e.wb_raster_graph) {
    const p = v.e.wb_raster_graph(),
      head = Array.from(new Int32Array(v.e.memory.buffer, p, 3));
    assert.equal(head[0], 1);
    assert.ok(head[1] > 0 && head[1] <= 16384);
    const words = Array.from(
      new Int32Array(v.e.memory.buffer, p + 12, head[1] * 16),
    );
    raster = {
      version: 1,
      root: head[2],
      nodes: Array.from({ length: head[1] }, (_, i) =>
        words.slice(i * 16, i * 16 + 16),
      ),
    };
  }
  return {
    map: raw[25],
    palette: raw[26],
    night: !!raw[27],
    cameraX: raw[23] | 0,
    cameraY: raw[24] | 0,
    draws: [],
    raster,
  };
}
const pages = [],
  buffers = [],
  sequence = [];
function capture(label) {
  const view = state();
  sequence.push(view);
  if (!reference) return;
  const indices = new Uint8Array(v.e.memory.buffer, v.e.wb_pixels(), 64000),
    colors = new Uint8Array(v.e.memory.buffer, v.e.wb_colors(), 1024),
    rgba = Buffer.alloc(256000);
  for (let i = 0; i < 64000; i++) {
    rgba[i * 4] = colors[indices[i] * 4];
    rgba[i * 4 + 1] = colors[indices[i] * 4 + 1];
    rgba[i * 4 + 2] = colors[indices[i] * 4 + 2];
    rgba[i * 4 + 3] = 255;
  }
  const hash = createHash("sha256").update(rgba).digest("hex");
  if (pages.at(-1)?.rgbaSHA256 === hash) return;
  pages.push({ label, view, rgbaSHA256: hash });
  buffers.push(rgba);
}
capture("choose-action");
if (mode === "coop") {
  for (let i = 0; battle()[6] !== 2; i++) {
    assert.ok(i < 16);
    step(0, 3);
    capture("coop-icon");
  }
  step(2);
  capture("coop-confirm");
}
for (let i = 0; ev.kind !== 3; i++) {
  assert.ok(i < 2048, "original combat did not finish");
  const b = battle();
  const keys = mode !== "coop" && b[0] && b[2] === 0 && b[3] !== 0 ? 2 : 0;
  step(keys, 4, ev.kind === 6 ? 1 : 0);
  capture(`${mode}-${i}`);
}
assert.equal(ev.value, 3);
const out = root + ".local/pal/verification/";
if (reference) {
  assert.ok(
    pages.length >= 20,
    "reference must include complete animation changes",
  );
  const baseline =
    root + `games/pal/resources/testdata/battle-${mode}-raster.json`;
  const hashes = pages.map((p) => p.rgbaSHA256);
  if (process.env.PAL_RECORD_BASELINE === "1")
    await fs.writeFile(
      baseline,
      JSON.stringify(
        {
          scope:
            "original C raster with pinned resources/font and real PAL_StartBattle",
          mode,
          hashes,
        },
        null,
        2,
      ) + "\n",
    );
  else
    assert.deepEqual(
      hashes,
      JSON.parse(await fs.readFile(baseline)).hashes,
      "presentation observations changed the original C raster sequence",
    );
  await fs.writeFile(
    out + `original-battle-${mode}-pages.rgba`,
    Buffer.concat(buffers),
  );
  await fs.writeFile(
    out + `original-battle-${mode}-reference.json`,
    JSON.stringify(
      {
        scope:
          "isolated original PAL_StartBattle C raster animation; no campaign completion claim",
        runtimeSHA256: createHash("sha256").update(wasm).digest("hex"),
        pages,
      },
      null,
      2,
    ) + "\n",
  );
  await fs.writeFile(
    out + `battle-${mode}-sequence.json`,
    JSON.stringify(sequence) + "\n",
  );
  console.log(
    "recorded original combat raster",
    mode,
    pages.length,
    "distinct frames",
  );
} else {
  assert.ok(
    sequence.every((p) => p.raster),
    "authority must expose original offscreen combat draws",
  );
  assert.deepEqual(
    sequence,
    JSON.parse(await fs.readFile(out + `battle-${mode}-sequence.json`)),
    "stripped authority differs from original C draw commands",
  );
  console.log(
    "PASS headless/original combat draw sequence",
    mode,
    sequence.length,
  );
}
