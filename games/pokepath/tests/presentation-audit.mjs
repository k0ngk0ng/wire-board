import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { inspectNativePixelInput } from "./pixel-response.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const verification = join(root, ".local/pokepath/verification");
const [sourcePath, extra] = process.argv.slice(2);
assert.ok(sourcePath && !extra, "Expected one owned native-frame report path");
const owned = path => { const candidate = resolve(path); assert.ok(candidate.startsWith(verification + "/"), "Pixel audit input must belong to the local verification directory"); return candidate; };
const sourceBytes = await readFile(owned(sourcePath));
const source = JSON.parse(sourceBytes);
const sha = bytes => createHash("sha256").update(bytes).digest("hex");
assert.ok(source.pixelResponse?.frames?.length > 0, "The source has no original native frames");
const output = join(verification, "native-pixel-audit");
const label = relative(verification, sourcePath).replace(/\.json$/, "").replace(/[^a-z0-9-]/g, "-");
const records = [];
let rect = source.pixelResponse.rect;
for (const frame of source.pixelResponse.frames) {
  const png = await readFile(owned(frame.screenshot.path));
  assert.equal(sha(png), frame.screenshot.sha256);
  if (!rect) {
    const sx = png.readUInt32BE(16) / frame.metadata.deviceWidth, sy = png.readUInt32BE(20) / frame.metadata.deviceHeight;
    rect = { x: frame.bounds.left / sx, y: frame.bounds.top / sy, width: frame.bounds.width / sx, height: frame.bounds.height / sy };
  }
  records.push({ id: frame.id, data: png.toString("base64"), metadata: frame.metadata, receivedAtMS: frame.receivedAtMS });
}
const trace = { frames: records, rect, input: source.pixelResponse.trustedInput, timeOriginMS: source.pixelResponse.timeOriginMS };
const pixelResponse = { selector: source.pixelResponse.selector, ...await inspectNativePixelInput({ trace, output, label }) };
const result = { ...source, originalReport: { path: owned(sourcePath), sha256: sha(sourceBytes) }, pixelResponse, auditOnly: true, imageAnalysis: "Offline native PNG decoding and cropping; no browser connection or page evaluation" };
const target = join(output, label + ".json");
await writeFile(target, JSON.stringify(result, null, 2) + "\n");
process.stdout.write(JSON.stringify({ path: target, available: pixelResponse.available, firstChangedCaptureMS: pixelResponse.firstChangedCaptureMS }) + "\n");
