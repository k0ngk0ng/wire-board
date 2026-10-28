// Failure inventory before human-only delivery: computer/auto-play controls
// remain visible; solo capacity disappears or cannot start; two humans cannot
// start; a one-seat room still asks for friends; no-bot metadata says computers
// auto-ready. Exercise real common templates and the actual autoplay gate.
import test from "node:test";
import assert from "node:assert/strict";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";
import { createRequire } from "node:module";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const out = path.join(root, ".local/contra-frontend-human");
mkdirSync(out, { recursive: true });
const main = readFileSync(path.join(root, "web/src/main.tsx"), "utf8");
const gate = main.match(/const canAutoPlay =([\s\S]*?);\n  const boardBusy/);
const button = main.match(
  /\{canAutoPlay && \(\s*(<button[\s\S]*?<\/button>)\s*\)\}/,
);
assert.ok(gate && button, "common autoplay control must have an explicit gate");
const built = await build({
  stdin: {
    contents: `
import React from "react";
import {renderToStaticMarkup} from "react-dom/server";
import {Hand,Bot} from "lucide-react";
import catalog from "./games/contra/web/catalog.tsx";
import {ModuleCreate,ModuleWaiting} from "./web/src/game-api/host.tsx";
export const metadata=catalog;
export function render(mode,room,capability=catalog.supportsBots){
  globalThis.__contraCapabilityFixture={...catalog,supportsBots:capability,Board:()=>null};
  if(mode==="playing"){
    const state={room},busy=false,autoPlay=!!room.seats[room.you]?.autoPlay;
    const isIndependentGame=()=>true,gameModule=()=>globalThis.__contraCapabilityFixture,roomCommand=()=>{};
    const canAutoPlay=${gate[1]};
    return renderToStaticMarkup(<div>{canAutoPlay&&(${button[1]})}</div>);
  }
  const props=mode==="waiting"?{room,busy:false,host:true,command:()=>{}}:
    {kind:"contra",busy:false,onSubmit:()=>{},draft:{name:"人类测试",password:""},onDraftChange:()=>{}};
  return renderToStaticMarkup(React.createElement(mode==="waiting"?ModuleWaiting:ModuleCreate,props));
}
`,
    resolveDir: root,
    sourcefile: "contra-human-mode-fixture.tsx",
    loader: "tsx",
  },
  nodePaths: [path.join(root, "web/node_modules")],
  bundle: true,
  platform: "node",
  format: "cjs",
  jsx: "automatic",
  write: false,
  loader: { ".css": "empty" },
  plugins: [
    {
      name: "isolated-contra-registry",
      setup(builder) {
        builder.onResolve({ filter: /^\.\.\/profiles$/ }, () => ({
          path: "profiles",
          namespace: "contra-human-profile",
        }));
        builder.onLoad(
          { filter: /.*/, namespace: "contra-human-profile" },
          () => ({
            contents: "export const PlayerName=({user})=>user.name;",
            loader: "js",
          }),
        );
        builder.onResolve({ filter: /^\.\/registry$/ }, (args) =>
          args.importer.endsWith("/game-api/host.tsx")
            ? { path: "registry", namespace: "contra-human-registry" }
            : null,
        );
        builder.onLoad(
          { filter: /.*/, namespace: "contra-human-registry" },
          () => ({
            contents:
              'export const gameModule=()=>globalThis.__contraCapabilityFixture; export const gameName=()=>"魂斗罗";',
            loader: "js",
          }),
        );
      },
    },
  ],
});
const compiled = path.join(out, "fixture.cjs");
writeFileSync(compiled, built.outputFiles[0].contents);
const { metadata, render } = createRequire(import.meta.url)(compiled);
const records = [];
function room(capacity, occupied = capacity, status = "waiting") {
  return {
    id: "contra-human",
    kind: "contra",
    name: "人类测试",
    capacity,
    status,
    version: 1,
    host: "h0",
    you: 0,
    game: status === "playing" ? { data: {}, eliminated: [] } : undefined,
    seats: Array.from({ length: occupied }, (_, n) => ({
      id: `h${n}`,
      name: `玩家${n}`,
      ready: true,
    })),
  };
}
function buttons(html) {
  return [...html.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)].map(
    (m) => ({ attributes: m[1], text: m[2].replace(/<[^>]*>/g, "") }),
  );
}

test("human catalog keeps one/two seats and no computer capability", () => {
  assert.equal(metadata.supportsBots, false);
  assert.equal(metadata.minPlayers, 1);
  assert.equal(metadata.maxPlayers, 2);
  assert.deepEqual(metadata.playerCounts, [1, 2]);
  assert.equal(metadata.requiresFullTable, true);
});

test("common create keeps solo and co-op options without computer invitation", () => {
  const html = render("create", room(1));
  assert.match(html, /<option value="1"[^>]*>1 人<\/option>/);
  assert.match(html, /<option value="2"[^>]*>2 人<\/option>/);
  assert.doesNotMatch(html, /添加电脑玩家体验/);
  assert.doesNotMatch(html, /电脑|托管/);
  records.push({ name: "create", html });
});

test("waiting allows ready human solo/co-op starts and offers no add-computer control", () => {
  for (const capacity of [1, 2]) {
    const html = render("waiting", room(capacity));
    const start = buttons(html).find((b) => b.text.includes("开始游戏"));
    assert.ok(start);
    assert.doesNotMatch(start.attributes, /\bdisabled\b/);
    assert.ok(buttons(html).every((b) => !/添加电脑|托管/.test(b.text)));
    assert.doesNotMatch(html, /电脑自动准备/);
    assert.doesNotMatch(html, /电脑|托管/);
    if (capacity === 1) assert.doesNotMatch(html, /好友到齐/);
    records.push({ name: `waiting-${capacity}`, html });
  }
  const incomplete = render("waiting", room(2, 1));
  assert.ok(buttons(incomplete).every((b) => !/添加电脑/.test(b.text)));
  assert.match(
    buttons(incomplete).find((b) => b.text.includes("开始游戏")).attributes,
    /\bdisabled\b/,
  );
  assert.ok(
    buttons(render("waiting", room(2, 1), true)).some((b) =>
      /添加电脑/.test(b.text),
    ),
    "positive capability control must remain available for supported modules",
  );
});

test("actual common playing control gate hides both enable and cancel autoplay", () => {
  for (const autoPlay of [false, true]) {
    const state = room(1, 1, "playing");
    state.seats[0].autoPlay = autoPlay;
    const html = render("playing", state);
    assert.equal(buttons(html).length, 0);
    assert.ok(
      buttons(render("playing", state, true)).some((b) => /托管/.test(b.text)),
    );
    records.push({ name: `playing-autoplay-${autoPlay}`, html });
  }
});

test.after(() =>
  writeFileSync(
    path.join(out, "render-evidence.json"),
    JSON.stringify(
      {
        scope:
          "Real React SSR common create/waiting plus extracted actual playing control gate; no gameplay simulated",
        records,
      },
      null,
      2,
    ) + "\n",
  ),
);
