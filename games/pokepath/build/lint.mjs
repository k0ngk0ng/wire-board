import { readFileSync, readdirSync, existsSync, realpathSync } from "node:fs";
import { basename, dirname, extname, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const postcss = require("postcss");
const policy = JSON.parse(readFileSync(new URL("./lint-rules.json", import.meta.url), "utf8"));

function within(path) {
  const name = relative(root, path);
  return name !== ".." && !name.startsWith(`..${sep}`) && !name.startsWith(sep);
}

function validate(language, name) {
  const rule = policy.languages[language];
  if (!rule || rule.comments !== false) throw Error(`Unsupported lint language: ${language}`);
  const path = resolve(root, name);
  if (!within(path)) throw Error(`Outside workspace: ${name}`);
  let parent = path;
  while (!existsSync(parent) && dirname(parent) !== parent) parent = dirname(parent);
  if (!within(realpathSync(parent))) throw Error(`Outside workspace through symlink: ${name}`);
  const local = relative(root, path).split(sep).join("/");
  const preserved = Boolean(rule.preservedDirectory && new RegExp(rule.preservedDirectory).test(local));
  if (!rule.extensions.includes(extname(path))) throw Error(`Invalid extension: ${local}`);
  if (!new RegExp(preserved ? rule.preservedFilename : rule.filename).test(basename(path))) throw Error(`Invalid ${language} filename: ${local}`);
  if (!new RegExp(preserved ? rule.preservedDirectory : rule.directory).test(local) && !policy.additionalTargets.includes(local)) throw Error(`Invalid ${language} directory: ${local}`);
  return { path, local, preserved };
}

function inspect(language, name, source, preserved) {
  if (language === "go") {
    return JSON.parse(execFileSync("go", ["run", "games/pokepath/build/lint_go.go"], { cwd: root, input: JSON.stringify({ Name: name, Source: source, ProhibitedEngineSelectors: policy.languages.go.prohibitedEngineSelectors }), encoding: "utf8" }));
  }
  if (language === "python") {
    return JSON.parse(execFileSync("python3", ["-B", "games/pokepath/build/lint-python.py"], { cwd: root, input: JSON.stringify({ Name: name, Source: source, FrameworkMethods: policy.languages.python.frameworkMethods }), encoding: "utf8" }));
  }
  const issues = [];
  if (language === "css") {
    try {
      postcss.parse(source, { from: name }).walkComments(node => issues.push(`${name}:${node.source.start.line}: prohibited code comment`));
    } catch (error) {
      issues.push(error.message);
    }
    return issues;
  }
  if (language === "html") {
    if (source.includes("<!--")) issues.push(`${name}: prohibited HTML comment`);
    return issues;
  }
  const kind = language === "javascript" ? ts.ScriptKind.JS : name.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const file = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, kind);
  for (const diagnostic of file.parseDiagnostics) issues.push(`${name}: ${ts.flattenDiagnosticMessageText(diagnostic.messageText, " ")}`);
  const comments = new Set();
  const collect = ranges => { for (const range of ranges ?? []) comments.add(range.pos); };
  const identifiers = new RegExp(policy.identifiers);
  const report = (node, message) => {
    const point = file.getLineAndCharacterOfPosition(node.getStart(file));
    issues.push(`${name}:${point.line + 1}:${point.character + 1}: ${message}`);
  };
  const walk = node => {
    collect(ts.getLeadingCommentRanges(source, node.getFullStart()));
    collect(ts.getTrailingCommentRanges(source, node.end));
    if (!preserved) {
      if ((ts.isVariableDeclaration(node) || ts.isParameter(node) || ts.isFunctionDeclaration(node) || ts.isClassDeclaration(node)) && node.name && ts.isIdentifier(node.name) && !identifiers.test(node.name.text)) report(node, `invalid identifier ${node.name.text}`);
      if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) {
        const path = node.moduleSpecifier;
        if (path && ts.isStringLiteral(path)) {
          const target = resolve(dirname(resolve(root, name)), path.text);
          if (path.text.includes("/games/") && !path.text.includes("/games/pokepath/")) report(node, "prohibited cross-game import");
          const gamesRoot = resolve(root, "games");
          if (path.text.startsWith(".") && target.startsWith(`${gamesRoot}${sep}`) && !target.startsWith(resolve(root, "games/pokepath") + sep)) report(node, "prohibited cross-game import");
        }
      }
      if (ts.isNewExpression(node) && ts.isIdentifier(node.expression) && policy.prohibitedConstructors.includes(node.expression.text)) report(node, `prohibited constructor ${node.expression.text}`);
      if (ts.isCallExpression(node)) {
        if (ts.isIdentifier(node.expression) && policy.prohibitedCalls.includes(node.expression.text)) report(node, `prohibited call ${node.expression.text}`);
        if (ts.isPropertyAccessExpression(node.expression) && policy.prohibitedStorageMethods.includes(node.expression.name.text) && /(?:^|\.)localStorage$/.test(node.expression.expression.getText(file))) report(node, "prohibited platform storage deletion");
      }
    }
    for (const child of node.getChildren(file)) walk(child);
  };
  walk(file);
  collect(ts.getLeadingCommentRanges(source, 0));
  for (const position of comments) {
    const point = file.getLineAndCharacterOfPosition(position);
    issues.push(`${name}:${point.line + 1}:${point.character + 1}: prohibited code comment`);
  }
  return issues;
}

function collect(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const path = resolve(directory, entry.name);
    if (entry.isSymbolicLink()) throw Error(`Symbolic link is not a lint target: ${path}`);
    return entry.isDirectory() ? collect(path) : [path];
  });
}

try {
  const args = process.argv.slice(2);
  const issues = [];
  let files = 0;
  if (args[0] === "--stdin") {
    if (args.length !== 3) throw Error("Expected --stdin language path");
    const result = validate(args[1], args[2]);
    issues.push(...inspect(args[1], result.local, readFileSync(0, "utf8"), result.preserved));
    files++;
  } else {
    const targets = args.length ? args : [...collect(resolve(root, policy.root)).map(path => relative(root, path).split(sep).join("/")), ...policy.additionalTargets];
    for (const name of targets) {
      const language = Object.keys(policy.languages).find(key => policy.languages[key].extensions.includes(extname(name)));
      if (!language) continue;
      const result = validate(language, name);
      issues.push(...inspect(language, result.local, readFileSync(result.path, "utf8"), result.preserved));
      files++;
    }
  }
  for (const issue of issues) process.stderr.write(issue + "\n");
  process.stdout.write(JSON.stringify({ passed: issues.length === 0, scope: policy.root, additionalTargets: policy.additionalTargets, files, issues: issues.length }) + "\n");
  process.exitCode = issues.length ? 1 : 0;
} catch (error) {
  process.stderr.write(error.message + "\n");
  process.exitCode = 1;
}
