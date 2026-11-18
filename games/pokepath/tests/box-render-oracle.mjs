import assert from "node:assert/strict";
import { referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const sources = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass("utils/Input.js", "Input", ["constructor", "render", "renderTypeText"]),
  referenceDOMClass("game/scenes/BoxScene.js", "BoxScene", ["render", "update", "searchByName", "displayUnits", "applyTabEffect", "displayPokemon", "sortUnits", "changesort", "changeView", "getBoxAttackShapeSymbol", "updateCollectionStats", "applyUnitBackgroundScale"])
]);
const member = (id, species, level, power, attackMS, range, criticalPct, item = "", attributes = {}) => ({ id, species, level, target: "first", item, tileID: -1, stats: { power, attackMS, range, criticalPct }, ...attributes });
const input = { chosen: 481, team: [member(481, "charmander", 19, 27, 1200, 100, 8, "protein"), member(278, "treecko", 17, 16, 810, 90, 13, "", { favorite: true })], box: [member(701, "froaki", 3, 9, 1010, 70, 2, "", { alias: "Ripple", shiny: true }), member(553, "pikachu", 11, 22, 905, 80, 9), member(820, "kabuto", 30, 40, 1800, 120, 20, "leftovers")] };
const reference = `
 const {catalog,dictionary,parameters,input}=window.oracleData,text=dictionary.text,abilityData=Object.fromEntries(Object.entries(catalog.abilities).map(([key,value])=>[key,{...value,name:value.names??value.name}])),playSound=()=>{};
 const allPokemon=Array(catalog.collection.length),sort=['team','alphabetical','level','ability','grass','water','mountain','power','speed','critical','range','shiny','acquisition'],TAB_CONTENT=['allTab','fieldTab','grassTab','waterTab','mountainTab','fossilTab'];
 ${sources.map(source=>source.code).join("\n")}
 const adapt=(owned,inGroup)=>{const specie=catalog.species[owned.species],ability={...abilityData[specie.abilityId],id:specie.abilityId},item=catalog.items[owned.item];return{...owned,ownedID:owned.id,id:specie.id,inGroup,isShiny:Boolean(owned.shiny),favorite:Boolean(owned.favorite),specie,ability,name:specie.names,lvl:owned.level,power:owned.stats.power,speed:owned.stats.attackMS,range:owned.stats.range,critical:owned.stats.criticalPct,tiles:specie.terrains,attackType:specie.attackType,rangeType:specie.rangeType,item:item?{...item,sprite:'./'+item.sprite}:undefined,acquisitionDate:owned.id,sprite:{base:'./'+specie.sprite.base}}};
 const team=input.team.map(pokemon=>adapt(pokemon,true)),box=input.box.map(pokemon=>adapt(pokemon,false)),pokemon=[...team,...box];
 const main={lang:Number(parameters.language),team:{pokemon:team},box:{pokemon:box},player:{teamSlots:6,stats:{pokemonOwned:pokemon.length},shinyAmount:pokemon.filter(p=>p.isShiny).length},area:{inChallenge:{}},indicatorShape:true,checkShinyCount(){}};
 const viewport=document.createElement('div'),root=document.createElement('div'),screen=document.createElement('div');
 viewport.id='view-port';root.id='screen-root';screen.id='screen';root.style.transform='scale(1)';
 screen.append(document.querySelector('.section-scene-container'));root.append(screen);viewport.append(root);document.body.append(viewport);
 const scene=Object.assign(Object.create(BoxScene.prototype),{main,container:document.querySelector('.section-scene-container'),pokemon,searchPokemon:pokemon.slice(),selected:pokemon.find(p=>p.ownedID===input.chosen),sorted:0,tabSelected:0,size:[44,50,58,67],sizePosition:0,setupUnitDragAndDrop(){}});
 window.boxReference=scene;
 scene.render();scene.update();
`;
const tables = {
  reference: { card: ".box-scene-unit", name: ".box-scene-unit-selected-name", label: ".box-scene-unit-text", sorting: ".box-scene-sort-value", search: ".box-scene-search", terrain: ".box-scene-tab", active: ".box-scene-tab.is-active", count: ".box-scene-collection-stat", button: ".box-scene-button", size: ".box-scene-view-button", next: ".box-scene-sort-arrow:last-child", before: ".box-scene-sort-arrow:first-child", favorite: ".box-scene-favorite-button" },
  native: { card: "[data-box-pokemon]", name: "[data-box-name]", label: ".pokepath-box-unit-label", sorting: "[data-box-sort]", search: "[data-box-search]", terrain: "[data-box-terrain]", active: "[data-box-terrain][aria-pressed=true]", count: "[data-box-count]", button: "[data-box-command]", size: "[data-box-size]", next: "[data-box-next]", before: "[data-box-previous]", favorite: "[data-box-favorite]" }
};
const probe = kind => `(()=>{const table=${JSON.stringify(tables[kind])},native=${kind === "native"},all=key=>[...document.querySelectorAll(table[key])],one=key=>document.querySelector(table[key]),css=node=>getComputedStyle(node),rect=node=>{const r=node.getBoundingClientRect();return[r.x,r.y,r.width,r.height]},text=node=>node?.innerText?.trim()??'';return {title:text(one('name')),sort:text(one('sorting')),count:all('count').map(text),buttons:all('button').map(node=>[text(node),css(node).filter,rect(node)]),active:text(one('active')),icons:all('card').filter(node=>node.getBoundingClientRect().width>0).map((node,index)=>[native?Number(node.dataset.boxPokemon):window.boxReference.searchPokemon[index].ownedID,text(node.querySelector(table.label)),css(node).filter,rect(node)])}})()`;
const click = (key, index = 0) => async ({ kind, browser }) => { await browser("click", `${tables[kind][key]}${index ? `:nth-child(${index + 1})` : ""}`); };
const fill = value => async ({ kind, browser }) => {
  if (value) await browser("fill", tables[kind].search, value);
  else { await browser("focus", tables[kind].search); await browser("press", "Home"); await browser("press", "Shift+End"); await browser("press", "Backspace"); }
  await browser("mouse", "move", "974", "643"); await browser("mouse", "down", "left"); await browser("mouse", "up", "left");
};
const steps = [{ name: "open" }, { name: "grass-dims", action: click("terrain", 2) }, { name: "fossil-dims", action: click("terrain", 5) }, { name: "all", action: async ({ kind, browser }) => { await browser("click", `${tables[kind].terrain}:first-child`); } }, { name: "select-reserve", action: click("card", 2) }, { name: "search-nickname", action: fill("Ripple") }, { name: "search-none", action: fill("zxq") }, { name: "clear-search", action: fill("") }];
for (let number = 0; number < 12; number++) steps.push({ name: `sort-${number + 1}`, action: click("next"), when: parameters => parameters.matrix === "all" || number < 3 });
steps.push({ name: "sort-wrap", action: click("next"), when: parameters => parameters.matrix === "all" });
for (let number = 0; number < 4; number++) steps.push({ name: `size-${number + 1}`, action: click("size"), when: parameters => parameters.matrix === "all" || number === 0 });

