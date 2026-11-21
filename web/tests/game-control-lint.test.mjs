import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { inspectGameControlFile, inspectGameControlContract, lintGameControls } from "../scripts/game-control-lint.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));

test("game-control lint enforces language names, directories and forbidden constructs", () => {
  assert.deepEqual(inspectGameControlFile("web/src/ui/game-control-guard.ts", "export const gameControls = true;"), []);
  assert.deepEqual(inspectGameControlFile("web/scripts/game-control-lint.json", '{"scope":"guard"}'), []);
  for (const [file, source, issue] of [
    ["web/src/ui/bad_name.ts", "const validName = true;", "filename"],
    ["web/elsewhere/guard.ts", "const validName = true;", "directory"],
    ["web/src/ui/game-control-guard.ts", "const bad_name = true;", "camel-case"],
    ["web/src/ui/game-control-guard.ts", "// forbidden\nconst validName = true;", "no-comments"],
    ["web/src/ui/game-control-guard.ts", "eval('1')", "no-dynamic-execution"],
    ["web/src/ui/game-control-guard.ts", "document.addEventListener('touchstart', event => event.preventDefault())", "no-global-input-block"],
    ["web/src/ui/game-control-guard.css", ".bad_name { color: red }", "kebab-case"],
    ["web/src/ui/game-control-guard.css", "/* forbidden */ .valid-name {}", "no-comments"],
    ["web/scripts/game-control-lint.json", "{", "syntax"],
  ]) assert.ok(inspectGameControlFile(file, source).includes(issue), file + ": " + issue);
});

test("shared boundary and WebKit protection cannot silently disappear", () => {
  const files = ["web/src/ui/room-fullscreen.tsx", "web/src/ui/game-control-guard.ts", "web/src/ui/game-control-guard.css", "web/src/main.tsx"];
  const sources = Object.fromEntries(files.map(file => [file, readFileSync(path.join(root, file), "utf8")]));
  assert.deepEqual(inspectGameControlContract(sources), []);
  const changed = (file, before, after) => ({ ...sources, [file]: sources[file].replace(before, after) });
  assert.ok(inspectGameControlContract(changed(files[0], 'className="game-fullscreen-board"\n          data-game-controls', 'className="game-fullscreen-board"')).includes("board-protection"));
  assert.ok(inspectGameControlContract(changed(files[0], "return protectGameControls(root)", "return undefined")).includes("guard-lifecycle"));
  assert.ok(inspectGameControlContract(changed(files[2], "-webkit-user-select: none;", "")).includes("webkit-selection"));
  assert.ok(inspectGameControlContract(changed(files[2], "-webkit-touch-callout: none;", "")).includes("webkit-callout"));
  assert.ok(inspectGameControlContract(changed(files[1], 'root.removeEventListener("contextmenu", preventNative, true);', "")).includes("listener-cleanup"));
});

test("all registered game-control files pass the enforced policy", () => {
  const report = lintGameControls(root);
  assert.equal(report.passed, true, JSON.stringify(report));
  assert.ok(report.files.length >= 6);
});
