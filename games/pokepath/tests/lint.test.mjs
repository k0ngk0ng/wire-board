import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
function check(language, path, source) {
  return spawnSync(process.execPath, ["games/pokepath/build/lint.mjs", "--stdin", language, path], { cwd: root, input: source, encoding: "utf8", env: { ...process.env, GOCACHE: `${root}.local/go-build`, GOMODCACHE: `${root}.local/go-mod`, GOTMPDIR: `${root}.local/tmp`, TMPDIR: `${root}.local/tmp`, GOTOOLCHAIN: "local" } });
}

test("lint accepts each supported language in its registered directory", () => {
  for (const [language, path, source] of [
    ["typescript", "games/pokepath/web/Board.tsx", "export const Board = () => <main />;"],
    ["typescript", "games/pokepath/web/map-viewport.d.ts", "export function attachMapViewport(): void;"],
    ["javascript", "games/pokepath/engine/session.mjs", "export const createSession = () => ({});"],
    ["go", "games/pokepath/server/module.go", "package pokepath\nfunc Module() {}\n"],
    ["go", "games/pokepath/tests/platform_e2e_test.go", "package pokepath_test\nfunc TestPlatform() {}\n"],
    ["go", "games/pokepath/cmd/browser/main_js.go", "package main\nfunc main() {}\n"],
    ["javascript", "web/tests/platform-audio-call-volume.test.mjs", "export const gain = 1;"],
    ["css", "games/pokepath/web/board.css", ".pokepath-board { color: #fff; background: #000; }"],
    ["html", "games/pokepath/web/index.html", "<!doctype html><html lang=zh-CN><body></body></html>"],
    ["python", "games/pokepath/tests/native-delivery-e2e.py", "class NativeDeliveryTest:\n    def setUpClass(self):\n        pass\n    def do_GET(self):\n        pass\n"],
    ...["scripts/cdn_delivery.py", "scripts/asset_reuse.py", "scripts/asset_reuse_e2e_test.py"].map(path => ["python", path, "def validate_asset():\n    pass\n"]),
  ]) {
    const result = check(language, path, source);
    assert.equal(result.status, 0, `${path}: ${result.stderr}`);
  }
});

test("lint rejects comments, wrong names, directory escapes and cross-game imports", () => {
  for (const [language, path, source] of [
    ["typescript", "games/pokepath/web/Board.tsx", "// forbidden\nexport const Board = () => null;"],
    ["javascript", "games/pokepath/engine/session.mjs", "/* forbidden */ export const value = 1;"],
    ["go", "games/pokepath/server/module.go", "package pokepath\n// forbidden\nfunc Module() {}\n"],
    ["css", "games/pokepath/web/board.css", "/* forbidden */ .pokepath-board {}"],
    ["html", "games/pokepath/web/index.html", "<!-- forbidden --><main></main>"],
    ["python", "games/pokepath/tests/native-delivery-e2e.py", "# forbidden\nvalue = 1\n"],
    ["python", "games/pokepath/tests/native-delivery-e2e.py", "def badName():\n    pass\n"],
    ["typescript", "games/pokepath/web/wrong_name.ts", "export const value = 1;"],
    ["javascript", "games/pokepath/engine/BadName.mjs", "export const value = 1;"],
    ["go", "games/pokepath/server/BadName.go", "package pokepath\n"],
    ["javascript", "games/pokepath/not-registered/code.mjs", "export const value = 1;"],
    ["javascript", "games/pokepath/upstream/src/js/game/component/Pokemon.js", "export class Pokemon {}"],
    ["css", "games/pokepath/upstream/board.css", ".pokepath-board {}"],
    ["python", "scripts/not_registered.py", "def validate_asset():\n    pass\n"],
    ["go", "internal/server/not_registered.go", "package server\n"],
    ["javascript", "../outside.mjs", "export const value = 1;"],
    ["typescript", "games/pokepath/web/board.ts", "import value from '../../starfort/web/types';"],
    ["go", "games/pokepath/server/module.go", 'package pokepath\nimport _ "github.com/k0ngk0ng/wire-board/games/starfort/server"\n'],
  ]) {
    const result = check(language, path, source);
    assert.notEqual(result.status, 0, path);
  }
});

test("lint rejects runtime evaluation and separate platform audio or storage deletion", () => {
  for (const source of [
    'eval("payload");',
    'const value = new Function("return 1");',
    'const audio = new Audio("sound.wav");',
    'const context = new AudioContext();',
    'localStorage.clear();',
    'window.localStorage.clear();',
  ]) {
    assert.notEqual(check("javascript", "games/pokepath/web/bridge.mjs", source).status, 0, source);
  }
});

test("lint parses string and template contents without treating text as comments", () => {
  const result = check("javascript", "games/pokepath/engine/session.mjs", 'export const values = ["// literal", "/* literal */", `https://example.test/path`];');
  assert.equal(result.status, 0, result.stderr);
});

test("engine math lint rejects architecture-dependent distance selectors and permits oracle tests", () => {
  for (const source of [
    'package engine\nimport "math"\nfunc distance(x,y float64) float64 { return math.Hypot(x,y) }\n',
    'package engine\nimport numeric "math"\nfunc distance(x,y float64) float64 { return numeric.Hypot(x,y) }\n',
    'package engine\nimport numeric "math"\nvar distance = numeric.Hypot\n',
    'package engine\nimport . "math"\nfunc distance(x,y float64) float64 { return Hypot(x,y) }\n',
  ]) assert.notEqual(check("go", "games/pokepath/engine/geometry.go", source).status, 0, source);
  for (const [path, source] of [
    ["games/pokepath/engine/geometry.go", 'package engine\nimport "math"\nfunc root(x float64) float64 { return math.Sqrt(x) }\n'],
    ["games/pokepath/engine/geometry_test.go", 'package engine\nimport "math"\nfunc reference(x,y float64) float64 { return math.Hypot(x,y) }\n'],
    ["games/pokepath/server/module.go", 'package pokepath\nimport "math"\nfunc distance(x,y float64) float64 { return math.Hypot(x,y) }\n'],
  ]) {
    const result = check("go", path, source);
    assert.equal(result.status, 0, result.stderr);
  }
});
