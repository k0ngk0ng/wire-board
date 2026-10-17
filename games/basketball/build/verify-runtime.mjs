import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync, readdirSync, realpathSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = realpathSync(fileURLToPath(new URL("../../../", import.meta.url)));
const dist = path.resolve(root, process.argv[2] ?? "web/dist");
const directory = path.join(dist, "basketball/v1/runtime");
const hash = (data) => createHash("sha256").update(data).digest("hex");
const manifest = JSON.parse(
  readFileSync(path.join(directory, "build.json"), "utf8"),
);
assert.equal(manifest.format, 1);
assert.equal(manifest.package, "./games/basketball/cmd/browser-wasm");
assert.deepEqual(manifest.flags, [
  "-trimpath",
  "-buildvcs=false",
  "-ldflags=-s -w",
]);
assert.deepEqual(readdirSync(directory).sort(), [
  "basketball.wasm",
  "build.json",
  "wasm_exec.js",
]);
assert.equal(
  manifest.builderSHA256,
  hash(readFileSync(path.join(root, "games/basketball/build/runtime.mjs"))),
  "Runtime builder does not match this checkout",
);
assert.ok(Array.isArray(manifest.inputs) && manifest.inputs.length > 2);
const seen = new Set();
for (const input of manifest.inputs) {
  assert.ok(
    typeof input.path === "string" &&
      !path.isAbsolute(input.path) &&
      !input.path.split("/").includes(".."),
    "Invalid source provenance path",
  );
  assert.ok(
    input.path === "go.mod" ||
      input.path === "go.sum" ||
      (input.path.startsWith("games/basketball/") &&
        input.path.endsWith(".go")),
    "Wasm source provenance includes unexpected private/non-Go content",
  );
  assert.ok(!seen.has(input.path), "Repeated source provenance");
  seen.add(input.path);
  const file = realpathSync(path.join(root, input.path));
  assert.ok(
    file.startsWith(root + path.sep),
    "Source provenance escaped the checkout",
  );
  assert.equal(
    hash(readFileSync(file)),
    input.sha256,
    "Runtime was built from a different source: " + input.path,
  );
}
assert.ok(
  seen.has("go.mod") &&
    seen.has("go.sum") &&
    seen.has("games/basketball/cmd/browser-wasm/main_js.go"),
);
assert.equal(hash(JSON.stringify(manifest.inputs)), manifest.sourceSHA256);
for (const file of ["basketball.wasm", "wasm_exec.js"]) {
  const data = readFileSync(path.join(directory, file));
  assert.equal(
    hash(data),
    manifest.files[file].sha256,
    "Runtime checksum mismatch: " + file,
  );
  assert.equal(data.length, manifest.files[file].bytes);
  if (file === "basketball.wasm")
    assert.ok(WebAssembly.validate(data), "Invalid compiled WebAssembly");
}
const go = JSON.parse(
  execFileSync("go", ["env", "-json", "GOROOT", "GOVERSION"], {
    cwd: root,
    env: { ...process.env, GOTOOLCHAIN: "local" },
    encoding: "utf8",
  }),
);
assert.equal(
  manifest.goVersion,
  go.GOVERSION,
  "Runtime and current Go toolchain differ",
);
assert.equal(
  manifest.loaderSHA256,
  hash(readFileSync(path.join(go.GOROOT, "lib/wasm/wasm_exec.js"))),
);
assert.equal(manifest.loaderSHA256, manifest.files["wasm_exec.js"].sha256);
console.log(
  JSON.stringify(
    {
      status: "passed",
      sourceSHA256: manifest.sourceSHA256,
      goVersion: manifest.goVersion,
      sourceFiles: manifest.inputs.length,
      files: manifest.files,
    },
    null,
    2,
  ),
);
