import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const postcss = require("postcss");
const rules = JSON.parse(readFileSync(new URL("./lint-rules.json", import.meta.url), "utf8"));
const args = process.argv.slice(2);
const targets = args[0] === "--stdin" ? [args[1]] : rules.targets;
let issues = 0;
for (const name of targets) {
  const rule = rules.languages[path.extname(name)];
  if (!rule || rule.comments !== false || !new RegExp(rule.directory).test(name) || !new RegExp(rule.filename).test(path.basename(name)) || name.split("/").includes("..")) throw Error(`Unregistered lint path: ${name}`);
  const source = readFileSync(args[0] === "--stdin" ? 0 : path.join(root, name), "utf8");
  const report = message => { issues++; process.stderr.write(`${name}: ${message}\n`); };
  if (name.endsWith(".py")) {
    const result = JSON.parse(execFileSync("python3", ["-B", path.join(root, "games/bubbleparty/build/lint-python.py")], { input: source, encoding: "utf8" }));
    for (const issue of result) report(issue);
  } else if (name.endsWith(".css")) {
    const css = postcss.parse(source);
    css.walkComments(() => report("prohibited code comment"));
    css.walkRules(node => { for (const match of node.selector.matchAll(/\.([A-Za-z_][A-Za-z0-9_-]*)/g)) if (!/^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(match[1])) report(`invalid CSS class: ${match[1]}`); });
    css.walkDecls(node => { if (node.prop.startsWith("--") && !/^--[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(node.prop)) report(`invalid CSS property: ${node.prop}`); });
  } else {
    const file = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, name.endsWith(".tsx") ? ts.ScriptKind.TSX : name.endsWith(".ts") ? ts.ScriptKind.TS : ts.ScriptKind.JS);
    const walk = node => {
      if (ts.getLeadingCommentRanges(source, node.getFullStart())?.length || ts.getTrailingCommentRanges(source, node.end)?.length) report("prohibited code comment");
      if ((ts.isVariableDeclaration(node) || ts.isParameter(node) || ts.isFunctionDeclaration(node)) && node.name && ts.isIdentifier(node.name) && !new RegExp(rules.identifiers).test(node.name.text)) report(`invalid identifier: ${node.name.text}`);
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "eval") report("prohibited eval");
      if (ts.isNewExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "Function") report("prohibited Function constructor");
      ts.forEachChild(node, walk);
    };
    walk(file);
    for (const diagnostic of file.parseDiagnostics) report(ts.flattenDiagnosticMessageText(diagnostic.messageText, " "));
  }
}
process.stdout.write(`${JSON.stringify({ passed: issues === 0, targets, issues })}\n`);
process.exitCode = issues ? 1 : 0;
