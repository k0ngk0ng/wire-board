// Failure inventory before implementation: mobile advice missing in one game,
// duplicated in a room, inherited by unrelated games, or inserted before the
// board and breaking fullscreen measurement. Browser checks cover visibility,
// contrast, fullscreen containment and unchanged controller geometry.
import test from "node:test";
import assert from "node:assert/strict";
import { build } from "esbuild";
import { createRequire } from "node:module";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const out = path.join(root, ".local/mobile-play-notice");
mkdirSync(out, { recursive: true });
const text = "建议使用电脑游玩。手机可能出现卡顿，组合按键需要多指操作。";
const built = await build({
  stdin: {
    contents: `
import React from "react";
import {renderToStaticMarkup} from "react-dom/server";
import basketball from "./games/basketball/web/catalog.tsx";
import contra from "./games/contra/web/catalog.tsx";
import {GameBoard,ModuleCreate,ModuleWaiting} from "./web/src/game-api/host.tsx";
export const catalogs={basketball,contra};
export function render(kind,mode,enabled){
 const catalog=catalogs[kind];
 globalThis.__mobileNoticeModule={...catalog,...(enabled===undefined?{}:{mobilePlayNotice:enabled}),Board:()=>React.createElement("section",{"data-test-board":"true"},"board")};
 const room={id:"notice",kind,name:"提示验收",capacity:1,status:mode==="playing"?"playing":"waiting",version:1,host:"h",you:0,seats:[{id:"h",name:"玩家",ready:true}],game:{data:{}}};
 const props=mode==="waiting"?{room,busy:false,host:true,command:()=>{}}:
  mode==="create"?{kind,busy:false,onSubmit:()=>{},draft:{name:"提示验收",password:""},onDraftChange:()=>{}}:
  {room,busy:false,assets:"",sound:false,audio:{},clock:{},act:()=>{},command:()=>{}};
 return renderToStaticMarkup(React.createElement(mode==="waiting"?ModuleWaiting:mode==="create"?ModuleCreate:GameBoard,props));
}
`,
    sourcefile: "mobile-play-notice-ssr.tsx",
    resolveDir: root,
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
      name: "mobile-notice-registry",
      setup(builder) {
        builder.onResolve({ filter: /^\.\.\/profiles$/ }, () => ({
          path: "profiles",
          namespace: "notice-profile",
        }));
        builder.onLoad({ filter: /.*/, namespace: "notice-profile" }, () => ({
          contents: "export const PlayerName=({user})=>user.name;",
          loader: "js",
        }));
        builder.onResolve({ filter: /^\.\/registry$/ }, (args) =>
          args.importer.endsWith("/game-api/host.tsx")
            ? { path: "registry", namespace: "notice-registry" }
            : null,
        );
        builder.onLoad({ filter: /.*/, namespace: "notice-registry" }, () => ({
          contents:
            "export const gameModule=()=>globalThis.__mobileNoticeModule;export const gameName=()=>globalThis.__mobileNoticeModule.name;",
          loader: "js",
        }));
      },
    },
  ],
});
const compiled = path.join(out, "ssr.cjs");
writeFileSync(compiled, built.outputFiles[0].contents);
const { catalogs, render } = createRequire(import.meta.url)(compiled);
const records = [];
for (const kind of ["basketball", "contra"]) {
  test(`${kind} opts into one shared waiting/playing notice`, () => {
    assert.equal(catalogs[kind].mobilePlayNotice, true);
    for (const mode of ["waiting", "playing"]) {
      const html = render(kind, mode);
      assert.equal(html.split(text).length - 1, 1);
      assert.equal((html.match(/data-mobile-play-notice/g) ?? []).length, 1);
      assert.doesNotMatch(html, /不支持多点/);
      if (mode === "playing")
        assert.match(html, /^<section data-test-board="true">/);
      records.push({ kind, mode, html });
    }
    assert.equal(render(kind, "create").split(text).length - 1, 0);
  });
}
test("modules without the capability keep their current surfaces", () => {
  for (const mode of ["waiting", "playing"])
    assert.doesNotMatch(
      render("contra", mode, false),
      /data-mobile-play-notice/,
    );
});
test.after(() =>
  writeFileSync(
    path.join(out, "ssr-evidence.json"),
    JSON.stringify(
      {
        scope:
          "Real common templates with catalog metadata; board contents are a non-gameplay fixture",
        records,
      },
      null,
      2,
    ) + "\n",
  ),
);
