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
  { file: "internal/gameapi/stream_unchanged_test.go", source: "package gameapi\nfunc TestUnchanged() {}", passed: true },
  { file: "internal/gameapi/stream_unchanged_test.go", source: "package gameapi_test\nfunc TestUnchanged() {}", passed: true },
  { file: "internal/gameapi/stream_unchanged_test.go", source: "package server\nfunc TestUnchanged() {}", passed: false },
  { file: "internal/server/realtime_unchanged_e2e_test.go", source: "package server\nfunc TestUnchanged() {}", passed: true },
  { file: "internal/server/realtime_unchanged_e2e_test.go", source: "package server\n// forbidden\nfunc TestUnchanged() {}", passed: false },
  { file: "internal/server/realtime_unchanged_e2e_test.go", source: "package server\nfunc test_unchanged() {}", passed: false },
  { file: "internal/server/realtime_unchanged_e2e_test.go", source: "package server\nvar frameSpeed = 0.5", passed: false },
  { file: "internal/server/realtime_unchanged_e2e_test.go", source: "package server\nimport \"os\"\nvar output = os.TempDir()", passed: false },
  { file: "internal/server/realtime_unchanged_e2e_test.go", source: "package server\nimport \"github.com/k0ngk0ng/wire-board/games/cs16/engine\"", passed: false },
  { file: "internal/gameapi/module.go", source: "package gameapi\n// existing\ntype Result struct { Unchanged bool }", changedLines: [3], passed: true },
  { file: "internal/gameapi/module.go", source: "package gameapi\nvar previous float64\nvar nextValue = 1", changedLines: [3], passed: true },
  { file: "internal/gameapi/module.go", source: "package gameapi\nvar previous = 1\nvar nextValue float64", changedLines: [3], passed: false },
  { file: "internal/gameapi/module.go", source: "package gameapi\nvar previous = 1\nvar next_value = 1", changedLines: [3], passed: false },
  { file: "internal/gameapi/module.go", source: "package gameapi\nvar previous = 1\n// forbidden", changedLines: [3], passed: false },
  { file: "internal/server/realtime_stream.go", source: "package server\n// existing\nfunc streamUnchanged() {}", changedLines: [3], passed: true },
  { file: "internal/server/realtime_stream.go", source: "package server\nvar old_value float64\nfunc streamUnchanged() {}", changedLines: [3], passed: true },
  { file: "internal/server/realtime_stream.go", source: "package server\nvar previous = 1\nvar nextValue = 0.5", changedLines: [3], passed: false },
  { file: "internal/server/other_test.go", source: "package server\nfunc TestOther() {}", passed: false },
  { file: "games/cs16/engine/motion.go", source: "package engine\nvar previous float64", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\nfunc newSessionConnector() {}", passed: true },
  { file: "internal/server/session_cache.go", source: "package gameapi\nfunc newSessionConnector() {}", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\n// forbidden\nfunc newSessionConnector() {}", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\nvar session_count = 1", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\nimport \"unsafe\"", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\nimport sq \"modernc.org/sqlite\"\nfunc register() { sq.RegisterConnectionHook(nil) }", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\nimport sq \"modernc.org/sqlite\"\nvar register = sq.RegisterConnectionHook", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\nimport sq \"database/sql\"\nfunc register() { sq.Register(\"sqlite\", nil) }", passed: false },
  { file: "internal/server/session_cache.go", source: "package server\nfunc register(connection HookRegisterer) { connection.RegisterPreUpdateHook(nil) }", passed: true },
  { file: "internal/server/session_cache_e2e_test.go", source: "package server\nfunc TestSessionCache() {}", passed: true },
  { file: "internal/server/server.go", source: "package server\n// existing\nfunc user() {}", changedLines: [3], passed: true },
  { file: "internal/server/server.go", source: "package server\nvar existing float64\nvar sessions = 1", changedLines: [3], passed: true },
  { file: "internal/server/server.go", source: "package server\nvar existing = 1\n// forbidden", changedLines: [3], passed: false },
  { file: "internal/gameapi/deferred_input_test.go", source: "package gameapi\nfunc TestDeferredInput() {}", passed: true },
  { file: "internal/gameapi/deferred_input_test.go", source: "package server\nfunc TestDeferredInput() {}", passed: false },
  { file: "internal/gameapi/deferred_input_test.go", source: "package gameapi\n// forbidden\nfunc TestDeferredInput() {}", passed: false },
  { file: "internal/server/realtime_deferred_input_e2e_test.go", source: "package server\nfunc TestDeferredInput() {}", passed: true },
  { file: "internal/server/realtime_deferred_input_e2e_test.go", source: "package server\nvar input_rate = 1000", passed: false },
  { file: "internal/server/realtime_batch.go", source: "package server\n// existing\nfunc deferredRealtimeInput() {}", changedLines: [3], passed: true },
  { file: "internal/server/realtime_batch.go", source: "package server\nvar existing float64\nvar nextInput = 1", changedLines: [3], passed: true },
  { file: "internal/server/realtime_batch.go", source: "package server\nvar existing = 1\nvar nextInput = 0.5", changedLines: [3], passed: false },
];

test("shared Go files preserve full and changed-line integer and naming rules", () => {
  const run = spawnSync(process.execPath, ["games/cs16/build/lint.mjs", "--batch"], { cwd: root, input: JSON.stringify(cases.map(row => ({ ...row, language: "go" }))), encoding: "utf8" });
  assert.equal(run.status, 0, run.stdout + run.stderr);
  const rows = JSON.parse(run.stdout);
  const report = { passed: rows.every((row, index) => row.passed === cases[index].passed), cases: rows.map((row, index) => ({ ...cases[index], actual: row.passed, issues: row.issues })) };
  writeFileSync(path.join(output, "shared-go.json"), JSON.stringify(report, null, 2) + "\n");
  for (const [index, row] of rows.entries()) assert.equal(row.passed, cases[index].passed, JSON.stringify(report.cases[index]));
});

test("shared Go coverage is explicit and does not select all internal sources", () => {
  const policy = JSON.parse(readFileSync(path.join(root, "games/cs16/build/lint-rules.json"), "utf8"));
  for (const file of ["internal/gameapi/stream_unchanged_test.go", "internal/server/realtime_unchanged_e2e_test.go", "internal/server/session_cache.go", "internal/server/session_cache_e2e_test.go", "internal/gameapi/deferred_input_test.go", "internal/server/realtime_deferred_input_e2e_test.go"]) assert(policy.sharedGoSources.includes(file));
  for (const file of ["internal/gameapi/module.go", "internal/server/realtime_stream.go", "internal/server/server.go", "internal/server/realtime_batch.go"]) assert(policy.sharedGoPatchSources.includes(file));
  assert(policy.sharedGoSources.every(file => /^internal\/(?:server|gameapi)\/[a-z][a-z0-9_]*\.go$/.test(file)));
  assert(!policy.directories.includes("internal"));
});
