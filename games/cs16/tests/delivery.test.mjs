import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, writeFileSync, symlinkSync, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";
import { resourceDescriptors, validateResource, validateAssets, sourceIdentity } from "../build/runtime-delivery.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const testRoot = path.join(root, ".local/cs16/verification/delivery-tests");
mkdirSync(testRoot, { recursive: true });
const sha = value => createHash("sha256").update(value).digest("hex");

function fixture(t) {
  const directory = mkdtempSync(path.join(testRoot, "case-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const content = Buffer.from("cs16-delivery-fixture");
  writeFileSync(path.join(directory, "asset.bin"), content);
  return { directory, content, item: { file: "asset.bin", bytes: content.length, sha256: sha(content), mime: "application/octet-stream" } };
}

test("资源目录读取完整性，重复描述只计一次", t => {
  const { directory, content, item } = fixture(t);
  assert.deepEqual(validateResource(directory, item), content);
  assert.deepEqual(resourceDescriptors({ a: item, b: [item] }), [item]);
  assert.throws(() => resourceDescriptors({ a: item, b: { ...item, bytes: item.bytes + 1 } }), /Conflicting/);
});

test("所有路径逃逸和符号链接均被拒绝", t => {
  const { directory, item } = fixture(t);
  for (const file of ["", "/asset.bin", "../asset.bin", "x/../asset.bin", "x/./asset.bin", "x//asset.bin", "x\\asset.bin", "x\u0000asset.bin"]) {
    assert.throws(() => validateResource(directory, { ...item, file }), /path/i, file);
  }
  symlinkSync(path.join(directory, "asset.bin"), path.join(directory, "linked.bin"));
  assert.throws(() => validateResource(directory, { ...item, file: "linked.bin" }), /symlink/i);
});

test("损坏的元数据和资源正文不能发布", t => {
  const { directory, item } = fixture(t);
  for (const bad of [{ bytes: 0 }, { bytes: 1 }, { bytes: Number.NaN }, { sha256: "bad" }, { sha256: "0".repeat(64) }, { mime: "" }, { mime: "text/plain\nheader" }]) {
    assert.throws(() => validateResource(directory, { ...item, ...bad }));
  }
  writeFileSync(path.join(directory, "asset.bin"), "tampered");
  assert.throws(() => validateResource(directory, item), /mismatch/i);
});

test("无损压缩资源同时验证传输字节与还原身份", t => {
  const { directory, content } = fixture(t);
  const packed = gzipSync(content);
  writeFileSync(path.join(directory, "asset.gz"), packed);
  const item = { file: "asset.gz", bytes: packed.length, sha256: sha(packed), mime: "application/octet-stream", encoding: "gzip", decodedBytes: content.length, decodedSHA256: sha(content) };
  assert.deepEqual(validateResource(directory, item), packed);
  assert.throws(() => validateResource(directory, { ...item, decodedSHA256: "0".repeat(64) }), /decoded/i);
  assert.throws(() => validateResource(directory, { ...item, encoding: "unknown" }), /encoding/i);
  assert.throws(() => resourceDescriptors({ a: item, b: { ...item, decodedBytes: item.decodedBytes + 1 } }), /Conflicting/);
});

test("首版资产必须完整包含四图八人物与24枪械", () => {
  assert.throws(() => validateAssets({ schema: 1, maps: {}, players: {}, weapons: {} }), /map/i);
});

test("有名字但缺少实际地图或枪械模型的清单不能发布", t => {
  const { item } = fixture(t);
  const manifest = {
    schema: 1,
    maps: Object.fromEntries(["de_dust2", "de_inferno", "de_aztec", "cs_assault"].map(id => [id, { render: item, rules: item, preview: item }])),
    players: Object.fromEntries(["terror", "leet", "arctic", "guerilla", "urban", "gsg9", "sas", "gign"].map(id => [id, item])),
    weapons: Object.fromEntries(["glock18", "usp", "p228", "deagle", "elite", "fiveseven", "m3", "xm1014", "mp5navy", "tmp", "p90", "mac10", "ump45", "galil", "famas", "ak47", "m4a1", "sg552", "aug", "scout", "awp", "g3sg1", "sg550", "m249"].map(id => [id, { view: item, player: item, world: item }])),
  };
  assert.doesNotThrow(() => validateAssets(manifest));
  const missingModel = structuredClone(manifest);
  delete missingModel.weapons.ak47.view;
  assert.throws(() => validateAssets(missingModel), /ak47/);
  const missingRules = structuredClone(manifest);
  delete missingRules.maps.de_dust2.rules;
  assert.throws(() => validateAssets(missingRules), /de_dust2/);
});

test("生产源码身份不受测试文件影响，源码修改必须换身份", t => {
  const { directory } = fixture(t);
  const engine = path.join(directory, "games/cs16/engine");
  mkdirSync(engine, { recursive: true });
  writeFileSync(path.join(directory, "go.mod"), "module fixture\ngo 1.26.0\n");
  writeFileSync(path.join(engine, "motion.go"), "package engine\nconst Unit = 256\n");
  const first = sourceIdentity(directory);
  writeFileSync(path.join(engine, "motion_test.go"), "package engine\n");
  assert.equal(sourceIdentity(directory).sha256, first.sha256);
  writeFileSync(path.join(engine, "motion.go"), "package engine\nconst Unit = 512\n");
  assert.notEqual(sourceIdentity(directory).sha256, first.sha256);
});
