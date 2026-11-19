import test from "node:test";
import assert from "node:assert/strict";
import { assetBase, assetURL } from "./cdn-base.mjs";

const origin = "https://cdn.example";
const base = origin + "/bga/";

test("local builds use local assets and releases use a configured stable bga root", () => {
  assert.equal(assetBase(), "/");
  assert.equal(assetBase(base, origin), base);
  for (const bad of ["http://cdn.example/bga/", "https://other.example/bga/", origin + "/bga/web/" + "a".repeat(40) + "/", origin + "/bga/v20261002-1/", origin + "/bga/../", "//cdn.example/bga/"]) assert.throws(() => assetBase(bad, origin));
});

test("asset references resolve once under the configured root", () => {
  assert.equal(assetURL("/", "assets/a.js"), "/assets/a.js");
  assert.equal(assetURL(base, "assets/a.js"), base + "assets/a.js");
});
