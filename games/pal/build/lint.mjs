import { readFileSync, existsSync, realpathSync } from "node:fs";
import { dirname, basename, extname, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript"), postcss = require("postcss");
const policy = JSON.parse(readFileSync(new URL("./lint-rules.json", import.meta.url), "utf8"));
const languages = ["typescript", "javascript", "python", "go", "css", "c"];

function within(path) {
  const name = relative(root, path);
  return name !== ".." && !name.startsWith(`..${sep}`) && !name.startsWith(sep);
}
function validate(language, name) {
  const rule = policy.languages[language];
  if (!languages.includes(language) || !rule) throw Error(`Unsupported lint language: ${language}`);
  const path = resolve(root, name);
  if (!within(path)) throw Error(`Outside workspace: ${name}`);
  let probe = path;
  while (!existsSync(probe) && dirname(probe) !== probe) probe = dirname(probe);
  if (!within(realpathSync(probe))) throw Error(`Outside workspace through symlink: ${name}`);
  const local = relative(root, path).split(sep).join("/");
  if (!rule.extensions.includes(extname(path))) throw Error(`Invalid extension: ${local}`);
  if (!new RegExp(rule.filename).test(basename(path))) throw Error(`Invalid ${language} filename: ${local}`);
  if (!new RegExp(rule.directory).test(local)) throw Error(`Invalid ${language} directory: ${local}`);
  return path;
}
function parse(language, name, source) {
  if (language === "python" || language === "go" || language === "c") {
    const payload = JSON.stringify({ Name: name, Source: source });
    const program = language === "go" ? "go" : "python3";
    const args = language === "go" ? ["run", "games/pal/build/lint_go.go"] : ["-B", `games/pal/build/lint-${language}.py`];
    return JSON.parse(execFileSync(program, args, { cwd: root, input: payload, encoding: "utf8" }));
  }
  const issues = [];
  if (language === "css") {
    try {
      postcss.parse(source, { from: name }).walkComments(node => issues.push(`${name}:${node.source.start.line}:${node.source.start.column}: prohibited code comment`));
    } catch (error) { issues.push(error.message); }
    return issues;
  }
  const kind = language === "javascript" ? ts.ScriptKind.JS : name.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const file = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, kind);
  for (const diagnostic of file.parseDiagnostics) issues.push(`${name}: ${ts.flattenDiagnosticMessageText(diagnostic.messageText, " ")}`);
  const comments = new Map();
  const collect = ranges => { for (const range of ranges ?? []) comments.set(range.pos, range); };
  const walk = node => {
    collect(ts.getLeadingCommentRanges(source, node.getFullStart()));
    collect(ts.getTrailingCommentRanges(source, node.end));
    for (const child of node.getChildren(file)) walk(child);
  };
  walk(file);
  collect(ts.getLeadingCommentRanges(source, 0));
  for (const [position] of comments) {
    const point = file.getLineAndCharacterOfPosition(position);
    issues.push(`${name}:${point.line+1}:${point.character+1}: prohibited code comment`);
  }
  return issues;
}

try {
  if (Object.keys(policy).some(key => !["languages", "targets"].includes(key)) || !Array.isArray(policy.targets) || Object.keys(policy.languages).sort().join() !== [...languages].sort().join()) throw Error("Invalid lint policy");
  for (const language of languages) {
    const rule = policy.languages[language];
    if (rule.comments !== false || !Array.isArray(rule.extensions) || typeof rule.filename !== "string" || typeof rule.directory !== "string" || Object.keys(rule).some(key => !["comments", "extensions", "filename", "directory"].includes(key))) throw Error(`Invalid ${language} lint rules`);
  }
  const args = process.argv.slice(2), issues = [];
  if (args[0] === "--stdin") {
    if (args.length !== 3) throw Error("Expected --stdin language path");
    validate(args[1], args[2]);
    issues.push(...parse(args[1], args[2], readFileSync(0, "utf8")));
  } else {
    for (const name of args.length ? args : policy.targets) {
      const language = languages.find(language => policy.languages[language].extensions.includes(extname(name)));
      const path = validate(language, name);
      issues.push(...parse(language, name, readFileSync(path, "utf8")));
    }
  }
  for (const issue of issues) process.stderr.write(issue + "\n");
  process.stdout.write(JSON.stringify({ passed: issues.length === 0, issues: issues.length }) + "\n");
  process.exitCode = issues.length ? 1 : 0;
} catch (error) {
  process.stderr.write(error.message + "\n");
  process.exitCode = 1;
}
