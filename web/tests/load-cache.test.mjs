// Failure scenarios specified before implementation: duplicate warm reads,
// failed/aborted loads becoming cached, version/origin/size collisions, and a
// warm resource skipping the caller's cancellation check.
import test from "node:test";
import assert from "node:assert/strict";
import { VerifiedAssetCache } from "../src/game-api/load-cache.ts";
const file = { url: "/game/v1/table.webp", bytes: 1024 };
const signal = () => new AbortController().signal;
test("same verified build resource is loaded once and reports warm completion", async () => {
  const cache = new VerifiedAssetCache();
  let calls = 0;
  const progress = [];
  const load = async () => {
    calls++;
  };
  assert.equal(
    await cache.load(
      file,
      "build-a",
      "https://cdn.test",
      signal(),
      (n) => progress.push(n),
      load,
    ),
    "loaded",
  );
  assert.equal(
    await cache.load(
      file,
      "build-a",
      "https://cdn.test",
      signal(),
      (n) => progress.push(n),
      load,
    ),
    "cached",
  );
  assert.equal(calls, 1);
  assert.equal(progress.at(-1), 1024);
});
test("failed validation never poisons retry", async () => {
  const cache = new VerifiedAssetCache();
  await assert.rejects(
    cache.load(
      file,
      "a",
      "",
      signal(),
      () => {},
      async () => {
        throw Error("size mismatch");
      },
    ),
  );
  assert.equal(cache.has(file, "a", ""), false);
  let retry = 0;
  await cache.load(
    file,
    "a",
    "",
    signal(),
    () => {},
    async () => {
      retry++;
    },
  );
  assert.equal(retry, 1);
});
test("aborted completed read is not retained", async () => {
  const cache = new VerifiedAssetCache(),
    abort = new AbortController();
  await assert.rejects(
    cache.load(
      file,
      "a",
      "",
      abort.signal,
      () => {},
      async () => {
        abort.abort();
      },
    ),
  );
  assert.equal(cache.has(file, "a", ""), false);
});
test("build, origin, URL and expected length independently invalidate cache", async () => {
  const cache = new VerifiedAssetCache();
  await cache.load(
    file,
    "a",
    "cdn-a",
    signal(),
    () => {},
    async () => {},
  );
  assert.equal(cache.has(file, "b", "cdn-a"), false);
  assert.equal(cache.has(file, "a", "cdn-b"), false);
  assert.equal(
    cache.has({ ...file, url: "/game/v2/table.webp" }, "a", "cdn-a"),
    false,
  );
  assert.equal(cache.has({ ...file, bytes: 2048 }, "a", "cdn-a"), false);
});
test("cached loads still reject a cancelled consumer", async () => {
  const cache = new VerifiedAssetCache();
  await cache.load(
    file,
    "a",
    "",
    signal(),
    () => {},
    async () => {},
  );
  const a = new AbortController();
  a.abort();
  await assert.rejects(
    cache.load(
      file,
      "a",
      "",
      a.signal,
      () => {},
      async () => assert.fail("unexpected read"),
    ),
  );
});
