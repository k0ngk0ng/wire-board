import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/cs16/verification/lint/numeric-comparisons");
mkdirSync(output, { recursive: true });

const goCases = [
  ["pure declaration", "package server\nvar display float64 = 1.25", true],
  ["pure function", "package server\nfunc mean(a, b float64) float64 { return (a + b) / 2 }", true],
  ["pure math", "package server\nimport \"math\"\nvar display = math.Sqrt(2)", true],
  ["pure complex", "package server\nvar display = 1 + 2i", true],
  ["complex equality", "package server\nvar value = 1 + 2i; var valid = value == 0", false],
  ["direct comparison", "package server\nvar valid = 1.25 < 2", false],
  ["variable comparison", "package server\nvar value = 1.25\nvar valid = value < 2", false],
  ["alias comparison", "package server\nvar value = 1.25\nvar alias = value\nvar valid = alias == 1", false],
  ["boxed variable", "package server\nvar value any = 1.25\nvar valid = value == 1", false],
  ["boxed conversion", "package server\nvar valid = any(1.25) == 1", false],
  ["boxed reassignment", "package server\nfunc valid() bool { var value any = 1; value = 1.25; return value == 1 }", false],
  ["boxed return", "package server\nfunc value() any { return 1.25 }; var valid = value() == 1", false],
  ["boxed identity return", "package server\nfunc value(input any) any { return input }; var valid = value(1.25) == 1", false],
  ["parameter comparison", "package server\nfunc valid(value float64) bool { return value < 2 }", false],
  ["return comparison", "package server\nfunc value() float64 { return 1.25 }\nvar valid = value() > 1", false],
  ["field comparison", "package server\ntype State struct { Value float64 }; var state State; var valid = state.Value < 2", false],
  ["array comparison", "package server\nvar values = []float64{1.25, 2}; var valid = values[0] < 2", false],
  ["tolerance", "package server\nimport \"math\"\nfunc same(a, b float64) bool { return math.Abs(a-b) < 0.001 }", false],
  ["min", "package server\nvar display = min(1.25, 2.5)", false],
  ["math max", "package server\nimport \"math\"\nvar display = math.Max(1.25, 2)", false],
  ["math alias", "package server\nimport \"math\"\nvar choose = math.Max; var display = choose(1.25, 2)", false],
  ["float sorting", "package server\nimport \"sort\"\nfunc order(values []float64) { sort.Float64s(values) }", false],
  ["generic sorting", "package server\nimport \"slices\"\nfunc order(values []float64) { slices.Sort(values) }", false],
  ["sorting callback", "package server\nimport \"sort\"\nfunc order(values []float64) { sort.Slice(values, func(i,j int) bool { return values[i] < values[j] }) }", false],
  ["cmp comparison", "package server\nimport \"cmp\"\nvar valid = cmp.Compare(1.25, 2.5)", false],
  ["integer comparison", "package server\nvar value int64 = 1250; var valid = value < 2000", true],
  ["integer sorting", "package server\nimport \"slices\"\nfunc order(values []int) { slices.Sort(values) }", true],
  ["quantized comparison", "package server\nimport \"math\"\nfunc valid(value float64) bool { units := int64(math.Round(value*1000)); return units < 2000 }", true],
  ["round still float", "package server\nimport \"math\"\nfunc valid(value float64) bool { return math.Round(value*1000) < 2000 }", false],
  ["old float new comparison", "package server\nvar value = 1.25\nvar valid = value < 2", false, [3]],
  ["old comparison new output", "package server\nvar value = 1.25; var valid = value < 2\nvar display = value * 2", true, [3]],
];

const strictCases = [
  ["go", "games/cs16/engine/motion.go", "package engine\nvar display = 1.25", false],
  ["typescript", "games/cs16/web/prediction.ts", "const display = 1.25;", false],
  ["typescript", "games/cs16/web/input.ts", "const display = Math.random();", false],
  ["typescript", "web/src/game-api/default-players.ts", "const display = 1.25 * 2;", true],
  ["typescript", "web/src/game-api/default-players.ts", "const display = 1.25; const valid = display < 2;", false],
  ["typescript", "web/src/realtime-stream.ts", "const generation = 1.25;\nconst valid = generation === 1;", false, [2]],
];

test("shared Go permits pure floating calculation while deterministic game scopes remain strict", () => {
  const items = [...goCases.map(([, source, , changedLines]) => ({ file: "internal/server/session_cache.go", language: "go", source, ...(changedLines ? { changedLines } : {}) })), ...strictCases.map(([language, file, source, , changedLines]) => ({ file, language, source, ...(changedLines ? { changedLines } : {}) }))];
  const run = spawnSync(process.execPath, ["games/cs16/build/lint.mjs", "--batch"], { cwd: root, input: JSON.stringify(items), encoding: "utf8" });
  assert.equal(run.status, 0, run.stdout + run.stderr);
  const rows = JSON.parse(run.stdout);
  const expected = [...goCases.map(row => row[2]), ...strictCases.map(row => row[3])];
  const cases = rows.map((row, index) => ({ name: index < goCases.length ? goCases[index][0] : strictCases[index - goCases.length][1], expected: expected[index], actual: row.passed, issues: row.issues }));
  writeFileSync(path.join(output, "go-and-scopes.json"), JSON.stringify({ passed: cases.every(row => row.actual === row.expected), cases }, null, 2) + "\n");
  for (const row of cases) assert.equal(row.actual, row.expected, JSON.stringify(row));
});
