// Build acceptance specified before implementation. Failures: absent/stale
// Wasm enters web/dist; mismatched Go loader; corrupted cache reused; source
// files/ROM exposed; a failed compile silently falls back to an old runtime.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  readdirSync,
  statSync,
  rmSync,
} from "node:fs";
import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { prepareBasketballRuntime } from "../../games/basketball/build/runtime.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const out = path.join(root, ".local/basketball/runtime-build-acceptance");
mkdirSync(out, { recursive: true });
const checks = [];
const hash = (data) => createHash("sha256").update(data).digest("hex");
const check = (name, value) => {
  assert.ok(value, name);
  checks.push(name);
};
function inspect(publicRoot) {
  const dir = path.join(publicRoot, "basketball/v1/runtime");
  const manifest = JSON.parse(
    readFileSync(path.join(dir, "build.json"), "utf8"),
  );
  const wasm = readFileSync(path.join(dir, "basketball.wasm")),
    loader = readFileSync(path.join(dir, "wasm_exec.js"));
  check(
    "runtime includes only compiled Wasm, matching loader and provenance",
    JSON.stringify(readdirSync(dir).sort()) ===
      JSON.stringify(["basketball.wasm", "build.json", "wasm_exec.js"]),
  );
  check(
    "actual generated module validates as WebAssembly",
    WebAssembly.validate(wasm),
  );
  check(
    "runtime manifest hashes bind both executable files",
    hash(wasm) === manifest.files["basketball.wasm"].sha256 &&
      hash(loader) === manifest.files["wasm_exec.js"].sha256,
  );
  return { manifest, dir };
}
try {
  const publicRoot = prepareBasketballRuntime(root);
  const real = inspect(publicRoot);
  const env = JSON.parse(
    execFileSync("go", ["env", "-json", "GOROOT", "GOVERSION"], {
      encoding: "utf8",
    }),
  );
  check(
    "loader comes from the exact compiling Go distribution",
    real.manifest.goVersion === env.GOVERSION &&
      hash(readFileSync(path.join(env.GOROOT, "lib/wasm/wasm_exec.js"))) ===
        real.manifest.files["wasm_exec.js"].sha256,
  );
  const before = statSync(path.join(real.dir, "basketball.wasm")).mtimeMs;
  check(
    "unchanged verified input reuses its immutable output",
    prepareBasketballRuntime(root) === publicRoot &&
      statSync(path.join(real.dir, "basketball.wasm")).mtimeMs === before,
  );

  const fixture = path.join(out, "workspace");
  rmSync(fixture, { recursive: true, force: true });
  mkdirSync(fixture, { recursive: true });
  for (const file of ["go.mod", "go.sum"])
    writeFileSync(
      path.join(fixture, file),
      readFileSync(path.join(root, file)),
    );
  for (const directory of ["engine", "runtimebridge", "cmd/browser-wasm"]) {
    const source = path.join(root, "games/basketball", directory),
      dest = path.join(fixture, "games/basketball", directory);
    mkdirSync(dest, { recursive: true });
    for (const entry of readdirSync(source, { withFileTypes: true }))
      if (
        entry.isFile() &&
        entry.name.endsWith(".go") &&
        !entry.name.endsWith("_test.go")
      )
        writeFileSync(
          path.join(dest, entry.name),
          readFileSync(path.join(source, entry.name)),
        );
  }
  const copied = inspect(prepareBasketballRuntime(fixture));
  check(
    "build is independent of absolute checkout location",
    real.manifest.sourceSHA256 === copied.manifest.sourceSHA256 &&
      real.manifest.files["basketball.wasm"].sha256 ===
        copied.manifest.files["basketball.wasm"].sha256,
  );
  writeFileSync(path.join(copied.dir, "basketball.wasm"), "corrupt");
  const repaired = inspect(prepareBasketballRuntime(fixture));
  check(
    "corrupted cached executable is rebuilt",
    repaired.manifest.files["basketball.wasm"].sha256 ===
      real.manifest.files["basketball.wasm"].sha256,
  );
  const badManifest = {
    ...repaired.manifest,
    goVersion: "incorrect-loader-version",
  };
  writeFileSync(
    path.join(repaired.dir, "build.json"),
    JSON.stringify(badManifest),
  );
  const corrected = inspect(prepareBasketballRuntime(fixture));
  check(
    "corrupted provenance is rejected alongside executable corruption",
    corrected.manifest.goVersion === env.GOVERSION,
  );
  const entry = path.join(
      fixture,
      "games/basketball/cmd/browser-wasm/main_js.go",
    ),
    original = readFileSync(entry);
  writeFileSync(
    entry,
    Buffer.concat([original, Buffer.from("\nthis is not valid Go\n")]),
  );
  let rejected = false;
  try {
    prepareBasketballRuntime(fixture);
  } catch (error) {
    rejected = true;
    writeFileSync(
      path.join(out, "expected-compile-error.txt"),
      String(error) + "\n",
    );
  }
  check("compile failure cannot reuse an older source build", rejected);
  writeFileSync(
    entry,
    Buffer.concat([
      original,
      Buffer.from("\n// Changed source must have distinct provenance.\n"),
    ]),
  );
  const changed = inspect(prepareBasketballRuntime(fixture));
  check(
    "source changes produce distinct provenance and output directory",
    changed.manifest.sourceSHA256 !== real.manifest.sourceSHA256 &&
      changed.dir !== copied.dir,
  );
  writeFileSync(
    path.join(out, "result.json"),
    JSON.stringify(
      { status: "passed", checks, runtime: real.manifest },
      null,
      2,
    ) + "\n",
  );
  console.log(JSON.stringify({ status: "passed", checks }));
} catch (error) {
  writeFileSync(
    path.join(out, "result.json"),
    JSON.stringify(
      { status: "failed", checks, error: String(error) },
      null,
      2,
    ) + "\n",
  );
  throw error;
}
