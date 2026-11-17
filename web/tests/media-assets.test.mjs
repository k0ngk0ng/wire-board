import test from "node:test";
import assert from "node:assert/strict";
import { mediaAssetsBase, codeMediaBase } from "../src/media-assets.ts";

test("official published media uses the verified application redirect catalog", () => {
  assert.equal(mediaAssetsBase("https://cdn.ichenj.com/bga/web/" + "a".repeat(40), "https://board.example"), "https://board.example/_media");
  assert.equal(mediaAssetsBase("https://cdn.ichenj.com/bga/web/" + "a".repeat(40) + "/", "https://board.example"), "https://board.example/_media");
  assert.equal(mediaAssetsBase("https://mirror.example/games", "https://board.example"), "https://mirror.example/games");
  assert.equal(mediaAssetsBase("", "https://board.example"), "");
  assert.equal(mediaAssetsBase("https://cdn.ichenj.com/bga/v20261002-1", "https://board.example"), "https://board.example/_media");
});

test("program roots preserve fixture roots and route published payloads through the catalog", () => {
  assert.equal(codeMediaBase("/"), "/");
  assert.equal(codeMediaBase("https://cdn.ichenj.com/bga/web/" + "a".repeat(40) + "/"), "/_media");
});
