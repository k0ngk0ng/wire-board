import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import * as builder from "../build/browser-runtime.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const cache = join(root, ".local/go-mod");
const output = join(root, ".local/pokepath/verification/native-runtime");
mkdirSync(output, { recursive: true });
const environment = { ...process.env, GOCACHE: join(root, ".local/go-build"), GOMODCACHE: cache, GOTMPDIR: join(root, ".local/tmp"), TMPDIR: join(root, ".local/tmp"), TEST_TELEMETRY_DIR: join(root, ".local/go-telemetry"), GOTOOLCHAIN: "local", GOWORK: "off", GOFLAGS: "" };
const module = JSON.parse(execFileSync("go", ["list", "-m", "-json", "golang.org/x/text"], { cwd: root, env: environment, encoding: "utf8" }));
const sha = bytes => createHash("sha256").update(bytes).digest("hex");
const scenarios = [];

test("the browser builder permits only the pinned Unicode dependency identity", () => {
  assert.equal(typeof builder.verifyUnicodeDependency, "function");
  const allowed = builder.verifyUnicodeDependency(module, cache);
  assert.equal(allowed.module, "golang.org/x/text");
  assert.equal(allowed.version, "v0.34.0");
  assert.equal(allowed.sum, "h1:oL/Qq0Kdaqxa1KbNeMKwQq0reLCCaFtqu2eNuSeNHbk=");
  assert.equal(allowed.goModSum, "h1:homfLqTYRFyVYemLBFl5GgL/DWEiH5wcsQ5gSh1yziA=");
  assert.equal(allowed.license, "BSD-3-Clause");
  for (const changed of [
    { Path: "github.com/dop251/goja" },
    { Version: "v0.43.0" },
    { Sum: "h1:unverified" },
    { GoModSum: "h1:unverified" },
    { Dir: join(root, "games/starfort/server") },
    { Replace: { Path: "../untrusted" } },
  ]) assert.throws(() => builder.verifyUnicodeDependency({ ...module, ...changed }, cache), JSON.stringify(changed));
  scenarios.push({ name: "exact-module-version-cache-checksums-and-license", passed: true, dependency: allowed });
});

test("real Wasm compilation inventories Unicode source and carries its fixed BSD notice", () => {
  const artifact = builder.buildBrowserRuntime();
  assert.equal(artifact.manifest.dependencies.length, 1);
  const dependency = artifact.manifest.dependencies[0];
  assert.equal(dependency.module, module.Path);
  assert.equal(dependency.version, module.Version);
  assert.deepEqual(dependency.packages, ["golang.org/x/text/transform", "golang.org/x/text/unicode/norm"]);
  const source = artifact.manifest.sourceFiles.filter(file => file.dependency === module.Path);
  assert.ok(source.length > 2);
  for (const file of source) {
    assert.ok(file.path.startsWith(dependency.sourcePrefix));
    const relative = file.path.slice(dependency.sourcePrefix.length);
    const disk = relative === "go.mod" ? module.GoMod : join(module.Dir, relative);
    const bytes = readFileSync(disk);
    assert.equal(file.bytes, bytes.length);
    assert.equal(file.sha256, sha(bytes));
    assert.ok(!file.path.includes("upstream") && !file.path.endsWith(".js"));
  }
  const license = readFileSync(join(artifact.runtimeRoot, "go-runtime-LICENSE.txt"), "utf8");
  assert.ok(license.includes("golang.org/x/text v0.34.0"));
  assert.ok(license.includes("Copyright 2009 The Go Authors."));
  assert.equal(dependency.licenseSHA256, sha(readFileSync(join(module.Dir, "LICENSE"))));
  scenarios.push({ name: "real-browser-wasm-source-and-license-inventory", passed: true, sourceSHA256: artifact.manifest.sourceSHA256, buildSHA256: artifact.manifest.buildSHA256, dependency, files: source });
});

test.after(() => writeFileSync(join(output, "unicode-dependency.json"), JSON.stringify({ command: "node --test games/pokepath/tests/native-runtime-unicode.test.mjs", passed: scenarios.length === 2, scenarios }, null, 2) + "\n"));
