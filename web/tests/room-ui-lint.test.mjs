import test from "node:test";
import assert from "node:assert/strict";
import {
  inspectRoomUiFile,
  inspectRoomToolsContract,
  inspectGameFocusTargets,
  lintRoomUi,
} from "../scripts/room-ui-lint.mjs";

test("mobile fullscreen evidence stays in the declared game workspace", () => {
  assert.deepEqual(inspectRoomUiFile("web/tests/board-fullscreen-browser.test.mjs", 'const output=".local/xiangqi/fullscreen";'), []);
  assert.ok(inspectRoomUiFile("web/tests/board-fullscreen-browser.test.mjs", 'const output=".local/room-tools";').includes("workspace-output-directory"));
  const file = "web/tests/mobile-fullscreen-browser.test.mjs";
  assert.deepEqual(inspectRoomUiFile(file, 'const output = ".local/yuyuhakusho/mobile-fullscreen";'), []);
  for (const source of ['const output = ".local/fullscreen";', 'const output = "/tmp/fullscreen";', 'const output = tmpdir();']) {
    assert.ok(inspectRoomUiFile(file, source).includes("workspace-output-directory"));
  }
});

test('fullscreen board layout sources and JSON fixtures have explicit lint scope', () => {
  assert.deepEqual(inspectRoomUiFile('games/chess/web/style.css', '.chess-board { height: 100%; }'), []);
  assert.deepEqual(inspectRoomUiFile('web/tests/fixtures/board-fullscreen.json', '{"cases":[]}'), []);
  assert.ok(inspectRoomUiFile('web/tests/fixtures/board-fullscreen.json', '{/* no */"cases":[]}').includes('syntax'));
  assert.ok(inspectRoomUiFile('games/other/web/style.css', '.board {}').includes('directory'));
});

test("room tools contract rejects incomplete icon conversion", () => {
  assert.deepEqual(inspectRoomUiFile('web/tests/ControllerOverlayFixture.tsx', 'export function ControllerOverlayFixture(){return <div/>;}'), []);
  assert.deepEqual(
    inspectRoomToolsContract(
      '<div className="room-tools"><RoomToolButton label="日志"/><SaveGameControl iconOnly/></div>',
    ),
    [],
  );
  assert.ok(
    inspectRoomToolsContract(
      '<div className="room-tools"><button>日志</button></div>',
    ).includes("icon-only-room-actions"),
  );
  assert.ok(
    inspectRoomToolsContract(
      '<div className="room-tools"><SaveGameControl/></div>',
    ).includes("icon-only-save-actions"),
  );
  assert.ok(
    inspectRoomToolsContract(
      "<GameFullscreen toolbarActions={<button>取消托管</button>}/>",
    ).includes("icon-only-room-actions"),
  );
  assert.ok(
    inspectRoomToolsContract(
      "<GameFullscreen toolbarActions={<SaveGameControl/>}/>",
    ).includes("icon-only-save-actions"),
  );
});

test("FC and MD game roots keep their shared keyboard focus markers", () => {
  const files = [
    "games/contra/web/runtime-board.tsx",
    "games/basketball/web/Board.tsx",
    "games/yuyuhakusho/web/Board.tsx",
  ];
  const sources = Object.fromEntries(
    files.map((file) => [
      file,
      "<div ref={root} tabIndex={0} data-game-focus/>",
    ]),
  );
  assert.deepEqual(inspectGameFocusTargets(sources), []);
  delete sources[files[1]];
  assert.ok(
    inspectGameFocusTargets(sources).includes("game-focus-target:" + files[1]),
  );
});

test("room UI lint accepts component, script and style conventions", () => {
  for (const [file, source] of [
    [
      "web/src/ui/example.tsx",
      "export function Example({label}:{label:string}){return <button aria-label={label}/>;}",
    ],
    ["web/tests/example.test.mjs", "const buttonLabel = '全屏';"],
    [
      "web/src/ui/example.css",
      ".room-tool-button { color: #fff; background: #123; }",
    ],
  ])
    assert.deepEqual(inspectRoomUiFile(file, source), [], file);
});

test("room UI lint rejects comments, invalid paths and names, and dynamic execution", () => {
  for (const [file, source, issue] of [
    [
      "web/src/ui/example.tsx",
      "// hidden behavior\nconst label = '全屏';",
      "no-comments",
    ],
    [
      "web/src/ui/example.css",
      "/* hidden behavior */ .button {color:#fff}",
      "no-comments",
    ],
    [
      "web/src/ui/example.tsx",
      "const button_label = '全屏';",
      "identifier-case",
    ],
    ["web/src/ui/example.css", ".button_label {color:#fff}", "class-case"],
    ["web/src/ui/Example.tsx", "const label = '全屏';", "filename"],
    ["web/src/ui/../example.tsx", "const label = '全屏';", "directory"],
    ["web/other/example.mjs", "const label = '全屏';", "directory"],
    ["web/src/ui/example.tsx", "eval('click()');", "no-dynamic-execution"],
    [
      "web/tests/example.test.mjs",
      "new Function('click()');",
      "no-dynamic-execution",
    ],
  ])
    assert.ok(
      inspectRoomUiFile(file, source).includes(issue),
      `${file}: ${issue}`,
    );
});

test("registered room UI files pass their scoped lint", () => {
  const result = lintRoomUi();
  assert.equal(result.passed, true, JSON.stringify(result));
});

test("Splendor fullscreen files and evidence follow registered conventions", () => {
  assert.deepEqual(inspectRoomUiFile("games/splendor/web/splendor-fullscreen-players.tsx", "export function Players(){return <div/>;}"), []);
  assert.deepEqual(inspectRoomUiFile("games/splendor/web/splendor-fullscreen.css", ".splendor-table { display: grid; }"), []);
  assert.deepEqual(inspectRoomUiFile("games/splendor/web/splendor-layout.css", ".splendor-board { display: grid; }"), []);
  assert.ok(lintRoomUi().files.some(file => file.file === "games/splendor/web/splendor-layout.css"));
  assert.ok(lintRoomUi().files.some(file => file.file === "games/fortressblue/web/style.css"));
  assert.ok(lintRoomUi().files.some(file => file.file === "games/racing/web/style.css"));
  assert.ok(inspectRoomUiFile("games/splendor/web/Bad_name.tsx", "const value=1;").includes("filename"));
  assert.ok(inspectRoomUiFile("games/splendor/web/splendor-fullscreen.css", "/* no */ .splendor-table{}").includes("no-comments"));
  assert.ok(inspectRoomUiFile("web/tests/tabletop-room-browser.mjs", 'const output=".local/unrelated";').includes("workspace-output-directory"));
});
