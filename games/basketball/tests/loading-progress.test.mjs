// Failures: fake timer progress, truncated transfer accepted as complete,
// integrity failure marked ready, failed HTTP silently retried from container.
import test from "node:test";
import assert from "node:assert/strict";
import { fetchLoadingBytes } from "../web/loading.ts";
test("streamed progress reflects downloaded bytes and verifies content", async () => {
  const saved = globalThis.fetch,
    progress = [];
  const bytes = new TextEncoder().encode("abcd");
  const sha = Buffer.from(
    await crypto.subtle.digest("SHA-256", bytes),
  ).toString("hex");
  globalThis.fetch = async () =>
    new Response(
      new ReadableStream({
        start(c) {
          c.enqueue(bytes.slice(0, 2));
          c.enqueue(bytes.slice(2));
          c.close();
        },
      }),
      { headers: { "Content-Length": "4" } },
    );
  try {
    assert.deepEqual(
      new Uint8Array(
        await fetchLoadingBytes(
          "https://example.test/runtime",
          (p) => progress.push(p),
          { bytes: 4, sha256: sha },
        ),
      ),
      bytes,
    );
    assert.deepEqual(
      progress.map((p) => p.loaded),
      [0, 2, 4],
    );
    assert.ok(progress.every((p) => p.total === 4));
    await assert.rejects(
      fetchLoadingBytes("https://example.test/runtime", () => {}, {
        bytes: 5,
        sha256: sha,
      }),
    );
    await assert.rejects(
      fetchLoadingBytes("https://example.test/runtime", () => {}, {
        bytes: 4,
        sha256: "0".repeat(64),
      }),
    );
    globalThis.fetch = async () => new Response("missing", { status: 404 });
    await assert.rejects(
      fetchLoadingBytes("https://example.test/runtime", () => {}),
    );
  } finally {
    globalThis.fetch = saved;
  }
});
