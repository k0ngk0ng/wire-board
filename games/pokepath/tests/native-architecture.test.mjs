import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join, extname } from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const game = join(root, "games/pokepath");
function files(directory) {
  if (!existsSync(directory)) return [];
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => entry.isDirectory() ? files(join(directory, entry.name)) : [join(directory, entry.name)]);
}

test("the delivered game uses independent native rules and UI", () => {
  assert.equal(existsSync(join(game, "upstream")), false, "original program must remain research-only");
  const sourceFiles = ["engine", "server", "web", "cmd"].flatMap(name => files(join(game, name))).filter(path => [".go", ".ts", ".tsx", ".js", ".mjs"].includes(extname(path)));
  assert.ok(sourceFiles.some(path => path.endsWith("/Board.tsx")));
  for (const path of sourceFiles) {
    const source = readFileSync(path, "utf8");
    for (const pattern of [/pokepath-original/i, /github\.com\/dop251\/goja/, /new\s+Main\s*\(/, /<iframe\b/i, /createElement\s*\(\s*["']iframe["']/i]) assert.equal(pattern.test(source), false, `${path}: embedded original runtime`);
  }
  assert.equal(readFileSync(join(root, "go.mod"), "utf8").includes("github.com/dop251/goja"), false);
});

test("original browser programs are excluded from public game assets", () => {
  const paths = files(join(game, "assets/public/pokepath/v1"));
  assert.ok(paths.some(path => path.endsWith(".png")), "reference artwork remains available");
  assert.deepEqual(paths.filter(path => [".js", ".html", ".css"].includes(extname(path))), []);
});

test("the live board uses retained prediction handles throughout its frame loop", () => {
  const source = readFileSync(join(game, "web/Board.tsx"), "utf8");
  assert.match(source, /const opaque = native\.createPredictionRuntime\(\)/);
  assert.match(source, /runtime: \{ clone: opaque\.clone, apply: opaque\.apply, advance: opaque\.advance, project: opaque\.frame, release: opaque\.release \}/);
  assert.doesNotMatch(source, /native\.(?:clone|advance|project|apply)\(/);
});
