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
  ["typescript", "games/cs16/web/input.ts", "const moveSpeed = 1000;", true],
  ["typescript", "games/cs16/web/touch-input.ts", "const moveScale = 1000;", true],
  ["typescript", "games/cs16/web/touch-input.ts", "const moveScale = 0.5;", false],
  ["typescript", "games/cs16/web/input.ts", "const move_speed = 1000;", false],
  ["typescript", "games/cs16/web/input.ts", "const MAX_SPEED = 1000;", true],
  ["typescript", "games/cs16/web/input.ts", "// forbidden\nconst moveSpeed = 1000;", false],
  ["typescript", "games/cs16/web/input.ts", "const moveSpeed = 1000; /* forbidden */", false],
  ["typescript", "games/cs16/web/input.ts", 'const sourceURL = "https://example.test/#literal";', true],
  ["typescript", "games/cs16/web/input.ts", "const value: any = 1;", false],
  ["typescript", "games/cs16/web/input.ts", "eval('1');", false],
  ["typescript", "games/cs16/web/input.ts", "(0, eval)('1');", false],
  ["typescript", "games/cs16/web/input.ts", "const execute = globalThis.eval; execute('1');", false],
  ["typescript", "games/cs16/web/input.ts", "Function('return 1')();", false],
  ["typescript", "games/cs16/web/input.ts", "new Function('return 1');", false],
  ["typescript", "games/cs16/web/input.ts", "globalThis['eval']('1');", false],
  ["typescript", "games/cs16/web/input.ts", "if (moveSpeed > 0.5) moveSpeed = 1;", false],
  ["typescript", "games/cs16/web/prediction.ts", "const epsilon = Number.EPSILON;", false],
  ["typescript", "games/cs16/web/prediction.ts", "const value = Math.random();", false],
  ["typescript", "games/cs16/web/attack.ts", "const cooldownFrames = 12;", true],
  ["typescript", "games/cs16/web/attack.ts", "const cooldownFrames = 0.5;", false],
  ["typescript", "games/cs16/web/feedback.ts", "const pendingFrames = 9;", true],
  ["typescript", "games/cs16/web/feedback.ts", "const pendingFrames = 0.5;", false],
  ["typescript", "games/cs16/web/renderer.ts", "const scale = 0.5;", true],
  ["typescript", "games/cs16/web/input.ts", "const value = ;", false],
  ["typescript", "games/cs16/web/Bad_Name.ts", "const value = 1;", false],
  ["typescript", "games/cs16/web/Board.tsx", "export function Board() { return <div />; }", true],
  ["javascript", "games/cs16/build/check.mjs", 'const pattern = /https?:\\/\\//; const text = `/* literal */`;', true],
  ["javascript", "games/cs16/build/check.mjs", "const value = 1; // forbidden", false],
  ["javascript", "games/cs16/build/check.js", "const value = 1;", true],
  ["javascript", "games/cs16/build/check.js", "const value = 1; // forbidden", false],
  ["typescript", "games/cs16/loose.ts", "const value = 1;", false],
  ["javascript", "games/other/build/check.mjs", "const value = 1;", false],
  ["javascript", "../outside.mjs", "const value = 1;", false],
  ["javascript", "/tmp/outside.mjs", "const value = 1;", false],
  ["javascript", "games/cs16/build/check.mjs", 'const output = ".local/another-game/results";', false],
  ["javascript", "games/cs16/build/check.mjs", 'const output = ".local/cs16/results";', true],
  ["javascript", "games/cs16/web/audio.mjs", "new AudioContext();", false],
  ["javascript", "games/cs16/web/audio.mjs", "new Audio('shot.wav');", false],
  ["javascript", "games/cs16/web/audio.mjs", "new window.AudioContext();", false],
  ["javascript", "games/cs16/web/audio.mjs", "new window['AudioContext']();", false],
  ["javascript", "games/cs16/web/audio.mjs", "const player = document.createElement('audio');", false],
  ["javascript", "games/cs16/web/audio.mjs", 'import { Board } from "../../racing/web/Board";', false],
  ["javascript", "games/cs16/web/audio.mjs", 'const module = import("../../racing/web/Board");', false],
  ["typescript", "games/cs16/web/input.ts", "class Controls { wrong_name() {} }", false],
  ["go", "games/cs16/engine/motion.go", "package engine\nfunc movePlayer() {}", true],
  ["go", "games/cs16/engine/motion.go", "package wrong\nfunc movePlayer() {}", false],
  ["go", "games/cs16/server/module.go", "package cs16\nfunc Module() {}", true],
  ["go", "games/cs16/cmd/wasm/main_js.go", "package main\nfunc main() {}", true],
  ["go", "games/cs16/engine/motion_test.go", "package engine_test\nfunc TestMotion() {}", true],
  ["go", "games/cs16/engine/motion.go", "package engine\n// forbidden\nfunc movePlayer() {}", false],
  ["go", "games/cs16/engine/motion.go", "package engine\nvar move_speed = 1", false],
  ["go", "games/cs16/engine/motion.go", "package engine\nvar moveSpeed float64", false],
  ["go", "games/cs16/engine/motion.go", "package engine\nvar moveSpeed = 0.5", false],
  ["go", "games/cs16/engine/motion.go", "package engine\nimport \"math\"\nvar direction = math.Sin(1)", false],
  ["go", "games/cs16/engine/motion.go", "package engine\ntype Player struct { move_speed int }", false],
  ["go", "games/cs16/engine/motion.go", 'package engine\nimport "github.com/k0ngk0ng/wire-board/games/racing/server"', false],
  ["python", "scripts/cs16/extract.py", "def extract_assets(frame_count):\n return frame_count\n", true],
  ["python", "scripts/cs16/extract.py", "def extractAssets():\n pass\n", false],
  ["python", "scripts/cs16/extract.py", "# forbidden\nvalue = 1\n", false],
  ["python", "scripts/cs16/extract.py", 'value = "# literal"\n', true],
  ["python", "scripts/cs16/extract.py", '"""forbidden"""\nvalue = 1\n', false],
  ["python", "scripts/cs16/extract.py", "eval('1')\n", false],
  ["python", "scripts/cs16/extract.py", "def broken(:\n", false],
  ["python", "scripts/cs16/extract.py", "import tempfile\nvalue = tempfile.TemporaryDirectory()\n", false],
  ["python", "scripts/cs16/extract.py", "import tempfile\nvalue = tempfile.TemporaryDirectory(dir=output)\n", true],
  ["python", "scripts/cs16/extract.py", "from tempfile import TemporaryDirectory\nvalue = TemporaryDirectory()\n", false],
  ["python", "scripts/cs16/extract.py", "from tempfile import TemporaryDirectory\nvalue = TemporaryDirectory(dir=output)\n", true],
  ["json", "games/cs16/assets/manifest.json", '{"schema":1,"files":[]}', true],
  ["json", "games/cs16/assets/manifest.json", '{"schema":}', false],
  ["json", "games/cs16/assets/manifest.json", '{"__proto__":{}}', false],
  ["css", "games/cs16/web/style.css", '.cs16-board {color:#fff;background:url("https://example.test/a#b")}', true],
  ["css", "games/cs16/web/style.css", ".bad_name {color:#fff}", false],
  ["css", "games/cs16/web/style.css", ".cs16-board {/* forbidden */color:#fff}", false],
];

