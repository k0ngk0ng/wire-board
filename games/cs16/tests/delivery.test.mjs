import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, writeFileSync, readFileSync, existsSync, symlinkSync, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync, gunzipSync } from "node:zlib";
import { resourceDescriptors, validateResource, validateAssets, sourceIdentity } from "../build/runtime-delivery.mjs";
import * as delivery from "../build/runtime-delivery.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const testRoot = path.join(root, ".local/cs16/verification/delivery-tests");
mkdirSync(testRoot, { recursive: true });
const sha = value => createHash("sha256").update(value).digest("hex");
const maps = ["de_dust2", "de_inferno", "de_aztec", "cs_assault", "fy_iceworld", "cs_bloodstrike"];
const mapMode = id => id === "cs_assault" ? "hostage" : id === "fy_iceworld" || id === "cs_bloodstrike" ? "elimination" : "bomb";

function manifestFixture(item) {
  return {
    schema: 1,
    maps: Object.fromEntries(maps.map(id => [id, { mode: mapMode(id), sourceSHA256: "1".repeat(64), render: item, rules: item, preview: item }])),
    players: Object.fromEntries(["terror", "leet", "arctic", "guerilla", "urban", "gsg9", "sas", "gign"].map(id => [id, item])),
    weapons: Object.fromEntries(["glock18", "usp", "p228", "deagle", "elite", "fiveseven", "m3", "xm1014", "mp5navy", "tmp", "p90", "mac10", "ump45", "galil", "famas", "ak47", "m4a1", "sg552", "aug", "scout", "awp", "g3sg1", "sg550", "m249"].map(id => [id, { view: item, player: item, world: item }])),
  };
}

function rulesFixture(t) {
  const { directory, item } = fixture(t);
  const worlds = maps.map(id => ({ id, mode: mapMode(id), sourceSHA256: "1".repeat(64), planes: [{ x: 1048576, y: 0, z: 0, dist: 0 }], sourceFields: { retained: true, escaped: "quoted \\\" { value }" } }));
  const data = path.join(directory, "games/cs16/data");
  mkdirSync(data, { recursive: true });
  const rows = worlds.map(world => JSON.stringify(world));
  writeFileSync(path.join(data, "maps.go"), "package data\n\nconst MapsJSON = `[" + rows.join(",") + "]`\n");
  const manifest = manifestFixture(item);
  for (const world of worlds) {
    const raw = Buffer.from(JSON.stringify(world));
    const packed = gzipSync(raw);
    const file = world.id + "-rules.json.gz";
    writeFileSync(path.join(directory, file), packed);
    manifest.maps[world.id].rules = { file, bytes: packed.length, sha256: sha(packed), mime: "application/json", encoding: "gzip", decodedBytes: raw.length, decodedSHA256: sha(raw) };
  }
  return { directory, manifest, worlds, rows };
}

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

test("首版资产必须完整包含六图八人物与24枪械", () => {
  assert.throws(() => validateAssets({ schema: 1, maps: {}, players: {}, weapons: {} }), /map/i);
});

test("有名字但缺少实际地图或枪械模型的清单不能发布", t => {
  const { item } = fixture(t);
  const manifest = manifestFixture(item);
  assert.doesNotThrow(() => validateAssets(manifest));
  const missingModel = structuredClone(manifest);
  delete missingModel.weapons.ak47.view;
  assert.throws(() => validateAssets(missingModel), /ak47/);
  const missingRules = structuredClone(manifest);
  delete missingRules.maps.de_dust2.rules;
  assert.throws(() => validateAssets(missingRules), /de_dust2/);
  for (const mapId of ["fy_iceworld", "cs_bloodstrike"]) {
    const missingMap = structuredClone(manifest);
    delete missingMap.maps[mapId];
    assert.throws(() => validateAssets(missingMap), new RegExp(mapId));
    const wrongMode = structuredClone(manifest);
    wrongMode.maps[mapId].mode = "hostage";
    assert.throws(() => validateAssets(wrongMode), new RegExp(mapId));
  }
});

test("运行时规则身份保留服务端原始对象字节和六图模式", t => {
  const { directory, worlds, rows, manifest } = rulesFixture(t);
  const identities = delivery.mapRulesIdentity(directory);
  assert.deepEqual(Object.keys(identities), maps);
  for (const [index, world] of worlds.entries()) {
    assert.equal(identities[world.id].rulesSHA256, sha(rows[index]));
    assert.equal(identities[world.id].rulesSHA256, manifest.maps[world.id].rules.decodedSHA256);
    assert.equal(identities[world.id].sourceSHA256, world.sourceSHA256);
    assert.equal(identities[world.id].mode, mapMode(world.id));
  }
  const padded = worlds.map(world => JSON.stringify(world, null, 2));
  writeFileSync(path.join(directory, "games/cs16/data/maps.go"), "package data\nconst MapsJSON = `[" + padded.join(",") + "]`\n");
  const exact = delivery.mapRulesIdentity(directory);
  assert.equal(exact.fy_iceworld.rulesSHA256, sha(padded[4]));
  assert.notEqual(exact.fy_iceworld.rulesSHA256, sha(JSON.stringify(worlds[4])));
});

test("公开规则即使重新计算资源摘要也不能偏离服务端规则", t => {
  const { directory, manifest, worlds } = rulesFixture(t);
  assert.deepEqual(Object.keys(delivery.validateMapRules(directory, directory, manifest)), maps);
  const changed = { ...worlds[4], planes: [{ x: 1048576, y: 0, z: 0, dist: 256 }] };
  const raw = Buffer.from(JSON.stringify(changed));
  const packed = gzipSync(raw);
  const item = manifest.maps.fy_iceworld.rules;
  writeFileSync(path.join(directory, item.file), packed);
  manifest.maps.fy_iceworld.rules = { ...item, bytes: packed.length, sha256: sha(packed), decodedBytes: raw.length, decodedSHA256: sha(raw) };
  assert.throws(() => delivery.validateMapRules(directory, directory, manifest), /fy_iceworld/);
});

test("真实清单的六图资源及公开规则逐字节绑定服务端", () => {
  const selected = [path.join(root, ".local/cs16/assets"), path.join(root, "games/cs16/assets/public/cs16")].find(directory => existsSync(path.join(directory, "manifest.json")));
  assert.ok(selected, "需要恢复或生成真实六图资源");
  const manifest = JSON.parse(readFileSync(path.join(selected, "manifest.json"), "utf8"));
  validateAssets(manifest);
  const identities = delivery.mapRulesIdentity(root);
  const verified = delivery.validateMapRules(root, selected, manifest, identities);
  for (const mapId of maps) {
    for (const key of ["render", "rules", "preview"]) validateResource(selected, manifest.maps[mapId][key]);
    const item = manifest.maps[mapId].rules;
    const packed = validateResource(selected, item);
    const raw = item.encoding === "gzip" ? gunzipSync(packed) : packed;
    const rules = JSON.parse(raw);
    assert.equal(sha(raw), item.decodedSHA256 ?? item.sha256);
    assert.equal(sha(raw), sha(JSON.stringify(rules)));
    assert.equal(sha(raw), identities[mapId].rulesSHA256);
    assert.equal(rules.mode, mapMode(mapId));
  }
  writeFileSync(path.join(root, ".local/cs16/verification/delivery-six-maps.json"), JSON.stringify({ passed: true, sourceSHA256: sourceIdentity(root).sha256, maps: verified }, null, 2) + "\n");
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
