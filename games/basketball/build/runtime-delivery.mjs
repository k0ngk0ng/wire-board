import assert from "node:assert/strict";
import {
  readFileSync,
  mkdirSync,
  writeFileSync,
  copyFileSync,
  rmSync,
} from "node:fs";
import path from "node:path";
import { prepareBasketballRuntime } from "./runtime.mjs";
export function verifyRuntimePin(manifest, pin) {
  const file = manifest.files["basketball.wasm"];
  assert.equal(
    pin.sourceSHA256,
    manifest.sourceSHA256,
    "Publish Wasm for the current source first",
  );
  assert.equal(
    pin.sha256,
    file.sha256,
    "Published Wasm differs from this build",
  );
  assert.equal(pin.bytes, file.bytes);
  assert.equal(
    pin.url,
    "https://cdn.ichenj.com/bga/basketball/runtime-" +
      file.sha256 +
      "/basketball.wasm",
  );
}
/** Compile/verify locally; ship only the JS loader and source-bound descriptor.
 * The Wasm binary stays in the research cache and immutable OSS object. */
export function prepareBasketballDelivery(root) {
  const compiled = path.join(
    prepareBasketballRuntime(root),
    "basketball/v1/runtime",
  );
  const manifest = JSON.parse(
    readFileSync(path.join(compiled, "build.json"), "utf8"),
  );
  const pin = JSON.parse(
    readFileSync(
      path.join(root, "games/basketball/assets/runtime-manifest.json"),
      "utf8",
    ),
  );
  verifyRuntimePin(manifest, pin);
  const publicRoot = path.join(
    root,
    ".local/basketball/runtime-delivery/public",
  );
  rmSync(publicRoot, { recursive: true, force: true });
  const target = path.join(publicRoot, "basketball/v1/runtime");
  mkdirSync(target, { recursive: true });
  copyFileSync(
    path.join(compiled, "wasm_exec.js"),
    path.join(target, "wasm_exec.js"),
  );
  writeFileSync(
    path.join(target, "build.json"),
    JSON.stringify({ ...manifest, wasm: pin }, null, 2) + "\n",
  );
  return publicRoot;
}
