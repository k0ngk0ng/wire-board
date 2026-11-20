import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, writeFileSync, readFileSync, statSync, existsSync, renameSync, cpSync, rmSync, symlinkSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { test } from "node:test";
import { prepareHotlineDelivery, verifyHotlineDelivery, buildHotlineRuntime } from "../build/runtime-delivery.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/hotlinemiami/verification/delivery");
mkdirSync(output, { recursive: true });
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const digest = bytes => createHash("sha256").update(bytes).digest("hex");
const results = [];

function fixture(counts = { maps: 6, characters: 8, weapons: 12 }) {
  const directory = mkdtempSync(path.join(output, "workspace-"));
  for (const folder of ["games/hotlinemiami/engine", "games/hotlinemiami/cmd/motion", "games/hotlinemiami/data", ".local/hotlinemiami/assets/images"]) mkdirSync(path.join(directory, folder), { recursive: true });
  writeFileSync(path.join(directory, "go.mod"), "module fixture.test/hotline\n\ngo 1.26.0\n");
  writeFileSync(path.join(directory, "games/hotlinemiami/engine/value.go"), "package engine\nfunc Value() int { return 1 }\n");
  writeFileSync(path.join(directory, "games/hotlinemiami/data/maps.go"), "package data\nvar Version = 1\n");
  writeFileSync(path.join(directory, "games/hotlinemiami/cmd/motion/main_js.go"), 'package main\nimport ("syscall/js"; "fixture.test/hotline/games/hotlinemiami/engine")\nfunc main() {js.Global().Set("fixtureVersion", engine.Value()); select {}}\n');
  const image = Buffer.from("fixture image bytes");
  writeFileSync(path.join(directory, ".local/hotlinemiami/assets/images/fixture.png"), image);
  const source = { version: 1, sourceSHA256: "a".repeat(64), images: { fixture: { file: "images/fixture.png", bytes: image.length, sha256: digest(image), width: 1, height: 1 } }, sprites: {}, maps: Array.from({ length: counts.maps }, (_, index) => ({ id: `map-${index}`, image: "fixture" })), characters: Array.from({ length: counts.characters }, (_, index) => ({ id: `character-${index}`, image: "fixture" })), weapons: Array.from({ length: counts.weapons }, (_, index) => ({ id: `weapon-${index}`, image: "fixture" })) };
  writeFileSync(path.join(directory, ".local/hotlinemiami/assets/manifest.json"), JSON.stringify(source));
  return { directory, source };
}

