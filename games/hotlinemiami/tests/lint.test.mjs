import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/hotlinemiami/verification/lint");
mkdirSync(output, { recursive: true });

const cases = [
  ["typescript", "games/hotlinemiami/web/input.ts", "const moveSpeed = 1000;", true],
  ["typescript", "games/hotlinemiami/web/input.ts", "const move_speed = 1000;", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "const MAX_SPEED = 1000;", true],
  ["typescript", "games/hotlinemiami/web/input.ts", "// forbidden\nconst moveSpeed = 1000;", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "const moveSpeed = 1000; /* forbidden */", false],
  ["typescript", "games/hotlinemiami/web/input.ts", 'const sourceURL = "https://example.test/#literal";', true],
  ["typescript", "games/hotlinemiami/web/input.ts", "const value: any = 1;", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "eval('1');", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "(0, eval)('1');", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "const execute = globalThis.eval; execute('1');", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "Function('return 1')();", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "new Function('return 1');", false],
  ["typescript", "games/hotlinemiami/web/input.ts", "if (moveSpeed > 0.5) moveSpeed = 1;", false],
  ["typescript", "games/hotlinemiami/web/prediction.ts", "const epsilon = Number.EPSILON;", false],
  ["typescript", "games/hotlinemiami/web/prediction.ts", "const value = Math.random();", false],
  ["typescript", "games/hotlinemiami/web/attack.ts", "const cooldownFrames = 12;", true],
  ["typescript", "games/hotlinemiami/web/attack.ts", "const cooldownFrames = 0.5;", false],
  ["typescript", "games/hotlinemiami/web/feedback.ts", "const pendingFrames = 9;", true],
  ["typescript", "games/hotlinemiami/web/feedback.ts", "const pendingFrames = 0.5;", false],
  ["typescript", "games/hotlinemiami/web/renderer.ts", "const scale = 0.5;", true],
  ["typescript", "games/hotlinemiami/web/input.ts", "const value = ;", false],
  ["typescript", "games/hotlinemiami/web/Bad_Name.ts", "const value = 1;", false],
  ["typescript", "games/hotlinemiami/web/Board.tsx", "export function Board() { return <div />; }", true],
  ["javascript", "games/hotlinemiami/build/check.mjs", 'const pattern = /https?:\\/\\//; const text = `/* literal */`;', true],
  ["javascript", "games/hotlinemiami/build/check.mjs", "const value = 1; // forbidden", false],
  ["javascript", "games/hotlinemiami/build/check.js", "const value = 1;", true],
  ["javascript", "games/hotlinemiami/build/check.js", "const value = 1; // forbidden", false],
  ["typescript", "games/hotlinemiami/loose.ts", "const value = 1;", false],
  ["javascript", "games/other/build/check.mjs", "const value = 1;", false],
  ["javascript", "../outside.mjs", "const value = 1;", false],
  ["javascript", "/tmp/outside.mjs", "const value = 1;", false],
  ["javascript", "games/hotlinemiami/build/check.mjs", 'const output = ".local/another-game/results";', false],
  ["javascript", "games/hotlinemiami/build/check.mjs", 'const output = ".local/hotlinemiami/results";', true],
  ["javascript", "games/hotlinemiami/web/audio.mjs", "new AudioContext();", false],
  ["javascript", "games/hotlinemiami/web/audio.mjs", "new Audio('shot.wav');", false],
  ["javascript", "games/hotlinemiami/web/audio.mjs", "new window.AudioContext();", false],
  ["javascript", "games/hotlinemiami/web/audio.mjs", 'import { Board } from "../../racing/web/Board";', false],
  ["javascript", "games/hotlinemiami/web/audio.mjs", 'const module = import("../../racing/web/Board");', false],
  ["typescript", "games/hotlinemiami/web/input.ts", "class Controls { wrong_name() {} }", false],
  ["go", "games/hotlinemiami/engine/motion.go", "package engine\nfunc movePlayer() {}", true],
  ["go", "games/hotlinemiami/engine/motion.go", "package wrong\nfunc movePlayer() {}", false],
  ["go", "games/hotlinemiami/server/module.go", "package hotlinemiami\nfunc Module() {}", true],
  ["go", "games/hotlinemiami/cmd/wasm/main_js.go", "package main\nfunc main() {}", true],
  ["go", "games/hotlinemiami/engine/motion_test.go", "package engine_test\nfunc TestMotion() {}", true],
  ["go", "games/hotlinemiami/engine/motion.go", "package engine\n// forbidden\nfunc movePlayer() {}", false],
  ["go", "games/hotlinemiami/engine/motion.go", "package engine\nvar move_speed = 1", false],
  ["go", "games/hotlinemiami/engine/motion.go", "package engine\nvar moveSpeed float64", false],
  ["go", "games/hotlinemiami/engine/motion.go", "package engine\nvar moveSpeed = 0.5", false],
  ["go", "games/hotlinemiami/engine/motion.go", "package engine\ntype Player struct { move_speed int }", false],
  ["go", "games/hotlinemiami/engine/motion.go", 'package engine\nimport "github.com/k0ngk0ng/wire-board/games/racing/server"', false],
  ["python", "scripts/hotlinemiami/extract.py", "def extract_assets(frame_count):\n return frame_count\n", true],
  ["python", "scripts/hotlinemiami/extract.py", "def extractAssets():\n pass\n", false],
  ["python", "scripts/hotlinemiami/extract.py", "# forbidden\nvalue = 1\n", false],
  ["python", "scripts/hotlinemiami/extract.py", 'value = "# literal"\n', true],
  ["python", "scripts/hotlinemiami/extract.py", '"""forbidden"""\nvalue = 1\n', false],
  ["python", "scripts/hotlinemiami/extract.py", "eval('1')\n", false],
  ["python", "scripts/hotlinemiami/extract.py", "def broken(:\n", false],
  ["python", "scripts/hotlinemiami/extract.py", "import tempfile\nvalue = tempfile.TemporaryDirectory()\n", false],
  ["python", "scripts/hotlinemiami/extract.py", "import tempfile\nvalue = tempfile.TemporaryDirectory(dir=output)\n", true],
  ["json", "games/hotlinemiami/assets/manifest.json", '{"schema":1,"files":[]}', true],
  ["json", "games/hotlinemiami/assets/manifest.json", '{"schema":}', false],
  ["json", "games/hotlinemiami/assets/manifest.json", '{"__proto__":{}}', false],
  ["css", "games/hotlinemiami/web/style.css", '.hotline-board {color:#fff;background:url("https://example.test/a#b")}', true],
  ["css", "games/hotlinemiami/web/style.css", ".bad_name {color:#fff}", false],
  ["css", "games/hotlinemiami/web/style.css", ".hotline-board {/* forbidden */color:#fff}", false],
];

