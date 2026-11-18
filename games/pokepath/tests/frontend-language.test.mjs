import test from "node:test";
import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { createRequire } from "node:module";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const React = require("react");
const { renderToStaticMarkup } = require("react-dom/server");

test("React language changes translate interface and catalog facts without changing rule values", async () => {
  const directory = new URL("../../../.local/pokepath/classic-ui-test/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const target = new URL("i18n.mjs", directory);
  await require("esbuild").build({ entryPoints: [new URL("../web/I18n.tsx", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [new URL("../../../web/node_modules", import.meta.url).pathname], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { I18nProvider, useLocalize } = await import(target.href);
  const words = (en, es, zh) => [en, es, "", "", "", "", "", "", zh, ""];
  const dictionary = { stats: words("Wave Data ", "Info oleada", "波次信息"), load: words("Load Team", "Cargar equipo", "加载队伍"), menu: words("Settings", "Ajustes", "设置"), shop: words("Shop", "Tienda", "商店"), autoStop: words("Stop before invisibles", "Detente ante invisibles", "在隐形敌人前停止") };
  const catalog = { species: { charmander: { names: words("Charmander", "Charmander", "小火龍"), descriptions: words("Burns enemies", "Quema enemigos", "灼烧敌人") } } };
  function Example() { const localize = useLocalize(); return localize(React.createElement("section", { "aria-label": "设置", "data-action": "商店" }, React.createElement("button", { title: "商店" }, "商店"), React.createElement("p", null, "小火龍 [1]"), React.createElement("p", null, "灼烧敌人"), React.createElement("p", null, "在隱形敵人前停止"), React.createElement("p", null, "傷害統計"), React.createElement("p", null, "載入隊伍"), React.createElement("input", { value: "小火龍", readOnly: true }), React.createElement("strong", { "data-i18n-literal": true }, "小火龍"))); }
  const render = language => renderToStaticMarkup(React.createElement(I18nProvider, { language, dictionary, catalog }, React.createElement(Example)));
  const english = render(0);
  assert.ok(english.includes('aria-label="Settings"'));
  assert.ok(english.includes(">Shop</button>"));
  assert.ok(english.includes("Charmander [1]"));
  assert.ok(english.includes("Burns enemies"));
  assert.ok(english.includes("Stop before invisibles"));
  assert.ok(english.includes("<p>Wave Data </p>"));
  assert.ok(english.includes("<p>Load Team</p>"));
  assert.ok(english.includes('data-action="商店"'));
  assert.ok(english.includes('value="小火龍"'));
  assert.ok(english.includes(">小火龍</strong>"));
  assert.ok(render(1).includes(">Tienda</button>"));
});

test("original language arrays including English Unown in Chinese select their requested entry", async () => {
  const target = new URL("../../../.local/pokepath/classic-ui-test/locale-facts.mjs", import.meta.url);
  await require("esbuild").build({ stdin: { contents: 'export { I18nProvider, useTextFact } from "./I18n"; export { RichText } from "./Panels";', resolveDir: new URL("../web/", import.meta.url).pathname }, outfile: target.pathname, bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [new URL("../../../web/node_modules", import.meta.url).pathname], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
  const { I18nProvider, RichText, useTextFact } = await import(target.href);
  const question = ["Can you read me?", "Puedes leerme?", "Peux tu me lire ?", "", "", "", "Can you read me?", "Can you read me?", "Can you read me?", ""];
  function Question() { return React.createElement("p", null, useTextFact("text.interfaceOptions.unownDialogue.0")); }
  const render = language => renderToStaticMarkup(React.createElement(I18nProvider, { language, dictionary: { text: { interfaceOptions: { unownDialogue: { 0: question } } } } }, React.createElement(React.Fragment, null, React.createElement(Question), React.createElement(RichText, { value: question }))));
  assert.equal(render(1), "<p>Puedes leerme?</p>Puedes leerme?");
  assert.equal(render(8), "<p>Can you read me?</p>Can you read me?");
});
