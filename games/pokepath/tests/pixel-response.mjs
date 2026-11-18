import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { cropPNG, decodePNG } from "./native-png.mjs";

const workspace = fileURLToPath(new URL("../../../", import.meta.url));
const digest = bytes => createHash("sha256").update(bytes).digest("hex");

export async function recordNativePixelInput({ call, evaluate, listen, rect, readInput, observeMS = 600 }) {
  assert.ok([rect.x, rect.y, rect.width, rect.height].every(Number.isFinite) && rect.x >= 0 && rect.y >= 0 && rect.width > 0 && rect.height > 0, "Invalid visible native control rectangle");
  assert.ok(observeMS >= 500 && observeMS <= 3000);
  const frames = [];
  let active = true, overflow = false, ready;
  const initial = new Promise(resolve => { ready = resolve; });
  const unsubscribe = listen((method, value) => {
    if (!active || method !== "Page.screencastFrame") return;
    if (frames.length >= 128) overflow = true;
    else frames.push({ id: frames.length + 1, data: value.data, metadata: value.metadata, receivedAtMS: Date.now() });
    ready();
  });
  let timeout;
  try {
    const timeOriginMS = await evaluate("performance.timeOrigin");
    await call("Page.enable");
    await call("Input.dispatchMouseEvent", { type: "mouseMoved", x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 });
    await evaluate(`(async()=>{const target=document.elementFromPoint(${rect.x + rect.width / 2},${rect.y + rect.height / 2})?.closest('button');if(target)await Promise.all(target.getAnimations({subtree:true}).filter(animation=>Number.isFinite(animation.effect?.getTiming().iterations)).map(animation=>animation.finished.catch(()=>{})));await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))));return true})()`);
    await call("Page.startScreencast", { format: "png", everyNthFrame: 1 });
    await Promise.race([initial, new Promise((_, reject) => { timeout = setTimeout(() => reject(Error("No initial native screenshot frame")), 3000); })]);
    clearTimeout(timeout);
    const position = { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2, button: "left", clickCount: 1 };
    await call("Input.dispatchMouseEvent", { type: "mousePressed", ...position });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...position });
    const input = await readInput();
    assert.ok(input?.type === "click" && Number.isFinite(input.at), "No trusted native click was observed");
    await new Promise(resolve => setTimeout(resolve, observeMS));
    assert.equal(overflow, false, "Native screenshot trace exceeded its bounded frame limit");
    return { frames, input, timeOriginMS, rect };
  } finally {
    clearTimeout(timeout);
    try { await call("Page.stopScreencast"); } finally { active = false; unsubscribe(); }
  }
}

