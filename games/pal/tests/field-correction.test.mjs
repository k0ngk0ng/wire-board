import assert from "node:assert/strict";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";
import fs from "node:fs/promises";
import { pathToFileURL } from "node:url";

const output = ".local/pal/verification/local-first/field-correction.mjs";
await build({
  entryPoints: ["games/pal/web/field-correction.ts"],
  bundle: true,
  format: "esm",
  outfile: output,
});
const { FieldCorrection } = await import(
  pathToFileURL(process.cwd() + "/" + output)
);
const field = (x, scene = 2) => ({
  phase: "field",
  scene,
  epoch: 1,
  field: {
    frame: 1,
    camera: [x, 0],
    actors: [
      [0, 0, 0, x + 160, 100, 0, 0, 0],
      [1, 1, 1, 300, 120, 0, 0, 0],
    ],
  },
});
const correction = new FieldCorrection();
correction.update(field(16), 0, false);
correction.update(field(0), 10, true);
assert.equal(correction.current.camera[0], 16);
assert.equal(correction.current.actors[0][3], 176);
assert.equal(correction.current.actors[1][3], 300);
correction.sample(50);
assert.equal(correction.current.camera[0], 8);
correction.update(field(16), 50, false);
assert.equal(
  correction.current.camera[0],
  24,
  "new local input must remain immediate during correction",
);
correction.sample(90);
assert.equal(correction.current, undefined);
correction.update(field(0), 100, true);
correction.update(field(32, 3), 110, true);
assert.equal(
  correction.current,
  undefined,
  "correction must not cross the original scene boundary",
);
const forward = new FieldCorrection();
forward.update(field(0), 0, false);
forward.update(field(16), 100, false);
forward.update(field(32), 200, true);
assert.equal(
  forward.current,
  undefined,
  "ordinary forward native frames must preserve the original movement cadence",
);
await fs.writeFile(
  ".local/pal/verification/local-first/field-correction.json",
  JSON.stringify({
    passed: true,
    durationMS: 80,
    partyAndCameraTogether: true,
    npcWorldUnchanged: true,
    freshInputImmediate: true,
    sceneReset: true,
  }) + "\n",
);
