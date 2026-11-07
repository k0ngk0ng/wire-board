import assert from "node:assert/strict";
import test from "node:test";
import { ResourceDownloads, runDownloads } from "../web/downloads.ts";

test("streamed chunks report aggregate real bytes, not completed-file counts", async () => {
  const samples = [];
  const d = new ResourceDownloads(
    { "/a": 8, "/b": 4 },
    (p) => samples.push(p),
    ["art"],
  );
  d.plan(["/a", "/b"]);
  d.planned("art");
  const fetcher = async (url) =>
    new Response(
      new ReadableStream({
        start(c) {
          c.enqueue(new Uint8Array(url === "/a" ? 3 : 1));
          setTimeout(() => {
            c.enqueue(new Uint8Array(url === "/a" ? 5 : 3));
            c.close();
          }, 10);
        },
      }),
    );
  await Promise.all([
    d.bytes("/a", "/a", undefined, fetcher),
    d.bytes("/b", "/b", undefined, fetcher),
  ]);
  assert.ok(
    samples.some((p) => p.loaded > 0 && p.loaded < 12 && p.total === 12),
  );
  assert.equal(samples.at(-1).loaded, 12);
  assert.equal(samples.at(-1).total, 12);
  assert.equal(samples.at(-1).percent, 100);
});
test("size mismatches fail and never count as a completed asset", async () => {
  const samples = [];
  const d = new ResourceDownloads({ "/a": 8 }, (p) => samples.push(p), ["art"]);
  d.plan(["/a"]);
  d.planned("art");
  await assert.rejects(
    d.bytes("/a", "/a", undefined, async () => new Response(new Uint8Array(3))),
    /大小|不完整/,
  );
  assert.equal(samples.at(-1).loaded, 0);
  assert.ok(samples.at(-1).percent < 100);
});
test("planning both runtime and art cannot prematurely reach 100 percent", () => {
  const samples = [];
  const d = new ResourceDownloads(
    { "/a": 8, "/wasm": 100 },
    (p) => samples.push(p),
    ["art", "runtime"],
  );
  d.plan(["/a"]);
  d.complete("/a");
  d.planned("art");
  assert.ok(samples.at(-1).percent < 100);
  d.plan(["/wasm"]);
  d.planned("runtime");
  assert.equal(samples.at(-1).total, 108);
  assert.equal(samples.at(-1).loaded, 8);
});
test("download concurrency is bounded while every asset is retained", async () => {
  let active = 0,
    max = 0,
    done = 0;
  await runDownloads(
    Array.from({ length: 31 }, (_, i) => i),
    async () => {
      active++;
      max = Math.max(max, active);
      await new Promise((r) => setTimeout(r, 1));
      active--;
      done++;
    },
    6,
  );
  assert.equal(done, 31);
  assert.ok(max <= 6 && max > 1);
});
