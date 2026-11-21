import { readFileSync, writeFileSync, mkdirSync, readdirSync, existsSync, realpathSync, lstatSync } from "node:fs";
import { createHash } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { inspectNumericComparisons } from "../../../web/scripts/numeric-comparisons.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const postcss = require("postcss");
const policy = JSON.parse(readFileSync(new URL("./lint-rules.json", import.meta.url), "utf8"));
const local = path.join(root, ".local/cs16");
const output = path.join(root, policy.output);
const temporary = path.join(local, "tmp");
const goCache = path.join(local, "go-cache");
const goModules = path.join(local, "go-mod");
for (const folder of [output, temporary, goCache, goModules]) mkdirSync(folder, { recursive: true });

function within(base, target) {
  const relative = path.relative(base, target);
  return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function sharedSource(file) {
  return policy.sharedSources.includes(file) || (policy.sharedPythonSources ?? []).includes(file) || (policy.sharedGoSources ?? []).includes(file) || policy.sharedTestPatterns.some(rule => path.posix.dirname(file) === rule.directory && new RegExp(rule.filename).test(path.posix.basename(file)));
}

function validate(item) {
  const target = path.resolve(root, item.file);
  const relative = path.relative(root, target).split(path.sep).join("/");
  const language = policy.languages[item.language];
  if (!language || !within(root, target)) return ["source-location"];
  const shared = sharedSource(relative) || Array.isArray(item.changedLines) && [...policy.sharedPatchSources, ...(policy.sharedPythonPatchSources ?? []), ...(policy.sharedGoPatchSources ?? [])].includes(relative);
  if (!shared && !policy.directories.some(directory => relative.startsWith(directory + "/"))) return ["source-location"];
  let ancestor = target;
  while (!existsSync(ancestor) && path.dirname(ancestor) !== ancestor) ancestor = path.dirname(ancestor);
  if (!within(root, realpathSync(ancestor))) return ["symlink-location"];
  if (!language.extensions.includes(path.extname(target))) return ["source-extension"];
  if (!new RegExp(language.filename).test(path.basename(target))) return ["source-filename"];
  if (shared) return [];
  const prefix = relative.startsWith("scripts/") ? "scripts/cs16/" : "games/cs16/";
  const tail = relative.slice(prefix.length);
  if (prefix.startsWith("games") && !tail.includes("/") && tail !== "controls.json") return ["source-directory"];
  if (prefix.startsWith("games") && tail.includes("/") && !/^(engine|server|web|tests|build|cmd|data|assets)\//.test(tail)) return ["source-directory"];
  if (tail.split("/").slice(0, -1).some(name => !/^[a-z][a-z0-9]*(?:[_-][a-z0-9]+)*$/.test(name))) return ["directory-name"];
  return [];
}

function inspectPaths(item, literals) {
  if (item.file.startsWith("games/cs16/tests/") || path.basename(item.file).startsWith("test_")) return [];
  const issues = [];
  for (const value of literals) {
    for (const match of value.matchAll(/\.local\/([a-z][a-z0-9_-]*)(?:\/|$|\s)/g)) if (match[1] !== "cs16") issues.push("local-output-directory");
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
  const platformAudio = ["web/src/audio-spatial.ts", "web/src/audio.ts"].includes(item.file);
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
  const deterministic = policy.businessTypeScript.includes(path.basename(item.file)) && policy.deterministicTypeScriptPrefixes.some(prefix => item.file.startsWith(prefix));
  const business = deterministic || policy.sharedComparisonTypeScript.includes(item.file);
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
    if (deterministic && ts.isNumericLiteral(node) && /[.eE]/.test(node.text)) emit("integer-business-number", node);
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "eval") emit("no-dynamic-evaluation", node);
    if (ts.isCallExpression(node) && ["tmpdir", "homedir"].includes(expressionName(node.expression))) emit("workspace-output-directory", node);
    if (ts.isNewExpression(node)) {
      const constructor = expressionName(node.expression);
      if (constructor === "Function") emit("no-dynamic-evaluation", node);
      if (!platformAudio && ["Audio", "AudioContext", "webkitAudioContext"].includes(constructor)) emit("platform-audio-only", node);
    }
    if (!platformAudio && ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "createElement" && node.arguments.some(argument => ts.isStringLiteral(argument) && ["audio", "video"].includes(argument.text.toLowerCase()))) emit("platform-audio-only", node);
    if (deterministic && ts.isPropertyAccessExpression(node) && (node.getText(file) === "Number.EPSILON" || node.getText(file) === "Math.random")) emit("integer-business-number", node);
    if ((ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) && node.moduleSpecifier && ts.isStringLiteral(node.moduleSpecifier)) {
      const imported = node.moduleSpecifier.text;
      if (imported.startsWith(".")) {
        const target = path.resolve(root, path.dirname(item.file), imported);
        if (within(path.join(root, "games"), target) && !within(path.join(root, "games/cs16"), target)) emit("no-cross-game-import", node.moduleSpecifier);
      }
    }
    if (ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword && node.arguments.length && ts.isStringLiteral(node.arguments[0])) {
      const imported = node.arguments[0].text;
      if (imported.startsWith(".")) {
        const target = path.resolve(root, path.dirname(item.file), imported);
        if (within(path.join(root, "games"), target) && !within(path.join(root, "games/cs16"), target)) emit("no-cross-game-import", node.arguments[0]);
      }
    }
    for (const child of node.getChildren(file)) walk(child);
  }
  walk(file);
  if (business && !deterministic) inspectNumericComparisons(file, emit);
  collect(ts.getLeadingCommentRanges(item.source, 0));
  if (comments.size) issues.push("no-comments");
  return [...issues, ...inspectPaths(item, literals)];
}

function inspectCSS(item) {
  const issues = [];
  try {
    const file = postcss.parse(item.source, { from: item.file });
    file.walkComments(() => issues.push("no-comments"));
    file.walkRules(rule => {
      for (const match of rule.selector.matchAll(/\.([A-Za-z_][\w-]*)/g)) if (!/^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(match[1])) issues.push("kebab-case-class");
    });
    file.walkDecls(declaration => {
      if (declaration.prop.startsWith("--") && !/^--[a-z][a-z0-9-]*$/.test(declaration.prop)) issues.push("kebab-case-property");
    });
  } catch {
    issues.push("syntax");
  }
  return issues;
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

function runHelper(language, items) {
  if (!items.length) return [];
  const executable = language === "go" ? "go" : "python3";
  const args = language === "go" ? ["run", "games/cs16/build/lint_go.go"] : ["-B", "games/cs16/build/lint-python.py"];
  const run = spawnSync(executable, args, {
    cwd: root,
    input: JSON.stringify(items.map(item => language === "go" ? { ...item, expectedPackage: policy.sharedGoPackages?.[path.posix.dirname(item.file)], strictIntegers: policy.deterministicGoPrefixes.some(prefix => item.file.startsWith(prefix)) } : item)),
    encoding: "utf8",
    env: { ...process.env, GOCACHE: goCache, GOMODCACHE: goModules, GOTMPDIR: temporary, TMPDIR: temporary, GOTOOLCHAIN: "local", GOTELEMETRY: "off" },
  });
  if (run.status !== 0) throw Error(run.stdout + run.stderr);
  return JSON.parse(run.stdout);
}

function inspect(items) {
  const results = items.map(item => ({ file: item.file, language: item.language, issues: validate(item) }));
  for (const language of ["go", "python"]) {
    const selected = items.flatMap((item, index) => item.language === language && !results[index].issues.length ? [{ item, index }] : []);
    const checked = runHelper(language, selected.map(row => row.item));
    for (let index = 0; index < selected.length; index++) results[selected[index].index].issues.push(...checked[index]);
  }
  for (let index = 0; index < items.length; index++) {
    const item = items[index];
    if (results[index].issues.length) continue;
    if (["typescript", "javascript"].includes(item.language)) results[index].issues.push(...inspectScript(item));
    if (item.language === "css") results[index].issues.push(...inspectCSS(item));
    if (item.language === "json") results[index].issues.push(...inspectJSON(item));
  }
  return results.map((row, index) => ({ ...row, coverage: Array.isArray(items[index].changedLines) ? "added-lines" : "full-source", changedLines: items[index].changedLines, sourceSHA256: createHash("sha256").update(items[index].source).digest("hex"), issues: [...new Set(row.issues)], passed: row.issues.length === 0 }));
}

function discover(directory, workspace) {
  if (!existsSync(directory)) return [];
  const relative = path.relative(workspace, directory).split(path.sep).join("/");
  if (relative === policy.externalResourceDirectory) return [];
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const target = path.join(directory, entry.name);
    if (entry.isSymbolicLink()) throw Error("Symlink inside source directory: " + target);
    if (entry.isDirectory()) return discover(target, workspace);
    const language = Object.keys(policy.languages).find(name => policy.languages[name].extensions.includes(path.extname(target)));
    return language ? [{ file: path.relative(workspace, target).split(path.sep).join("/"), language, source: readFileSync(target, "utf8") }] : [];
  });
}

