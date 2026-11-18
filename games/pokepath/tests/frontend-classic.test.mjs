import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const React = require("react");
const { renderToStaticMarkup } = require("react-dom/server");
const { build } = require("esbuild");

test("the independent scene preserves the original three-column visual structure", async () => {
  const directory = new URL("../../../.local/pokepath/classic-ui-test/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const target = new URL("scene.mjs", directory);
  await build({ entryPoints: [new URL("../web/ClassicScene.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", loader: { ".css": "empty" }, nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { ClassicScene } = await import(target.href);
  const pokemon = { id: 17, species: "charmander", level: 1, target: "notBurned", item: "", tileID: -1, stats: { power: 15, attackMS: 1100, range: 110, criticalPct: 0.6 } };
  const view = { epoch: 1, sequence: 0, clock: 0, gold: 50, health: 14, maxHealth: 14, stars: 0, ribbons: 0, route: 0, wave: 1, waveActive: false, paused: false, speed: 0.8, team: [pokemon], box: [], items: [], towers: [], enemies: [], projectiles: [], events: [], tiles: [], progression: { formations: [{ members: null }], challenge: { noItems: false }, settings: {}, canChallenge: false } };
  const assets = { images: new Map(), urls: new Map(), version: 0, ready: false, catalog: { species: { charmander: { name: "小火龍", color: "#ff8463", terrains: [1], sprite: {} } }, routes: {}, enemies: {} } };
  const html = renderToStaticMarkup(React.createElement(ClassicScene, { view, assets, selected: 17, placing: false, panel: "team", disabled: false, onAct() {}, onSelect() {}, onDetails() {}, onPanel() {}, onPoint() {}, onReady() {}, stage: React.createRef() }));
  for (const name of ["pokepath-classic-scene", "pokepath-trainer", "pokepath-central", "pokepath-monitor", "pokepath-formations-save", "pokepath-formations-load"]) assert.ok(html.includes(name), name);
  assert.equal((html.match(/data-party-slot=/g) ?? []).length, 10);
  assert.equal((html.match(/data-classic-section=/g) ?? []).length, 9);
  assert.ok(html.includes('data-logical-width="1238"'));
  assert.ok(html.includes('data-logical-height="674"'));
  assert.ok(html.includes("<canvas"));
  assert.ok(!html.includes("<iframe"));
});

test("details preserve exact cumulative prices and stable owned identities", async () => {
  const directory = new URL("../../../.local/pokepath/classic-ui-test/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const target = new URL("details.mjs", directory);
  await build({ entryPoints: [new URL("../web/PokemonDetails.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { PokemonDetails } = await import(target.href);
  const pokemon = { id: 17, species: "charmander", level: 1, target: "notBurned", item: "", tileID: -1, upgradeCost: 40, upgradeCosts: { 1: 40, 5: 302, 10: 919 }, stats: { power: 15, attackMS: 1100, range: 110, criticalPct: 0.6 } };
  const view = { gold: 50, team: [pokemon], box: [], towers: [], items: [] };
  const assets = { images: new Map(), urls: new Map(), version: 0, ready: false, catalog: { species: { charmander: { id: 1, name: "小火龍", color: "#ff8463", terrains: [1], sprite: {} } }, abilities: {}, items: {} } };
  const html = renderToStaticMarkup(React.createElement(PokemonDetails, { pokemon, view, assets, disabled: false, onAct() {}, onClose() {}, onSelect() {}, onInventory() {} }));
  assert.ok(html.includes('data-details-pokemon="17"'));
  for (const value of ["$40", "$302", "$919", 'data-upgrade-count="1"', 'data-upgrade-count="5"', 'data-upgrade-count="10"', 'aria-label="攻击目标"']) assert.ok(html.includes(value), value);
});

test("shop and box cover only the central field and box includes the team", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/panels.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/Panels.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { GamePanel } = await import(target.href);
  const pokemon = { id: 17, species: "charmander", level: 1, target: "first", item: "", tileID: -1, stats: {} };
  const props = { view: { team: [pokemon], box: [], towers: [], items: [], gold: 50, progression: { remainingEggs: 256, eggPrice: 5500, shop: [] } }, assets: { catalog: { species: { charmander: { name: "小火龍", sprite: {}, terrains: [1] } } } }, disabled: false, onAct() {}, onSelect() {}, onClose() {} };
  const shop = renderToStaticMarkup(React.createElement(GamePanel, { ...props, panel: "shop" }));
  assert.ok(shop.includes("pokepath-central-panel"));
  assert.ok(shop.includes("pokepath-classic-shop"));
  assert.ok(shop.includes("$5.500"));
  const egg = renderToStaticMarkup(React.createElement(GamePanel, { ...props, panel: "shop", store: { images: new Map(), path: value => value, url: value => value } }));
  assert.ok(egg.includes('data-resource="src/assets/images/icons/egg.png"'));
  const box = renderToStaticMarkup(React.createElement(GamePanel, { ...props, panel: "box" }));
  assert.ok(box.includes("pokepath-classic-box"));
  assert.ok(box.includes('data-box-pokemon="17" data-box-member="team"'));
});

test("draft resolves owned identities and preserves its authoritative round token", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/draft.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/DraftPanel.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { DraftPanel } = await import(target.href);
  const pokemon = { id: 917, species: "charmander", level: 1, target: "first", item: "", tileID: -1, stats: {} };
  const html = renderToStaticMarkup(React.createElement(DraftPanel, { view: { team: [], box: [pokemon], progression: { draft: { active: true, round: 0, rounds: 10, token: 49, offers: [917], canReroll: true } } }, assets: { catalog: { species: { charmander: { name: "小火龍", sprite: {} } } } }, disabled: true, onAct() {}, onDetails() {} }));
  assert.ok(html.includes('data-draft-token="49"'));
  assert.ok(html.includes('data-pokemon-id="917"'));
  assert.ok(html.includes("小火龍"));
  assert.ok(/aria-label="小火龍"[^>]*disabled/.test(html));
});

test("collection sections retain the original grid and use normalized route positions", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/collections.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/CollectionPanels.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { InventoryPanel, RoutesPanel, DexPanel } = await import(target.href);
  const assets = { catalog: { species: { charmander: { id: 1, name: "小火龍", sprite: {}, terrains: [1] } }, routes: { 0: { id: 0, name: "路線 1-1", position: 0, unlockStars: 0 } }, collection: ["charmander"], items: {} } };
  const props = { view: { stars: 0, team: [], box: [], items: [], records: {}, progression: { settings: { font: "PressStart2P" } } }, assets, disabled: false, onAct() {}, onClose() {} };
  const inventory = renderToStaticMarkup(React.createElement(InventoryPanel, props));
  assert.equal((inventory.match(/data-inventory-slot=/g) ?? []).length, 180);
  const routes = renderToStaticMarkup(React.createElement(RoutesPanel, props));
  assert.ok(routes.includes('data-route-id="0"'));
  assert.ok(routes.includes("grid-column:1"));
  assert.equal((routes.match(/data-route-placeholder=/g) ?? []).length, 53);
  assert.ok(routes.includes("pokepath-route-record"));
  assert.ok(!routes.includes("波次編輯器"));
  const dex = renderToStaticMarkup(React.createElement(DexPanel, props));
  assert.ok(dex.includes('data-dex-mode="tutorials"'));
});

test("colored facts preserve text without interpreting HTML", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/rich-text.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/Panels.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { RichText } = await import(target.href);
  const html = renderToStaticMarkup(React.createElement(RichText, { value: "攻击[POWER]5[/POWER]<img src=x onerror=x>" }));
  assert.ok(html.includes("color:"));
  assert.ok(html.includes("5"));
  assert.ok(html.includes("&lt;img"));
  assert.ok(!html.includes("<img"));
  const plain = renderToStaticMarkup(React.createElement(RichText, { value: "[POWER]5[/POWER]", enabled: false }));
  assert.equal(plain, "5");
});

test("profile exposes twenty-one authoritative statistics and the original portrait selector", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/profile.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/ProfilePanel.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { ProfilePanel } = await import(target.href);
  const html = renderToStaticMarkup(React.createElement(ProfilePanel, { view: { stars: 0, progression: { portrait: 79, profileName: "Player", profileStats: { timePlayed: 66, pokemonOwned: 1, shinyAmount: 0, highestPokemonLevel: 1, totalPokemonLevel: 1, totalGold: 50, itemAmount: 0, wavesCompleted: 0, highestHit: 0, defeatedEnemies: 0, defeatedSpecies: 0, appliedStuns: 0, appliedSlows: 0, appliedBurns: 0, appliedPoisons: 0, appliedCurses: 0, resets: 0, stolenGold: 0, maxGoldPerWave: { value: 0, routeTag: "" }, maxGoldPerTime: { value: 0, routeTag: "" }, pokemonTotal: 256, itemTotal: 178, enemyTotal: 345 } } }, assets: { catalog: { achievements: [] } }, disabled: false, onAct() {}, onClose() {} }));
  assert.equal((html.match(/data-profile-stat=/g) ?? []).length, 21);
  assert.ok(html.includes("1:06"));
  assert.ok(html.includes("$50"));
  assert.ok(html.includes('maxLength="11"'));
  assert.ok(html.includes('aria-label="下一个训练家头像"'));
});

test("settings use the original four gain groups and five fonts", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/settings.mjs", import.meta.url);
  await build({ stdin: { contents: 'export { SettingsPanel } from "./SettingsPanel"; export { I18nProvider } from "./I18n";', resolveDir: fileURLToPath(new URL("../web/", import.meta.url)) }, outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { SettingsPanel, I18nProvider } = await import(target.href);
  const dictionary = JSON.parse(await readFile(new URL("../assets/public/pokepath/v1/ui-text.json", import.meta.url), "utf8"));
  const panel = React.createElement(SettingsPanel, { view: { progression: { settings: { audio: { master: 12, music: 4, ui: 4, effects: 1 }, language: 8, theme: "original", font: "PressStart2P", showDamage: true, showHealth: false, mapEffects: true, colorTags: true, effectAlpha: 100 } } }, disabled: false, onAct() {}, onClose() {} });
  const html = renderToStaticMarkup(React.createElement(I18nProvider, { language: 8, dictionary }, panel));
  for (const label of ["主音量", "音乐音量", "界面音量", "音效音量", "语言", "主题", "字体"]) assert.ok(html.includes(label), label);
  assert.ok(!html.includes("NGC"));
  assert.ok(!html.includes("<audio"));
  assert.equal((html.match(/max="20"/g) ?? []).length, 4);
  assert.equal((html.match(/<kbd\b/g) ?? []).length, 13);
  assert.ok(html.includes("F11"));
});

test("card quick pickers use owned IDs and authoritative item eligibility", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/picker.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/FastPicker.tsx", import.meta.url).pathname], outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { FastPicker } = await import(target.href);
  const pokemon = { id: 917, species: "charmander", item: "", equipableItems: ["protein"] };
  const props = { view: { box: [pokemon], items: [{ id: "protein" }, { id: "carbos" }] }, assets: { catalog: { species: { charmander: { name: "小火龍", color: "#ff8463", sprite: {} } }, items: { protein: { name: "蛋白質", equipable: true }, carbos: { name: "速度強化劑", equipable: true } } } }, pokemon, disabled: true, onAct() {}, onClose() {} };
  const partners = renderToStaticMarkup(React.createElement(FastPicker, { ...props, mode: "pokemon" }));
  assert.ok(partners.includes('data-pokemon-id="917"'));
  const items = renderToStaticMarkup(React.createElement(FastPicker, { ...props, mode: "item" }));
  assert.ok(items.includes('data-item-id="protein"'));
  assert.ok(!items.includes('data-item-id="carbos"'));
  assert.ok(/aria-label="装备蛋白質"[^>]*disabled/.test(items));
});

test("damage monitor uses team order and independent authority totals", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/damage-monitor.mjs", import.meta.url);
  await build({ entryPoints: [new URL("../web/DamageMonitor.tsx", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { DamageMonitor } = await import(target.href);
  const props = { view: { epoch: 1, route: 0, wave: 1, totalDamage: 400, totalOverdamage: 1000, team: [{ id: 917, species: "charmander", damage: 100, overdamage: 120 }, { id: 73, species: "charizard", damage: 200, overdamage: 300 }, { id: 83, species: "helper", damage: 0, overdamage: 0 }] }, assets: { catalog: { species: { charmander: { id: 1, name: "小火龍", color: "#ff8463", sprite: {} }, charizard: { id: 3, name: "噴火龍", color: "#ff8463", sprite: {} }, helper: { id: 19, name: "支援", color: "#ffd663", sprite: {} } } } }, mode: "trueDamage", onMode() {} };
  const html = renderToStaticMarkup(React.createElement(DamageMonitor, props));
  assert.equal((html.match(/data-damage-row=/g) ?? []).length, 10);
  assert.ok(html.indexOf('data-pokemon-id="917"') < html.indexOf('data-pokemon-id="73"'));
  assert.ok(html.includes('(25%)'));
  assert.ok(html.includes('(50%)'));
  assert.ok(html.includes("正在协助"));
  assert.ok(!html.includes("重置"));
  const overflow = renderToStaticMarkup(React.createElement(DamageMonitor, { ...props, mode: "overdamage" }));
  assert.ok(overflow.includes('(12%)'));
  assert.ok(overflow.includes('(30%)'));
  const zero = renderToStaticMarkup(React.createElement(DamageMonitor, { ...props, view: { ...props.view, totalDamage: 0 } }));
  assert.ok(!zero.includes("NaN") && !zero.includes("Infinity"));
});
