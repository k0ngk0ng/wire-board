import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const execute = promisify(execFile);
const digest = bytes => createHash("sha256").update(bytes).digest("hex");
const options = process.argv.slice(2);
assert.ok(options.length === 0 || options.length === 1 && ["--items", "--criticals"].includes(options[0]), "Expected optional --items or --criticals comparison mode");
const critical = options.includes("--criticals");
const items = critical || options.includes("--items");
const archive = join(root, ".local/pokepath/verification", critical ? "first-wave-critical-items" : items ? "first-wave-items" : "first-wave-speeds");
const speeds = critical ? [.4, .8, 1] : [.4, .8, 1, 1.2, 1.7, 2, 2.5, 5];
const report = { command: `node games/pokepath/tests/first-wave-speeds.mjs${critical ? " --criticals" : items ? " --items" : ""}`, scope: "One complete route 1-1 wave per starter and playback speed, all original enemy sprites decoded before spawn, fixed original real 60 Hz, manual first targeting and recorded starter defaults", itemCombinations: items, controlledCriticalAndJitter: critical, speeds, cases: [], passed: false };
await mkdir(archive, { recursive: true });
try {
  for (const defaults of [false, true]) for (const speed of defaults ? [.8, 1] : speeds) {
    const args = ["games/pokepath/tests/first-wave-reference.mjs", "--ideal-60hz", "--warm-enemies", `--speed=${speed}`];
    if (defaults) args.push("--starter-defaults");
    if (items) args.push("--item-combinations");
    if (critical) args.push("--critical-combinations", "--coupled-jitter");
    const { stdout } = await execute(process.execPath, args, { cwd: root, env: process.env, timeout: 90000, maxBuffer: 4 << 20 });
    const summary = JSON.parse(stdout.trim().split("\n").at(-1));
    const path = join(summary.output, "result.json");
    const bytes = await readFile(path);
    const source = JSON.parse(bytes);
    assert.equal(source.passed, true);
    assert.equal(source.cases.length, critical ? 64 : items ? 22 : 4);
    for (const scene of source.cases) {
      assert.equal(scene.checks.outcome, true);
      assert.equal(scene.checks.hitDifference, null);
      assert.equal(scene.checks.shotDifference, null);
      assert.equal(digest(await readFile(scene.screenshot.path)), scene.screenshot.sha256);
    }
    report.cases.push({ speed, defaults, passed: true, scenarios: source.cases.length, rulesVersion: source.cases[0].native.rulesVersion, sha256: digest(bytes), path, largestShotDriftMS: Math.max(...source.cases.map(scene => scene.checks.largestShotDriftMS)), largestHitDriftMS: Math.max(...source.cases.map(scene => scene.checks.largestHitDriftMS)), largestCompletionDriftMS: Math.max(...source.cases.map(scene => Math.abs(scene.checks.completionDriftMS))) });
    process.stdout.write(`PASS ${speed}x ${defaults ? "starter defaults" : "first targeting"}\n`);
  }
  report.passed = true;
} finally {
  await writeFile(join(archive, "result.json"), JSON.stringify(report, null, 2) + "\n");
}