test("missing or incomplete assets fail without publishing a manifest", () => {
  const { directory, source } = fixture();
  try {
    source.maps = [];
    writeFileSync(path.join(directory, ".local/hotlinemiami/assets/manifest.json"), JSON.stringify(source));
    assert.throws(() => prepareHotlineDelivery(directory), /maps|地图|complete/i);
    rmSync(path.join(directory, ".local/hotlinemiami/assets"), { recursive: true });
    assert.throws(() => prepareHotlineDelivery(directory), /assets|素材|manifest/i);
    assert.equal(existsSync(path.join(directory, ".local/hotlinemiami/delivery/public/hotlinemiami/manifest.json")), false);
    results.push({ scenario: "missing-assets", passed: true });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("actual Go WASM delivery binds source, verifies assets and reuses identical builds", () => {
  const { directory } = fixture();
  try {
    const publicRoot = prepareHotlineDelivery(directory);
    assert.equal(publicRoot, path.join(directory, ".local/hotlinemiami/delivery/public"));
    const gameRoot = path.join(publicRoot, "hotlinemiami");
    const before = JSON.parse(readFileSync(path.join(gameRoot, "manifest.json"), "utf8"));
    const verified = verifyHotlineDelivery(directory, publicRoot);
    assert.equal(verified.passed, true);
    assert(WebAssembly.validate(readFileSync(path.join(gameRoot, before.runtime.wasm.file))));
    for (const descriptor of [before.runtime.wasm, before.runtime.script, before.runtime.license]) {
      const bytes = readFileSync(path.join(gameRoot, descriptor.file));
      assert.equal(bytes.length, descriptor.bytes);
      assert.equal(digest(bytes), descriptor.sha256);
    }
    const script = readFileSync(path.join(gameRoot, before.runtime.script.file), "utf8");
    const scanner = ts.createScanner(ts.ScriptTarget.Latest, false, ts.LanguageVariant.Standard, script);
    for (let kind = scanner.scan(); kind !== ts.SyntaxKind.EndOfFileToken; kind = scanner.scan()) assert(![ts.SyntaxKind.SingleLineCommentTrivia, ts.SyntaxKind.MultiLineCommentTrivia].includes(kind));
    assert.match(script, /globalThis\.Go/);
    assert.match(readFileSync(path.join(gameRoot, before.runtime.license.file), "utf8"), /Redistribution and use in source and binary forms/);
    const modifiedAt = statSync(path.join(gameRoot, "manifest.json"), { bigint: true }).mtimeNs;
    assert.equal(prepareHotlineDelivery(directory), publicRoot);
    assert.equal(statSync(path.join(gameRoot, "manifest.json"), { bigint: true }).mtimeNs, modifiedAt);
    assert.equal(buildHotlineRuntime(directory).cached, true);
    writeFileSync(path.join(directory, "games/hotlinemiami/engine/value.go"), "package engine\nfunc Value() int { return 2 }\n");
    prepareHotlineDelivery(directory);
    const after = JSON.parse(readFileSync(path.join(gameRoot, "manifest.json"), "utf8"));
    assert.notEqual(after.runtime.sourceSHA256, before.runtime.sourceSHA256);
    assert.notEqual(after.runtime.buildSHA256, before.runtime.buildSHA256);
    assert.notEqual(after.runtime.wasm.sha256, before.runtime.wasm.sha256);
    const sourceBefore = after.runtime.sourceSHA256;
    writeFileSync(path.join(directory, "games/hotlinemiami/data/maps.go"), "package data\nvar Version = 2\n");
    prepareHotlineDelivery(directory);
    const withData = JSON.parse(readFileSync(path.join(gameRoot, "manifest.json"), "utf8"));
    assert.notEqual(withData.runtime.sourceSHA256, sourceBefore);
    results.push({ scenario: "actual-build-cache-source-binding", passed: true, sourceSHA256: withData.runtime.sourceSHA256, wasmSHA256: withData.runtime.wasm.sha256 });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("corruption and path traversal are rejected while the previous delivery is preserved", () => {
  const { directory, source } = fixture();
  try {
    const publicRoot = prepareHotlineDelivery(directory);
    const manifestPath = path.join(publicRoot, "hotlinemiami/manifest.json");
    const previous = readFileSync(manifestPath);
    writeFileSync(path.join(directory, ".local/hotlinemiami/assets/images/fixture.png"), "corrupt");
    assert.throws(() => prepareHotlineDelivery(directory), /digest|size|完整|摘要|bytes/i);
    assert.deepEqual(readFileSync(manifestPath), previous);
    source.images.fixture.file = "../escape.png";
    writeFileSync(path.join(directory, ".local/hotlinemiami/assets/manifest.json"), JSON.stringify(source));
    assert.throws(() => prepareHotlineDelivery(directory), /path|路径|escape/i);
    const manifest = JSON.parse(previous);
    writeFileSync(path.join(publicRoot, "hotlinemiami", manifest.runtime.wasm.file), "invalid wasm");
    assert.throws(() => verifyHotlineDelivery(directory, publicRoot), /digest|size|WASM|bytes/i);
    results.push({ scenario: "corrupt-and-traversal", passed: true });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("hydrated formal resources remain verifiable without local research assets", () => {
  const { directory } = fixture();
  try {
    const publicRoot = prepareHotlineDelivery(directory);
    const hydrated = path.join(directory, "games/hotlinemiami/assets/public/hotlinemiami");
    cpSync(path.join(publicRoot, "hotlinemiami"), hydrated, { recursive: true });
    renameSync(path.join(directory, ".local/hotlinemiami/assets"), path.join(directory, ".local/hotlinemiami/assets-backup"));
    rmSync(publicRoot, { recursive: true });
    assert.equal(prepareHotlineDelivery(directory), publicRoot);
    assert.equal(verifyHotlineDelivery(directory, publicRoot).passed, true);
    results.push({ scenario: "hydrated-source", passed: true });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("the previous three-map five-character seven-weapon source cannot satisfy the expanded release", () => {
  const { directory } = fixture({ maps: 3, characters: 5, weapons: 7 });
  try {
    assert.throws(() => prepareHotlineDelivery(directory), /Incomplete Hotline maps: expected 6/);
    assert.equal(existsSync(path.join(directory, ".local/hotlinemiami/delivery/public/hotlinemiami/manifest.json")), false);
    results.push({ scenario: "previous-source-content-rejected", passed: true });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("a fully hydrated previous content manifest is rejected despite valid runtime and resource hashes", () => {
  const { directory } = fixture();
  try {
    const publicRoot = prepareHotlineDelivery(directory);
    const hydrated = path.join(directory, "games/hotlinemiami/assets/public/hotlinemiami");
    cpSync(path.join(publicRoot, "hotlinemiami"), hydrated, { recursive: true });
    const previous = JSON.parse(readFileSync(path.join(hydrated, "manifest.json"), "utf8"));
    previous.maps = previous.maps.slice(0, 3);
    previous.characters = previous.characters.slice(0, 5);
    previous.weapons = previous.weapons.slice(0, 7);
    writeFileSync(path.join(hydrated, "manifest.json"), JSON.stringify(previous));
    renameSync(path.join(directory, ".local/hotlinemiami/assets"), path.join(directory, ".local/hotlinemiami/assets-backup"));
    rmSync(publicRoot, { recursive: true });
    assert.throws(() => prepareHotlineDelivery(directory), /Incomplete Hotline maps: expected 6/);
    assert.equal(existsSync(path.join(publicRoot, "hotlinemiami/manifest.json")), false);
    results.push({ scenario: "previous-hydrated-content-rejected", passed: true });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("output symlinks cannot redirect delivery into another local directory", () => {
  const { directory } = fixture();
  try {
    const redirected = path.join(directory, ".local/redirected");
    mkdirSync(redirected, { recursive: true });
    symlinkSync(redirected, path.join(directory, ".local/hotlinemiami/delivery"), "dir");
    assert.throws(() => prepareHotlineDelivery(directory), /symlink|path/i);
    assert.equal(existsSync(path.join(redirected, "public")), false);
    results.push({ scenario: "output-symlink-rejected", passed: true });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("resource symlinks are rejected even when they point inside the asset root", () => {
  const { directory } = fixture();
  try {
    const image = path.join(directory, ".local/hotlinemiami/assets/images/fixture.png");
    const target = path.join(directory, ".local/hotlinemiami/assets/images/actual.png");
    renameSync(image, target);
    symlinkSync(target, image);
    assert.throws(() => prepareHotlineDelivery(directory), /symlink|path/i);
    results.push({ scenario: "resource-symlink-rejected", passed: true });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test.after(() => {
  writeFileSync(path.join(output, "result.json"), JSON.stringify({ passed: results.length === 8 && results.every(row => row.passed), cases: results }, null, 2) + "\n");
});
