import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { domSHA, referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/scenes/DexScene.js", import.meta.url), "utf8");
const parsed = ts.createSourceFile("DexScene.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
const constants = parsed.statements.filter(node => ts.isVariableStatement(node)).map(node => node.getText(parsed)).join("\n");
const sources = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass("utils/Input.js", "Input", ["constructor", "render", "renderTypeText"]),
  referenceDOMClass("utils/Utility.js", "Utility", ["numberDot", "minutsToTime", "parseText"]),
  referenceDOMClass("game/scenes/DexScene.js", "DexScene", ["render", "update", "changeMode", "updateSort", "renderTutorials", "renderPokemon", "renderEnemies", "sortPokemonList", "sortEnemyList", "createEntry", "getSearchValue", "getName", "isEnemyDefeated", "changeSort", "toggleEntry", "openEntry", "closeEntry", "renderTutorialData", "renderPokemonData", "renderEnemyData", "addDataRow", "addDescription", "getYesNo", "getPokemonRangeType", "getPokemonTerrain", "getEnemyStats", "getPokemonStats", "isUnownEntry", "isUnownConversationOpen", "renderUnownDialogue", "handleUnownInput", "buildPokemonStatRanks", "getPokemonEntries"])
]);
const tagsSource = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/data/tagsData.js", import.meta.url), "utf8");
const tags = ts.transpileModule(tagsSource, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
const tutorialSource = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/data/tutorialData.js", import.meta.url), "utf8");
const dataSource = ts.createSourceFile("tutorialData.js", tutorialSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
const dataDeclarations = dataSource.statements.filter(node => ts.isVariableStatement(node)).flatMap(node => node.declarationList.declarations);
const secretKeys = dataDeclarations.find(node => node.name.getText(dataSource) === "secretPokemon").initializer.elements.map(node => node.text);
const bottomIllustrations = dataDeclarations.find(node => node.name.getText(dataSource) === "tutorials").initializer.elements.flatMap(object => {
  const property = name => object.properties.find(node => node.name?.getText(dataSource) === name)?.initializer?.text;
  return property("imagePosition") === "bottom" ? [property("key")] : [];
});
const input = { keys: { collection: ["charmander", "treecko", "froaki", "unown", "natu", "mimikyu"], enemies: ["ledyba", "rattata", "rookidee", "sableye", "mantine", "shaymin", "ditto", "superCaterpie", "wishiwashiSolo"], tutorials: ["towers", "towersAttackType", "secrets"] }, view: { team: [{ id: 917, species: "charmander", alias: "Fire Guard", level: 12, dex: { damage: 121234.5, highestHit: 129.99, defeated: 512, deployed: 77, active: 3720, waves: 84, bosses: 4, challenges: 0 }, stats: { power: 42, attackMS: 1100, criticalPct: 2, range: 124 } }], box: [{ id: 73, species: "natu", level: 3, dex: { damage: 127000.5, highestHit: 126.2, defeated: 513, deployed: 51, active: 4500, waves: 60, bosses: 1, challenges: 0 }, stats: { power: 9, attackMS: 1300, criticalPct: 1, range: 135 } }], items: [{ id: "thundersDrive" }], progression: { settings: { colorTags: true, font: "PressStart2P" }, enemyStats: { rattata: { defeated: 4, escaped: 1, damage: 278.5 }, rookidee: { defeated: 2, escaped: 0, damage: 47 } } } } };
const reference = `const{catalog,dictionary,input,parameters}=window.oracleData;const text=dictionary.text,playSound=()=>{},secretPokemon=${JSON.stringify(secretKeys)},bottomIllustrations=new Set(${JSON.stringify(bottomIllustrations)});${constants}\n${tags}\n${sources.map(source=>source.code).join("\n")}const allPokemon=input.keys.collection;const pokemonData=Object.fromEntries(allPokemon.map(key=>{const source=catalog.species[key];const ability=catalog.abilities[source.abilityId];return[key,{...source,name:source.names,power:source.powerBase,speed:source.attackMsBase,range:source.rangeBase,critical:source.criticalBase,tiles:source.terrains,ability:ability&&{name:ability.names,description:ability.descriptions}}]}));const owner=pokemon=>{const source=pokemonData[pokemon.species];return{...source,alias:pokemon.alias,acquisitionDate:pokemon.id,stadistics:pokemon.dex,lvl:pokemon.level,power:pokemon.stats.power,speed:pokemon.stats.attackMS,range:pokemon.stats.range,critical:pokemon.stats.criticalPct}};const enemyData=Object.fromEntries(input.keys.enemies.map(key=>{const source=catalog.enemies[key];const passive=catalog.passives[source.passiveId];return[key,{...source,name:source.names,hp:source.health,speed:source.speed/60,isElite:source.isElite??false,isBoss:source.isBoss??false,passive:passive&&{name:passive.names,description:passive.descriptions}}]}));const tutorials=input.keys.tutorials.map(key=>({key,title:catalog.tutorials[key].names,text:catalog.tutorials[key].descriptions,image:catalog.tutorials[key].sprite,imagePosition:bottomIllustrations.has(key)?'bottom':'top'}));window.dexActions=[];const scene=Object.create(DexScene.prototype);Object.assign(scene,{main:{lang:Number(parameters.language),font:parameters.font==='Unown'?4:0,utility:new Utility(),team:{pokemon:input.view.team.map(owner)},box:{pokemon:input.view.box.map(owner)},player:{secrets:{},hasThunder:true,hasNox:parameters.variant==='nox',unownState:0,enemyStats:input.view.progression.enemyStats},UI:{getSecret(){window.dexActions.push({type:'tutorialSecret',name:'secrets'})}}},container:document.querySelector('.section-scene-container'),mode:'tutorials',selected:null,tabs:[],entries:[],sortedTower:0,sortedEnemy:0,pokemonStatRanks:new Map()});window.dexReference=scene;scene.buildPokemonStatRanks();const handle=scene.handleUnownInput;scene.handleUnownInput=function(){const name=this.search.value.value,targetID=this.main.player.unownState;if(name.trim())window.dexActions.push({type:'unown',name,targetID});handle.call(this)};scene.render();`;
const click = (reference, native) => async ({ kind, browser }) => { await browser("click", kind === "reference" ? reference : native); };
const clickTab = index => click(`.dex-scene-tab:nth-child(${index})`, `.pokepath-classic-dex>nav>button:nth-child(${index})`);
const clickRow = index => click(`.dex-scene-entry:nth-child(${index}) .dex-scene-entry-name`, `.pokepath-dex-list article:nth-child(${index})>button`);
const nextSort = click(".dex-scene-sort-arrow:last-child", ".pokepath-dex-sort>button:last-child");
const search = value => async ({ kind, browser, evaluate }) => {
  await browser("fill", kind === "reference" ? ".dex-scene-search" : ".pokepath-dex-search input", value);
  await evaluate("document.activeElement.blur();true");
};
const steps = [{ name: "tutorials" }, { name: "tutorial-art-bottom", action: clickRow(2) }, { name: "switch-to-towers", action: clickTab(2) }, { name: "owned-tower-detail", action: clickRow(1) }];
for (let index = 1; index <= 13; index++) steps.push({ name: `tower-sort-${index}`, action: nextSort });
steps.push({ name: "tower-alias-search", action: search("fire guard") }, { name: "tower-clear-search", action: search("") }, { name: "enemy-list", action: clickTab(3) }, { name: "locked-enemy-detail", action: clickRow(1) });
for (let index = 1; index <= 18; index++) steps.push({ name: `enemy-sort-${index}`, action: nextSort });
steps.push({ name: "search-enemy-id", action: search("rookidee") }, { name: "enemy-clear-search", action: search("") }, { name: "tutorial-secret-panel", action: async value => { await clickTab(1)(value); await clickRow(3)(value); } }, { name: "tutorial-secret-action", action: click(".dex-scene-tutorial-image-bottom", ".pokepath-dex-illustration.bottom") });
for (const step of steps) step.when = parameters => parameters.variant === "base";
steps.push({ name: "nox-open", when: parameters => parameters.variant === "nox", action: clickTab(2) }, { name: "nox-owned", when: parameters => parameters.variant === "nox", action: clickRow(1) }, { name: "unown-open", when: parameters => parameters.variant === "unown", action: clickTab(2) }, { name: "unown-question", when: parameters => parameters.variant === "unown", action: clickRow(4) }, { name: "unown-first-answer", when: parameters => parameters.variant === "unown", action: async ({ kind, browser }) => { await browser("fill", kind === "reference" ? ".dex-scene-search" : ".pokepath-dex-search input", "yes"); await browser("press", "Enter"); } });
const verifyNative = async ({ browser, evaluate, ready }) => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const screenshot = join(root, ".local/pokepath/verification/dex-render/mobile-unown.png");
  await browser("set", "viewport", "320", "674");
  await browser("reload");
  await ready();
  const tap = async selector => {
    const point = await evaluate(`(()=>{const node=document.querySelector(${JSON.stringify(selector)}),r=node.getBoundingClientRect(),x=Math.round(r.left+r.width/2),y=Math.round(r.top+r.height/2);return{x,y,w:r.width,h:r.height,visible:node.contains(document.elementFromPoint(x,y))}})()`);
    assert.ok(point.w >= 24 && point.h >= 24 && point.x >= 0 && point.x < 320 && point.y < 674 && point.visible, JSON.stringify({ selector, ...point }));
    const endpoint = (await browser("get", "cdp-url")).cdpUrl;
    const origin = await evaluate("location.origin");
    execFileSync(process.execPath, [join(root, "scripts/browser_touch.mjs"), endpoint, origin, "tap", String(point.x), String(point.y), String(point.x), String(point.y)], { cwd: root, env: { ...process.env, TMPDIR: join(root, ".local/tmp") }, encoding: "utf8", timeout: 15000 });
    await ready();
    return point;
  };
  const tabs = await tap(".pokepath-classic-dex>nav>button:nth-child(2)");
  const unown = await tap('[data-dex-species="unown"]>button');
  await browser("fill", ".pokepath-dex-search input", "yes");
  const count = await evaluate("document.querySelectorAll('[data-dex-species]').length");
  assert.ok(count > 1, "answer text filtered the Dex instead of staying in the dialogue");
  const submit = await tap(".pokepath-dex-submit");
  const actions = await evaluate("window.dexActions");
  assert.deepEqual(actions, [{ type: "unown", name: "yes", targetID: 0 }]);
  assert.equal(await evaluate("document.documentElement.scrollWidth<=innerWidth"), true);
  await browser("screenshot", screenshot);
  return { passed: true, screenshot, actions, width: 320, targets: { tabs, unown, submit } };
};
const selectors = {
  reference: { tabs: ".dex-scene-tab", sort: ".dex-scene-sort-value", items: ".dex-scene-entry", name: ".dex-scene-entry-name", status: ".dex-scene-entry-level", rows: ".dex-scene-entry-data-row", description: ".dex-scene-entry-description-text, .dex-scene-tutorial-text", image: ".dex-scene-tutorial-image-bottom, .dex-scene-tutorial-image-top" },
  native: { tabs: ".pokepath-classic-dex>nav>button", sort: ".pokepath-dex-sort-value", items: ".pokepath-dex-list article", name: "strong", status: "small", rows: ".pokepath-dex-expanded dl > div", description: ".pokepath-dex-description>div:last-child, .pokepath-dex-tutorial p", image: ".pokepath-dex-illustration" }
};
const measure = Object.fromEntries(Object.entries(selectors).map(([kind, sel]) => [kind, `(()=>{const s=${JSON.stringify(sel)},visible=element=>element?.getBoundingClientRect().width>0,text=node=>node?.textContent??'',query=selector=>[...document.querySelectorAll(selector)],d=document.querySelector(s.description),image=document.querySelector(s.image);return{tabs:query(s.tabs).map(text),sort:visible(document.querySelector(s.sort))?text(document.querySelector(s.sort)):'',items:query(s.items).map(entry=>({name:text(entry.querySelector(s.name)),status:text(entry.querySelector(s.status)),locked:entry.classList.contains('locked')})),rows:query(s.rows).map(row=>[text(row.firstElementChild),text(row.lastElementChild),Number(row.dataset.rank??(row.classList.contains('rank-gold')?1:row.classList.contains('rank-silver')?2:row.classList.contains('rank-bronze')?3:0))]),description:d?[...d.children].map(text).map(line=>line.trim()):[],image:image&&d?Boolean(d.compareDocumentPosition(image)&Node.DOCUMENT_POSITION_FOLLOWING):null,actions:window.dexActions}})()`]));
await verifyDOMOracle({ name: "dex-render", command: "node games/pokepath/tests/dex-render-oracle.mjs", sources: [...sources.map(source => source.identity), { path: "game/data/tagsData.js", sha256: domSHA(tagsSource) }, { path: "game/data/tutorialData.js", sha256: domSHA(tutorialSource) }], styles: [], reference, native: 'import "./games/pokepath/tests/DexFixture.tsx";', input, cases: [{ language: "8", theme: "original", font: "PressStart2P", variant: "base" }, { language: "8", theme: "original", font: "PressStart2P", variant: "nox" }, { language: "8", theme: "original", font: "Unown", variant: "unown" }], steps, measure, verifyNative });
