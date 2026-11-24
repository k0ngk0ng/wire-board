import test from "node:test";
import assert from "node:assert/strict";
import { loadingClientID } from "../src/game-api/loading-session.ts";

test("automatic remounts and explicit retries retain the page loading identity", () => {
  const client = loadingClientID();
  assert.match(client, /^[a-f0-9-]{36}$/);
  assert.equal(new Set(Array.from({ length: 100 }, () => loadingClientID())).size, 1);
  assert.equal(loadingClientID(), client);
});

test("independent page module instances cannot confirm each other's load", async () => {
  const first = await import("../src/game-api/loading-session.ts?first-page");
  const second = await import("../src/game-api/loading-session.ts?second-page");
  assert.notEqual(first.loadingClientID(), second.loadingClientID());
  assert.notEqual(first.loadingClientID(), loadingClientID());
  assert.equal(first.loadingClientID(), first.loadingClientID());
});
