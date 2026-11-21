import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import path from "node:path";

const require = createRequire(new URL("../package.json", import.meta.url));
const ts = require("typescript");
const postcss = require("postcss");
const root = fileURLToPath(new URL("../../", import.meta.url));
const files = [
  "web/src/ui/board-host.tsx",
  "web/src/ui/result-overlay.tsx",
  "web/src/ui/results.tsx",
  "web/src/ui/leave-room-control.tsx",
  "web/src/ui/room-tool-button.tsx",
  "web/src/ui/room-actions.css",
  "web/scripts/room-ui-lint.mjs",
  "web/tests/room-ui-lint.test.mjs",
  "web/tests/room-flow-browser.test.mjs",
];
const languages = {
  ".tsx": /^web\/src\/ui\//,
  ".css": /^web\/src\/ui\//,
  ".mjs": /^web\/(scripts|tests)\//,
};

export function inspectRoomUiFile(file, source) {
  const issues = [];
  const extension = path.extname(file);
  if (!languages[extension]?.test(file) || file.split("/").includes(".."))
    issues.push("directory");
  if (
    !/^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*\.(tsx|css|mjs)$/.test(
      path.basename(file),
    )
  )
    issues.push("filename");
  if (extension === ".css") {
    try {
      const parsed = postcss.parse(source);
      parsed.walkComments(() => issues.push("no-comments"));
      parsed.walkRules((node) => {
        for (const match of node.selector.matchAll(/\.([A-Za-z_][\w-]*)/g))
          if (!/^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(match[1]))
            issues.push("class-case");
      });
    } catch {
      issues.push("syntax");
    }
    return [...new Set(issues)];
  }
  const parsed = ts.createSourceFile(
    file,
    source,
    ts.ScriptTarget.Latest,
    true,
    extension === ".tsx" ? ts.ScriptKind.TSX : ts.ScriptKind.JS,
  );
  if (parsed.parseDiagnostics.length) issues.push("syntax");
  const visit = (node) => {
    if (
      ts.getLeadingCommentRanges(source, node.pos)?.length ||
      ts.getTrailingCommentRanges(source, node.end)?.length
    )
      issues.push("no-comments");
    if (
      (ts.isVariableDeclaration(node) ||
        ts.isFunctionDeclaration(node) ||
        ts.isParameter(node) ||
        ts.isTypeAliasDeclaration(node) ||
        ts.isInterfaceDeclaration(node)) &&
      node.name &&
      ts.isIdentifier(node.name) &&
      !/^[A-Za-z][A-Za-z0-9]*$/.test(node.name.text)
    )
      issues.push("identifier-case");
    if (
      (ts.isCallExpression(node) &&
        ts.isIdentifier(node.expression) &&
        node.expression.text === "eval") ||
      (ts.isNewExpression(node) &&
        ts.isIdentifier(node.expression) &&
        node.expression.text === "Function")
    )
      issues.push("no-dynamic-execution");
    ts.forEachChild(node, visit);
  };
  visit(parsed);
  return [...new Set(issues)];
}

export function lintRoomUi() {
  const results = files.map((file) => ({
    file,
    issues: inspectRoomUiFile(
      file,
      readFileSync(path.join(root, file), "utf8"),
    ),
  }));
  const result = {
    passed: results.every((file) => !file.issues.length),
    scope:
      "Registered shared settlement, leave confirmation, icon tools and related tests; excludes existing main.tsx and global styles",
    languages: Object.keys(languages),
    files: results,
  };
  const out = path.join(root, ".local/room-flow/verification");
  mkdirSync(out, { recursive: true });
  writeFileSync(
    path.join(out, "lint.json"),
    JSON.stringify(result, null, 2) + "\n",
  );
  return result;
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const result = lintRoomUi();
  console.log(JSON.stringify(result));
  process.exitCode = result.passed ? 0 : 1;
}
