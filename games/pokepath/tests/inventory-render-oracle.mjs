import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { referenceDOMClass, verifyDOMOracle, domSHA } from "./dom-oracle.mjs";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/scenes/InventoryScene.js", import.meta.url), "utf8");
const parsed = ts.createSourceFile("InventoryScene.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
const declaration = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "InventoryScene");
const methods = declaration.members.filter(node => ts.isMethodDeclaration(node) && !["open", "close"].includes(node.name.getText(parsed))).map(node => node.name.getText(parsed));
const classes = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass("utils/Input.js", "Input", ["constructor", "render", "renderTypeText"]),
  referenceDOMClass("utils/Utility.js", "Utility", ["colorTier", "darkenColor", "parseText"]),
  referenceDOMClass("game/scenes/InventoryScene.js", "InventoryScene", methods)
]);
const constants = parsed.statements.filter(node => ts.isVariableStatement(node)).map(node => node.getText(parsed)).join("\n");
const tagsSource = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/data/tagsData.js", import.meta.url), "utf8");
const tags = ts.transpileModule(tagsSource, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
const catalog = JSON.parse(await readFile(new URL("../assets/public/pokepath/v1/catalog.json", import.meta.url), "utf8"));
const keys = ["protein", "silphScope", "quickClaw", "revysBook", "wideLens", "lifeOrb", "amuletCoin", "lightBall", "charizarditeX", "heatRock", "airBalloon", "ringTarget"].filter(key => catalog.items[key]);
const team = [{ id: 917, species: "charmander", level: 1, target: "first", item: "", tileID: -1, stats: {}, equipableItems: keys.filter(key => !["wideLens", "lightBall", "revysBook"].includes(key)) }, { id: 73, species: "pikachu", level: 1, target: "first", item: "", tileID: -1, stats: {}, equipableItems: keys.filter(key => !["revysBook"].includes(key)) }];
const input = { view: { team, box: [], items: keys.map(id => ({ id })), progression: { settings: { colorTags: true }, profileStats: { itemTotal: 178 } } } };
const reference = `const {catalog,dictionary,input,parameters}=window.oracleData;const text=dictionary.text,itemCollectionTotal=178,playSound=()=>{};${constants}\n${tags}\n${classes.map(value => value.code).join("\n")}const items=input.view.items.map(item=>({...catalog.items[item.id],name:catalog.items[item.id].names,description:catalog.items[item.id].descriptions}));const pokemon=input.view.team.map(pokemon=>({...pokemon,sprite:catalog.species[pokemon.species].sprite}));window.inventoryActions=[];const scene=Object.create(InventoryScene.prototype);scene.container=document.querySelector('.section-scene-container');scene.slotTotal=180;scene.itemArray=[];scene.itemSelected=null;scene.pokemonArray=[];scene.pokemonSelected=null;scene.tabSelected=0;scene.sorted=0;scene.size=[42,47,53,60,38];scene.sizePosition=0;scene.main={lang:Number(parameters.language),utility:new Utility(),team:{pokemon},player:{items,itemAmount:items.length,swapItems(first,second){window.inventoryActions.push({type:'swapItems',itemID:items[first].id,name:items[second].id});[items[first],items[second]]=[items[second],items[first]]}},itemController:{getItems:()=>items,getAllPokemon:()=>pokemon,getAvailablePokemon:item=>pokemon.filter(pokemon=>pokemon.equipableItems.includes(item.id)),equip(item,pokemon){window.inventoryActions.push({type:'equip',itemID:item.id,pokemonID:pokemon.id})}},UI:{pokemon:[],updatePokemon(){}},game:{animate(){}}};window.inventoryReference=scene;scene.render();scene.update();if(items.length)scene.selectItem(0);scene.main.UI.pokemon=pokemon.map((pokemon,index)=>{const node=document.createElement('div');node.className='ui-pokemon';node.dataset.index=index;Object.assign(node.style,{position:'absolute',left:'0px',top:(103+index*114)+'px',width:'129px',height:'114px',background:'#253528'});document.body.appendChild(node);return node});`;
const click = (reference, native) => async ({ kind, browser }) => { await browser("click", kind === "reference" ? reference : native); };
const fill = value => async ({ kind, browser }) => { await browser("fill", kind === "reference" ? ".inventory-scene-search" : ".pokepath-inventory-search", value); };
const steps = [{ name: "initial" }];
for (let index = 1; index <= 7; index++) steps.push({ name: `sort-${index}`, inspectFonts: index === 1, action: click(".box-scene-sort-arrow:last-child", ".pokepath-inventory-filter>button:last-child") });
steps.push({ name: "power", action: click(".box-scene-tab:nth-child(2)", ".pokepath-box-terrains>button:nth-child(2)") }, { name: "utility", action: click(".box-scene-tab:nth-child(6)", ".pokepath-box-terrains>button:nth-child(6)") }, { name: "all", action: click(".box-scene-tab:first-child", ".pokepath-box-terrains>button:first-child") }, { name: "search-mega", action: fill("mega") }, { name: "search-prefix", action: fill("pro") }, { name: "search-clear", action: fill("") }, { name: "pokemon-filter", action: click(".inventory-scene-team-slot:first-child", ".pokepath-inventory-team>button:first-child") }, { name: "pokemon-filter-clear", action: click(".inventory-scene-team-slot:first-child", ".pokepath-inventory-team>button:first-child") });
for (let index = 1; index <= 5; index++) steps.push({ name: `size-${index}`, action: click(".box-scene-view-button", ".pokepath-inventory-size") });
const dragItem = (itemID, destination, short = false) => async ({ kind, browser, evaluate }) => {
  const source = kind === "reference" ? `window.inventoryReference.itemSlot[window.inventoryReference.itemArray.findIndex(item=>item.id===${JSON.stringify(itemID)})]` : `document.querySelector('[data-inventory-item="${itemID}"]')`;
  const target = destination === "pokemon" ? kind === "reference" ? "window.inventoryReference.main.UI.pokemon[0]" : "document.querySelector('[data-party-slot=\"0\"]')" : kind === "reference" ? `window.inventoryReference.itemSlot[window.inventoryReference.itemArray.findIndex(item=>item.id===${JSON.stringify(destination)})]` : `document.querySelector('[data-inventory-item="${destination}"]')`;
  const points = await evaluate(`(()=>{const center=node=>{const r=node.getBoundingClientRect();return[Math.round(r.left+r.width/2),Math.round(r.top+r.height/2)]};return{from:center(${source}),to:center(${target})}})()`);
  await browser("mouse", "move", ...points.from.map(String));
  await browser("mouse", "down", "left");
  await browser("mouse", "move", String(points.from[0] + (short ? 3 : 8)), String(points.from[1]));
  if (!short) await browser("mouse", "move", ...points.to.map(String));
  await browser("mouse", "up", "left");
};
steps.push({ name: "short-pointer", action: dragItem("protein", "quickClaw", true) }, { name: "reorder-stable-items", action: dragItem("protein", "quickClaw") }, { name: "equip-stable-pokemon", action: dragItem("protein", "pokemon") }, { name: "reject-ineligible-drag", action: dragItem("revysBook", "pokemon") });
for (const animationMS of [500, 1000, 2000, 2999]) steps.push({ name: `admin-${animationMS}ms`, animationMS });
const verifyNative = async ({ browser, evaluate, ready }) => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const output = join(root, ".local/pokepath/verification/inventory-render");
  const report = { passed: false, scope: "Real isolated native React inventory at 320px with trusted CDP touch, verified asset bytes and stable action receipts; server rules checked separately", checks: [], measurements: [] };
  const touch = async selector => {
    await browser("scrollintoview", selector);
    const point = await evaluate(`(()=>{const node=document.querySelector(${JSON.stringify(selector)}),r=node.getBoundingClientRect();return{x:Math.round(r.left+r.width/2),y:Math.round(r.top+r.height/2),width:r.width,height:r.height}})()`);
    assert.ok(point.width > 0 && point.height > 0 && point.x >= 0 && point.x < 320 && point.y >= 0 && point.y < 674, `${selector}: touch target outside viewport ${JSON.stringify(point)}`);
    const cdp = (await browser("get", "cdp-url")).cdpUrl;
    const origin = await evaluate("location.origin");
    execFileSync(process.execPath, [join(root, "scripts/browser_touch.mjs"), cdp, origin, "tap", String(point.x), String(point.y), String(point.x), String(point.y)], { cwd: root, env: { ...process.env, TMPDIR: join(root, ".local/tmp") }, encoding: "utf8", timeout: 15000 });
    await ready();
  };
  try {
    const before = await evaluate("JSON.stringify(window.inventoryActions)");
    await evaluate("document.querySelector('.pokepath-classic-scene').style.setProperty('--pp-scene-scale','0.5')");
    await ready();
    const start = await evaluate("(()=>{const r=document.querySelector('[data-inventory-item=\"protein\"]').getBoundingClientRect();return{x:Math.round(r.left+r.width/2),y:Math.round(r.top+r.height/2)}})()");
    await browser("mouse", "move", String(start.x), String(start.y));
    await browser("mouse", "down", "left");
    await browser("mouse", "move", String(start.x + 8), String(start.y));
    await ready();
    const ghost = await evaluate("(()=>{const r=document.querySelector('.pokepath-inventory-drag').getBoundingClientRect();return{x:r.left+r.width/2,y:r.top+r.height/2,width:r.width,height:r.height}})()");
    report.measurements.push({ selector: ".pokepath-inventory-drag", ...ghost });
    assert.ok(Math.abs(ghost.x - start.x - 8) < 1 && Math.abs(ghost.y - start.y) < 1 && Math.abs(ghost.width - 46.2) < 0.2 && Math.abs(ghost.height - 46.2) < 0.2, `scaled map inventory ghost differs from original fixed screen size ${JSON.stringify(ghost)}`);
    assert.equal(await evaluate("document.querySelector('.pokepath-classic-scene').dataset.inventoryDrag"), "protein");
    await browser("mouse", "move", "1220", "660");
    await browser("mouse", "up", "left");
    await ready();
    assert.equal(await evaluate("JSON.stringify(window.inventoryActions)"), before);
    assert.equal(await evaluate("document.querySelector('.pokepath-inventory-drag')===null"), true);
    assert.equal(await evaluate("document.querySelector('.pokepath-classic-scene').dataset.inventoryDrag??null"), null);
    report.checks.push("scaled native game keeps the original 44px drag image under the pointer and clears it on an invalid drop");
    await browser("set", "viewport", "320", "674");
    await browser("reload");
    await ready();
    await browser("screenshot", join(output, "mobile-inventory.png"));
    assert.equal(await evaluate("document.documentElement.scrollWidth<=innerWidth"), true, "mobile document overflow");
    for (const selector of [".pokepath-inventory-size", ".pokepath-inventory-search", ".pokepath-box-terrains>button:first-child", "[data-inventory-pokemon=\"917\"]", "[data-inventory-item=\"protein\"]"]) {
      await browser("scrollintoview", selector);
      const geometry = await evaluate(`(()=>{const node=document.querySelector(${JSON.stringify(selector)}),r=node.getBoundingClientRect(),hit=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return{x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,inactive:getComputedStyle(node).pointerEvents==='none',covered:!hit||!node.contains(hit)&&!(node.getAttribute('aria-pressed')==='true'&&getComputedStyle(node).pointerEvents==='none'&&hit===node.parentElement)}})()`);
      report.measurements.push({ selector, ...geometry });
      assert.ok(geometry.x >= 0 && geometry.right <= 320 && geometry.width >= 24 && geometry.height >= 24 && !geometry.covered, `${selector}: mobile control clipped, too small or covered ${JSON.stringify(geometry)}`);
    }
    report.checks.push("320px visible unclipped touch controls");
    await touch('[data-inventory-pokemon="917"]');
    await touch('[data-inventory-item="protein"]');
    await touch(".pokepath-inventory-equipped-action");
    const actions = await evaluate("window.inventoryActions");
    assert.deepEqual(actions, [{ type: "equip", pokemonID: 917, itemID: "protein" }]);
    report.checks.push("touch equips exactly one stable eligible partner and item");
    await browser("screenshot", join(output, "mobile-equipped.png"));
    const readonlyURL = await evaluate("(()=>{const url=new URL(location.href);url.searchParams.set('readonly','true');return url.href})()");
    await browser("open", readonlyURL);
    await ready();
    await touch('[data-inventory-pokemon="917"]');
    await touch('[data-inventory-item="protein"]');
    assert.equal(await evaluate("document.querySelector('.pokepath-inventory-equipped-action').disabled"), true);
    await touch(".pokepath-inventory-equipped-action");
    assert.deepEqual(await evaluate("window.inventoryActions"), []);
    report.checks.push("readonly native panel cannot emit equipment actions");
    report.passed = true;
    return report;
  } finally { await writeFile(join(output, "mobile-result.json"), JSON.stringify(report, null, 2) + "\n"); }
};
await verifyDOMOracle({ referenceControl: process.env.POKEPATH_DOM_REFERENCE_CONTROL === "1", name: "inventory-render", command: "node games/pokepath/tests/inventory-render-oracle.mjs", sources: [...classes.map(value => value.identity), { path: "game/data/tagsData.js", sha256: domSHA(tagsSource) }], styles: [".local/pokepath/rejected-embedding/assets/src/css/box.css", ".local/pokepath/rejected-embedding/assets/src/css/inventory.css"], reference, native: 'import "./games/pokepath/tests/InventoryFixture.tsx";', input, cases: [{ language: "8", theme: "original", font: "PressStart2P" }], steps, verifyNative, fontSelectors: { reference: ".box-scene-sort-value", native: ".pokepath-inventory-sort-value" }, measure: { reference: "({items:window.inventoryReference.itemArray.map(item=>item.id),sort:window.inventoryReference.sortValue.textContent,name:window.inventoryReference.itemSelectedName.textContent,description:window.inventoryReference.itemSelectedDescription.textContent,size:window.inventoryReference.itemSlot[0].getBoundingClientRect().width,actions:window.inventoryActions,sortGeometry:(()=>{const element=document.querySelector('.box-scene-sort-value'),r=element.getBoundingClientRect(),s=getComputedStyle(element);const range=document.createRange();range.selectNodeContents(element);const text=range.getBoundingClientRect();return{rect:[r.left,r.top,r.width,r.height],text:[text.left,text.top,text.width,text.height],style:Object.fromEntries(['fontFamily','fontSize','fontWeight','lineHeight','letterSpacing','textShadow','fontKerning','fontFeatureSettings','textRendering','fontSynthesis','fontOpticalSizing','fontVariationSettings','fontVariant','fontStretch','fontStyle','textSizeAdjust','webkitFontSmoothing','webkitTextFillColor','webkitTextStrokeColor','webkitTextStrokeWidth','color','opacity','filter'].map(key=>[key,s[key]]))}})()})", native: "({items:[...document.querySelectorAll('[data-inventory-item]')].map(node=>node.dataset.inventoryItem),sort:document.querySelector('.pokepath-inventory-sort-value').textContent,name:document.querySelector('.pokepath-inventory-description strong').textContent,description:document.querySelector('.pokepath-inventory-description p').textContent,size:document.querySelector('[data-inventory-slot]').getBoundingClientRect().width,actions:window.inventoryActions,sortGeometry:(()=>{const element=document.querySelector('.pokepath-inventory-sort-value'),r=element.getBoundingClientRect(),s=getComputedStyle(element);const range=document.createRange();range.selectNodeContents(element);const text=range.getBoundingClientRect();return{rect:[r.left,r.top,r.width,r.height],text:[text.left,text.top,text.width,text.height],style:Object.fromEntries(['fontFamily','fontSize','fontWeight','lineHeight','letterSpacing','textShadow','fontKerning','fontFeatureSettings','textRendering','fontSynthesis','fontOpticalSizing','fontVariationSettings','fontVariant','fontStretch','fontStyle','textSizeAdjust','webkitFontSmoothing','webkitTextFillColor','webkitTextStrokeColor','webkitTextStrokeWidth','color','opacity','filter'].map(key=>[key,s[key]]))}})()})" } });
