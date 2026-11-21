import { readFileSync, existsSync, readdirSync, lstatSync, realpathSync } from "node:fs";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const require = createRequire(new URL("../package.json", import.meta.url));
const ts = require("typescript");
const policy = JSON.parse(readFileSync(new URL("./platform-source-lint.json", import.meta.url), "utf8"));

function within(base, target) {
  const relative = path.relative(base, target);
  return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function registered(file) {
  return policy.sources.includes(file) || policy.patterns.some(rule => path.posix.dirname(file) === rule.directory && new RegExp(rule.filename).test(path.posix.basename(file)));
}

function validate(item) {
  const target = path.resolve(root, item.file);
  const relative = path.relative(root, target).split(path.sep).join("/");
  if (!within(root, target) || !registered(relative) && !(Array.isArray(item.changedLines) && policy.patchSources.includes(relative))) return ["source-location"];
  if (!new RegExp(policy.filename).test(path.basename(target))) return ["source-filename"];
  if (!['typescript', 'javascript', 'json'].includes(item.language)) return ["source-language"];
  let ancestor = target;
  while (!existsSync(ancestor) && path.dirname(ancestor) !== ancestor) ancestor = path.dirname(ancestor);
  if (!within(root, realpathSync(ancestor))) return ["source-symlink"];
  while (within(root, ancestor)) {
    if (lstatSync(ancestor).isSymbolicLink()) return ["source-symlink"];
    if (ancestor === root || ancestor === path.dirname(ancestor)) break;
    ancestor = path.dirname(ancestor);
  }
  return [];
}

function inspectPaths(item, literals) {
  const issues = [];
  for (const value of literals) {
    for (const match of value.matchAll(/\.local\/[a-z][a-z0-9_-]*(?:\/|$|\s)/g)) if (!policy.localOutputRoots.some(folder => match[0].startsWith(folder + "/") || match[0].trimEnd() === folder)) issues.push("local-output-directory");
    if (/^(?:\/tmp\/.+|\/private\/tmp\/.+|~\/.+)/.test(value)) issues.push("workspace-output-directory");
    if (/data:(?:image|audio|video|application\/wasm)[^,]*;base64,/i.test(value)) issues.push("no-embedded-media");
  }
  return issues;
}

function inspectScript(item) {
  const issues = [];
  const file = ts.createSourceFile(item.file, item.source, ts.ScriptTarget.Latest, true, item.language === "javascript" ? ts.ScriptKind.JS : item.file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
  const changed = Array.isArray(item.changedLines) ? new Set(item.changedLines) : undefined;
  const active = (start, end = start) => {
    if (!changed) return true;
    const first = file.getLineAndCharacterOfPosition(Math.min(Math.max(0, start), item.source.length)).line + 1;
    const last = file.getLineAndCharacterOfPosition(Math.min(Math.max(start, end - 1), item.source.length)).line + 1;
    for (let line = first; line <= last; line++) if (changed.has(line)) return true;
    return false;
  };
  const emit = (issue, node) => {
    if (active(node.getStart?.(file) ?? node.pos, node.end)) issues.push(issue);
  };
  for (const diagnostic of file.parseDiagnostics) if (active(diagnostic.start ?? 0, (diagnostic.start ?? 0) + (diagnostic.length ?? 0))) issues.push("syntax:" + ts.flattenDiagnosticMessageText(diagnostic.messageText, " "));
  const comments = new Set();
  const literals = [];
  const collect = ranges => { for (const range of ranges ?? []) if (active(range.pos, range.end)) comments.add(range.pos); };
  const platformAudio = policy.platformAudioSources.includes(item.file);
  const contextNames = new Set();
  const expressionName = node => ts.isIdentifier(node) ? node.text : ts.isPropertyAccessExpression(node) ? node.name.text : "";
  const audioConstructor = node => ts.isNewExpression(node) && ["AudioContext", "webkitAudioContext"].includes(expressionName(node.expression));
  function collectContexts(node) {
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.initializer && audioConstructor(node.initializer)) contextNames.add(node.name.text);
    if (ts.isPropertyDeclaration(node) && node.type && /\b(?:AudioContext|BaseAudioContext)\b/.test(node.type.getText(file))) contextNames.add(expressionName(node.name));
    if (ts.isBinaryExpression(node) && audioConstructor(node.right)) contextNames.add(expressionName(node.left));
    for (const child of node.getChildren(file)) collectContexts(child);
  }
  collectContexts(file);
  const contextOwner = node => /^(?:context|audioContext|ctx)$/i.test(expressionName(node)) || contextNames.has(expressionName(node)) || audioConstructor(node);
  const destinationBindings = new Set();
  function destinationValue(node) {
    if (ts.isPropertyAccessExpression(node) && node.name.text === "destination" && contextOwner(node.expression)) return true;
    if (ts.isElementAccessExpression(node) && node.argumentExpression && ts.isStringLiteral(node.argumentExpression) && node.argumentExpression.text === "destination" && contextOwner(node.expression)) return true;
    if (ts.isIdentifier(node) && destinationBindings.has(node.text)) return true;
    return node.getChildren(file).some(destinationValue);
  }
  function collectDestinationBindings(node) {
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.initializer && destinationValue(node.initializer)) destinationBindings.add(node.name.text);
    if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken && ts.isIdentifier(node.left) && destinationValue(node.right)) destinationBindings.add(node.left.text);
    for (const child of node.getChildren(file)) collectDestinationBindings(child);
  }
  collectDestinationBindings(file);
  const binding = node => {
    if (ts.isIdentifier(node)) {
      if (node.text.includes("_") && node.text !== "_" && !/^[A-Z][A-Z0-9_]*$/.test(node.text)) emit("camel-case-binding", node);
    } else if (ts.isObjectBindingPattern(node) || ts.isArrayBindingPattern(node)) {
      for (const element of node.elements) if (ts.isBindingElement(element)) binding(element.name);
    }
  };
  const business = policy.businessSources.includes(item.file);
  function walk(node) {
    collect(ts.getLeadingCommentRanges(item.source, node.getFullStart()));
    collect(ts.getTrailingCommentRanges(item.source, node.end));
    if ((ts.isVariableDeclaration(node) || ts.isFunctionDeclaration(node) || ts.isParameter(node) || ts.isClassDeclaration(node) || ts.isMethodDeclaration(node) || ts.isTypeAliasDeclaration(node) || ts.isInterfaceDeclaration(node)) && node.name) binding(node.name);
    if (node.kind === ts.SyntaxKind.AnyKeyword) emit("no-any", node);
    if (ts.isIdentifier(node) && ["eval", "Function"].includes(node.text)) emit("no-dynamic-evaluation", node);
    if (ts.isElementAccessExpression(node) && node.argumentExpression && ts.isStringLiteral(node.argumentExpression)) {
      if (["eval", "Function"].includes(node.argumentExpression.text)) emit("no-dynamic-evaluation", node);
      if (!platformAudio && ["Audio", "AudioContext", "webkitAudioContext"].includes(node.argumentExpression.text)) emit("platform-audio-only", node);
    }
    if (ts.isCallExpression(node) && expressionName(node.expression) === "connect" && node.arguments.some(destinationValue)) emit("native-media-output-only", node);
    if (ts.isStringLiteralLike(node) && active(node.getStart(file), node.end)) literals.push(node.text);
    if (business && ts.isNumericLiteral(node) && /[.eE]/.test(node.text)) emit("integer-business-number", node);
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "eval") emit("no-dynamic-evaluation", node);
    if (ts.isCallExpression(node) && ["tmpdir", "homedir"].includes(expressionName(node.expression))) emit("workspace-output-directory", node);
    if (ts.isNewExpression(node)) {
      const constructor = expressionName(node.expression);
      if (constructor === "Function") emit("no-dynamic-evaluation", node);
      if (!platformAudio && ["Audio", "AudioContext", "webkitAudioContext"].includes(constructor)) emit("platform-audio-only", node);
    }
    if (!platformAudio && ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "createElement" && node.arguments.some(argument => ts.isStringLiteral(argument) && ["audio", "video"].includes(argument.text.toLowerCase()))) emit("platform-audio-only", node);
    if (business && ts.isPropertyAccessExpression(node) && (node.getText(file) === "Number.EPSILON" || node.getText(file) === "Math.random")) emit("integer-business-number", node);
    if ((ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) && node.moduleSpecifier && ts.isStringLiteral(node.moduleSpecifier)) {
      const imported = node.moduleSpecifier.text;
      if (imported.startsWith(".")) {
        const target = path.resolve(root, path.dirname(item.file), imported);
        if (within(path.join(root, "games"), target)) emit("no-cross-game-import", node.moduleSpecifier);
      }
    }
    if (ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword && node.arguments.length && ts.isStringLiteral(node.arguments[0])) {
      const imported = node.arguments[0].text;
      if (imported.startsWith(".")) {
        const target = path.resolve(root, path.dirname(item.file), imported);
        if (within(path.join(root, "games"), target)) emit("no-cross-game-import", node.arguments[0]);
      }
    }
    for (const child of node.getChildren(file)) walk(child);
  }
  walk(file);
  collect(ts.getLeadingCommentRanges(item.source, 0));
  if (comments.size) issues.push("no-comments");
  return [...issues, ...inspectPaths(item, literals)];
}

