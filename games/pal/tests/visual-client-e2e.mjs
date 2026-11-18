import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createRequire } from "node:module";
import { createHash } from "node:crypto";

const root = new URL("../../../", import.meta.url);
const out = new URL(".local/pal/visual-client-e2e/", root);
await fs.mkdir(out, { recursive: true });
const { build } = createRequire(new URL("web/package.json", root))("esbuild");
await build({ entryPoints: [new URL("games/pal/web/scene-renderer.ts", root).pathname], bundle: true, outfile: new URL("renderer.mjs", out).pathname, format: "esm", platform: "node" });
const { SceneRenderer } = await import(new URL("renderer.mjs", out));
const media = new URL(".local/pal/visual/pal/v1/", root);
const raw = await fs.readFile(new URL("index.json", media));
const assetID = createHash("sha256").update(raw).digest("hex");
const index = JSON.parse(raw), name = Object.keys(index.files).find(name => name.startsWith("atlases/mgo/") && name.endsWith(".png"));
const payload = await fs.readFile(new URL(name, media));
const base = "https://cdn.ichenj.com/bga/visual-fixture/";
const publication = { assetID, files: {
  "index.json": { url: base + "index.json", bytes: raw.length, sha256: assetID, mime: "application/json", encoding: "gzip" },
  [name]: { url: base + "retained-image.png", bytes: payload.length, sha256: index.files[name].sha256, mime: "image/png", encoding: "" },
} };
let calls = [], override;
globalThis.fetch = async (url) => {
  calls.push(url);
  assert.ok(String(url).startsWith(base), "visual publication attempted a local fallback");
  const file = url === publication.files["index.json"].url ? raw : payload;
  return override?.(url, file) ?? new Response(file, { headers: { "Content-Type": url.endsWith(".json") ? "application/json" : "image/png" } });
};
const renderer = (pin = publication) => new SceneRenderer("/unavailable/pal/v1", { publication: pin });
assert.deepEqual(Buffer.from(await renderer().bytes(name)), payload);
assert.deepEqual(calls, [publication.files["index.json"].url, publication.files[name].url]);
const cases = ["retained-url-with-no-local-assets"];
async function reject(label, pin, response, pattern) {
  override = response;
  await assert.rejects(renderer(pin).bytes(name), pattern);
  cases.push(label);
  override = undefined;
}
await reject("wrong-index-identity", { ...publication, assetID: "0".repeat(64) }, undefined, /清单校验/);
await reject("missing-published-file", { ...publication, files: { "index.json": publication.files["index.json"] } }, undefined, /发布记录/);
await reject("inconsistent-published-file", { ...publication, files: { ...publication.files, [name]: { ...publication.files[name], bytes: payload.length + 1 } } }, undefined, /发布记录/);
await reject("foreign-url", { ...publication, files: { ...publication.files, "index.json": { ...publication.files["index.json"], url: "https://example.com/index.json" } } }, undefined, /发布记录/);
await reject("wrong-mime", publication, () => new Response(raw, { headers: { "Content-Type": "text/plain" } }), /格式/);
await reject("oversized-stream", publication, () => new Response(Buffer.concat([raw, Buffer.from("x")]), { headers: { "Content-Type": "application/json" } }), /大小限制/);
await reject("corrupt-payload", publication, (url, file) => {
  if (url.endsWith(".json")) return undefined;
  const corrupt = Buffer.from(file);
  corrupt[0] ^= 1;
  return new Response(corrupt, { headers: { "Content-Type": "image/png" } });
}, /校验失败/);
const retry = renderer();
override = (url) => url.endsWith(".png") ? new Response("offline", { status: 503 }) : undefined;
await assert.rejects(retry.bytes(name), /加载失败/);
override = undefined;
assert.deepEqual(Buffer.from(await retry.bytes(name)), payload);
cases.push("failed-download-can-retry");
const report = { passed: true, scope: "actual renderer CDN download boundary with original visual index and PNG; no remote writes", assetID, cases };
await fs.writeFile(new URL("result.json", out), JSON.stringify(report, null, 2) + "\n");
process.stdout.write(JSON.stringify(report) + "\n");