test("lint enforces authored-language rules with executable positive and negative examples", () => {
  const run = spawnSync(process.execPath, ["games/hotlinemiami/build/lint.mjs", "--batch"], {
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

test("changed-game CI discovers this game without expanding unrelated game suites", () => {
  const program = "import importlib.util,json,pathlib\np=pathlib.Path('scripts/ci/scope.py')\ns=importlib.util.spec_from_file_location('scope',p)\nm=importlib.util.module_from_spec(s)\ns.loader.exec_module(m)\nroot=pathlib.Path.cwd()\ngames=m.game_names(root)\nprint(json.dumps({'plan':m.plan(['games/hotlinemiami/web/input.ts'],games),'lintPlan':m.plan(['games/hotlinemiami/build/lint_go.go'],games),'build':m.build_commands(root,'hotlinemiami')}))\n";
  const run = spawnSync("python3", ["-B", "-c", program], { cwd: root, encoding: "utf8" });
  assert.equal(run.status, 0, run.stdout + run.stderr);
  const actual = JSON.parse(run.stdout);
  assert.deepEqual(actual.plan.games, [{ game: "hotlinemiami", backend: false }]);
  assert.deepEqual(actual.lintPlan.games, [{ game: "hotlinemiami", backend: false }]);
  assert.equal(actual.build.length, 1);
  assert(actual.build[0].join(" ").includes("games/hotlinemiami/build/lint.mjs"));
  const workflow = readFileSync(path.join(root, ".github/workflows/build.yml"), "utf8");
  const gameJob = workflow.split(/^  games:\s*$/m)[1]?.split(/^  verify:\s*$/m)[0] ?? "";
  const cachedRoute = /uses:\s*\.\/\.github\/actions\/restore-resources/.test(gameJob);
  const directRoute = /resource_store\.py\s+hydrate\b/.test(gameJob) && /--oss\b/.test(gameJob) && /--game\b/.test(gameJob);
  assert(cachedRoute || directRoute, "Game CI must restore its declared resource scope");
  if (cachedRoute) {
    assert.match(gameJob, /scope:\s*game/);
    assert.match(gameJob, /game:\s*\$\{\{\s*matrix\.game\s*\}\}/);
    const action = readFileSync(path.join(root, ".github/actions/restore-resources/action.yml"), "utf8");
    assert.match(action, /scripts\/ci\/resources\.py/);
  }
  const restoreProgram = [
    "import hashlib,importlib.util,json,pathlib,sys,tempfile",
    "sys.path.insert(0,str(pathlib.Path('scripts').resolve()))",
    "import resource_store as store",
    "class FixtureSource:",
    " def __init__(self,body): self.body,self.calls=body,[]",
    " def get(self,key): self.calls.append(key); return self.body,'image/png'",
    "with tempfile.TemporaryDirectory(dir='.local/hotlinemiami/verification/lint') as folder:",
    " root=pathlib.Path(folder).resolve()",
    " (root/'assets').mkdir(parents=True)",
    " (root/'games/hotlinemiami').mkdir(parents=True)",
    " body=b'bounded restoration fixture'",
    " digest=hashlib.sha256(body).hexdigest()",
    " own='games/hotlinemiami/assets/public/hotlinemiami/image.png'",
    " other='games/racing/assets/public/racing/image.png'",
    " files={name:{'bytes':len(body),'sha256':digest,'mime':'image/png','key':'bga/'+game+'/'+digest+'.png'} for name,game in [(own,'hotlinemiami'),(other,'racing')]}",
    " registry={'version':1,'files':files}",
    " (root/'assets/resources.json').write_text(json.dumps(registry))",
    " paths=store.hydration_paths(registry,root,'hotlinemiami')",
    " assert paths=={own}",
    " source=FixtureSource(body)",
    " restored=store.hydrate(registry,root,'https://fixture.invalid',paths=paths,client=source)",
    " assert (root/own).read_bytes()==body",
    " assert not (root/other).exists()",
    " assert restored==[own] and source.calls==[files[own]['key']]",
    " result={'direct':{'passed':True,'files':restored,'sourceReads':len(source.calls)}}",
    " p=pathlib.Path('scripts/ci/resources.py')",
    " if p.is_file():",
    "  s=importlib.util.spec_from_file_location('resources',p)",
    "  m=importlib.util.module_from_spec(s)",
    "  s.loader.exec_module(m)",
    "  (root/own).unlink()",
    "  plan=m.resource_plan(root,'game','hotlinemiami')",
    "  assert set(plan['files'])=={own}",
    "  cache=root/'.local/hotlinemiami/cache/ci-resources'",
    "  cached=cache/'objects'/files[own]['key']",
    "  cached.parent.mkdir(parents=True)",
    "  cached.write_bytes(body)",
    "  report={'downloadedBytes':0,'getRequests':0,'budgetBytes':0}",
    "  m.restore(plan,root,cache,'restore',0,report)",
    "  assert (root/own).read_bytes()==body and not (root/other).exists()",
    "  result['cache']={'available':True,'plan':{'scope':plan['scope'],'game':plan['game'],'files':list(plan['files'])},'restore':report}",
    " else: result['cache']={'available':False}",
    " print(json.dumps(result))",
  ].join("\n") + "\n";
  const restored = spawnSync("python3", ["-B", "-c", restoreProgram], { cwd: root, encoding: "utf8" });
  assert.equal(restored.status, 0, restored.stdout + restored.stderr);
  const resources = JSON.parse(restored.stdout);
  assert.equal(resources.direct.passed, true);
  assert.equal(resources.direct.sourceReads, 1);
  if (resources.cache.available) {
    assert.equal(resources.cache.restore.passed, true);
    assert.equal(resources.cache.restore.restoredFiles, 1);
    assert.equal(resources.cache.restore.downloadedBytes, 0);
    assert.equal(resources.cache.restore.getRequests, 0);
  } else assert(directRoute);
  writeFileSync(path.join(output, "ci-scope.json"), JSON.stringify({ passed: true, ...actual, route: cachedRoute ? "cache" : "direct", resources }, null, 2) + "\n");
});