export async function inspectNativePixelInput({ trace, output, label }) {
  assert.match(label, /^[a-z0-9][a-z0-9-]+$/);
  const local = relative(workspace, resolve(output));
  assert.ok(!local.startsWith("..") && !local.startsWith("/") && local.startsWith(".local/pokepath/verification"));
  assert.ok(trace.frames.length > 0 && Number.isFinite(trace.timeOriginMS));
  for (const frame of trace.frames) {
    assert.ok(Number.isFinite(frame.metadata?.timestamp), "Native frame has no compositor timestamp");
    assert.ok(frame.metadata.timestamp * 1000 >= trace.timeOriginMS - 1000, "Native frame uses a different timestamp domain");
    assert.ok(frame.metadata.deviceWidth > 0 && frame.metadata.deviceHeight > 0 && frame.metadata.offsetTop === 0, "Unsupported native screenshot geometry");
    assert.ok(frame.metadata.pageScaleFactor === 1, "Page zoom changed during native input capture");
  }
  await mkdir(output, { recursive: true });
  const records = [];
  const contentBuffers = new Map();
  for (const frame of trace.frames) {
    const raw = Buffer.from(frame.data, "base64"), image = decodePNG(raw), rect = trace.rect;
    const sx = image.width / frame.metadata.deviceWidth, sy = image.height / frame.metadata.deviceHeight;
    const left = Math.floor(rect.x * sx), top = Math.floor(rect.y * sy), right = Math.ceil((rect.x + rect.width) * sx), bottom = Math.ceil((rect.y + rect.height) * sy);
    const bounds = { left, top, width: right - left, height: bottom - top };
    const ix = Math.min(Math.round(6 * sx), Math.floor(bounds.width / 4)), iy = Math.min(Math.round(6 * sy), Math.floor(bounds.height / 4));
    const contentBounds = { left: left + ix, top: top + iy, width: bounds.width - ix * 2, height: bounds.height - iy * 2 };
    const croppedPixels = cropPNG(image, bounds), innerPixels = cropPNG(image, contentBounds);
    const cropped = croppedPixels.png, contentPixels = innerPixels.png;
    const screenshot = join(output, `${label}-frame-${frame.id}.png`), crop = join(output, `${label}-crop-${frame.id}.png`), content = join(output, `${label}-content-${frame.id}.png`);
    await writeFile(screenshot, raw);
    await writeFile(crop, cropped);
    await writeFile(content, contentPixels);
    const contentData = innerPixels.rgba;
    contentBuffers.set(frame.id, contentData);
    records.push({ id: frame.id, capturedAtMS: frame.metadata.timestamp * 1000, receivedAtMS: frame.receivedAtMS, metadata: frame.metadata, bounds, contentBounds, rgbaSHA256: digest(croppedPixels.rgba), contentSHA256: digest(contentData), screenshot: { path: screenshot, bytes: raw.length, sha256: digest(raw) }, crop: { path: crop, bytes: cropped.length, sha256: digest(cropped) }, content: { path: content, bytes: contentPixels.length, sha256: digest(contentPixels) } });
  }
  records.sort((a, b) => a.capturedAtMS - b.capturedAtMS || a.id - b.id);
  const inputAtMS = trace.timeOriginMS + trace.input.at;
  const before = records.filter(frame => frame.capturedAtMS <= inputAtMS).at(-1);
  assert.ok(before, "No native screenshot precedes the input");
  const final = records.at(-1);
  const baselinePixels = contentBuffers.get(before.id), finalPixels = contentBuffers.get(final.id), eligible = [];
  const channels = [0, 1, 2];
  const chromaticResidual = (pixels, pixel) => {
    const baselineSum = channels.reduce((sum, channel) => sum + baselinePixels[pixel + channel], 0);
    if (!baselineSum) return [0, 0, 0];
    const scale = channels.reduce((sum, channel) => sum + pixels[pixel + channel], 0) / baselineSum;
    return channels.map(channel => pixels[pixel + channel] - baselinePixels[pixel + channel] * scale);
  };
  assert.equal(baselinePixels.length, finalPixels.length, "Native content rectangle changed size");
  for (let pixel = 0; pixel < baselinePixels.length; pixel += 4) {
    const brightens = channels.some(channel => finalPixels[pixel + channel] > baselinePixels[pixel + channel]);
    const residual = chromaticResidual(finalPixels, pixel);
    if (brightens || residual.some(value => Math.abs(value) >= 4)) eligible.push({ pixel, brightens, residual });
  }
  const minimumPixels = Math.ceil(eligible.length / 2);
  for (const frame of records) {
    const current = contentBuffers.get(frame.id);
    assert.equal(current.length, baselinePixels.length, "Native content rectangle changed size");
    frame.resultPixels = eligible.filter(({ pixel, brightens, residual }) => [0, 1, 2, 3].every(channel => {
      const offset = pixel + channel, value = current[offset];
      return value >= Math.min(baselinePixels[offset], finalPixels[offset]) && value <= Math.max(baselinePixels[offset], finalPixels[offset]);
    }) && (brightens ? channels.some(channel => current[pixel + channel] > baselinePixels[pixel + channel]) : (() => {
      const moved = chromaticResidual(current, pixel);
      return moved.every((value, channel) => value >= Math.min(0, residual[channel]) - 1 && value <= Math.max(0, residual[channel]) + 1) && moved.some((value, channel) => Math.abs(value) >= 2 && value * residual[channel] > 0);
    })())).length;
  }
  const after = final.contentSHA256 !== before.contentSHA256 ? records.find(frame => frame.capturedAtMS > inputAtMS && (eligible.length ? frame.resultPixels >= minimumPixels : frame.contentSHA256 === final.contentSHA256)) : undefined;
  const result = { available: Boolean(after), method: "Actual native compositor PNG content; conservative visible-result evidence excludes focus borders and proportional press darkening, with stable-target fallback; frame-swap and reception timestamps are separate", trustedInput: trace.input, timeOriginMS: trace.timeOriginMS, rect: trace.rect, frames: records, baselineFrame: before.id, finalFrame: final.id, resultEvidence: { eligiblePixels: eligible.length, positivePixels: eligible.filter(pixel => pixel.brightens).length, chromaticPixels: eligible.filter(pixel => !pixel.brightens).length, minimumPixels, chromaticTargetMinimum: 4, chromaticMovementMinimum: 2, chromaticRoundingTolerance: 1, rule: "At least half of result pixels move toward the stable result with every RGBA channel within its before/final interval; darkening requires chromatic residual beyond proportional brightness and rounding" } };
  if (after) {
    const previous = records.filter(frame => frame.capturedAtMS <= after.capturedAtMS && frame.contentSHA256 === before.contentSHA256).at(-1);
    result.firstChangedFrame = after.id;
    result.firstChangedCaptureMS = { lower: Math.max(0, previous.capturedAtMS - inputAtMS), upper: after.capturedAtMS - inputAtMS };
    result.firstChangedReceiptMS = after.receivedAtMS - inputAtMS;
  } else result.reason = "No changed native control pixels were captured after the trusted input";
  await writeFile(join(output, `${label}-native-pixels.json`), JSON.stringify(result, null, 2) + "\n");
  return result;
}
