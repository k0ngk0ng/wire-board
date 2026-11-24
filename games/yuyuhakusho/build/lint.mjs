import { readFileSync } from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";
import { inspectNumericComparisons } from "../../../web/scripts/numeric-comparisons.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const postcss = require("postcss");
const rules = JSON.parse(readFileSync(new URL("./lint-rules.json", import.meta.url), "utf8"));
const args = process.argv.slice(2);
const targets = args[0] === "--stdin" ? [args[1]] : rules.targets;
let issues = 0;
const goSources = [];
for (const name of targets) {
  const rule = rules.languages[path.extname(name)];
  if (!rule || rule.comments !== false || !new RegExp(rule.directory).test(name) || !new RegExp(rule.filename).test(path.basename(name)) || name.split("/").includes("..")) throw Error(`Unregistered lint path: ${name}`);
  const source = readFileSync(args[0] === "--stdin" ? 0 : path.join(root, name), "utf8");
  const report = message => { issues++; process.stderr.write(`${name}: ${message}\n`); };
  if (name.endsWith(".go")) {
    goSources.push({ File: name, Source: source });
  } else if (name.endsWith(".css")) {
    postcss.parse(source).walkComments(() => report("prohibited code comment"));
  } else {
    const file = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, name.endsWith(".tsx") ? ts.ScriptKind.TSX : name.endsWith(".ts") ? ts.ScriptKind.TS : ts.ScriptKind.JS);
    const walk = node => {
      if (ts.getLeadingCommentRanges(source, node.getFullStart())?.length || ts.getTrailingCommentRanges(source, node.end)?.length) report("prohibited code comment");
      if ((ts.isVariableDeclaration(node) || ts.isParameter(node) || ts.isFunctionDeclaration(node)) && node.name && ts.isIdentifier(node.name) && !new RegExp(rules.identifiers).test(node.name.text)) report(`invalid identifier: ${node.name.text}`);
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "eval") report("prohibited eval");
      if (ts.isNewExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "Function") report("prohibited Function constructor");
      if (rules.outputSources?.includes(name)) {
        if (ts.isStringLiteral(node) && !rules.localInputPaths?.includes(node.text) && ((node.text.includes(".local/") && !rules.localOutputRoots.some(folder => node.text.startsWith(folder))) || /^(?:\/tmp\/|\/private\/tmp\/|~\/)/.test(node.text))) report("invalid game output directory");
        if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && ["tmpdir", "homedir"].includes(node.expression.text)) report("prohibited external temporary directory");
      }
      ts.forEachChild(node, walk);
    };
    walk(file);
    if (rules.numericSources?.includes(name)) inspectNumericComparisons(file, message => report(message));
    for (const diagnostic of file.parseDiagnostics) report(ts.flattenDiagnosticMessageText(diagnostic.messageText, " "));
  }
}
if (goSources.length > 0) {
  const result = JSON.parse(execFileSync("go", ["run", "games/yuyuhakusho/build/lint_go.go"], { cwd: root, input: JSON.stringify(goSources), encoding: "utf8" }));
  result.forEach((messages, index) => messages.forEach(message => {
    issues++;
    process.stderr.write(`${goSources[index].File}: ${message}\n`);
  }));
}
process.stdout.write(`${JSON.stringify({ passed: issues === 0, targets, issues })}\n`);
process.exitCode = issues ? 1 : 0;
