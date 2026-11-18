import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const sources = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass("utils/GameScene.js", "GameScene", ["constructor"]),
  referenceDOMClass("game/scenes/DraftScene.js", "DraftScene", ["render", "update", "generatePicks"])
]);
const box = [
  { id: 73, species: "spoink", alias: "Kite" },
  { id: 57, species: "charmander", sprite: { base: "src/assets/images/pokemon/shiny/charmander.png" } },
  { id: 94, species: "frillishM" }
];
const input = { view: { route: 0, team: [], box, progression: { draft: { active: true, round: 0, rounds: 6, token: 51, offers: box.map(pokemon => pokemon.id), canReroll: true }, settings: {} } } };
const reference = `const{catalog,dictionary,parameters,input}=window.oracleData;
const text=dictionary.text,playSound=()=>{};
${sources.map(entry => entry.code).join("\n")}
const short=parameters.variant==='short',value=input.view.progression.draft;
const world=new GameScene(400,200),scene=Object.assign(Object.create(DraftScene.prototype),world);
const owned=input.view.box.map(p=>{const s=catalog.species[p.species];return{owned:p.id,id:s.id,name:s.names,alias:p.alias,specie:s,tiles:s.terrains,ability:{id:s.abilityId},sprite:{base:'./'+(p.sprite?.base??s.sprite.base)}}});
window.draftReceipts=[];
scene.header.removeChild(scene.closeButton);
scene.round=short?5:value.round;
scene.roundMax=value.rounds;
scene.canReroll=!short;
scene.pool=short?owned.slice(0,2):owned;
scene.main={lang:Number(parameters.language),area:{map:{id:0}},pokemonScene:{open(pokemon,position,options,blocked){if(pokemon)window.draftReceipts.push({type:'details',pokemonID:pokemon.owned,blocked})}}};
scene.selectPick=index=>{if(scene.currentPicks[index])window.draftReceipts.push({type:'draftPick',pick:index,targetID:value.token})};
scene.reroll=()=>window.draftReceipts.push({type:'draftReroll',targetID:value.token});
scene.restart=()=>window.draftReceipts.push({type:'draftRestart',targetID:value.token});
scene.render();
const random=Math.random;
try{Math.random=()=>.9999;scene.update()}finally{Math.random=random}
const viewport=document.createElement('div'),root=document.createElement('div'),screen=document.createElement('div');
viewport.id='view-port';root.id='screen-root';screen.id='screen';root.style.transform='scale(1)';
screen.append(document.querySelector('.section-scene-container'),scene.background);root.append(screen);viewport.append(root);document.body.append(viewport);`;

const selectors = {
  reference: { outer: ".game-scene-window", title: ".game-scene-title", round: ".tutorial-scene-title", names: ".draft-scene-pokemon-label", offer: ".draft-scene-pokemon", info: ".draft-scene-pokemon-data", reroll: ".draft-scene-reroll-button", restart: ".draft-scene-restart-button" },
  native: { outer: ".pokepath-draft", title: ".pokepath-draft-header span", round: ".pokepath-draft-round", names: ".pokepath-draft-choice span", offer: ".pokepath-draft-choice", info: ".pokepath-draft-info", reroll: "[data-draft-reroll]", restart: "[data-draft-restart]" }
};
const clicked = (key, slot = 0) => async ({ kind, browser, evaluate }) => {
  const selector = selectors[kind][key];
  const point = await evaluate(`(()=>{const node=document.querySelectorAll(${JSON.stringify(selector)})[${slot}],r=node.getBoundingClientRect();return [Math.round(r.left+r.width/2),Math.round(r.top+r.height/2)]})()`);
  await browser("mouse", "move", ...point.map(String));
  await browser("mouse", "down", "left");
  await browser("mouse", "up", "left");
};
const measure = table => `(()=>{const s=${JSON.stringify(table)},find=key=>document.querySelector(s[key]),all=key=>[...document.querySelectorAll(s[key])],text=node=>node?.innerText?.trim()??'',size=node=>{const r=node.getBoundingClientRect();return [r.x,r.y,r.width,r.height]};return{title:text(find('title')),round:text(find('round')),names:all('names').map(text),info:all('info').map(text),reroll:[text(find('reroll')),getComputedStyle(find('reroll')).filter],restart:text(find('restart')),bounds:size(find('outer')),actions:window.draftReceipts}})()`;

const verifyNative = async ({ browser, evaluate, ready }) => {
  const url = new URL(await evaluate("location.href"));
  await browser("set", "viewport", "320", "674");
  await browser("open", url.toString());
  await ready();
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const cdp = (await browser("get", "cdp-url")).cdpUrl;
  const tap = async selector => {
    const point = await evaluate(`(()=>{const node=document.querySelector(${JSON.stringify(selector)}),r=node.getBoundingClientRect(),x=Math.round(r.x+r.width/2),y=Math.round(r.y+r.height/2);return {x,y,hit:node.contains(document.elementFromPoint(x,y))}})()`);
    assert.ok(point.hit && point.x > 0 && point.x < 320 && point.y >= 0 && point.y < 674, JSON.stringify({ selector, point }));
    execFileSync(process.execPath, ["scripts/browser_touch.mjs", cdp, url.toString(), "tap", String(point.x), String(point.y), String(point.x), String(point.y)], { cwd: root, env: { ...process.env, TMPDIR: `${root}.local/tmp` }, timeout: 15000 });
    return point;
  };
  const fit = await evaluate("(()=>{const r=document.querySelector('.pokepath-draft').getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&document.documentElement.scrollWidth===innerWidth})()");
  assert.equal(fit, true);
  const touch = await tap("[data-draft-slot='1'] .pokepath-draft-choice");
  assert.deepEqual(await evaluate("window.draftReceipts"), [{ type: "draftPick", pick: 1, targetID: 51 }]);
  url.searchParams.set("variant", "spectator");
  await browser("open", url.toString());
  await ready();
  const disabled = await evaluate("Array.from(document.querySelectorAll('.pokepath-draft-choice,[data-draft-reroll],[data-draft-restart]')).every(node=>node.disabled)");
  assert.equal(disabled, true);
  await tap("[data-draft-slot='0'] .pokepath-draft-info");
  assert.deepEqual(await evaluate("window.draftReceipts"), [{ type: "details", pokemonID: 73, blocked: true }]);
  return { width: 320, touch, viewerActionsDisabled: disabled, viewerReadsDetails: true };
};
await verifyDOMOracle({
  name: "draft-render", command: "node games/pokepath/tests/draft-render-oracle.mjs",
  sources: sources.map(entry => entry.identity), styles: [], reference,
  native: 'import "./games/pokepath/tests/DraftFixture.tsx";', input, platformStyles: true,
  cases: [
    { language: "8", theme: "original", font: "PressStart2P", variant: "complete" },
    { language: "2", theme: "halloween", font: "Lexend", variant: "short" },
    { language: "0", theme: "classic", font: "Rubik", variant: "complete" }
  ],
  steps: [
    { name: "opened" },
    { name: "details", action: clicked("info", 0) },
    { name: "choice", action: clicked("offer", 1) },
    { name: "reroll", action: clicked("reroll") },
    { name: "restart", action: clicked("restart") }
  ],
  measure: { reference: measure(selectors.reference), native: measure(selectors.native) },
  verifyNative
});
