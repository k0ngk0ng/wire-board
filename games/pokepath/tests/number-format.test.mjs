import test from "node:test";
import assert from "node:assert/strict";
import { readFile, mkdir } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { runInNewContext } from "node:vm";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");

test("game number and gold formatting preserve the original ten-language output without rounding", async () => {
  const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/utils/Utility.js", import.meta.url), "utf8");
  const parsed = ts.createSourceFile("Utility.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const declaration = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "Utility");
  const methods = declaration.members.filter(node => ["numberDot", "formatGold"].includes(node.name?.getText(parsed))).map(node => node.getText(parsed));
  const reference = runInNewContext(`new (class{${methods.join("\n")}})()`);
  const directory = new URL("../../../.local/pokepath/number-format/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const target = new URL("format.mjs", directory);
  await require("esbuild").build({ entryPoints: [fileURLToPath(new URL("../web/number-format.ts", import.meta.url))], outfile: fileURLToPath(target), platform: "node", format: "esm" });
  const { formatGameNumber, formatGameGold } = await import(target.href);
  const numbers = [0, -1, -1234567.891234, 999, 1000, 1234, 123456789, 1234.5678, 1234.567891, 99999999998, 99999999999, 1e21, 1e-7];
  for (let language = 0; language < 10; language++) for (const value of numbers) {
    assert.equal(formatGameNumber(value, language), reference.numberDot(value, language), `${language}/${value}`);
    assert.equal(formatGameGold(value, language), reference.formatGold(value, language), `${language}/${value}: gold`);
  }
  assert.equal(formatGameNumber(25600), "25.600");
});

test("the shared money display and shop prices follow the selected language", async () => {
  const React = require("react");
  const { renderToStaticMarkup } = require("react-dom/server");
  const directory = new URL("../../../.local/pokepath/number-format/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const target = new URL("scene.mjs", directory);
  await require("esbuild").build({ stdin: { contents: 'export {ClassicScene} from "./games/pokepath/web/ClassicScene";export {GamePanel} from "./games/pokepath/web/Panels";export {I18nProvider} from "./games/pokepath/web/I18n";', resolveDir: fileURLToPath(new URL("../../../", import.meta.url)) }, outfile: fileURLToPath(target), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { ClassicScene, GamePanel, I18nProvider } = await import(target.href);
  const view = { epoch: 1, sequence: 0, clock: 0, gold: 1234567, health: 14, stars: 0, ribbons: 0, route: 0, wave: 1, speed: 0.8, team: [], box: [], items: [], towers: [], enemies: [], projectiles: [], events: [], tiles: [], progression: { formations: [], challenge: {}, settings: {}, remainingEggs: 256, eggPrice: 5500, shop: [{ itemID: "test", stock: 0, tier: "common", price: 1234567, name: "test", affordable: true }] } };
  const assets = { images: new Map(), urls: new Map(), ready: false, catalog: { species: {}, routes: {}, enemies: {}, items: {} } };
  const props = { view, assets, placing: false, panel: "team", disabled: false, stage: React.createRef(), onAct() {}, onSelect() {}, onDetails() {}, onPanel() {}, onPoint() {}, onReady() {}, onClose() {} };
  for (let language = 0; language < 10; language++) {
    const separator = language === 2 ? " " : [0, 6, 7].includes(language) ? "," : ".";
    const wrap = element => renderToStaticMarkup(React.createElement(I18nProvider, { language }, element));
    const scene = wrap(React.createElement(ClassicScene, props));
    assert.ok(scene.includes(`aria-label="共享金币 1234567">$1${separator}234${separator}567</span>`), `money language ${language}`);
    const shop = wrap(React.createElement(GamePanel, { ...props, panel: "shop" }));
    assert.ok(shop.includes(`<strong class="pokepath-shop-price">$5${separator}500</strong>`), `egg language ${language}`);
    assert.ok(shop.includes(`<strong class="pokepath-shop-price">$1${separator}234${separator}567</strong>`), `item language ${language}`);
  }
  const rich = renderToStaticMarkup(React.createElement(ClassicScene, { ...props, view: { ...view, gold: 1e15 } }));
  assert.ok(rich.includes(">$1.00e+15</span>"));
});
