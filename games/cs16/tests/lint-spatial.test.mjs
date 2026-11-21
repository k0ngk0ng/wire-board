import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/cs16/verification/lint");
mkdirSync(output, { recursive: true });

const cases = [
  { file: "web/src/audio-spatial.ts", source: "const context = new AudioContext(); const nativeOutput = context.createMediaStreamDestination();", passed: true },
  { file: "web/src/audio-spatial.ts", source: "const context = new AudioContext(); const media = new Audio();", passed: true },
  { file: "web/src/audio-spatial.ts", source: "const output_name = 1;", passed: false },
  { file: "web/src/audio-spatial.ts", source: "const value: any = 1;", passed: false },
  { file: "web/src/audio-spatial.ts", source: "// forbidden\nconst value = 1;", passed: false },
  { file: "web/src/audio-spatial.ts", source: "gain.connect(context.destination);", passed: false },
  { file: "web/src/audio-spatial.ts", source: "gain.connect(context['destination']);", passed: false },
  { file: "web/src/audio-spatial.ts", source: "const output = audioContext.destination; gain.connect(output);", passed: false },
  { file: "web/src/audio-spatial.ts", source: "class SpatialAudio { render() { this.gain.connect(this.context.destination); } }", passed: false },
  { file: "web/src/audio-spatial.ts", source: "class SpatialAudio { render() { this.gain.connect(this.destination); } }", passed: true },
  { file: "web/src/audio-spatial.ts", source: "const source = new AudioContext(); gain.connect(source.destination);", passed: false },
  { file: "web/src/audio-spatial.ts", source: 'const output = ".local/cs16/verification/audio";', passed: true },
  { file: "web/src/audio-spatial.ts", source: 'const output = ".local/other-game/verification/audio";', passed: false },
  { file: "web/src/audio-spatial.ts", source: 'const output = "/tmp/spatial.wav";', passed: false },
  { file: "web/tests/audio-spatial.test.mjs", source: "class FakeContext { constructor() { this.destination = {}; } }", passed: true },
  { file: "web/tests/audio-spatial.test.mjs", source: "assert(connections.every(connection => connection.target !== ctx.destination));", passed: true },
  { file: "web/tests/audio-spatial-browser.test.mjs", source: "const responseTimes = [];", passed: true },
  { file: "web/tests/audio-spatial-browser.test.mjs", source: 'const output = ".local/other-game/audio";', passed: false },
  { file: "web/tests/audio-spatial-browser.test.mjs", source: 'const output = "/tmp/audio";', passed: false },
  { file: "web/tests/audio-spatial-browser.test.mjs", source: "import { tmpdir } from 'node:os'; const output = tmpdir();", passed: false },
  { file: "web/tests/audio-spatial-browser.test.mjs", source: "// forbidden\nconst responseTimes = [];", passed: false },
  { file: "web/src/audio-other.ts", source: "const value = 1;", passed: false },
  { file: "web/src/audio.ts", source: "// existing\nconst newValue = 1;", changedLines: [2], passed: true },
  { file: "web/src/audio.ts", source: "const old_name = 1;\nconst newValue = 1;", changedLines: [2], passed: true },
  { file: "web/src/audio.ts", source: "const oldValue = 1;\nconst new_name = 1;", changedLines: [2], passed: false },
  { file: "web/src/audio.ts", source: "const oldValue = 1;\n// new forbidden", changedLines: [2], passed: false },
  { file: "web/src/audio.ts", source: "const oldValue = 1;\ngain.connect(context.destination);", changedLines: [2], passed: false },
  { file: "web/src/game-api/module.ts", source: "// existing\nexport type SpatialPoint = { x: number };", changedLines: [2], passed: true },
  { file: "web/src/game-api/default-players.ts", source: "export const defaultPlayers = 10;", passed: true },
  { file: "web/src/game-api/default-players.ts", source: "export const default_players = 10;", passed: false },
  { file: "web/src/game-api/default-players.ts", source: "const media = new Audio();", passed: false },
  { file: "web/src/game-api/default-players.ts", source: "gain.connect(context.destination);", passed: false },
  { file: "web/tests/game-default-players.test.mjs", source: "const playerCounts = [2, 4, 10];", passed: true },
  { file: "web/src/game-api/host.tsx", source: "// existing\nconst defaultPlayers = 10;", changedLines: [2], passed: true },
  { file: "web/src/game-api/action-scope.ts", source: "export class ActionScopes {}", passed: true },
  { file: "web/src/game-api/action-scope.ts", source: "// forbidden\nexport class ActionScopes {}", passed: false },
  { file: "web/tests/action-scope.test.mjs", source: "const source = {};", passed: true },
  { file: "web/src/main.tsx", source: "// existing\nconst actionScopes = {};", changedLines: [2], passed: true },
  { file: "scripts/test_cdn_manual_gzip.py", source: "def check_manifest():\n return True\n", passed: true },
  { file: "scripts/test_cdn_manual_gzip.py", source: "# forbidden\nvalue = 1\n", passed: false },
  { file: "scripts/cdn_delivery.py", source: "\"\"\"existing\"\"\"\ndef check_manifest():\n return True\n", changedLines: [2, 3], passed: true },
  { file: "scripts/cdn_delivery.py", source: "value = 1\n# forbidden\n", changedLines: [2], passed: false },
];

test("shared spatial audio is fully scoped while existing platform files check only new lines", () => {
  const inputs = cases.map(row => ({ ...row, language: row.file.endsWith(".mjs") ? "javascript" : row.file.endsWith(".py") ? "python" : "typescript" }));
  const run = spawnSync(process.execPath, ["games/cs16/build/lint.mjs", "--batch"], { cwd: root, input: JSON.stringify(inputs), encoding: "utf8" });
  assert.equal(run.status, 0, run.stdout + run.stderr);
  const rows = JSON.parse(run.stdout);
  const report = { passed: rows.every((row, index) => row.passed === cases[index].passed), cases: rows.map((row, index) => ({ ...cases[index], actual: row.passed, issues: row.issues })) };
  writeFileSync(path.join(output, "spatial-scope.json"), JSON.stringify(report, null, 2) + "\n");
  for (const [index, row] of rows.entries()) assert.equal(row.passed, cases[index].passed, JSON.stringify(report.cases[index]));
});

test("policy explicitly registers shared source and patch coverage without scanning all platform files", () => {
  const policy = JSON.parse(readFileSync(path.join(root, "games/cs16/build/lint-rules.json"), "utf8"));
  assert.deepEqual(policy.sharedSources, ["web/src/audio-spatial.ts", "web/src/game-api/default-players.ts", "web/src/game-api/action-scope.ts"]);
  assert(policy.sharedTestPatterns.some(row => row.directory === "web/tests" && new RegExp(row.filename).test("audio-spatial.test.mjs")));
  for (const file of ["web/src/audio.ts", "web/src/game-api/module.ts", "web/src/game-api/host.tsx", "web/src/main.tsx"]) assert(policy.sharedPatchSources.includes(file));
  assert(policy.sharedPatchSources.every(file => file.startsWith("web/src/") && /\.tsx?$/.test(file)));
  assert(!policy.directories.includes("web/src"));
});
