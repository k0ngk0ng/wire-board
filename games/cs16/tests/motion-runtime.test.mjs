import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";
import { gunzipSync } from "node:zlib";
import { buildCS16Runtime } from "../build/runtime-delivery.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/cs16/verification");
const assets = [path.join(root, ".local/cs16/assets"), path.join(root, "games/cs16/assets/public/cs16")].find(directory => existsSync(path.join(directory, "manifest.json")));
if (!assets) throw Error("Missing verified CS16 assets; generate or restore the registered resources first");
for (const name of ["verification", "go-cache", "go-mod", "tmp"]) mkdirSync(path.join(root, ".local/cs16", name), { recursive: true });
const env = { ...process.env, GOCACHE: path.join(root, ".local/cs16/go-cache"), GOMODCACHE: path.join(root, ".local/cs16/go-mod"), GOTMPDIR: path.join(root, ".local/cs16/tmp"), TMPDIR: path.join(root, ".local/cs16/tmp"), GOTOOLCHAIN: "local", GOTELEMETRY: "off", GOENV: "off" };

test("四张原图的原生与WASM共享运动逐步一致", { timeout: 180000 }, async () => {
  const compiled = buildCS16Runtime(root);
  await import(pathToFileURL(path.join(compiled.root, compiled.runtime.script.file)).href);
  const runtime = new globalThis.Go();
  const { instance } = await WebAssembly.instantiate(readFileSync(path.join(compiled.root, compiled.runtime.wasm.file)), runtime.importObject);
  void runtime.run(instance);
  assert.equal(globalThis.cs16Motion.version, 1);
  assert.equal(globalThis.cs16Motion.unit, 256);
  assert.equal(globalThis.cs16Motion.fps, 100);
  assert.equal(globalThis.cs16Motion.sourceSHA256, compiled.runtime.sourceSHA256);
  assert.equal(typeof globalThis.cs16Motion.canSmooth, "function");
  assert.equal(globalThis.cs16Motion.init("{}"), false);
  const manifest = JSON.parse(readFileSync(path.join(assets, "manifest.json"), "utf8"));
  const report = { runtime: compiled.runtime.buildSHA256, passed: true, maps: [] };
  for (const mapId of ["de_dust2", "de_inferno", "de_aztec", "cs_assault"]) {
    const descriptor = manifest.maps[mapId].rules;
    const packed = readFileSync(path.join(assets, descriptor.file));
    const map = (descriptor.encoding === "gzip" ? gunzipSync(packed) : packed).toString("utf8");
    assert.equal(globalThis.cs16Motion.init(map), true, mapId);
    const rules = JSON.parse(map);
    const steps = Array.from({ length: 120 }, (_, index) => ({ frames: 2, brushes: (rules.brushes ?? []).map(brush => ({ id: brush.id, open: index >= 60 && brush.kind.includes("door"), health: brush.health })), input: { seq: index + 1, life: 1, viewFrame: 0, moveX: index < 60 ? 0 : 1000, moveY: 1000, yaw: Math.floor(index / 30) * 16384, pitch: 0, fire: false, secondary: false, jump: index === 20 || index === 80, crouch: index >= 40 && index < 60, walk: index >= 100, reload: false, use: false, drop: false, select: "" } }));
    const oracle = spawnSync("go", ["run", "./games/cs16/cmd/motioncheck"], { cwd: root, env, encoding: "utf8", input: JSON.stringify({ mapId, steps }), maxBuffer: 8 * 1024 * 1024 });
    assert.equal(oracle.status, 0, oracle.stderr);
    const native = JSON.parse(oracle.stdout);
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
    report.maps.push({ id: mapId, steps: steps.length, frames: steps.length * 2, matched: true });
  }
  writeFileSync(path.join(output, "motion-runtime.json"), JSON.stringify(report, null, 2) + "\n");
});
