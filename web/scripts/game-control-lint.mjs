import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { createRequire } from "node:module";

const require = createRequire(new URL("../package.json", import.meta.url));
const ts = require("typescript");
const postcss = require("postcss");
const policy = JSON.parse(readFileSync(new URL("./game-control-lint.json", import.meta.url), "utf8"));

export function inspectGameControlFile(file, source) {
  const issues = [];
  const rule = policy.languages[path.extname(file)];
  if (!rule || !new RegExp(rule.directory).test(file) || file.split("/").includes("..")) issues.push("directory");
  if (rule && !new RegExp(rule.filename).test(path.basename(file))) issues.push("filename");
  if (file.endsWith(".json")) {
    try { JSON.parse(source); } catch { issues.push("syntax"); }
    return issues;
  }
  if (file.endsWith(".css")) {
    try {
      const parsed = postcss.parse(source);
      parsed.walkComments(() => issues.push("no-comments"));
      parsed.walkRules(node => {
        for (const match of node.selector.matchAll(/\.([A-Za-z_][\w-]*)/g)) if (!/^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(match[1])) issues.push("kebab-case");
      });
    } catch { issues.push("syntax"); }
    return [...new Set(issues)];
  }
  const parsed = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
  if (parsed.parseDiagnostics.length) issues.push("syntax");
  const visit = node => {
    if (ts.getLeadingCommentRanges(source, node.pos)?.length || ts.getTrailingCommentRanges(source, node.end)?.length) issues.push("no-comments");
    if ((ts.isVariableDeclaration(node) || ts.isFunctionDeclaration(node) || ts.isParameter(node) || ts.isTypeAliasDeclaration(node) || ts.isInterfaceDeclaration(node) || ts.isClassDeclaration(node)) && node.name && ts.isIdentifier(node.name) && !/^[A-Za-z][A-Za-z0-9]*$/.test(node.name.text)) issues.push("camel-case");
    if (ts.isCallExpression(node)) {
      if (ts.isIdentifier(node.expression) && ["eval", "exec"].includes(node.expression.text)) issues.push("no-dynamic-execution");
      if (ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "addEventListener" && ["document", "window", "doc"].includes(node.expression.expression.getText(parsed)) && node.arguments[0] && ts.isStringLiteral(node.arguments[0]) && ["touchstart", "touchmove", "pointerdown", "pointermove", "contextmenu", "selectstart"].includes(node.arguments[0].text)) issues.push("no-global-input-block");
    }
    if (ts.isNewExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "Function") issues.push("no-dynamic-execution");
    ts.forEachChild(node, visit);
  };
  visit(parsed);
  return [...new Set(issues)];
}

export function inspectGameControlContract(sources) {
  const issues = [];
  const host = sources["web/src/ui/room-fullscreen.tsx"] ?? "";
  const guard = sources["web/src/ui/game-control-guard.ts"] ?? "";
  const css = sources["web/src/ui/game-control-guard.css"] ?? "";
  const parsed = ts.createSourceFile("host.tsx", host, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const protectedClasses = new Set();
  const visit = node => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const attributes = node.attributes.properties.filter(ts.isJsxAttribute);
      const className = attributes.find(attribute => attribute.name.getText(parsed) === "className")?.initializer;
      if (className && ts.isStringLiteral(className) && attributes.some(attribute => attribute.name.getText(parsed) === "data-game-controls")) protectedClasses.add(className.text);
    }
    ts.forEachChild(node, visit);
  };
  visit(parsed);
  if (!protectedClasses.has("game-fullscreen-board")) issues.push("board-protection");
  if (!protectedClasses.has("game-fullscreen-toolbar")) issues.push("toolbar-protection");
  if (!host.includes("return protectGameControls(root)") || !host.includes('import "./game-control-guard.css"')) issues.push("guard-lifecycle");
  for (const event of ["contextmenu", "selectstart", "pointerdown"]) {
    if (!guard.includes(`root.addEventListener("${event}",`) || !guard.includes(`root.removeEventListener("${event}",`)) issues.push("listener-cleanup");
  }
  if (!guard.includes('document.removeEventListener("selectionchange", clearSelection)')) issues.push("listener-cleanup");
  try {
    const rules = postcss.parse(css).nodes.filter(node => node.type === "rule");
    const primary = rules.find(node => node.selector.includes("[data-game-controls] *") && !node.selector.includes("contenteditable"));
    const declarations = Object.fromEntries((primary?.nodes ?? []).filter(node => node.type === "decl").map(node => [node.prop, node.value]));
    for (const [property, issue] of [["user-select", "selection"], ["-webkit-user-select", "webkit-selection"], ["-webkit-touch-callout", "webkit-callout"]]) if (declarations[property] !== "none") issues.push(issue);
    if (!rules.some(node => node.selector.includes("contenteditable") && node.nodes.some(child => child.type === "decl" && child.prop === "user-select" && child.value === "text"))) issues.push("editor-selection");
  } catch { issues.push("style-contract"); }
  const main = sources["web/src/main.tsx"] ?? "";
  const boardAt = main.indexOf("<GameBoard");
  if (boardAt < 0 || main.lastIndexOf("<GameFullscreen", boardAt) <= main.lastIndexOf("</GameFullscreen>", boardAt)) issues.push("shared-host-boundary");
  return [...new Set(issues)];
}

export function lintGameControls(root) {
  const sources = Object.fromEntries([...policy.files, ...policy.contractFiles].map(file => [file, readFileSync(path.join(root, file), "utf8")]));
  const files = policy.files.map(file => ({ file, issues: inspectGameControlFile(file, sources[file]) }));
  const contract = inspectGameControlContract(sources);
  const report = { passed: files.every(file => !file.issues.length) && !contract.length, scope: policy.scope, files, contract };
  const output = path.join(root, ".local/hotlinemiami/verification/game-control-lint");
  mkdirSync(output, { recursive: true });
  writeFileSync(path.join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  return report;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const result = lintGameControls(fileURLToPath(new URL("../../", import.meta.url)));
  console.log(JSON.stringify(result));
  if (!result.passed) process.exitCode = 1;
}
