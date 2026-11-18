import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { createServer } from "node:http";
import { execFile, execFileSync } from "node:child_process";
import { promisify } from "node:util";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = new URL("../../../.local/pokepath/native-editor-ui/", import.meta.url);
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const React = require("react");
const { renderToStaticMarkup } = require("react-dom/server");
const { build } = require("esbuild");
const results = [];
const execute = promisify(execFile);
await mkdir(output, { recursive: true });
const catalog = {
  version: "editor-ui-fixture-v1", species: {}, enemies: { rattata: { name: "小拉达", sprite: {} }, pidgey: { name: "波波", sprite: {} }, ditto: { name: "百变怪", sprite: {} } },
  routes: { 0: { id: 0, name: "起始小径", width: 720, height: 624, unlockStars: 0, background: "" }, 1: { id: 1, name: "林间小径", width: 720, height: 624, unlockStars: 5, background: "" }, 20: { id: 20, name: "秘密洞窟", width: 720, height: 624, unlockStars: 0, isSecret: true, background: "" } },
  items: {}, abilities: {}, passives: {}, achievements: [], weather: {}, projectiles: {}, eggPool: [], collection: [],
};
const view = { rulesVersion: "native-editor-ui-fixture", epoch: 1, sequence: 0, clock: 0, gold: 50, health: 14, stars: 3, ribbons: 0, route: 0, wave: 1, waveActive: false, paused: false, speed: 0.8, team: [{ id: 17, species: "charmander", level: 1, target: "first", item: "", tileID: -1, stats: {} }], box: [], items: [], tiles: [], towers: [], enemies: [], projectiles: [], events: [], random: 1, progression: { custom: false, secrets: {}, challenge: { active: false }, draft: { active: false } } };
const valid = { mapId: 0, hearts: 14, health: 0, armor: 0, wave: [{ enemy: "rattata", amount: 3 }, { enemy: null, amount: 1 }] };
let module;
async function loadEditor() {
  if (module) return module;
  const file = new URL("editor-node.mjs", output);
  await build({ entryPoints: [fileURLToPath(new URL("../web/Editor.tsx", import.meta.url))], outfile: fileURLToPath(file), bundle: true, platform: "node", format: "esm", target: "es2022", jsx: "automatic", loader: { ".css": "empty" }, nodePaths: [`${root}web/node_modules`], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  module = await import(file.href);
  return module;
}

function props(extra = {}) { return { view, catalog, onAct() {}, disabled: false, onClose() {}, ...extra }; }

test("editor renders native controls and only unlocked route choices", async () => {
  const { Editor } = await loadEditor();
  const html = renderToStaticMarkup(React.createElement(Editor, props()));
  for (const label of ["波次编辑", "路线", "防线生命", "生命强化", "护甲强化", "加入敌人", "加入间隔", "波次预览", "开始实践", "分享码"]) assert.ok(html.includes(label), label);
  assert.ok(html.includes("<select"));
  assert.ok(html.includes('type="number"'));
  assert.ok(!html.includes("<iframe"));
  assert.ok(!html.includes("林间小径"));
  assert.ok(!html.includes("秘密洞窟"));
  assert.ok(!html.includes('type="hidden"'));
  results.push({ name: "native-controls-and-unlocks", passed: true });
});

test("editor codes match Go fields and retain interval slots without internal return metadata", async () => {
  const { validateEditorData, encodeEditorCode, decodeEditorCode } = await loadEditor();
  assert.deepEqual(validateEditorData(valid, view, catalog), valid);
  const code = encodeEditorCode(valid);
  assert.deepEqual(JSON.parse(Buffer.from(code, "base64").toString("utf8")), valid);
  assert.deepEqual(decodeEditorCode(code, view, catalog), valid);
  assert.deepEqual(decodeEditorCode(` \n${code}\n `, view, catalog), valid);
  assert.deepEqual(decodeEditorCode(code.match(/.{1,40}/g).join("\r\n"), view, catalog), valid);
  assert.throws(() => decodeEditorCode(Buffer.from(JSON.stringify({ ...valid, returnRoute: 9 })).toString("base64"), view, catalog));
  results.push({ name: "native-code-roundtrip", passed: true });
});

test("editor rejects malformed, locked, empty and oversized waves before sending actions", async () => {
  const { validateEditorData, decodeEditorCode } = await loadEditor();
  for (const changed of [
    { hearts: 0 }, { hearts: 15 }, { health: -5 }, { health: 201 }, { health: 3 }, { armor: 4 }, { armor: 205 }, { mapId: 1 }, { mapId: 20 }, { mapId: 999 },
    { wave: [] }, { wave: [{ enemy: null, amount: 3 }] }, { wave: [{ enemy: "unknown", amount: 1 }] }, { wave: [{ enemy: "rattata", amount: 0 }] }, { wave: [{ enemy: "rattata", amount: 1.5 }] },
    { wave: [{ enemy: "rattata", amount: 4096 }, { enemy: null, amount: 1 }] }, { wave: Array.from({ length: 257 }, () => ({ enemy: "rattata", amount: 1 })) },
  ]) assert.throws(() => validateEditorData({ ...valid, ...changed }, view, catalog), JSON.stringify(changed).slice(0, 100));
  for (const code of ["", "not-a-code", "Zg=A", "A".repeat(32769), Buffer.from("{}").toString("base64"), Buffer.from(JSON.stringify({ ...valid, wave: [{ enemy: "rattata", amount: "3" }] })).toString("base64")]) assert.throws(() => decodeEditorCode(code, view, catalog));
  assert.equal(validateEditorData({ ...valid, wave: [{ enemy: "rattata", amount: 4096 }] }, view, catalog).wave[0].amount, 4096);
  assert.equal(validateEditorData({ ...valid, wave: Array.from({ length: 256 }, () => ({ enemy: "rattata", amount: 1 })) }, view, catalog).wave.length, 256);
  const unlocked = { ...view, stars: 10, progression: { ...view.progression, secrets: { manaphy: true } } };
  assert.equal(validateEditorData({ ...valid, mapId: 1 }, unlocked, catalog).mapId, 1);
  assert.equal(validateEditorData({ ...valid, mapId: 20 }, unlocked, catalog).mapId, 20);
  results.push({ name: "code-and-capacity-boundaries", passed: true });
});

test("readonly and active-world editor states cannot start or mutate practice", async () => {
  const { Editor } = await loadEditor();
  for (const extra of [{ disabled: true }, { view: { ...view, waveActive: true } }, { view: { ...view, paused: true } }, { view: { ...view, progression: { ...view.progression, challenge: { active: true } } } }]) {
    const html = renderToStaticMarkup(React.createElement(Editor, props(extra)));
    const start = html.match(/<button[^>]*aria-label="开始实践"[^>]*>/)?.[0];
    assert.ok(start?.includes("disabled"));
  }
  const html = renderToStaticMarkup(React.createElement(Editor, props({ disabled: true })));
  assert.ok(html.match(/<select[^>]*disabled/));
  assert.ok(html.match(/<input[^>]*type="number"[^>]*disabled/));
  const custom = renderToStaticMarkup(React.createElement(Editor, props({ view: { ...view, progression: { ...view.progression, custom: true, customData: valid } } })));
  assert.ok(custom.includes("退出实践"));
  assert.ok(custom.includes("重试实践"));
  results.push({ name: "readonly-and-practice-controls", passed: true });
});

test("real DOM editor groups, sharing, selection isolation and 320px layout", { skip: process.env.WIRE_BOARD_EDITOR_BROWSER !== "1", timeout: 120000 }, async () => {
  const browserFile = new URL("editor-browser.js", output);
  const entry = `import React,{useState} from "react";import{createRoot}from"react-dom/client";import{Editor}from${JSON.stringify(fileURLToPath(new URL("../web/Editor.tsx", import.meta.url)))};const initial=${JSON.stringify(view)};const catalogue=${JSON.stringify(catalog)};window.__editorActions=[];window.__editorClosed=0;function App(){const[state,setState]=useState({view:initial,disabled:false});window.__setEditor=next=>setState(old=>({...old,...next}));return React.createElement(Editor,{...state,catalog:catalogue,onAct:action=>{if(window.__editorReject)return Promise.reject(new Error("实践尚未准备好"));window.__editorActions.push(structuredClone(action))},onClose:()=>{window.__editorClosed++}})}createRoot(document.getElementById("root")).render(React.createElement(App));`;
  await build({ stdin: { contents: entry, resolveDir: `${root}web`, sourcefile: "native-editor-browser.tsx", loader: "tsx" }, outfile: fileURLToPath(browserFile), bundle: true, platform: "browser", format: "esm", target: "es2022", jsx: "automatic", nodePaths: [`${root}web/node_modules`] });
  const css = (await readFile(new URL("../web/style.css", import.meta.url), "utf8")) + "\n" + (await readFile(new URL("../web/editor.css", import.meta.url), "utf8"));
  const server = createServer(async (request, response) => {
    if (request.url === "/editor.js") { response.setHeader("Content-Type", "text/javascript"); response.end(await readFile(browserFile)); }
    else if (request.url === "/style.css") { response.setHeader("Content-Type", "text/css"); response.end(css); }
    else { response.setHeader("Content-Type", "text/html; charset=utf-8"); response.end('<!doctype html><html lang="zh-CN"><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/style.css"><style>*{box-sizing:border-box}body{margin:0}#root{height:100dvh;min-height:0;border-radius:0}</style></head><body><div id="root" class="pokepath-board"></div><script type="module" src="/editor.js"></script></body></html>'); }
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const base = `http://127.0.0.1:${server.address().port}`;
  const profile = fileURLToPath(new URL("profile/", output));
  const env = { ...process.env, AGENT_BROWSER_SOCKET_DIR: `${root}.local/ab`, AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_NO_VERSION_CHECK: "1", TMPDIR: `${root}.local/tmp` };
  await mkdir(env.AGENT_BROWSER_SOCKET_DIR, { recursive: true });
  const session = execFileSync("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "edui"], { cwd: root, env, encoding: "utf8" }).trim();
  const browser = async (...args) => {
    const { stdout: raw } = await execute("agent-browser", ["--session", session, "--json", ...args], { cwd: root, env, encoding: "utf8", timeout: 35000 });
    const value = JSON.parse(raw);
    assert.equal(value.success, true, raw);
    return value.data;
  };
  const evaluate = async source => (await browser("eval", "-b", Buffer.from(source).toString("base64"))).result;
  const click = name => browser("find", "role", "button", "click", "--name", name, "--exact");
  try {
    await browser("set", "viewport", "1180", "900");
    await browser("open", base);
    await browser("wait", "[data-native-editor]");
    await browser("snapshot", "-i");
    await browser("fill", '[aria-label="第 1 组数量"]', "4");
    await browser("select", '[aria-label="第 1 组敌人"]', "pidgey");
    await click("加入间隔");
    await click("增加第 2 组数量");
    await click("加入敌人");
    await click("上移第 3 组");
    await click("减少第 3 组数量");
    assert.equal(await evaluate('document.querySelector(\'[aria-label="第 3 组数量"]\').value'), "1");
    await click("增加第 3 组数量");
    await click("加入敌人");
    await click("删除第 4 组");
    assert.equal(await evaluate('document.querySelectorAll("[data-editor-group]").length'), 3);
    await browser("fill", '[aria-label="防线生命"]', "10");
    await browser("fill", '[aria-label="生命强化"]', "20");
    await browser("fill", '[aria-label="护甲强化"]', "10");
    await evaluate(`window.__setEditor({view:{...${JSON.stringify(view)},team:[{id:99,species:"treecko",level:1}]}})`);
    await browser("wait", "--fn", 'document.querySelectorAll("[data-editor-group]").length===3');
    assert.equal(await evaluate('document.querySelector(\'[aria-label="第 1 组敌人"]\').value'), "pidgey");
    await browser("click", "summary");
    const expected = { mapId: 0, hearts: 10, health: 20, armor: 10, wave: [{ enemy: "pidgey", amount: 4 }, { enemy: "rattata", amount: 1 }, { enemy: null, amount: 2 }] };
    const code = await evaluate('document.querySelector(\'[aria-label="分享码"]\').value');
    assert.deepEqual(JSON.parse(Buffer.from(code, "base64").toString("utf8")), expected);
    await click("复制分享码");
    await browser("wait", "--text", "已复制");
    await click("开始实践");
    await browser("wait", "--fn", "window.__editorActions.length===1");
    assert.deepEqual(await evaluate("window.__editorActions[0]"), { type: "custom", settings: expected });
    await browser("screenshot", fileURLToPath(new URL("desktop.png", output)));
    await evaluate("window.__setEditor({disabled:true})");
    await browser("wait", "--fn", 'document.querySelector(\'[aria-label="开始实践"]\').disabled');
    assert.equal(await evaluate('[...document.querySelectorAll("select,input[type=number]")].every(element=>element.disabled)'), true);
    await evaluate('document.querySelector(\'[aria-label="开始实践"]\').click()');
    assert.equal(await evaluate("window.__editorActions.length"), 1);
    await evaluate("window.__setEditor({disabled:false})");
    await browser("wait", "--fn", '!document.querySelector(\'[aria-label="防线生命"]\').disabled');
    await browser("fill", '[aria-label="分享码"]', "not-a-code");
    await click("导入分享码");
    await browser("wait", '[role="alert"]');
    assert.equal(await evaluate("window.__editorActions.length"), 1);
    await browser("fill", '[aria-label="分享码"]', Buffer.from(JSON.stringify(valid)).toString("base64"));
    await click("导入分享码");
    await browser("wait", "--fn", 'document.querySelectorAll("[data-editor-group]").length===2');
    assert.equal(await evaluate('document.querySelector(\'[aria-label="防线生命"]\').value'), "14");
    await browser("set", "viewport", "320", "740");
    const measure = await evaluate('(()=>{const root=document.querySelector("[data-native-editor]"),button=document.querySelector(\'[aria-label="开始实践"]\'),a=root.getBoundingClientRect(),b=button.getBoundingClientRect();return{pageWidth:document.documentElement.scrollWidth,viewport:innerWidth,root:a.toJSON(),start:b.toJSON(),controls:[...root.querySelectorAll("button,input,select,textarea")].filter(element=>element.getClientRects().length).map(element=>({tag:element.tagName,name:element.getAttribute("aria-label"),width:element.getBoundingClientRect().width,left:element.getBoundingClientRect().left,right:element.getBoundingClientRect().right}))}})()');
    assert.ok(measure.pageWidth <= measure.viewport + 1, JSON.stringify(measure));
    assert.ok(measure.start.left >= 0 && measure.start.right <= 321 && measure.start.top >= 0 && measure.start.bottom <= 741, JSON.stringify(measure));
    assert.ok(measure.controls.every(control => control.left >= -1 && control.right <= 321), JSON.stringify(measure));
    await browser("screenshot", fileURLToPath(new URL("mobile.png", output)));
    await click("增加第 1 组数量");
    assert.equal(await evaluate('document.querySelector(\'[aria-label="第 1 组数量"]\').value'), "4");
    await evaluate(`window.__setEditor({view:{...${JSON.stringify(view)},epoch:2,progression:{...${JSON.stringify(view.progression)},custom:true,customData:${JSON.stringify(valid)}}}})`);
    await browser("wait", '[aria-label="重试实践"]');
    await click("重试实践");
    await browser("wait", "--fn", "window.__editorActions.length===2");
    assert.deepEqual(await evaluate("window.__editorActions[1]"), { type: "custom_retry" });
    await click("退出实践");
    await browser("wait", "--fn", "window.__editorActions.length===3");
    assert.deepEqual(await evaluate("window.__editorActions[2]"), { type: "custom_exit" });
    await evaluate("window.__editorReject=true");
    await click("开始实践");
    await browser("wait", "--text", "实践尚未准备好");
    assert.equal(await evaluate("window.__editorActions.length"), 3);
    assert.equal(await evaluate('document.querySelector(\'[aria-label="第 1 组数量"]\').value'), "3");
    await writeFile(new URL("browser.json", output), JSON.stringify({ passed: true, command: "WIRE_BOARD_EDITOR_BROWSER=1 node --test games/pokepath/tests/native-editor-ui.test.mjs", sent: expected, actions: await evaluate("window.__editorActions"), readonlyBlocked: true, invalidCodeDidNotSubmit: true, rejectedPracticeKeptDraft: true, measure }, null, 2) + "\n");
    results.push({ name: "real-dom-desktop-mobile", passed: true });
  } finally {
    try { await browser("close"); await rm(profile, { recursive: true, force: true }); } finally { await new Promise(resolve => server.close(resolve)); }
  }
});

test.after(async () => {
  await writeFile(new URL("result.json", output), JSON.stringify({ command: "node --test games/pokepath/tests/native-editor-ui.test.mjs", browserRequested: process.env.WIRE_BOARD_EDITOR_BROWSER === "1", passed: results.length === (process.env.WIRE_BOARD_EDITOR_BROWSER === "1" ? 5 : 4), results }, null, 2) + "\n");
});
