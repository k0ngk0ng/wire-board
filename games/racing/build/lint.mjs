import assert from "node:assert/strict";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const config = JSON.parse(readFileSync(new URL("./lint-rules.json", import.meta.url), "utf8"));
const out = path.join(root, ".local/racing-dynamics/evidence");
const temp = path.join(root, ".local/racing-dynamics/tmp");
mkdirSync(out, { recursive: true });
mkdirSync(temp, { recursive: true });

function inspectScript(source) {
  const errors = [];
  const file = ts.createSourceFile("source.ts", source, ts.ScriptTarget.Latest, true);
  function visit(node) {
    if (ts.getLeadingCommentRanges(source, node.pos)?.length || ts.getTrailingCommentRanges(source, node.end)?.length) errors.push("no-comments");
    if ((ts.isFunctionDeclaration(node) || ts.isVariableDeclaration(node) || ts.isTypeAliasDeclaration(node) || ts.isInterfaceDeclaration(node)) && node.name && ts.isIdentifier(node.name) && node.name.text.includes("_")) errors.push("camel-case-binding");
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
    env: { ...process.env, GOCACHE: path.join(root, ".local/go-build"), GOMODCACHE: path.join(root, ".local/go-mod"), GOTMPDIR: temp, TMPDIR: temp, GOTOOLCHAIN: "local" },
  }));
}

const results = [];
const goInputs = [];
for (const file of config.files) {
  const relative = file.replace("games/racing/", "");
  assert(/^(server|web|tests|build)\//.test(relative), `directory: ${file}`);
  const name = path.basename(file);
  assert(file.endsWith(".go") ? /^[a-z][a-z0-9_]*\.go$/.test(name) : /^[a-z][a-z0-9.-]*\.(ts|mjs)$/.test(name), `filename: ${file}`);
  const source = readFileSync(path.join(root, file), "utf8");
  if (file.endsWith(".go")) goInputs.push({ path: file, source });
  else results.push({ file, errors: inspectScript(source) });
}
const goResults = inspectGo(goInputs);
for (let i = 0; i < goInputs.length; i++) results.push({ file: goInputs[i].path, errors: goResults[i] });
let examples = 0;
if (process.argv.includes("--self-test")) {
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
}
const passed = results.every(row => row.errors.length === 0);
writeFileSync(path.join(out, "lint.json"), `${JSON.stringify({ passed, scope: config.scope, files: results, examples }, null, 2)}\n`);
assert(passed, JSON.stringify(results.filter(row => row.errors.length)));
console.log(`Validated ${results.length} registered racing files and ${examples} positive/negative examples.`);
