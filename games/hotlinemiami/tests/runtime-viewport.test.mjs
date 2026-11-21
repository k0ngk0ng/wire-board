import assert from "node:assert/strict";
import { createHash, webcrypto } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/hotlinemiami/verification/large-map-quality");
mkdirSync(output, { recursive: true });
const digest = bytes => createHash("sha256").update(bytes).digest("hex");

function currentSourcePaths() {
  const files = [];
  function visit(directory) {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const selected = path.join(directory, entry.name);
      assert(!entry.isSymbolicLink(), "Native rule sources cannot be symlinks");
      if (entry.isDirectory()) visit(selected);
      else if (entry.name.endsWith(".go")) files.push(path.relative(root, selected).split(path.sep).join("/"));
    }
  }
  for (const area of ["engine", "cmd", "data"]) visit(path.join(root, "games/hotlinemiami", area));
  for (const name of ["go.mod", "go.sum"]) if (existsSync(path.join(root, name))) files.push(name);
  return files.sort();
}

test("the published viewport comes from the current source-bound WASM metadata", async () => {
  const report = { passed: false };
  try {
    const publicRoot = path.join(root, "games/hotlinemiami/assets/public/hotlinemiami");
    const manifest = JSON.parse(readFileSync(path.join(publicRoot, "manifest.json"), "utf8"));
    const runtime = manifest.runtime;
    assert.deepEqual(runtime.sourceFiles.map(row => row.path), currentSourcePaths());
    for (const row of runtime.sourceFiles) {
      const bytes = readFileSync(path.join(root, row.path));
      assert.equal(bytes.length, row.bytes, row.path);
      assert.equal(digest(bytes), row.sha256, row.path);
    }
    assert.equal(runtime.sourceSHA256, digest(JSON.stringify(runtime.sourceFiles)));
    assert.equal(runtime.builderSHA256, digest(readFileSync(path.join(root, "games/hotlinemiami/build/runtime-delivery.mjs"))));
    for (const [key, file] of [["wasm", "hotline-motion.wasm"], ["script", "wasm-exec.js"], ["license", "go-runtime-LICENSE.txt"]]) {
      const item = runtime[key];
      assert.equal(item.file, file, key);
      const bytes = readFileSync(path.join(publicRoot, item.file));
      assert.equal(bytes.length, item.bytes, key);
      assert.equal(digest(bytes), item.sha256, key);
    }
    globalThis.crypto ??= webcrypto;
    await import(pathToFileURL(path.join(publicRoot, runtime.script.file)).href);
    const go = new globalThis.Go();
    const module = await WebAssembly.instantiate(readFileSync(path.join(publicRoot, runtime.wasm.file)), go.importObject);
    void go.run(module.instance);
    const motion = globalThis.hotlineMotion;
    assert.equal(typeof motion.viewport, "function");
    const viewport = motion.viewport();
    assert(viewport && typeof viewport === "object" && !Array.isArray(viewport));
    assert(Number.isSafeInteger(viewport.width) && Number.isSafeInteger(viewport.height));
    assert(viewport.width > 0 && viewport.height > 0 && viewport.width * 5 === viewport.height * 8);
    assert.deepEqual({ width: viewport.width, height: viewport.height }, { width: 576, height: 360 });
    Object.assign(report, { passed: true, viewport, sourceSHA256: runtime.sourceSHA256, buildSHA256: runtime.buildSHA256, wasmSHA256: runtime.wasm.sha256 });
  } finally {
    writeFileSync(path.join(output, "viewport-metadata.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
