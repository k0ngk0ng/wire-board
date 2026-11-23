import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { inspectRoomUiFile } from "../../../web/scripts/room-ui-lint.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const game = process.argv[2] ?? "basketball";
assert.ok(["basketball", "splendor"].includes(game));
const folder = game === "basketball" ? "engine" : "server";
const expectedPackage = game === "basketball" ? "engine" : "splendor";
const directory = `games/${game}/${folder}`;
const out = path.join(root, ".local", game, game === "basketball" ? "audit" : "responsive");
const temporary = path.join(out, "tmp");
mkdirSync(temporary, { recursive: true });
const env = { ...process.env, GOCACHE: path.join(root, ".local/go-build"), GOMODCACHE: path.join(root, ".local/go-mod"), GOTMPDIR: temporary, TMPDIR: temporary, GOTOOLCHAIN: "local", GOTELEMETRY: "off" };
for (const cache of [env.GOCACHE, env.GOMODCACHE, env.GOTMPDIR, env.TMPDIR]) mkdirSync(cache, { recursive: true });
const git = (...args) => execFileSync("git", args, { cwd: root, encoding: "utf8" });
const changed = new Map();
let current;
let lineNumber = 0;
let patch = git("diff", "HEAD", "--unified=0", "--", directory);
const untracked = git("ls-files", "--others", "--exclude-standard", directory).trim().split("\n").filter(Boolean);
if (!patch && !untracked.length) patch = git("diff", "HEAD^", "HEAD", "--unified=0", "--", directory);
for (const line of patch.split("\n")) {
  if (line.startsWith("+++ b/")) { current = line.slice(6); changed.set(current, []); }
  else if (line.startsWith("@@")) lineNumber = Number(/\+(\d+)/.exec(line)[1]);
  else if (current && line.startsWith("+")) changed.get(current).push(lineNumber++);
}
const names = [...new Set([...changed.keys(), ...untracked])].filter(file => file.endsWith(".go"));
const inputs = names.map(file => {
  assert.match(file, new RegExp(`^${directory}/[a-z][a-z0-9_]*\\.go$`));
  return { file, source: readFileSync(path.join(root, file), "utf8"), expectedPackage, changedLines: untracked.includes(file) ? undefined : changed.get(file), strictIntegers: true };
});
const examples = [
  { source: "package engine\nvar frameCount = 1", issue: undefined },
  { source: "package engine\nvar frame_count = 1", issue: "camel-case-binding" },
  { source: "package engine\n// invalid\nvar count = 1", issue: "no-comments" },
  { source: "package other\nvar count = 1", issue: "package-directory" },
  { source: "package engine\nvar count = 1.5", issue: "integer-business-number" },
  { source: 'package engine\nimport "github.com/k0ngk0ng/wire-board/games/basketball/campaign"', issue: undefined },
  { source: 'package engine\nimport "github.com/k0ngk0ng/wire-board/games/splendor/server"', issue: "no-cross-game-import" },
];
const results = JSON.parse(execFileSync("go", ["run", "games/cs16/build/lint_go.go"], { cwd: root, env, input: JSON.stringify([...examples.map(example => ({ file: "games/basketball/engine/example.go", source: example.source, expectedPackage: "engine", strictIntegers: true })), ...inputs]), encoding: "utf8" }));
for (const [index, example] of examples.entries()) example.issue ? assert.ok(results[index].includes(example.issue)) : assert.deepEqual(results[index], []);
const self = "games/basketball/build/lint.mjs";
const files = inputs.map((input, index) => ({ file: input.file, coverage: input.changedLines ? "added-lines" : "full-source", sha256: createHash("sha256").update(input.source).digest("hex"), issues: results[index + examples.length] }));
files.push({ file: self, coverage: "full-source", issues: inspectRoomUiFile(self, readFileSync(path.join(root, self), "utf8")) });
const report = { passed: files.every(file => file.issues.length === 0), scope: `Changed ${game} ${folder} Go lines, complete new Go tests and this JavaScript entry; clean checkouts inspect the latest commit; shared Go AST only, no game rules`, examples: examples.length, files };
writeFileSync(path.join(out, game === "basketball" ? "lint.json" : "go-lint.json"), JSON.stringify(report, null, 2) + "\n");
console.log(JSON.stringify(report));
process.exitCode = report.passed ? 0 : 1;