function inspectJSON(item) {
  const issues = [];
  try {
    JSON.parse(item.source, (key, value) => {
      if (["__proto__", "constructor", "prototype"].includes(key)) issues.push("no-prototype-key");
      return value;
    });
  } catch {
    issues.push("syntax");
  }
  return issues;
}

export function inspectSources(items) {
  return items.map(item => {
    const issues = validate(item);
    if (!issues.length) issues.push(...(item.language === "json" ? inspectJSON(item) : inspectScript(item)));
    return { file: item.file, language: item.language, coverage: Array.isArray(item.changedLines) ? "added-lines" : "full-source", changedLines: item.changedLines, sourceSHA256: createHash("sha256").update(item.source).digest("hex"), issues: [...new Set(issues)], passed: issues.length === 0 };
  });
}

function changedSources(workspace) {
  const selected = policy.patchSources.filter(file => existsSync(path.join(workspace, file)));
  if (!selected.length) return [];
  const run = spawnSync("git", ["diff", "HEAD", "--no-ext-diff", "--no-color", "--unified=0", "--", ...selected], { cwd: workspace, encoding: "utf8" });
  if (run.status !== 0) throw Error(run.stdout + run.stderr);
  const changes = new Map();
  let current;
  let lineNumber = 0;
  for (const line of run.stdout.split("\n")) {
    if (line.startsWith("+++ b/")) {
      const file = line.slice(6);
      current = selected.includes(file) ? file : undefined;
      if (current) changes.set(current, new Set());
    } else if (line.startsWith("@@")) {
      const match = /\+(\d+)(?:,\d+)? @@/.exec(line);
      if (match) lineNumber = Number(match[1]);
    } else if (current && line.startsWith("+")) changes.get(current).add(lineNumber++);
    else if (current && line.startsWith(" ")) lineNumber++;
  }
  return [...changes].flatMap(([file, lines]) => lines.size ? [{ file, language: "typescript", source: readFileSync(path.join(workspace, file), "utf8"), changedLines: [...lines].sort((left, right) => left - right) }] : []);
}

