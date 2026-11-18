import { spawnSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const cases = [
  ["typescript", "games/pal/web/clean.ts", 'const url = "https://example.test/#x";\n', true],
  ["typescript", "games/pal/web/clean.ts", "// rejected\nconst value = 1;\n", false],
  ["typescript", "games/pal/web/clean.ts", "const value = 1; /* rejected */", false],
  ["typescript", "games/pal/web/clean.ts", 'const value = ;', false],
  ["typescript", "games/pal/web/Bad_Name.ts", "const value = 1;", false],
  ["typescript", "games/pal/tests/story-shake-probe.ts", "const value = 1;", true],
  ["typescript", "games/pal/tests/other-probe.ts", "const value = 1;", false],
  ["javascript", "games/pal/tests/clean.mjs", 'const pattern = /https?:\\/\\//; const text = `/* literal */`;', true],
  ["javascript", "games/pal/tests/clean.mjs", "const value = 1; // rejected", false],
  ["javascript", "games/pal/tests/clean.js", 'const url = "https://example.test";', true],
  ["javascript", "games/pal/tests/clean.js", "const value = 1; /* rejected */", false],
  ["javascript", "games/pal/tests/Bad_Name.js", "const value = 1;", false],
  ["javascript", "games/other/clean.js", "const value = 1;", false],
  ["python", "games/pal/tests/clean.py", 'value = "# literal"\n', true],
  ["python", "games/pal/tests/clean.py", "value = 1 # rejected\n", false],
  ["python", "games/pal/tests/clean.py", '"""rejected"""\nvalue = 1\n', false],
  ["python", "games/pal/tests/clean.py", "def broken(:\n", false],
  ["go", "games/pal/tests/clean.go", 'package clean\nvar url = "https://example.test"\n', true],
  ["go", "games/pal/tests/clean.go", "package clean\n/* rejected */\n", false],
  ["go", "games/pal/tests/clean.go", "package clean\nvar value = \n", false],
  ["css", "games/pal/web/clean.css", '.pal-board { background: url("https://example.test/a#b"); }', true],
  ["css", "games/pal/web/clean.css", ".pal-board { /* rejected */ color: white; }", false],
  ["css", "games/pal/tests/clean.css", ".pal-board { color: white; }", false],
  ["typescript", "../outside.ts", "const value = 1;", false],
  ["unknown", "games/pal/web/clean.ts", "", false],
  ["c", "games/pal/native/clean.c", 'const char *url = "https://example.test"; int clean(void){return 1;}\n', true],
  ["c", "games/pal/native/clean.c", "/* rejected */ int clean(void){return 1;}\n", false],
  ["c", "games/pal/native/clean.c", "int clean(void){return 1;} // rejected\n", false],
  ["c", "games/pal/native/clean.c", "int clean( {\n", false],
  ["c", "games/pal/native/Bad_Name.c", "int clean(void){return 1;}", false],
  ["c", "games/pal/tests/clean.c", "int clean(void){return 1;}", false],
];
const result = [];
for (const [language, path, source, expected] of cases) {
  const run = spawnSync(process.execPath, ["games/pal/build/lint.mjs", "--stdin", language, path], { cwd: root, input: source, encoding: "utf8" });
  const passed = run.status === 0;
  result.push({ language, path, source, expected, passed, exitCode: run.status, output: run.stdout + run.stderr });
}
const out = new URL("../../../.local/pal/verification/lint-e2e.json", import.meta.url);
mkdirSync(new URL("./", out), { recursive: true });
const passed = result.every(row => row.expected === row.passed);
writeFileSync(out, JSON.stringify({ passed, cases: result }, null, 2) + "\n");
process.stdout.write(JSON.stringify({ passed, cases: result.length }) + "\n");
process.exitCode = passed ? 0 : 1;
