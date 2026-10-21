import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { resolveBasketballAssetsBase } from "../../games/basketball/web/asset-base.ts";

test("basketball pin changes only the known production host, preserving explicit mirrors", () => {
  const pin = JSON.parse(readFileSync(new URL("../../games/basketball/assets/release-manifest.json", import.meta.url)));
  assert.equal(resolveBasketballAssetsBase(""), pin.assetsBaseURL);
  assert.equal(resolveBasketballAssetsBase("/"), pin.assetsBaseURL);
  assert.equal(resolveBasketballAssetsBase("https://cdn.ichenj.com/bga/v20261002-1/"), pin.assetsBaseURL);
  assert.equal(resolveBasketballAssetsBase("https://cdn.ichenj.com/another-game"), pin.assetsBaseURL);
  assert.equal(resolveBasketballAssetsBase("http://127.0.0.1:9911/fixture/"), "http://127.0.0.1:9911/fixture");
  assert.equal(resolveBasketballAssetsBase("http://cdn.ichenj.com:9911/fixture"), "http://cdn.ichenj.com:9911/fixture");
  assert.equal(resolveBasketballAssetsBase("/custom/local"), "/custom/local");
  assert.equal(resolveBasketballAssetsBase("https://mirror.example/bga"), "https://mirror.example/bga");
  assert.equal(resolveBasketballAssetsBase("https://cdn.ichenj.com.evil.invalid/base"), "https://cdn.ichenj.com.evil.invalid/base");
});