export function discoverAuthoredSources(workspace = root) {
  const authored = policy.directories.flatMap(directory => discover(path.join(workspace, directory), workspace));
  const shared = [...policy.sharedSources, ...(policy.sharedPythonSources ?? []), ...(policy.sharedGoSources ?? [])];
  for (const rule of policy.sharedTestPatterns) {
    const directory = path.join(workspace, rule.directory);
    if (!existsSync(directory)) continue;
    for (const entry of readdirSync(directory, { withFileTypes: true })) if (new RegExp(rule.filename).test(entry.name)) shared.push(path.posix.join(rule.directory, entry.name));
  }
  for (const file of shared) {
    const target = path.join(workspace, file);
    if (!existsSync(target)) continue;
    if (lstatSync(target).isSymbolicLink()) throw Error("Symlink inside shared source: " + file);
    const language = Object.keys(policy.languages).find(name => policy.languages[name].extensions.includes(path.extname(target)));
    authored.push({ file, language, source: readFileSync(target, "utf8") });
  }
  return [...authored, ...discoverChangedPlatformSources(workspace)];
}

function discoverChangedPlatformSources(workspace) {
  const selected = [...policy.sharedPatchSources, ...(policy.sharedPythonPatchSources ?? []), ...(policy.sharedGoPatchSources ?? [])].filter(file => existsSync(path.join(workspace, file)));
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
    } else if (current && line.startsWith("+")) {
      changes.get(current).add(lineNumber++);
    } else if (current && line.startsWith(" ")) lineNumber++;
  }
  return [...changes].flatMap(([file, lines]) => {
    if (!lines.size) return [];
    const target = path.join(workspace, file);
    if (lstatSync(target).isSymbolicLink()) throw Error("Symlink inside shared patch source: " + file);
    return [{ file, language: file.endsWith(".py") ? "python" : file.endsWith(".go") ? "go" : "typescript", source: readFileSync(target, "utf8"), changedLines: [...lines].sort((left, right) => left - right) }];
  });
}

