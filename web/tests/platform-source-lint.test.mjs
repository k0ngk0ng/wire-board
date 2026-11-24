import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { discoverPlatformSources, inspectSources } from "../scripts/platform-source-lint.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const output = path.join(root, ".local/cs16/verification/lint");
mkdirSync(output, { recursive: true });

const cases = [
  { file: "web/src/audio-spatial.ts", source: "const context = new AudioContext(); const output = context.createMediaStreamDestination(); gain.connect(output);", passed: true },
  { file: "web/src/audio-spatial.ts", source: "const audio_context = new AudioContext();", passed: false },
  { file: "web/src/audio-spatial.ts", source: "// forbidden\nconst context = new AudioContext();", passed: false },
  { file: "web/src/audio-spatial.ts", source: "const value: any = 1;", passed: false },
  { file: "web/src/audio-spatial.ts", source: "gain.connect(context.destination);", passed: false },
  { file: "web/src/audio-spatial.ts", source: "gain.connect(context['destination']);", passed: false },
  { file: "web/src/audio-spatial.ts", source: "const target = context.destination; gain.connect(target);", passed: false },
  { file: "web/src/audio-spatial.ts", source: "const source = new AudioContext(); gain.connect(source.destination);", passed: false },
  { file: "web/src/audio-spatial.ts", source: "class NativeAudio { draw() { this.gain.connect(this.destination); } }", passed: true },
  { file: "web/src/audio-spatial.ts", source: 'import { Module } from "../../games/cs16/server/module";', passed: false },
  { file: "web/src/audio-spatial.ts", source: 'const module = import("../../games/cs16/web/module");', passed: false },
  { file: "web/src/audio-spatial.ts", source: "eval('1');", passed: false },
  { file: "web/src/audio-spatial.ts", source: "new Function('return 1');", passed: false },
  { file: "web/src/audio-spatial.ts", source: 'const label = "https://example.invalid/#literal";', passed: true },
  { file: "web/src/game-api/default-players.ts", source: "const defaultPlayers = 10;", passed: true },
  { file: "web/src/game-api/default-players.ts", source: "const defaultPlayers = 4.5;", passed: true },
  { file: "web/src/game-api/default-players.ts", source: "const defaultPlayers = Math.random();", passed: true },
  { file: "web/src/game-api/default-players.ts", source: "const media = new Audio();", passed: false },
  { file: "web/src/game-api/default-players.ts", source: "gain.connect(context.destination);", passed: false },
  { file: "web/src/game-api/module.ts", source: "// old comment\nexport type PlayerCount = number;", changedLines: [2], passed: true },
  { file: "web/src/game-api/host.tsx", source: "const old_name = 1;\nconst defaultPlayers = 10;", changedLines: [2], passed: true },
  { file: "web/src/game-api/host.tsx", source: "const oldName = 1;\n// new comment", changedLines: [2], passed: false },
  { file: "web/src/audio.ts", source: "// old comment\ngain.connect(context.destination);", changedLines: [2], passed: false },
  { file: "web/tests/audio-spatial.test.mjs", source: "assert(connections.every(connection => connection.target !== ctx.destination));", passed: true },
  { file: "web/tests/game-default-players.test.mjs", source: "const defaultPlayers = 4.5;", passed: true },
  { file: "web/scripts/platform-source-lint.mjs", source: "import { tmpdir } from 'node:os'; const output = tmpdir();", passed: false },
  { file: "web/src/unregistered.ts", source: "const value = 1;", passed: false },
  { file: "games/retired_registry_test.go", source: "package games\nvar coverValid = true\n", language: "go", passed: true },
  { file: "games/retired_registry_test.go", source: "package wrong\nvar coverValid = true\n", language: "go", passed: false },
  { file: "games/retired_registry_test.go", source: "package games\nvar cover_valid = true\n", language: "go", passed: false },
  { file: "games/retired_registry_test.go", source: "package games\n// forbidden\nvar coverValid = true\n", language: "go", passed: false },
  { file: "games/RetiredRegistryTest.go", source: "package games\n", language: "go", passed: false },
];

test("public source policy rejects scoped failures with positive examples", () => {
  const rows = inspectSources(cases.map(row => ({ ...row, language: row.language ?? (row.file.endsWith(".mjs") ? "javascript" : "typescript") })));
  for (const [index, row] of rows.entries()) assert.equal(row.passed, cases[index].passed, JSON.stringify({ ...cases[index], actual: row }));
});

test("new platform sources pass without importing or executing a game check", () => {
  const sources = discoverPlatformSources(root);
  assert(sources.some(row => row.file === "web/src/audio-spatial.ts"));
  assert(sources.some(row => row.file === "web/src/game-api/default-players.ts"));
  assert(sources.every(row => ["web/", "internal/server/", "assets/", "games/"].some(prefix => row.file.startsWith(prefix))));
  const rows = inspectSources(sources);
  const report = { passed: rows.every(row => row.passed), files: rows, examples: cases.length };
  writeFileSync(path.join(output, "platform-source.json"), JSON.stringify(report, null, 2) + "\n");
  assert(report.passed, JSON.stringify(rows.filter(row => !row.passed)));
});

test("public lint tool changes select frontend checks without any game suite", () => {
  const program = [
    "import importlib.util,json,pathlib",
    "root=pathlib.Path.cwd()",
    "spec=importlib.util.spec_from_file_location('scope',root/'scripts/ci/scope.py')",
    "scope=importlib.util.module_from_spec(spec)",
    "spec.loader.exec_module(scope)",
    "print(json.dumps([scope.plan([file],scope.game_names(root)) for file in ['web/scripts/platform-source-lint.mjs','web/scripts/platform-source-lint.json','web/scripts/numeric-comparisons.mjs']]))",
  ].join("\n") + "\n";
  const run = spawnSync("python3", ["-B", "-c", program], { cwd: root, encoding: "utf8" });
  assert.equal(run.status, 0, run.stdout + run.stderr);
  for (const plan of JSON.parse(run.stdout)) {
    assert.deepEqual(plan.games, []);
    assert.deepEqual(plan.quick_components, ["frontend"]);
    assert.equal(plan.has_shared, false);
    assert.equal(plan.frontend_checks, true);
  }
});
