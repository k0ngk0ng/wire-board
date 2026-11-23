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
  "web/src/ui/room-fullscreen.tsx",
  "web/src/ui/game-mobile-play-notice.tsx",
  "web/src/ui/room-fullscreen.css",
  "web/src/ui/fullscreen-overlay.ts",
  "web/src/ui/map-controls.ts",
  "web/src/ui/create-room.tsx",
  "web/src/ui/room-settings.tsx",
  "web/src/ui/room-command-queue.ts",
  "web/tests/room-command-queue.test.mjs",
  "web/src/ui/game-choices.tsx",
  "web/src/ui/result-overlay.tsx",
  "web/src/ui/results.tsx",
  "web/src/ui/leave-room-control.tsx",
  "web/src/ui/room-tool-button.tsx",
  "web/src/ui/saved-games.tsx",
  "web/src/ui/game-focus.ts",
  "web/src/ui/chat-position.ts",
  "web/src/ui/chat-anchor.ts",
  "web/src/ui/room-waiting.css",
  "web/src/ui/room-actions.css",
  "web/scripts/room-ui-lint.mjs",
  "web/tests/room-ui-lint.test.mjs",
  "web/tests/room-flow-browser.test.mjs",
  "web/tests/room-tools.test.mjs",
  "web/tests/controller-focus-browser.test.mjs",
  "web/tests/controller-overlay-browser.mjs",
  "web/tests/ControllerOverlayFixture.tsx",
  "web/tests/BoardFullscreenFixture.tsx",
  "web/tests/board-fullscreen-browser.test.mjs",
  "web/tests/fullscreen-overlay.test.mjs",
  "web/tests/mobile-fullscreen-browser.test.mjs",
  "web/tests/fixtures/board-fullscreen.json",
  "games/gomoku/web/Board.tsx",
  "games/gomoku/web/module.tsx",
  "games/gomoku/web/style.css",
  "games/weiqi/web/style.css",
  "games/chess/web/style.css",
  "games/xiangqi/web/style.css",
  "games/junqi/web/style.css",
  "games/contra/web/human-mode.test.mjs",
  "games/splendor/web/board.tsx",
  "games/splendor/web/splendor-fullscreen-players.tsx",
  "games/splendor/web/splendor-fullscreen.css",
  "games/splendor/web/splendor-layout.css",
  "games/splendor/web/splendor-cities.tsx",
  "games/splendor/web/splendor-rules.tsx",
  "games/fortressblue/web/style.css",
  "games/racing/web/style.css",
  "games/snooker/web/module.tsx",
  "games/yuyuhakusho/web/network.css",
  "games/basketball/build/lint.mjs",
  "web/src/ui/solo-rematch.ts",
  "web/tests/solo-rematch.test.mjs",
  "web/tests/solo-rematch-browser.mjs",
  "web/tests/mobile-table-viewport.test.mjs",
  "web/tests/tabletop-room-browser.mjs",
];
const languages = {
  ".ts": /^web\/src\/ui\//,
  ".tsx": /^(?:web\/(?:src\/ui|tests)\/|games\/(?:gomoku|splendor|snooker)\/web\/)/,
  ".css": /^(?:web\/src\/ui\/|games\/(?:gomoku|weiqi|chess|xiangqi|junqi|splendor|racing|fortressblue|yuyuhakusho)\/web\/)/,
  ".mjs": /^(?:web\/(?:scripts|tests)\/|games\/(?:contra\/web|basketball\/build)\/)/,
  ".json": /^web\/tests\/fixtures\//,
};

