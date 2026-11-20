import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const React = require("react");
const { renderToStaticMarkup } = require("react-dom/server");
const { build } = require("esbuild");
let module;

async function loadModule() {
  if (module) return module;
  const directory = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const file = new URL("module.mjs", directory);
  await build({ entryPoints: [new URL("../web/module.tsx", import.meta.url).pathname], outfile: fileURLToPath(file), bundle: true, platform: "node", format: "esm", target: "es2022", jsx: "automatic", loader: { ".css": "empty" }, nodePaths: [`${root}/web/node_modules`], plugins: [{ name: "same-react-instance", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  module = (await import(`${file.href}?native=${Date.now()}`)).default;
  return module;
}

function fixture(spectating = false) {
  const view = { rulesVersion: "pokepath-native-1", epoch: 1, sequence: 0, clock: 0, gold: 0, health: 0, stars: 0, ribbons: 0, route: 0, wave: 1, waveActive: false, paused: false, speed: 1, team: [{ id: 17, species: "charmander", level: 1, target: "first", item: "", tileID: -1, stats: { power: 15, attackMS: 1100, range: 110, criticalPct: 1 }, sprite: {} }], box: [], items: [], tiles: [], towers: [], enemies: [], projectiles: [], events: [], random: 1 };
  const room = { id: "native-test", name: "合作守图", kind: "pokepath", host: "alice", capacity: 2, status: "playing", version: 1, you: spectating ? -1 : 0, spectating, seats: [{ id: "alice", name: "甲", ready: true }, { id: "bob", name: "乙", ready: true }], game: { data: view, actors: [0, 1], finished: false, winners: [], log: [] } };
  return { data: view, room, busy: false, assets: "", sound: false, audio: { stop() {} }, clock: { serverNow: 0, receivedAt: 0 }, act: async () => {}, command: async () => {} };
}

test("the native game renders its own Canvas and shared-resource controls", async () => {
  const game = await loadModule();
  const html = renderToStaticMarkup(React.createElement(game.Board, fixture()));
  assert.ok(html.includes("<canvas"));
  assert.ok(!html.includes("<iframe"));
  assert.ok(html.includes("data-pokepath-board"));
  for (const label of ["下一波", "商店", "路线", "仓库", "背包", "挑战", "图鉴"]) assert.ok(html.includes(label), label);
  const source = await readFile(new URL("../web/Board.tsx", import.meta.url), "utf8");
  assert.ok(!/postMessage|frame\.html|pokepath-original|upstream\//.test(source));
});

test("zero shared resources remain visible for every participant", async () => {
  const game = await loadModule();
  const props = fixture();
  for (const seat of [0, 1]) {
    const html = renderToStaticMarkup(React.createElement(game.PlayerSummary, { room: props.room, data: props.data, seat, assets: "" }));
    assert.ok(html.includes("金币 0"));
    assert.ok(html.includes("生命 0"));
  }
});

test("spectators receive disabled world-changing controls", async () => {
  const game = await loadModule();
  const html = renderToStaticMarkup(React.createElement(game.Board, fixture(true)));
  const wave = html.match(/<button[^>]*aria-label="下一波"[^>]*>/)?.[0];
  assert.ok(wave?.includes("disabled"));
});

test("the native catalogue describes all three independent starting partners", async () => {
  const game = await loadModule();
  assert.equal(game.minPlayers, 1);
  assert.equal(game.maxPlayers, 4);
  assert.equal(game.asyncBoard, true);
  assert.equal(game.Options, undefined);
  const html = renderToStaticMarkup(React.createElement(game.Rules));
  for (const name of ["小火龙", "木守宫", "呱呱泡蛙"]) assert.ok(html.includes(name));
});

test("a compact first stream requests synchronization before constructing a battlefield", async () => {
  const game = await loadModule();
  const props = fixture();
  props.data = { epoch: 1, sequence: 20, clock: 1000, fromSequence: 19, gold: 0, health: 14, wave: 1, route: 0, paused: false, operations: [{ type: "tick", milliseconds: 20 }] };
  const html = renderToStaticMarkup(React.createElement(game.Board, props));
  assert.ok(html.includes("正在同步战场"));
  assert.ok(!html.includes("<canvas"));
  assert.ok(!html.includes("<iframe"));
});

test("retained projections preserve iterable empty collections after loading a formation", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/runtime-empty.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/native-runtime.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { createNativePredictionRuntime } = await import(target.href);
  const value = { tiles: [], paths: [], team: [], box: null, towers: null, enemies: null, projectiles: null, events: null, items: null };
  const request = input => input.op === "session.project" ? structuredClone(value) : { nativeHandle: 1, projectionRevision: 1, clock: 0, epoch: 1, sequence: 0 };
  const runtime = createNativePredictionRuntime(request, () => ({}));
  const state = runtime.import({});
  for (let index = 0; index < 2; index++) {
    const view = runtime.project(state);
    for (const field of ["team", "box", "towers", "enemies", "projectiles", "events", "items"]) assert.deepEqual(view[field], [], field);
  }
  runtime.dispose();
});
