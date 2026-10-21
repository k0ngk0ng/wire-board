import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, symlinkSync, existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { materializeBasketballAssets, prepareBasketballAssets, verifyBasketballAssets, verifyBasketballPublicFiles, loadBasketballAssetPin } from "../../games/basketball/build/assets.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const parent = path.join(root, ".local/basketball-asset-build-tests");
mkdirSync(parent, { recursive: true });
const sha = b => createHash("sha256").update(b).digest("hex");
const canonical = value => JSON.stringify(value, Object.keys(value).sort());
const encode = v => Buffer.from(JSON.stringify(sort(v)) + "\n");
function sort(v) { if (Array.isArray(v)) return v.map(sort); if (v && typeof v === "object") return Object.fromEntries(Object.keys(v).sort().map(k => [k, sort(v[k])])); return v; }
const ROM = "0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b";
function fixture(t) {
  const dir = mkdtempSync(path.join(parent, "fixture-")); t.after(() => rmSync(dir, { recursive: true, force: true }));
  const files = new Map([
    ["/basketball/v1/audio/music-076-intro-loop.mp3", [Buffer.from("ID3-delivery"), "audio/mpeg"]],
    ["/basketball/v1/graphics/court/tile.png", [Buffer.from("89504e470d0a1a0a", "hex"), "image/png"]],
    ["/basketball/v1/selection/manifest.json", [Buffer.from('{"runtime":true}\n'), "application/json"]],
  ]);
  const entries = [...files].map(([urlPath, [data, mime]]) => ({ urlPath, output: "public" + urlPath, sha256: sha(data), bytes: data.length, mime }));
  const contentSHA256 = sha(encode({ sourceROM: ROM, entries }));
  const pin = { schema: 1, game: "basketball", sourceROM: ROM, contentSHA256, assetsBaseURL: "https://cdn.ichenj.com/bga/basketball/v1-" + contentSHA256, entries: entries.map(({ output, ...e }) => e) };
  const pinFile = path.join(dir, "games/basketball/assets/release-manifest.json"); mkdirSync(path.dirname(pinFile), { recursive: true }); writeFileSync(pinFile, encode(pin));
  const pkg = path.join(dir, "candidate"); mkdirSync(pkg);
  writeFileSync(path.join(pkg, "package-manifest.json"), encode({ ...pin, entries, ossBucket: "stoneage-assets", ossPrefix: "bga/basketball/v1-" + contentSHA256 }));
  for (const [p, [data]] of files) { const file = path.join(pkg, "public", p); mkdirSync(path.dirname(file), { recursive: true }); writeFileSync(file, data); }
  return { dir, files, entries, pin, pinFile, pkg };
}
test("cold immutable download, verified warm cache, corrupt cache repair and failed repair", async t => {
  const f = fixture(t), calls = [];
  const fetchImpl = async (url, options) => { calls.push(url); assert.equal(options.redirect, "error"); assert.ok(url.startsWith(f.pin.assetsBaseURL)); const [data, mime] = f.files.get(url.slice(f.pin.assetsBaseURL.length)); return new Response(data, { headers: { "Content-Type": mime, ...(mime === "application/json" ? { "Content-Encoding": "gzip", "Content-Length": "2" } : {}) } }); };
  const publicRoot = await materializeBasketballAssets(f.dir, { fetchImpl });
  assert.equal(calls.length, 3); verifyBasketballAssets(f.dir, publicRoot);
  const again = await materializeBasketballAssets(f.dir, { fetchImpl: () => { throw Error("warm cache fetched"); } }); assert.equal(again, publicRoot);
  const damaged = path.join(publicRoot, "basketball/v1/audio/music-076-intro-loop.mp3"); writeFileSync(damaged, "corrupt");
  await assert.rejects(materializeBasketballAssets(f.dir, { fetchImpl: async () => new Response("missing", { status: 404 }) }));
  assert.throws(() => verifyBasketballAssets(f.dir, publicRoot));
  await materializeBasketballAssets(f.dir, { fetchImpl }); assert.equal(sha(readFileSync(damaged)), f.entries[0].sha256);
});
test("wrong hash/MIME/length and network failure never return a public artifact", async t => {
  for (const mode of ["hash", "mime", "length", "network"]) {
    const f = fixture(t);
    await assert.rejects(materializeBasketballAssets(f.dir, { fetchImpl: async url => {
      if (mode === "network") throw Error("offline");
      const [data, mime] = f.files.get(url.slice(f.pin.assetsBaseURL.length));
      return new Response(mode === "hash" ? Buffer.alloc(data.length) : mode === "length" ? Buffer.concat([data, Buffer.from("x")]) : data, { headers: { "Content-Type": mode === "mime" ? "text/html" : mime } });
    } }));
    assert.throws(() => verifyBasketballAssets(f.dir));
  }
});
test("explicit offline package and sync hook match committed pin without network", async t => {
  const f = fixture(t);
  const p = await materializeBasketballAssets(f.dir, { packageDirectory: f.pkg, fetchImpl: () => { throw Error("offline package fetched"); } });
  assert.equal(prepareBasketballAssets(f.dir, { packageDirectory: f.pkg }), p);
  writeFileSync(path.join(f.pkg, f.entries[0].output), "bad");
  await assert.rejects(materializeBasketballAssets(f.dir, { packageDirectory: f.pkg }));
  assert.throws(() => prepareBasketballAssets(f.dir, { packageDirectory: f.pkg }));
});
test("pin identity, MIME, paths, duplicates and source mismatch fail closed", async t => {
  for (const change of [p => p.sourceROM = "0".repeat(64), p => p.entries[0].urlPath = "/basketball/v1/../private.nes", p => p.assetsBaseURL = "https://evil.invalid", p => p.entries.push(p.entries[0]), p => p.entries[0].mime = "text/html", p => p.contentSHA256 = "0".repeat(64)]) {
    const f = fixture(t); change(f.pin); writeFileSync(f.pinFile, encode(f.pin)); assert.throws(() => loadBasketballAssetPin(f.dir));
  }
});
test("source/cache symlinks and extra files cannot enter the collector", async t => {
  const f = fixture(t), target = path.join(f.pkg, f.entries[0].output); rmSync(target); symlinkSync(path.join(f.pkg, f.entries[1].output), target);
  await assert.rejects(materializeBasketballAssets(f.dir, { packageDirectory: f.pkg }));
  rmSync(target); writeFileSync(target, f.files.get(f.entries[0].urlPath)[0]);
  const p = await materializeBasketballAssets(f.dir, { packageDirectory: f.pkg });
  writeFileSync(path.join(p, "secret.nes"), "private"); assert.throws(() => verifyBasketballAssets(f.dir, p));
  writeFileSync(path.join(p, "basketball/v1/graphics/court/private.json"), "{}"); assert.throws(() => verifyBasketballPublicFiles(f.dir, p, { exact: false }));
  await materializeBasketballAssets(f.dir, { packageDirectory: f.pkg }); assert.ok(!existsSync(path.join(p, "secret.nes")));
  assert.throws(() => prepareBasketballAssets(f.dir, { packageDirectory: root }));
});
test("a changed pin does not fall back to a previous verified identity", async t => {
  const f = fixture(t); const old = await materializeBasketballAssets(f.dir, { packageDirectory: f.pkg });
  const p = structuredClone(f.pin); p.entries[0].sha256 = "1".repeat(64); const entries = p.entries.map(e => ({ ...e, output: "public" + e.urlPath })); p.contentSHA256 = sha(encode({ sourceROM: ROM, entries })); p.assetsBaseURL = "https://cdn.ichenj.com/bga/basketball/v1-" + p.contentSHA256; writeFileSync(f.pinFile, encode(p));
  await assert.rejects(materializeBasketballAssets(f.dir, { fetchImpl: async () => new Response(null, { status: 404 }) }));
  assert.ok(existsSync(old)); assert.throws(() => verifyBasketballAssets(f.dir, old));
});
test("music76 accepts only its exact intro-loop delivery filename", t => {
  const f = fixture(t); assert.equal(loadBasketballAssetPin(f.dir).entries[0].urlPath, "/basketball/v1/audio/music-076-intro-loop.mp3");
  for (const name of ["music-075-intro-loop.mp3", "music-076-loop.mp3", "music-076.mp3", "music-076-intro-loop.wav"]) {
    const pin = structuredClone(f.pin); pin.entries[0].urlPath = "/basketball/v1/audio/" + name;
    const entries = pin.entries.map(e => ({ ...e, output: "public" + e.urlPath })); pin.contentSHA256 = sha(encode({ sourceROM: ROM, entries })); pin.assetsBaseURL = "https://cdn.ichenj.com/bga/basketball/v1-" + pin.contentSHA256;
    writeFileSync(f.pinFile, encode(pin)); assert.throws(() => loadBasketballAssetPin(f.dir), name);
  }
});
