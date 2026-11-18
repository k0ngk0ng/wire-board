import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { pathToFileURL } from "node:url";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";

const folder = ".local/pal/verification/local-first";
await build({
  entryPoints: ["games/pal/web/local-simulation.ts"],
  bundle: true,
  format: "esm",
  platform: "browser",
  nodePaths: ["web/node_modules"],
  outfile: folder + "/local-simulation.mjs",
});
const { LocalSimulation } = await import(
  pathToFileURL(process.cwd() + "/" + folder + "/local-simulation.mjs")
);
const fixture = JSON.parse(
  await fs.readFile(folder + "/native-replay-fixture.json"),
);
const compiled = await WebAssembly.compile(
  await fs.readFile(".local/pal/native/rules.wasm"),
);
const encoded = Uint8Array.from(
  Buffer.from(fixture.results[3].checkpoint, "base64"),
);
const meta = {
  generation: 1234,
  revision: 0,
  core: fixture.core,
  remainingMS: 100,
  serverAt: 100000,
  seq: 0,
  appliedSeq: 0,
  discardedSeq: 0,
  direction: 4,
  until: 0,
};
const sim = await LocalSimulation.create(compiled, encoded, meta, 100000);
assert.equal(sim.frame().menu.kind, 1);
const began = performance.now();
const action = sim.input({ seq: 1, direction: 4, keys: 2 }, 100000);
assert.equal(action.type, "local_input");
assert.equal(action.epoch, 1234);
assert.ok(action.boundary);
assert.equal(
  sim.frame().menu.kind,
  2,
  "original status confirmation still waits for authority",
);
const responseMS = performance.now() - began;
assert.ok(responseMS < 50, responseMS);
const checkpointRuntime = await import(
  pathToFileURL(process.cwd() + "/" + folder + "/native-runtime.mjs")
);
const checkpoint = checkpointRuntime.decodeCheckpoint(encoded);
await sim.confirm(
  {
    ...meta,
    revision: 1,
    digest: checkpoint.snapshot.memoryID,
    records: [
      {
        revision: 1,
        operations: [{ kind: "release" }],
        digest: checkpoint.snapshot.memoryID,
        appliedSeq: 0,
      },
    ],
  },
  100010,
);
assert.equal(
  sim.frame().menu.kind,
  2,
  "ordinary confirmation delayed an already displayed native action",
);
await sim.confirm(
  {
    ...meta,
    revision: 1,
    digest: checkpoint.snapshot.memoryID,
    records: [
      {
        revision: 1,
        operations: [{ kind: "release" }],
        digest: checkpoint.snapshot.memoryID,
        appliedSeq: 0,
      },
    ],
  },
  100015,
);
assert.equal(
  sim.frame().menu.kind,
  2,
  "redundant native record replayed an already confirmed operation",
);
sim.input({ seq: 2, direction: 4, keys: 1 }, 100020);
assert.equal(
  sim.frame().phase,
  "field",
  "consecutive native action could not cross menu boundary",
);
assert.equal(sim.pendingCount, 2);
sim.tick(101021);
assert.equal(sim.pendingCount, 0);
assert.equal(
  sim.frame().menu.kind,
  1,
  "timeout did not return to confirmed original state",
);
assert.equal(sim.suspended, true);
assert.equal(sim.input({ seq: 3, direction: 4, keys: 2 }, 101022), undefined);
const mismatch = await LocalSimulation.create(compiled, encoded, meta, 100000);
await assert.rejects(() =>
  mismatch.confirm(
    {
      ...meta,
      revision: 1,
      digest: "0".repeat(64),
      records: [
        {
          revision: 1,
          operations: [{ kind: "release" }],
          digest: "0".repeat(64),
          appliedSeq: 0,
        },
      ],
    },
    100010,
  ),
);
assert.equal(mismatch.suspended, true);
assert.equal(mismatch.pendingCount, 0);
const field = await LocalSimulation.create(
  compiled,
  Uint8Array.from(Buffer.from(fixture.initial, "base64")),
  meta,
  100000,
);
const position = [field.frame().x, field.frame().y];
field.input({ seq: 1, direction: 4, keys: 16 }, 100000);
assert.notDeepEqual([field.frame().x, field.frame().y], position);
field.cancel(1, 100010);
assert.deepEqual(
  [field.frame().x, field.frame().y],
  position,
  "cancelled touch preview changed confirmed position",
);
assert.equal(field.pendingCount, 0);
const record = {
  passed: true,
  responseMS,
  crossPhase: true,
  timeoutRollback: true,
  conflictSuspended: true,
  core: fixture.core,
};
await fs.writeFile(
  folder + "/local-simulation-e2e.json",
  JSON.stringify(record, null, 2) + "\n",
);
console.log(JSON.stringify(record));