export function inspectRoomUiFile(file, source) {
  const issues = [];
  const extension = path.extname(file);
  if (!languages[extension]?.test(file) || file.split("/").includes(".."))
    issues.push("directory");
  if (
    !/^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*\.(ts|tsx|css|mjs|json)$/.test(
      path.basename(file),
    ) && !((file.startsWith('web/tests/') || file.startsWith('games/gomoku/web/')) && /^[A-Z][A-Za-z0-9]*\.tsx$/.test(path.basename(file)))
  )
    issues.push("filename");
  if (extension === '.json') {
    try { JSON.parse(source); } catch { issues.push('syntax'); }
    return [...new Set(issues)];
  }
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
    extension === ".tsx"
      ? ts.ScriptKind.TSX
      : extension === ".ts"
        ? ts.ScriptKind.TS
        : ts.ScriptKind.JS,
  );
  if (parsed.parseDiagnostics.length) issues.push("syntax");
  const visit = (node) => {
    if (file === "web/tests/mobile-fullscreen-browser.test.mjs" || file === "web/tests/tabletop-room-browser.mjs" || file === "web/tests/board-fullscreen-browser.test.mjs") {
      if (ts.isStringLiteral(node) && ((node.text.includes(".local/") && !node.text.startsWith(file === "web/tests/tabletop-room-browser.mjs" ? ".local/splendor/" : file === "web/tests/board-fullscreen-browser.test.mjs" ? ".local/xiangqi/" : ".local/yuyuhakusho/")) || /^(?:\/tmp\/|\/private\/tmp\/|~\/)/.test(node.text))) issues.push("workspace-output-directory");
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && ["tmpdir", "homedir"].includes(node.expression.text)) issues.push("workspace-output-directory");
    }
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

export function inspectRoomToolsContract(source) {
  const issues = [];
  const parsed = ts.createSourceFile(
    "room.tsx",
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const inspect = (node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const name = node.tagName.getText(parsed);
      if (name === "button") issues.push("icon-only-room-actions");
      if (
        name === "SaveGameControl" &&
        !node.attributes.properties.some(
          (attribute) =>
            ts.isJsxAttribute(attribute) &&
            attribute.name.getText(parsed) === "iconOnly" &&
            (!attribute.initializer ||
              (ts.isJsxExpression(attribute.initializer) &&
                attribute.initializer.expression?.kind ===
                  ts.SyntaxKind.TrueKeyword)),
        )
      )
        issues.push("icon-only-save-actions");
    }
    ts.forEachChild(node, inspect);
  };
  const visit = (node) => {
    if (
      ts.isJsxAttribute(node) &&
      node.name.getText(parsed) === "toolbarActions" &&
      node.initializer
    )
      inspect(node.initializer);
    if (
      ts.isJsxElement(node) &&
      node.openingElement.attributes.properties.some(
        (attribute) =>
          ts.isJsxAttribute(attribute) &&
          attribute.name.getText(parsed) === "className" &&
          attribute.initializer &&
          ts.isStringLiteral(attribute.initializer) &&
          attribute.initializer.text === "room-tools",
      )
    )
      inspect(node);
    else ts.forEachChild(node, visit);
  };
  visit(parsed);
  return [...new Set(issues)];
}

export function inspectGameFocusTargets(sources) {
  const issues = [];
  for (const file of [
    "games/contra/web/runtime-board.tsx",
    "games/basketball/web/Board.tsx",
    "games/yuyuhakusho/web/Board.tsx",
  ]) {
    const parsed = ts.createSourceFile(
      file,
      sources[file] ?? "",
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.TSX,
    );
    let marked = false;
    const visit = (node) => {
      if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
        const names = node.attributes.properties
          .filter(ts.isJsxAttribute)
          .map((attribute) => attribute.name.getText(parsed));
        if (
          ["ref", "tabIndex", "data-game-focus"].every((name) =>
            names.includes(name),
          )
        )
          marked = true;
      }
      ts.forEachChild(node, visit);
    };
    visit(parsed);
    if (!marked) issues.push("game-focus-target:" + file);
  }
  return issues;
}

export function lintRoomUi() {
  const results = files.map((file) => ({
    file,
    issues: inspectRoomUiFile(
      file,
      readFileSync(path.join(root, file), "utf8"),
    ),
  }));
  const contractFiles = [
    "web/src/main.tsx",
    "games/contra/web/runtime-board.tsx",
    "games/basketball/web/Board.tsx",
    "games/yuyuhakusho/web/Board.tsx",
  ];
  const sources = Object.fromEntries(
    contractFiles.map((file) => [
      file,
      readFileSync(path.join(root, file), "utf8"),
    ]),
  );
  const contract = [
    ...inspectRoomToolsContract(sources["web/src/main.tsx"]),
    ...inspectGameFocusTargets(sources),
  ];
  const result = {
    passed: results.every((file) => !file.issues.length) && !contract.length,
    scope:
      "Registered shared settlement, save controls, focus recovery, icon tools, compact waiting CSS, touch chat avoidance and controller UI fixtures; main room-actions and FC/MD focus markers checked as contracts",
    languages: Object.keys(languages),
    files: results,
    contractFiles,
    contract,
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
