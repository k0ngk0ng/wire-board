/** Real offline candidate -> game build hook -> existing public collector.
 * Uses an isolated collector workspace; does not replace active web public files. */
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { loadBasketballAssetPin, prepareBasketballAssets, verifyBasketballAssets, verifyBasketballPublicFiles } from "../../games/basketball/build/assets.mjs";
import { preparePublicAssets } from "../../web/scripts/public-assets.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const packageDirectory = process.argv[2];
assert.ok(packageDirectory, "Pass a verified package directory inside this checkout");
const out = path.join(root, ".local/basketball-asset-build-acceptance");
mkdirSync(out, { recursive: true });
const publicRoot = prepareBasketballAssets(root, { packageDirectory });
verifyBasketballAssets(root, publicRoot);
const pin = loadBasketballAssetPin(root);
const first = path.join(publicRoot, pin.entries[0].urlPath.slice(1)), mtime = statSync(first).mtimeMs;
assert.equal(prepareBasketballAssets(root, { packageDirectory }), publicRoot);
assert.equal(statSync(first).mtimeMs, mtime, "warm cache rewrote asset bytes");
const workspace = path.join(out, "collector-workspace");
for (const sub of ["web/public", "games"]) mkdirSync(path.join(workspace, sub), { recursive: true });
writeFileSync(path.join(workspace, "web/public/asset-build-health.txt"), "collector test\n");
const collected = preparePublicAssets(workspace, [publicRoot]);
verifyBasketballPublicFiles(root, collected, { exact: false });
assert.ok(existsSync(path.join(collected, "asset-build-health.txt")));
assert.ok(!existsSync(path.join(collected, "provenance.json")));
assert.ok(!existsSync(path.join(collected, "package-manifest.json")));
const hash = b => createHash("sha256").update(b).digest("hex");
const paths = directory => readdirSync(directory, { withFileTypes: true }).flatMap(e => e.isDirectory() ? paths(path.join(directory, e.name)) : [path.relative(collected, path.join(directory, e.name)).replaceAll(path.sep, "/")]);
const files = paths(collected).sort();
assert.equal(files.length, pin.entries.length + 1);
assert.ok(files.every(p => !/\.(nes|nsf|bin|html|log)$/.test(p)));
const result = { status: "passed", source: "verified offline package", objects: pin.entries.length, bytes: pin.entries.reduce((n, e) => n + e.bytes, 0), contentSHA256: pin.contentSHA256, publicRoot: path.relative(root, publicRoot), collected: path.relative(root, collected), pinSHA256: hash(readFileSync(path.join(root, "games/basketball/assets/release-manifest.json"))), warmCacheUnchanged: true, collectorHasFullLocalFallback: true, networkUsed: false, uploaded: false };
writeFileSync(path.join(out, "result.json"), JSON.stringify(result, null, 2) + "\n");
console.log(JSON.stringify(result));
