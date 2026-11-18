import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { webcrypto } from "node:crypto";
import { pathToFileURL } from "node:url";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";

globalThis.crypto ??= webcrypto;
const folder = ".local/pal/verification/local-first";
await fs.mkdir(folder, { recursive: true });
await build({
  entryPoints: ["games/pal/web/native-runtime.ts"],
  bundle: true,
  format: "esm",
  platform: "browser",
  nodePaths: ["web/node_modules"],
  outfile: folder + "/native-runtime.mjs",
});
const { NativeRuntime, decodeCheckpoint } = await import(
  pathToFileURL(process.cwd() + "/" + folder + "/native-runtime.mjs")
);
const fixture = JSON.parse(
  await fs.readFile(folder + "/native-replay-fixture.json"),
);
const compiled = await WebAssembly.compile(
  await fs.readFile(".local/pal/native/rules.wasm"),
);
const initial = decodeCheckpoint(
  Uint8Array.from(Buffer.from(fixture.initial, "base64")),
);
const runtime = await NativeRuntime.create(compiled, initial);
assert.equal(await runtime.digest(), initial.snapshot.memoryID);
const previews = runtime.previewMenus();
assert.ok(
  previews.some((view) => view.menu.kind === 2),
  "original current-party status resources were not prepared",
);
assert.equal(
  await runtime.digest(),
  initial.snapshot.memoryID,
  "menu resource planning changed the original campaign",
);
let count = 0;
const times = [];
for (const result of fixture.results) {
  const began = performance.now();
  for (const operation of result.operations) runtime.replay(operation);
  const expected = decodeCheckpoint(
    Uint8Array.from(Buffer.from(result.checkpoint, "base64")),
  );
  assert.deepEqual(runtime.snapshot.view, expected.snapshot.view);
  assert.equal(runtime.snapshot.stackPointer, expected.snapshot.stackPointer);
  const actual = runtime.memory();
  if (!Buffer.from(actual).equals(Buffer.from(expected.memory))) {
    const changed = [];
    for (let at = 0; at < actual.length && changed.length < 32; at++)
      if (actual[at] !== expected.memory[at])
        changed.push([at, actual[at], expected.memory[at]]);
    assert.fail(JSON.stringify({ result: count, changed }));
  }
  assert.equal(await runtime.digest(), expected.snapshot.memoryID);
  times.push(performance.now() - began);
  count += result.operations.length;
}
const report = {
  passed: true,
  core: fixture.core,
  operations: count,
  memoryBytes: initial.memory.length,
  checkpointBytes: Buffer.from(fixture.initial, "base64").length,
  timesMS: times,
  scope:
    "published original native core, Go authority to browser WASM exact memory, stack, presentation, deterministic input replay",
};
await fs.writeFile(
  folder + "/native-browser-replay.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log(JSON.stringify(report));
