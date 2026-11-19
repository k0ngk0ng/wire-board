import assert from "node:assert/strict";
import { readFileSync, mkdirSync, writeFileSync, renameSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const postcss = require("postcss");
const selectedConfig = process.argv.indexOf("--config");
const configPath = selectedConfig < 0 ? new URL("./lint-rules.json", import.meta.url) : path.join(root, process.argv[selectedConfig + 1]);
const config = JSON.parse(readFileSync(configPath, "utf8"));
const out = path.join(root, config.evidenceDirectory ?? ".local/racing/dynamics/evidence");
const temp = path.join(root, ".local/racing/dynamics/tmp");
mkdirSync(out, { recursive: true });
mkdirSync(temp, { recursive: true });

function inspectScript(source) {
  const errors = [];
  const file = ts.createSourceFile("source.ts", source, ts.ScriptTarget.Latest, true);
  function visit(node) {
    if (ts.getLeadingCommentRanges(source, node.pos)?.length || ts.getTrailingCommentRanges(source, node.end)?.length) errors.push("no-comments");
    if ((ts.isFunctionDeclaration(node) || ts.isVariableDeclaration(node) || ts.isTypeAliasDeclaration(node) || ts.isInterfaceDeclaration(node)) && node.name && ts.isIdentifier(node.name) && (node.name.text.includes("_") && !(config.allowUpperSnakeConstants && /^[A-Z][A-Z0-9_]*$/.test(node.name.text)))) errors.push("camel-case-binding");
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "eval") errors.push("no-eval");
    if (ts.isNewExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "Function") errors.push("no-function-constructor");
    ts.forEachChild(node, visit);
  }
  visit(file);
  return errors;
}

function inspectGo(inputs) {
  return JSON.parse(execFileSync("go", ["run", "games/racing/build/lint_go.go"], {
    cwd: root, input: JSON.stringify(inputs), encoding: "utf8",
    env: { ...process.env, GOCACHE: path.join(root, ".local/go-build"), GOMODCACHE: path.join(root, ".local/go-mod"), GOTMPDIR: temp, TMPDIR: temp, GOTOOLCHAIN: "local", GOTELEMETRY: "off" },
  }));
}

function inspectPython(inputs) {
  return JSON.parse(execFileSync("python3", ["-B", "games/racing/build/lint_python.py"], { cwd: root, input: JSON.stringify(inputs), encoding: "utf8" }));
}

function inspectCSS(source) {
  const errors = [];
  try {
    const file = postcss.parse(source);
    file.walkComments(() => errors.push("no-comments"));
    file.walkRules(rule => {
      for (const match of rule.selector.matchAll(/\.([A-Za-z_][\w-]*)/g)) {
        if (!/^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(match[1])) errors.push("kebab-case-class");
      }
    });
    file.walkDecls(decl => { if (decl.prop.startsWith("--") && !/^--[a-z][a-z0-9-]*$/.test(decl.prop)) errors.push("kebab-case-property"); });
  } catch { errors.push("syntax"); }
  return errors;
}

const results = [];
const goInputs = [];
const pythonInputs = [];
for (const file of config.files) {
  const relative = file.replace("games/racing/", "");
  assert(config.directories ? config.directories.some(directory => file.startsWith(directory)) : /^(server|web|tests|build)\//.test(relative) || /^scripts\/racing\//.test(file), `directory: ${file}`);
  const name = path.basename(file);
  const pythonFilename = new RegExp(config.pythonFilename ?? (file.startsWith("scripts/racing/") ? "^[a-z][a-z0-9-]*\\.py$" : "^[a-z][a-z0-9_]*\\.py$"));
  assert(config.reactPascalCase && /^[A-Z][A-Za-z0-9]*\.tsx$/.test(name) || (file.endsWith(".py") ? pythonFilename.test(name) : file.endsWith(".go") ? /^[a-z][a-z0-9_]*\.go$/.test(name) : /^[a-z][a-z0-9.-]*\.(ts|tsx|mjs|cjs|css)$/.test(name)), `filename: ${file}`);
  const source = readFileSync(path.join(root, file), "utf8");
  if (file.endsWith(".go")) goInputs.push({ path: file, source });
  else if (file.endsWith(".py")) pythonInputs.push({ path: file, source });
  else if (file.endsWith(".css")) results.push({ file, errors: inspectCSS(source) });
  else results.push({ file, errors: inspectScript(source) });
}
const goResults = inspectGo(goInputs);
for (let i = 0; i < goInputs.length; i++) results.push({ file: goInputs[i].path, errors: goResults[i] });
const pythonResults = inspectPython(pythonInputs);
for (let i = 0; i < pythonInputs.length; i++) results.push({ file: pythonInputs[i].path, errors: pythonResults[i] });
let examples = 0;
if (process.argv.includes("--self-test")) {
  const pythonFilename = new RegExp(config.pythonFilename ?? "^[a-z][a-z0-9_]*\\.py$");
  for (const name of ["resource_store.py", "publish-audio.py", "BadName.py"]) {
    assert.equal(pythonFilename.test(name), name === "resource_store.py" || name === "publish-audio.py" && !!config.pythonFilename);
    examples++;
  }
  const samples = [
    ["const driveSpeed = 1;", false],
    ["const bad_name = 1;", true],
    ["// forbidden\nconst driveSpeed = 1;", true],
    ["eval('1')", true],
    ["new Function('return 1')", true],
  ];
  for (const [source, invalid] of samples) { assert.equal(inspectScript(source).length > 0, invalid); examples++; }
  const fixtures = ["package racing\nfunc driveCar() {}", "package racing\nfunc bad_name() {}", "package racing\n// forbidden\nfunc driveCar() {}", "package wrong\nfunc driveCar() {}", "package racing\nvar bad_name = 1"];
  const checked = inspectGo(fixtures.map(source => ({ path: "games/racing/server/fixture.go", source })));
  for (let i = 0; i < checked.length; i++) { assert.equal(checked[i].length > 0, i > 0); examples++; }
  const python = inspectPython(["def drive_car(max_speed):\n return max_speed", "def wrongName():\n pass", "# forbidden\npass", "eval('1')"].map(source => ({ source })));
  for (let i = 0; i < python.length; i++) { assert.equal(python[i].length > 0, i > 0); examples++; }
  for (const [source, invalid] of [[".racing-preview {color:#203d37}", false], [".bad_name {color:#000}", true], ["/* forbidden */ .racing-preview {}", true]]) { assert.equal(inspectCSS(source).length > 0, invalid); examples++; }
}
const passed = results.every(row => row.errors.length === 0);
const report = path.join(out, `lint-${process.pid}.json`);
writeFileSync(report, `${JSON.stringify({ passed, scope: config.scope, languages: ["go", "typescript", "javascript", "python", "css"], files: results, examples }, null, 2)}\n`);
renameSync(report, path.join(out, "lint.json"));
assert(passed, JSON.stringify(results.filter(row => row.errors.length)));
console.log(`Validated ${results.length} registered files and ${examples} positive/negative examples.`);
