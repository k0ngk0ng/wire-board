import test from "node:test";
import assert from "node:assert/strict";
import { mediaAssetsBase, codeMediaBase } from "../src/media-assets.ts";

test("official published media uses the verified application redirect catalog", () => {
  assert.equal(mediaAssetsBase("https://cdn.ichenj.com/bga/web/" + "a".repeat(40), "https://board.example"), "https://board.example/_media");
  assert.equal(mediaAssetsBase("https://cdn.ichenj.com/bga/web/" + "a".repeat(40) + "/", "https://board.example"), "https://board.example/_media");
  assert.equal(mediaAssetsBase("https://mirror.example/games", "https://board.example"), "https://mirror.example/games");
  assert.equal(mediaAssetsBase("", "https://board.example"), "");
});

test("legacy and external media roots retain their published image addresses", () => {
  for (const base of [
    "https://cdn.ichenj.com/bga/v20261002-1",
    "https://cdn.ichenj.com/bga/v20261002-1/",
    "https://cdn.ichenj.com/bga/v20261002-1/splendor",
    "https://cdn.ichenj.com/bga/web/" + "a".repeat(39),
    "https://cdn.ichenj.com/bga/web/" + "a".repeat(40) + "/splendor",
    "https://cdn.ichenj.com.example/bga/web/" + "a".repeat(40),
  ]) assert.equal(mediaAssetsBase(base, "https://board.example"), base);
});

test("program roots preserve fixture roots and route published payloads through the catalog", () => {
  assert.equal(codeMediaBase("/"), "/");
  assert.equal(codeMediaBase("https://cdn.ichenj.com/bga/web/" + "a".repeat(40) + "/"), "/_media");
});

test("canonical game roots use the bga route independently of the deployment domain", () => {
  assert.equal(mediaAssetsBase("https://cdn.example/bga", "https://board.example"), "https://board.example/bga");
  assert.equal(mediaAssetsBase("https://cdn.example/bga/", "https://board.example"), "https://board.example/bga");
  assert.equal(codeMediaBase("https://cdn.example/bga/"), "/bga");
});
