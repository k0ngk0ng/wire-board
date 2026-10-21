import { loadCoverPin, verifyCover } from "./cover.mjs";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { closeSync, copyFileSync, existsSync, lstatSync, mkdirSync, mkdtempSync, openSync, readFileSync, readdirSync, readSync, realpathSync, renameSync, rmSync, writeFileSync, writeSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROM = "0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b";
const filename = fileURLToPath(import.meta.url);
const mimeTypes = { ".png": "image/png", ".json": "application/json", ".mp3": "audio/mpeg", ".wav": "audio/wav" };
const assert = (value, message) => { if (!value) throw Error(message); };
function sorted(v) { if (Array.isArray(v)) return v.map(sorted); if (v && typeof v === "object") return Object.fromEntries(Object.keys(v).sort().map(k => [k, sorted(v[k])])); return v; }
const encode = v => Buffer.from(JSON.stringify(sorted(v)) + "\n");
const hash = data => createHash("sha256").update(data).digest("hex");
function within(root, input) {
  const target = path.resolve(root, input), relative = path.relative(root, target);
  assert(relative && !relative.startsWith(".." + path.sep) && relative !== ".." && !path.isAbsolute(relative), "Asset path must stay inside checkout");
  let current = root;
  for (const piece of relative.split(path.sep)) { current = path.join(current, piece); const entry = lstatSync(current, { throwIfNoEntry: false }); if (entry) assert(!entry.isSymbolicLink(), "Asset path contains a symlink"); }
  return target;
}
function relativeAsset(value) {
  assert(typeof value === "string" && /^\/basketball\/v1\/[A-Za-z0-9_.\-/]+$/.test(value), "Invalid asset URL path");
  const pieces = value.slice(1).split("/"); assert(pieces.every(p => p && p !== "." && p !== ".."), "Invalid asset path component");
  const group = pieces[2], suffix = path.posix.extname(value), name = pieces.at(-1);
  if (group === "graphics") assert(pieces.length >= 5 && ["court", "courts", "static-atlas", "static-objects", "raster", "results", "postresult"].includes(pieces[3]) && [".png", ".json"].includes(suffix), "Unexpected graphic asset");
  else if (group === "selection") assert([".png", ".json"].includes(suffix), "Unexpected selection asset");
  else if (group === "audio") {
    const ordinaryMusic = /^music-(\d{3})(?:-loop)?\.mp3$/.exec(name);
    const music = name === "music-076-intro-loop.mp3" || (ordinaryMusic && ordinaryMusic[1] !== "076");
    assert(pieces.length === 4 && (/^sound-\d{3}\.wav$/.test(name) || music || name === "audio-playback.json"), "Unexpected audio asset");
  }
  else throw Error("Unexpected public group");
  if (suffix === ".json") assert(["manifest.json", "atlas.json", "audio-playback.json", "court-map.json", "initial-ring.json", "stream-columns.json"].includes(name), "Private JSON is not a public asset");
  return value.slice(1);
}
function hashFile(file) {
  const digest = createHash("sha256"), buffer = Buffer.allocUnsafe(1 << 20), fd = openSync(file, "r"); let bytes = 0;
  try { for (;;) { const n = readSync(fd, buffer, 0, buffer.length, null); if (!n) break; digest.update(buffer.subarray(0, n)); bytes += n; } } finally { closeSync(fd); }
  return { sha256: digest.digest("hex"), bytes };
}
export function loadBasketballAssetPin(directory) {
  const root = realpathSync(directory), file = within(root, "games/basketball/assets/release-manifest.json");
  const pin = JSON.parse(readFileSync(file, "utf8"));
  assert(pin.schema === 1 && pin.game === "basketball" && pin.sourceROM === ROM, "Basketball asset source identity differs");
  assert(Array.isArray(pin.entries) && pin.entries.length > 0 && /^[a-f0-9]{64}$/.test(pin.contentSHA256), "Invalid basketball asset pin");
  let previous = "";
  for (const e of pin.entries) {
    relativeAsset(e.urlPath); assert(e.urlPath > previous, "Duplicate or unsorted asset path"); previous = e.urlPath;
    assert(/^[a-f0-9]{64}$/.test(e.sha256) && Number.isSafeInteger(e.bytes) && e.bytes > 0 && e.bytes <= 512 * 1024 * 1024, "Invalid pinned asset digest/length");
    assert(e.mime === mimeTypes[path.posix.extname(e.urlPath)], "Pinned asset MIME differs");
    assert(Object.keys(e).sort().join(",") === "bytes,mime,sha256,urlPath", "Unexpected compact pin fields");
  }
  const entries = pin.entries.map(e => ({ ...e, output: "public" + e.urlPath }));
  assert(hash(encode({ sourceROM: ROM, entries })) === pin.contentSHA256, "Pinned content hash differs");
  assert(pin.assetsBaseURL === "https://cdn.ichenj.com/bga/basketball/v1-" + pin.contentSHA256, "Pinned CDN target differs");
  return pin;
}
function context(directory) {
  const root = realpathSync(directory), pin = loadBasketballAssetPin(root), parent = within(root, ".local/basketball-asset-build"), cache = within(root, path.join(parent, pin.contentSHA256));
  return { root, pin, parent, cache, publicRoot: path.join(cache, "public") };
}
function publicInventory(directory, prefix = "") {
  const out = [];
  for (const e of readdirSync(directory, { withFileTypes: true })) {
    const relative = prefix + e.name, p = path.join(directory, e.name);
    if (e.isDirectory()) out.push(...publicInventory(p, relative + "/"));
    else { assert(e.isFile(), "Public asset must be a regular file"); out.push(relative); }
  }
  return out.sort();
}
export function verifyBasketballPublicFiles(directory, publicDirectory, { exact = true } = {}) {
  const root = realpathSync(directory), pin = loadBasketballAssetPin(root), publicRoot = within(root, publicDirectory);
  for (const e of pin.entries) {
    const p = within(root, path.join(publicRoot, relativeAsset(e.urlPath))); assert(lstatSync(p).isFile(), "Asset is not a regular file");
    const got = hashFile(p); assert(got.bytes === e.bytes && got.sha256 === e.sha256, "Public asset differs: " + e.urlPath);
  }
  if (exact) assert(JSON.stringify(publicInventory(publicRoot)) === JSON.stringify(pin.entries.map(e => e.urlPath.slice(1))), "Unlisted or missing public assets");
  else {
    // A complete application dist also contains other games and this game's
    // separately verified Wasm runtime. Every basketball media file is pinned.
    const actual = publicInventory(path.join(publicRoot, "basketball/v1")).filter(p => !p.startsWith("runtime/")).map(p => "basketball/v1/" + p);
    const expected = pin.entries.map(e => e.urlPath.slice(1));
    if (existsSync(within(root, "games/basketball/assets/cover-manifest.json"))) {
      verifyCover(root, publicRoot);
      expected.push(loadCoverPin(root).localPath.slice(1));
    }
    assert(JSON.stringify(actual) === JSON.stringify(expected.sort()), "Unlisted basketball media in application artifact");
  }
  return pin;
}
export function verifyBasketballAssets(directory, suppliedPublicRoot) {
  const c = context(directory), publicRoot = suppliedPublicRoot ? within(c.root, suppliedPublicRoot) : c.publicRoot;
  assert(publicRoot === c.publicRoot, "Asset cache belongs to another pin");
  const stamp = JSON.parse(readFileSync(within(c.root, path.join(c.cache, "build.json")), "utf8"));
  assert(stamp.schema === 1 && stamp.contentSHA256 === c.pin.contentSHA256 && stamp.pinSHA256 === hash(encode(c.pin)), "Asset cache stamp differs");
  verifyBasketballPublicFiles(c.root, publicRoot); return publicRoot;
}
function valid(c) { try { return verifyBasketballAssets(c.root); } catch { return false; } }
function verifiedPackage(c, input) {
  const pkg = within(c.root, input), record = JSON.parse(readFileSync(within(c.root, path.join(pkg, "package-manifest.json")), "utf8"));
  assert(record.sourceROM === c.pin.sourceROM && record.contentSHA256 === c.pin.contentSHA256 && record.assetsBaseURL === c.pin.assetsBaseURL, "Candidate package does not match committed pin");
  assert(record.ossBucket === "stoneage-assets" && record.ossPrefix === "bga/basketball/v1-" + c.pin.contentSHA256, "Candidate package OSS target differs");
  const entries = c.pin.entries.map(e => ({ ...e, output: "public" + e.urlPath })); assert(hash(encode(entries)) === hash(encode(record.entries)), "Candidate public object list differs");
  verifyBasketballPublicFiles(c.root, path.join(pkg, "public")); return pkg;
}
async function download(c, entry, target, fetchImpl) {
  const response = await fetchImpl(c.pin.assetsBaseURL + entry.urlPath, { redirect: "error", signal: AbortSignal.timeout(60_000) });
  assert(response.ok && response.status === 200 && response.body, "Pinned asset download failed: " + entry.urlPath);
  assert((response.headers.get("content-type") ?? "").split(";")[0].trim().toLowerCase() === entry.mime, "Downloaded asset MIME differs: " + entry.urlPath);
  // Fetch exposes decoded bytes while Content-Length may describe compressed
  // transfer bytes. The streamed decoded count/hash below always stays exact.
  const declared = response.headers.get("content-length"), encoding = response.headers.get("content-encoding");
  if (declared !== null && (!encoding || encoding === "identity")) assert(Number(declared) === entry.bytes, "Downloaded asset length header differs");
  const fd = openSync(target, "wx"), reader = response.body.getReader(), digest = createHash("sha256"); let bytes = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read(); if (done) break; bytes += value.byteLength;
      assert(bytes <= entry.bytes, "Downloaded asset exceeds pinned size"); digest.update(value);
      let offset = 0; while (offset < value.byteLength) offset += writeSync(fd, value, offset, value.byteLength - offset);
    }
    assert(bytes === entry.bytes && digest.digest("hex") === entry.sha256, "Downloaded asset hash/length differs: " + entry.urlPath);
  } finally { closeSync(fd); await reader.cancel().catch(() => {}); }
}
export async function materializeBasketballAssets(directory, { packageDirectory, fetchImpl = fetch } = {}) {
  const c = context(directory), pkg = packageDirectory ? verifiedPackage(c, packageDirectory) : undefined;
  if (valid(c)) return c.publicRoot;
  mkdirSync(c.parent, { recursive: true }); const stage = mkdtempSync(path.join(c.parent, "prepare-"));
  try {
    const publicRoot = path.join(stage, "public"); mkdirSync(publicRoot);
    let next = 0;
    const workers = Array.from({ length: 4 }, async () => {
      for (;;) {
        const entry = c.pin.entries[next++]; if (!entry) return;
        const target = within(c.root, path.join(publicRoot, relativeAsset(entry.urlPath))); mkdirSync(path.dirname(target), { recursive: true });
        if (pkg) copyFileSync(within(c.root, path.join(pkg, "public", relativeAsset(entry.urlPath))), target);
        else await download(c, entry, target, fetchImpl);
      }
    });
    // Await all workers before deleting a failed staging tree.
    const results = await Promise.allSettled(workers), failed = results.find(r => r.status === "rejected"); if (failed) throw failed.reason;
    verifyBasketballPublicFiles(c.root, publicRoot);
    writeFileSync(path.join(stage, "build.json"), encode({ schema: 1, contentSHA256: c.pin.contentSHA256, pinSHA256: hash(encode(c.pin)), objects: c.pin.entries.length }));
    if (valid(c)) return c.publicRoot;
    within(c.root, c.cache); if (existsSync(c.cache)) rmSync(c.cache, { recursive: true, force: true });
    try { renameSync(stage, c.cache); } catch (error) { if (!valid(c)) throw error; }
    return verifyBasketballAssets(c.root);
  } finally { rmSync(stage, { recursive: true, force: true }); }
}
/** Synchronous Vite/public-collector hook. Only this Node module and the
 * committed compact pin are required in a clean checkout or Docker frontend. */
export function prepareBasketballAssets(directory, { packageDirectory = process.env.BASKETBALL_ASSET_PACKAGE } = {}) {
  const c = context(directory); if (packageDirectory) verifiedPackage(c, packageDirectory);
  if (valid(c)) return c.publicRoot;
  execFileSync(process.execPath, [filename, "--prepare", c.root, ...(packageDirectory ? ["--package", within(c.root, packageDirectory)] : [])], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"], maxBuffer: 1 << 20 });
  return verifyBasketballAssets(c.root);
}
if (process.argv[1] && path.resolve(process.argv[1]) === filename) {
  try {
    assert(process.argv[2] === "--prepare" && process.argv[3], "Use --prepare CHECKOUT [--package CANDIDATE]");
    const packageDirectory = process.argv[4] === "--package" ? process.argv[5] : undefined;
    const publicRoot = await materializeBasketballAssets(process.argv[3], { packageDirectory }); console.log(JSON.stringify({ publicRoot }));
  } catch (error) { console.error("Basketball asset preparation failed:", error.message); process.exitCode = 1; }
}