export function discoverPlatformSources(workspace = root) {
  const selected = new Set(policy.sources);
  for (const rule of policy.patterns) {
    const directory = path.join(workspace, rule.directory);
    if (!existsSync(directory)) throw Error("Missing source directory: " + rule.directory);
    for (const entry of readdirSync(directory, { withFileTypes: true })) if (new RegExp(rule.filename).test(entry.name)) selected.add(path.posix.join(rule.directory, entry.name));
  }
  return [...selected].map(file => {
    const target = path.join(workspace, file);
    if (!existsSync(target)) throw Error("Missing registered source: " + file);
    if (lstatSync(target).isSymbolicLink()) throw Error("Symlink registered source: " + file);
    return { file, language: file.endsWith(".mjs") ? "javascript" : file.endsWith(".json") ? "json" : "typescript", source: readFileSync(target, "utf8") };
  }).concat(changedSources(workspace));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) try {
  const items = process.argv[2] === "--batch" ? JSON.parse(readFileSync(0, "utf8")) : discoverPlatformSources();
  const files = inspectSources(items);
  const report = { passed: files.every(file => file.passed), scope: policy.scope, files };
  process.stdout.write(JSON.stringify(report) + "\n");
  process.exitCode = report.passed ? 0 : 1;
} catch (error) {
  process.stderr.write(String(error) + "\n");
  process.exitCode = 1;
}
