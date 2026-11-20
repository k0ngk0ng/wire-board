import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, writeFileSync, rmSync, symlinkSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { discoverAuthoredSources, inspectExternalResources } from "../build/lint.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/hotlinemiami/verification/lint");
mkdirSync(output, { recursive: true });
const digest = bytes => createHash("sha256").update(bytes).digest("hex");
const results = [];

function fixture() {
  const directory = mkdtempSync(path.join(output, "external-"));
  const external = "games/hotlinemiami/assets/public/hotlinemiami/wasm-exec.js";
  const metadata = "games/hotlinemiami/assets/source-manifest.json";
  const script = Buffer.from("globalThis.Go = function () { const _inst = 1; return _inst; };\n");
  for (const folder of ["assets", "games/hotlinemiami/assets/public/hotlinemiami"]) mkdirSync(path.join(directory, folder), { recursive: true });
  writeFileSync(path.join(directory, external), script);
  writeFileSync(path.join(directory, metadata), '{"source":"authored-metadata"}\n');
  const record = { bytes: script.length, sha256: digest(script), mime: "text/javascript", key: "bga/hotlinemiami/" + digest(script) + ".js" };
  const registry = { version: 1, files: { [external]: record } };
  writeFileSync(path.join(directory, "assets/resources.json"), JSON.stringify(registry));
  return { directory, external, metadata, registry };
}

test("restored runtime payload is excluded from authored lint only after registry integrity verification", () => {
  const { directory, external, metadata } = fixture();
  try {
    const checked = inspectExternalResources(directory);
    assert.equal(checked.passed, true, JSON.stringify(checked));
    assert.deepEqual(checked.files.map(row => row.file), [external]);
    const authored = discoverAuthoredSources(directory);
    assert(authored.some(row => row.file === metadata));
    assert(!authored.some(row => row.file.startsWith("games/hotlinemiami/assets/public/")));
    results.push({ scenario: "external-boundary-and-authored-metadata", passed: true, checked: checked.files.length });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("missing, corrupted, unregistered and symlinked external payloads fail the boundary", () => {
  for (const kind of ["missing", "corrupted", "unregistered", "symlink"]) {
    const { directory, external } = fixture();
    try {
      const file = path.join(directory, external);
      if (kind === "missing") rmSync(file);
      if (kind === "corrupted") writeFileSync(file, "corrupt");
      if (kind === "unregistered") writeFileSync(path.join(path.dirname(file), "unregistered.js"), "const injected = 1;");
      if (kind === "symlink") {
        rmSync(file);
        const target = path.join(directory, "external.js");
        writeFileSync(target, "globalThis.Go = function () { const _inst = 1; return _inst; };\n");
        symlinkSync(target, file);
      }
      const checked = inspectExternalResources(directory);
      assert.equal(checked.passed, false, JSON.stringify({ kind, checked }));
      results.push({ scenario: kind, passed: true, issues: checked.issues });
    } finally {
      rmSync(directory, { recursive: true, force: true });
    }
  }
});

test.after(() => {
  writeFileSync(path.join(output, "external-boundary.json"), JSON.stringify({ passed: results.length === 5 && results.every(row => row.passed), cases: results }, null, 2) + "\n");
});
