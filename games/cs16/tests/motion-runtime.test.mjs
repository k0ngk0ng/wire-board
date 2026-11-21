import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";
import { gunzipSync } from "node:zlib";
import { buildCS16Runtime, sourceIdentity, mapRulesIdentity, validateAssets, validateResource, validateMapRules } from "../build/runtime-delivery.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/cs16/verification");
const assets = [path.join(root, ".local/cs16/assets"), path.join(root, "games/cs16/assets/public/cs16")].find(directory => existsSync(path.join(directory, "manifest.json")));
if (!assets) throw Error("Missing verified CS16 assets; generate or restore the registered resources first");
for (const name of ["verification", "go-cache", "go-mod", "tmp"]) mkdirSync(path.join(root, ".local/cs16", name), { recursive: true });
const env = { ...process.env, GOCACHE: path.join(root, ".local/cs16/go-cache"), GOMODCACHE: path.join(root, ".local/cs16/go-mod"), GOTMPDIR: path.join(root, ".local/cs16/tmp"), TMPDIR: path.join(root, ".local/cs16/tmp"), GOTOOLCHAIN: "local", GOTELEMETRY: "off", GOENV: "off" };
const maps = ["de_dust2", "de_inferno", "de_aztec", "cs_assault", "fy_iceworld", "cs_bloodstrike"];
const sha = value => createHash("sha256").update(value).digest("hex");

test("六张原图的实际资源与原生WASM规则身份及共享运动逐步一致", { timeout: 180000 }, async () => {
  const manifest = JSON.parse(readFileSync(path.join(assets, "manifest.json"), "utf8"));
  validateAssets(manifest);
  const identities = mapRulesIdentity(root);
  validateMapRules(root, assets, manifest, identities);
  const compiled = buildCS16Runtime(root);
  await import(pathToFileURL(path.join(compiled.root, compiled.runtime.script.file)).href);
  const runtime = new globalThis.Go();
  const { instance } = await WebAssembly.instantiate(readFileSync(path.join(compiled.root, compiled.runtime.wasm.file)), runtime.importObject);
  void runtime.run(instance);
  assert.equal(globalThis.cs16Motion.version, 1);
  assert.equal(globalThis.cs16Motion.unit, 256);
  assert.equal(globalThis.cs16Motion.fps, 100);
  assert.equal(globalThis.cs16Motion.sourceSHA256, compiled.runtime.sourceSHA256);
  assert.equal(compiled.runtime.sourceSHA256, sourceIdentity(root).sha256);
  assert.deepEqual(compiled.runtime.mapRules, identities);
  assert.equal(typeof globalThis.cs16Motion.canSmooth, "function");
  assert.equal(globalThis.cs16Motion.init("{}"), false);
  const report = { runtime: compiled.runtime.buildSHA256, sourceSHA256: compiled.runtime.sourceSHA256, wasmSHA256: compiled.runtime.wasm.sha256, passed: true, maps: [] };
  for (const mapId of maps) {
    const descriptor = manifest.maps[mapId].rules;
    const packed = validateResource(assets, descriptor);
    const map = (descriptor.encoding === "gzip" ? gunzipSync(packed) : packed).toString("utf8");
    assert.equal(globalThis.cs16Motion.init(map), true, mapId);
    const rules = JSON.parse(map);
    assert.equal(rules.id, mapId);
    assert.equal(rules.mode, identities[mapId].mode);
    assert.equal(rules.sourceSHA256, identities[mapId].sourceSHA256);
    assert.equal(sha(map), identities[mapId].rulesSHA256);
    assert.equal(sha(map), descriptor.decodedSHA256 ?? descriptor.sha256);
    assert.equal(sha(JSON.stringify(rules)), identities[mapId].rulesSHA256);
    if (mapId === "fy_iceworld" || mapId === "cs_bloodstrike") {
      assert.equal(rules.mode, "elimination");
      assert.equal(rules.bombsites.length, 0);
      assert.equal(rules.hostages.length, 0);
    }
    if (mapId === "fy_iceworld") {
      assert.equal(rules.worldWeapons.length, 24);
      assert.ok(rules.buyZones.length > 0);
    }
    const steps = Array.from({ length: 120 }, (_, index) => ({ frames: 2, brushes: (rules.brushes ?? []).map(brush => ({ id: brush.id, open: index >= 60 && brush.kind.includes("door"), health: brush.health })), input: { seq: index + 1, life: 1, viewFrame: 0, moveX: index < 60 ? 0 : 1000, moveY: 1000, yaw: Math.floor(index / 30) * 16384, pitch: 0, fire: false, secondary: false, jump: index === 20 || index === 80, crouch: index >= 40 && index < 60, walk: index >= 100, reload: false, use: false, drop: false, select: "" } }));
    const oracle = spawnSync("go", ["run", "./games/cs16/cmd/motioncheck"], { cwd: root, env, encoding: "utf8", input: JSON.stringify({ mapId, steps }), maxBuffer: 8 * 1024 * 1024 });
    assert.equal(oracle.status, 0, oracle.stderr);
    const native = JSON.parse(oracle.stdout);
    assert.equal(native.players.length, steps.length);
    assert.ok(native.players.some(player => player.x !== native.initial.x || player.y !== native.initial.y), mapId + " trajectory must move");
    const from = { x: native.initial.x, y: native.initial.y, z: native.initial.z };
    assert.equal(globalThis.cs16Motion.canSmooth(JSON.stringify({ from, to: from, crouch: false })), true);
    assert.equal(globalThis.cs16Motion.canSmooth(JSON.stringify({ from, to: { ...from, z: -32000 * 256 }, crouch: false })), false);
    assert.equal(globalThis.cs16Motion.canSmooth("{}"), false);
    assert.equal(globalThis.cs16Motion.canSmooth("invalid"), false);
    let player = native.initial;
    for (let index = 0; index < steps.length; index++) {
      const result = JSON.parse(globalThis.cs16Motion.move(JSON.stringify({ player, ...steps[index] })));
      assert.equal(result.error, undefined);
      assert.deepEqual(result, native.players[index], `${mapId} step ${index}`);
      player = result;
    }
    assert.equal(JSON.parse(globalThis.cs16Motion.move(JSON.stringify({ player, input: steps[0].input, frames: 10000 }))).error !== undefined, true);
    report.maps.push({ id: mapId, mode: rules.mode, sourceSHA256: identities[mapId].sourceSHA256, rulesSHA256: identities[mapId].rulesSHA256, resourceSHA256: descriptor.sha256, steps: steps.length, frames: steps.length * 2, inputsSHA256: sha(JSON.stringify(steps)), trajectorySHA256: sha(JSON.stringify(native.players)), matched: true });
  }
  assert.equal(sourceIdentity(root).sha256, compiled.runtime.sourceSHA256);
  writeFileSync(path.join(output, "motion-runtime.json"), JSON.stringify(report, null, 2) + "\n");
});
