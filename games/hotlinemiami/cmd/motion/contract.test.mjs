import assert from "node:assert/strict";
import { test } from "node:test";
import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { webcrypto } from "node:crypto";

const root = resolve(import.meta.dirname, "../../../..");
const output = resolve(root, ".local/hotlinemiami/verification/motion");
mkdirSync(output, { recursive: true });

test("native WASM movement hook shares integer collision and bounded prediction", async () => {
  const built = spawnSync("go", ["build", "-o", resolve(output, "motion.wasm"), "./games/hotlinemiami/cmd/motion"], {
    cwd: root,
    env: { ...process.env, GOOS: "js", GOARCH: "wasm", GOTELEMETRY: "off", GOCACHE: resolve(root, ".local/hotlinemiami/cache/go"), GOTMPDIR: resolve(root, ".local/hotlinemiami/tmp") },
    encoding: "utf8",
  });
  assert.equal(built.status, 0, built.stderr);
  const goroot = spawnSync("go", ["env", "GOROOT"], { cwd: root, env: { ...process.env, GOTELEMETRY: "off" }, encoding: "utf8" }).stdout.trim();
  globalThis.crypto ??= webcrypto;
  await import(pathToFileURL(resolve(goroot, "lib/wasm/wasm_exec.js")).href);
  const go = new globalThis.Go();
  const result = await WebAssembly.instantiate(readFileSync(resolve(output, "motion.wasm")), go.importObject);
  void go.run(result.instance);
  const motion = globalThis.hotlineMotion;
  assert.equal(motion.version, 1);
  assert.equal(motion.unit, 256);
  const viewport = motion.viewport();
  assert.deepEqual(viewport, { width: 576, height: 360 });
  assert.equal(Number.isInteger(viewport.width) && Number.isInteger(viewport.height), true);
  assert.equal(viewport.width * 5, viewport.height * 8);
  const weapons = JSON.parse(motion.weapons());
  assert.equal(weapons.length, 12);
  assert.equal(weapons.find((weapon) => weapon.id === "pistol").cooldown, 15);
  const unarmed = JSON.parse(motion.unarmed());
  assert.equal(unarmed.id, "");
  assert.equal(unarmed.cooldown, 22);
  assert.equal(unarmed.range, 25 * 256);
  assert.equal(unarmed.melee, true);
  assert.equal(weapons.some((weapon) => weapon.id === ""), false);
  assert.equal(motion.init(JSON.stringify({ id: "fixture", width: 163840, height: 122880, walls: [{ x: 32768, y: 0, w: 256, h: 122880 }], spawns: [], doors: [] })), true);
  const moved = motion.move(25600, 51200, 0, 1000, 0, 9);
  assert.ok(moved[0] > 25600 && moved[0] <= 32768 - 9 * 256);
  assert.deepEqual(moved.slice(1), [51200, 0]);
  const blocked = motion.move(moved[0], 51200, 0, 1000, 0, 9);
  assert.equal(blocked[0], moved[0]);
  assert.equal(motion.init("{}"), false);
  writeFileSync(resolve(output, "result.json"), JSON.stringify({ status: "pass", moved, blocked, viewport, unit: 256, frames: 9 }) + "\n");
});