export function inspectPayloadBoundary(files) {
  const payloads = files.filter(file => /\.(?:bsp|wad|mdl|wav|flac|ogg|mp3|aac|m4a|mp4|webm|png|jpe?g|webp|gif|svg|ico|glb|gltf|wasm|zip|7z|tar|gz|br|bin|ttf|otf|woff2?)$/i.test(file));
  return { passed: payloads.length === 0, files: payloads, issues: payloads.map(file => file + ": no-client-payload-in-git") };
}

function repositoryPayloads() {
  const run = spawnSync("git", ["ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ...policy.directories], { cwd: root, encoding: "utf8" });
  if (run.status !== 0) throw Error(run.stdout + run.stderr);
  return inspectPayloadBoundary(run.stdout.split("\0").filter(Boolean));
}

export function inspectExternalResources(workspace = root) {
  const issues = [];
  const files = [];
  const prefix = policy.externalResourceDirectory + "/";
  const selected = new Set();
  const present = new Set();
  let registry;
  try {
    registry = JSON.parse(readFileSync(path.join(workspace, "assets/resources.json"), "utf8"));
    if (registry.version !== 1 || !registry.files || typeof registry.files !== "object") throw Error("Invalid source resource registry");
  } catch (error) {
    return { passed: false, directory: policy.externalResourceDirectory, files, issues: [String(error)] };
  }
  for (const [file, record] of Object.entries(registry.files).filter(([file]) => file.startsWith(prefix)).sort(([left], [right]) => left.localeCompare(right))) {
    selected.add(file);
    const row = { file, bytes: record.bytes, sha256: record.sha256, mime: record.mime, passed: false };
    files.push(row);
    try {
      if (file.includes("\\") || file.split("/").some(part => !part || part === "." || part === "..") || !within(path.resolve(workspace, policy.externalResourceDirectory), path.resolve(workspace, file))) throw Error("external-resource-path");
      if (!Number.isSafeInteger(record.bytes) || record.bytes <= 0 || !/^[0-9a-f]{64}$/.test(record.sha256) || typeof record.mime !== "string" || !record.mime || !new RegExp("^bga/cs16/" + record.sha256 + "(?:-gzip)?\\.[a-z0-9]+$").test(record.key)) throw Error("external-resource-record");
      let ancestor = path.resolve(workspace, file);
      while (within(path.resolve(workspace), ancestor)) {
        if (lstatSync(ancestor).isSymbolicLink()) throw Error("external-resource-symlink");
        if (ancestor === path.resolve(workspace)) break;
        ancestor = path.dirname(ancestor);
      }
      const bytes = readFileSync(path.resolve(workspace, file));
      if (bytes.length !== record.bytes || createHash("sha256").update(bytes).digest("hex") !== record.sha256) throw Error("external-resource-size-or-digest");
      row.passed = true;
    } catch (error) {
      issues.push(file + ": " + String(error));
    }
  }
  function visit(directory) {
    if (!existsSync(directory)) return;
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const target = path.join(directory, entry.name);
      const file = path.relative(workspace, target).split(path.sep).join("/");
      if (entry.isSymbolicLink()) {
        issues.push(file + ": external-resource-symlink");
      } else if (entry.isDirectory()) {
        visit(target);
      } else {
        present.add(file);
        if (!selected.has(file)) issues.push(file + ": unregistered-external-resource");
      }
    }
  }
  visit(path.join(workspace, policy.externalResourceDirectory));
  for (const file of selected) if (!present.has(file)) issues.push(file + ": missing-external-resource");
  return { passed: issues.length === 0, directory: policy.externalResourceDirectory, files, issues: [...new Set(issues)] };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) try {
  const args = process.argv.slice(2);
  if (args[0] === "--batch") {
    process.stdout.write(JSON.stringify(inspect(JSON.parse(readFileSync(0, "utf8")))) + "\n");
  } else {
    const items = args[0] === "--stdin" ? [{ language: args[1], file: args[2], source: readFileSync(0, "utf8") }] : discoverAuthoredSources();
    const files = inspect(items);
    const externalResources = args[0] === "--stdin" ? undefined : inspectExternalResources();
    const payloadBoundary = args[0] === "--stdin" ? undefined : repositoryPayloads();
    const report = { passed: files.every(file => file.passed) && externalResources?.passed !== false && payloadBoundary?.passed !== false, scope: policy.scope, files, externalResources, payloadBoundary };
    writeFileSync(path.join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
    process.stdout.write(JSON.stringify(report) + "\n");
    process.exitCode = report.passed ? 0 : 1;
  }
} catch (error) {
  process.stderr.write(String(error) + "\n");
  process.exitCode = 1;
}
