import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { classicFLACDataURL } from "../web/classic-media.ts";
import { ClassicHornPlayback } from "../web/classic-horn.ts";

// Browser E2E has already shown blob media blocked by the production CSP.
// Failure cases before this fix: disallowed URL scheme, changed binary bytes,
// base64 padding inserted between chunks, argument-stack overflow on large
// arrays, source mutation, private-player ownership, and lost loop releases.
const decode = url => Buffer.from(url.slice(url.indexOf(",") + 1), "base64");
const bytes = size => Uint8Array.from({ length: size }, (_, i) => (i * 197 + (i >>> 8)) & 255);

test("verified FLAC bytes use a data scheme permitted by the existing media CSP", () => {
  const source = bytes(256), original = source.slice();
  const url = classicFLACDataURL(source.buffer);
  assert.ok(url.startsWith("data:audio/flac;base64,"));
  assert.equal(new URL(url).protocol, "data:");
  const server = readFileSync(new URL("../../../internal/server/server.go", import.meta.url), "utf8");
  assert.match(server, /media-src 'self' data:/);
  assert.doesNotMatch(server.match(/media-src[^;]+/)[0], /blob:/);
  assert.deepEqual(decode(url), Buffer.from(source));
  assert.deepEqual(source, original);
  assert.throws(() => classicFLACDataURL(new ArrayBuffer(0)), /空/);
});

test("arbitrary bytes preserve padding across base64 chunk boundaries", () => {
  for (const length of [1, 2, 3, 4, 24575, 24576, 24577, 49151, 49152, 49153]) {
    const source = bytes(length), url = classicFLACDataURL(source.buffer);
    assert.equal(url.slice(url.indexOf(",") + 1), Buffer.from(source).toString("base64"));
    assert.deepEqual(decode(url), Buffer.from(source));
  }
});

test("large FLAC payloads encode without a giant argument spread or stack overflow", () => {
  const source = bytes(4 * 1024 * 1024 + 2), url = classicFLACDataURL(source.buffer);
  assert.equal(url.length, "data:audio/flac;base64,".length + 4 * Math.ceil(source.byteLength / 3));
  assert.deepEqual(decode(url), Buffer.from(source));
});

test("data sources retain native platform loop keys and stop ownership", () => {
  const starts = [], stops = [], url = classicFLACDataURL(bytes(256).buffer);
  const controller = new ClassicHornPlayback({
    loop(key, sources, options) { starts.push({ key, sources, options }); return Promise.resolve(true); },
    stopLoop(key) { stops.push(key); },
  });
  const cars = [{seat:0,x:0,y:0,finishAt:0,protected:true,horn:false}, {seat:1,x:10,y:0,finishAt:0,protected:false,horn:true}];
  controller.update(url, cars, 0, true, true);
  assert.deepEqual(starts.map(s => s.key), ["racing-horn-0", "racing-horn-1"]);
  assert.ok(starts.every(s => s.sources[0] === url && s.options.rate === 1));
  controller.update(url, cars, 0, false, true);
  assert.deepEqual(stops, ["racing-horn-0"]);
  controller.stop();
  assert.deepEqual(stops, ["racing-horn-0", "racing-horn-1"]);
});

test("both hooks stay within the platform media boundary without object URL resources", () => {
  for (const name of ["use-classic-engine.ts", "use-classic-horn.ts"]) {
    const source = readFileSync(new URL(`../web/${name}`, import.meta.url), "utf8");
    assert.doesNotMatch(source, /URL\.(?:createObjectURL|revokeObjectURL)/, name);
    assert.doesNotMatch(source, /new\s+(?:Audio|AudioContext|webkitAudioContext)\s*\(/, name);
  }
});