const verifyNative = async ({ browser, evaluate, ready }) => {
  await browser("click", "[data-box-pokemon='701']");
  await browser("click", "[data-box-name]");
  await browser("fill", "[data-box-rename-input]", "Rain Drop");
  await browser("click", "[data-box-rename-accept]");
  await ready();
  assert.match(await evaluate("document.querySelector('[data-box-name]').innerText"), /RAIN DROP/);
  await browser("click", "[data-box-favorite]");
  await browser("click", "[data-box-command='team']");
  await ready();
  await browser("click", "[data-box-command='box']");
  await ready();
  const points = await evaluate("(()=>{const center=s=>{const r=document.querySelector(s).getBoundingClientRect();return[Math.round(r.left+r.width/2),Math.round(r.top+r.height/2)]};return{origin:center('[data-box-pokemon=\"701\"]'),end:center('[data-party-slot=\"0\"]')}})()");
  await browser("mouse", "move", ...points.origin.map(String)); await browser("mouse", "down", "left");
  await browser("mouse", "move", String(points.origin[0] - 12), String(points.origin[1]));
  await browser("mouse", "move", ...points.end.map(String)); await browser("mouse", "up", "left");
  const actions = await evaluate("window.boxActions");
  assert.deepEqual(actions, [{ type: "rename", pokemonID: 701, name: "Rain Drop" }, { type: "favorite", pokemonID: 701, enabled: true }, { type: "team", pokemonID: 701, mode: "team" }, { type: "team", pokemonID: 701, mode: "box" }, { type: "teamSlot", pokemonID: 701, slot: 0, targetID: 481 }]);
  return { actions };
};

await verifyDOMOracle({ name: "box-render", command: "node games/pokepath/tests/box-render-oracle.mjs", sources: sources.map(source => source.identity), styles: [".local/pokepath/rejected-embedding/assets/src/css/box.css", ".local/pokepath/rejected-embedding/assets/src/css/inventory.css"], reference, input, native: 'import "./games/pokepath/tests/BoxFixture.tsx";', platformStyles: true, cases: [{ language: "8", theme: "original", font: "PressStart2P", matrix: "all" }, { language: "2", theme: "classic", font: "Lexend", matrix: "compact" }, { language: "0", theme: "halloween", font: "Rubik", matrix: "compact" }], steps, measure: { native: probe("native"), reference: probe("reference") }, verifyNative });