test("lint enforces authored-language rules with executable positive and negative examples", () => {
  const run = spawnSync(process.execPath, ["games/cs16/build/lint.mjs", "--batch"], {
    cwd: root,
    input: JSON.stringify(cases.map(([language, file, source]) => ({ language, file, source }))),
    encoding: "utf8",
  });
  assert.equal(run.status, 0, run.stdout + run.stderr);
  const rows = JSON.parse(run.stdout);
  assert.equal(rows.length, cases.length);
  const result = cases.map(([language, file, source, expected], index) => ({ language, file, source, expected, passed: rows[index].passed, issues: rows[index].issues }));
  writeFileSync(path.join(output, "examples.json"), JSON.stringify({ passed: result.every(row => row.expected === row.passed), cases: result }, null, 2) + "\n");
  for (const row of result) assert.equal(row.passed, row.expected, JSON.stringify(row));
});


test("changed-game CI selects CS16 frontend, backend and lint independently", () => {
  const program = [
    "import importlib.util,json,pathlib",
    "p=pathlib.Path('scripts/ci/scope.py')",
    "s=importlib.util.spec_from_file_location('scope',p)",
    "m=importlib.util.module_from_spec(s)",
    "s.loader.exec_module(m)",
    "root=pathlib.Path.cwd()",
    "games=m.game_names(root)",
    "print(json.dumps({'web':m.plan(['games/cs16/web/input.ts'],games),'engine':m.plan(['games/cs16/engine/combat.go'],games),'lint':m.plan(['games/cs16/build/lint_go.go'],games),'build':m.build_commands(root,'cs16'),'laterShard':m.build_commands(root,'cs16',1),'existingGame':m.build_commands(root,'hotlinemiami'),'frontend':[str(p.relative_to(root)) for p in m.frontend_tests(root,'cs16')]}))",
  ].join("\n") + "\n";
  const run = spawnSync("python3", ["-B", "-c", program], { cwd: root, encoding: "utf8" });
  assert.equal(run.status, 0, run.stdout + run.stderr);
  const actual = JSON.parse(run.stdout);
  assert.deepEqual(actual.web.games, [{ game: "cs16", backend: false }]);
  assert.deepEqual(actual.engine.games, [{ game: "cs16", backend: true }]);
  assert.deepEqual(actual.lint.games, [{ game: "cs16", backend: false }]);
  assert.deepEqual(actual.build, [["node", path.join(root, "games/cs16/build/lint.mjs")]]);
  assert.deepEqual(actual.laterShard, []);
  assert.deepEqual(actual.existingGame, [["node", path.join(root, "games/hotlinemiami/build/lint.mjs")]]);
  assert(actual.frontend.includes("games/cs16/tests/lint.test.mjs"));
  assert(actual.frontend.every(file => file.startsWith("games/cs16/")));
  const workflow = readFileSync(path.join(root, ".github/workflows/build.yml"), "utf8");
  const gameJob = workflow.split(/^  games:\s*$/m)[1]?.split(/^  verify:\s*$/m)[0] ?? "";
  assert.match(gameJob, /uses:\s*\.\/\.github\/actions\/restore-resources/);
  assert.match(gameJob, /scope:\s*game/);
  assert.match(gameJob, /game:\s*\$\{\{\s*matrix\.game\s*\}\}/);
  writeFileSync(path.join(output, "ci-scope.json"), JSON.stringify({ passed: true, ...actual }, null, 2) + "\n");
});
