import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const source = "game/scenes/ChallengeScene.js";
const sources = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass(source, "ChallengeScene", ["render", "update", "selectChallenge", "addChallenge", "displayRibbon"]),
  referenceDOMClass(source, "RewardChallengeScene", ["constructor", "render", "update"])
]);
const choices = { lvlCap: [false, 75, 50], slotLimit: [false, 8, 6], toughEnemies: [false, 25, 50], draft: [false, true], noItems: [false, true], permadeath: [false, true], mirror: [false, true] };
const idle = { levelCap: 0, slotLimit: 0, toughEnemies: 0, draft: false, noItems: false, permadeath: false, mirror: false };
const input = { view: { route: 0, stars: 540, health: 14, wave: 1, waveActive: false, paused: false, team: [], box: [], items: [], records: { 0: 100 }, progression: { challenge: idle, canChallenge: true, currentRibbons: 2, ribbons: { 0: { levelCap: 1, draft: 1 } }, claims: {}, rewardThresholds: [2, 3, 5, 1], settings: {} } } };
const reference = `const{catalog,dictionary,parameters,input}=window.oracleData;
const CHALLENGES_OPTION=${JSON.stringify(choices)},CHALLENGES_LIST=Object.keys(CHALLENGES_OPTION),text=dictionary.text,playSound=()=>{};
const itemData=Object.fromEntries(Object.entries(catalog.items).map(([id,item])=>[id,{...item,isEquipable:item.equipable,sprite:'./'+item.sprite}]));
const pokemonData=catalog.species;
${sources.map(entry => entry.code).join("\n")}
const view=input.view,variant=parameters.variant,active=variant==='active';
const scene=Object.create(ChallengeScene.prototype);
const world=catalog.routes[String(view.route)];
const titleToRule=key=>key==='lvlCap'?'levelCap':key;
const levels=variant==='ready'?{levelCap:2,slotLimit:1,mirror:1,noItems:1}:variant==='empty'?{}:view.progression.ribbons[view.route];
const challenges=Object.fromEntries(Object.entries(levels).map(([key,count])=>[key==='levelCap'?'lvlCap':key,{[view.route]:Array(count).fill(true)}]));
const route={...world,name:world.names,challengeReward:world.challengeRewards};
const rewards=[0,1,2,3].map((index)=>({[view.route]:variant==='ready'&&index!==2}));
scene.container=document.querySelector('.section-scene-container');
scene.challenges=Object.fromEntries(CHALLENGES_LIST.map((key,index)=>[key,active?CHALLENGES_OPTION[key].at(-1):false]));
scene.challengesIndex=CHALLENGES_LIST.map(key=>CHALLENGES_OPTION[key].indexOf(scene.challenges[key]));
scene.challengeSelected=0;
scene.main={lang:Number(parameters.language),area:{routeNumber:view.route,map:route,inChallenge:active},player:{challenges,rewards,items:[]},team:{pokemon:[]},box:{pokemon:variant==='ready'?[{specie:pokemonData.bidoof}]:[]},tooltip:{bindTo(){}},challengeScene:scene};
window.challengeActions=[];
scene.startChallenge=()=>{
 if(variant==='spectator')return;
 if(active)window.challengeActions.push({type:'surrender'});
 else window.challengeActions.push({type:'challenge',settings:Object.fromEntries(Object.entries(scene.challenges).map(([key,value])=>[titleToRule(key),typeof CHALLENGES_OPTION[key][1]==='number'?Number(value):value]))});
 window.challengeActions.push({type:'close'});
};
scene.render();
scene.displayRibbon();
scene.update();
const rewardScene=new RewardChallengeScene(scene.main,scene);
const claim=slot=>{
 if(variant==='spectator'||scene.ribbonsMap<rewardScene.prices[slot]||variant==='ready'&&[1,3].includes(slot))return;
 window.challengeActions.push({type:'claimReward',slot});
};
rewardScene.claimItem=claim;
rewardScene.claimPokemon=claim;
rewardScene.update();`;

const selectors = {
  reference: { row: ".challenge-scene-list-row", title: ".challenge-scene-route-name", infoTitle: ".challenge-scene-info-title", infoText: ".challenge-scene-info-text", warning: ".challenge-warning,.challenge-debuff", submit: ".challenge-scene-start-button", reward: ".reward-scene-item", claim: ".reward-scene-claim", state: ".challenge-scene-list-row-result", ribbon: ".challenge-scene-list-row-ribbon", label: ".challenge-scene-list-row-label" },
  native: { row: "[data-challenge-rule]", title: ".pokepath-challenge-route", infoTitle: ".pokepath-challenge-info h2", infoText: ".pokepath-challenge-info p", warning: ".pokepath-challenge-warning,.pokepath-challenge-debuff", submit: "[data-challenge-submit]", reward: "[data-challenge-reward]", claim: ".pokepath-challenge-claim", state: ".pokepath-challenge-result", ribbon: ".pokepath-challenge-ribbons > span", label: ".pokepath-challenge-label" }
};
const action = (field, index, event = "click") => async ({ kind, browser, evaluate }) => {
  const selector = selectors[kind][field];
  const point = await evaluate(`(()=>{const target=document.querySelectorAll(${JSON.stringify(selector)})[${index}],r=target.getBoundingClientRect();return [Math.round(r.left+r.width/2),Math.round(r.top+r.height/2)]})()`);
  await browser("mouse", "move", ...point.map(String));
  if (event !== "hover") { await browser("mouse", "down", "left"); await browser("mouse", "up", "left"); }
};
const measure = table => `(()=>{const s=${JSON.stringify(table)},find=key=>document.querySelector(s[key]),all=(node,key)=>[...node.querySelectorAll(s[key])],text=node=>node?.innerText?.replace(/\u00a0/g,' ').trim()??'',plain=node=>text(node).replace(/\\s+/g,' ');const submit=find('submit');return{title:text(find('title')),rows:all(document,'row').map(row=>({label:text(row.querySelector(s.label)),state:text(row.querySelector(s.state)),ribbons:all(row,'ribbon').map(icon=>getComputedStyle(icon).filter==='none')})),info:[text(find('infoTitle')),plain(find('infoText')),plain(find('warning'))],submit:[text(submit),getComputedStyle(submit).filter],rewards:all(document,'reward').map(node=>[text(node.querySelector(s.claim)),node.rewardIndex??Number(node.dataset.challengeReward),getComputedStyle(node).filter]),actions:window.challengeActions}})()`;
const layout = table => `(()=>{const s=${JSON.stringify(table)},keys=['title','row','ribbon','infoTitle','infoText','reward'];const result={};for(const key of keys){const node=document.querySelector(s[key]),rect=node.getBoundingClientRect(),style=getComputedStyle(node);result[key]={box:[rect.left,rect.top,rect.width,rect.height],properties:Object.fromEntries(['position','display','lineHeight','backgroundSize','backgroundPosition','filter','imageRendering','textShadow'].map(prop=>[prop,style[prop]]))}}return result})()`;
const mobile = async ({ browser, evaluate, ready }) => {
  await browser("set", "viewport", "320", "674");
  await ready();
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const endpoint = (await browser("get", "cdp-url")).cdpUrl;
  const origin = await evaluate("location.href");
  assert.equal(await evaluate("(()=>{const node=document.querySelector('.pokepath-classic-challenge');return node.clientWidth===node.scrollWidth})()"), true);
  const snapshots = [];
  for (const selector of ['[data-challenge-rule="draft"]', ".pokepath-classic-challenge .pokepath-central-close"]) {
    const point = await evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)}),r=element.getBoundingClientRect();return {x:Math.round(r.left+r.width/2),y:Math.round(r.top+r.height/2)}})()`);
    assert.ok(point.x > 0 && point.x < 320 && point.y >= 44 && point.y < 674, JSON.stringify({ selector, point }));
    execFileSync(process.execPath, ["scripts/browser_touch.mjs", endpoint, origin, "tap", String(point.x), String(point.y), String(point.x), String(point.y)], { cwd: root, env: { ...process.env, TMPDIR: `${root}.local/tmp` }, timeout: 15000 });
    snapshots.push({ selector, point });
  }
  assert.match(await evaluate("document.querySelector('.pokepath-challenge-info h2').textContent"), /Draft/i);
  assert.equal(await evaluate("document.querySelector('[data-challenge-rule=draft] .pokepath-challenge-result').textContent.toUpperCase()"), "ON");
  assert.equal(await evaluate("window.challengeActions.at(-1).type"), "close");
  return { width: 320, snapshots };
};
await verifyDOMOracle({
  name: "challenge-render", command: "node games/pokepath/tests/challenge-render-oracle.mjs",
  sources: sources.map(entry => entry.identity),
  styles: [".local/pokepath/rejected-embedding/assets/src/css/challenge.css"],
  reference, native: 'import "./games/pokepath/tests/ChallengeFixture.tsx";', input,
  cases: [{ language: "8", theme: "original", font: "PressStart2P", variant: "empty" }, { language: "2", theme: "halloween", font: "Lexend", variant: "ready" }, { language: "0", theme: "classic", font: "Rubik", variant: "active" }, { language: "0", theme: "original", font: "PressStart2P", variant: "normal" }],
  steps: [
    { name: "initial" },
    { name: "locked-reward", when: p => p.variant === "empty", action: action("reward", 0) },
    { name: "level-75", when: p => p.variant === "normal", action: action("row", 0) },
    { name: "level-50", when: p => p.variant === "normal", action: action("row", 0) },
    { name: "slot-eight", when: p => p.variant === "normal", action: action("row", 1) },
    { name: "tough-hover", when: p => p.variant === "normal", action: action("row", 2, "hover") },
    { name: "tough-25", when: p => p.variant === "normal", action: action("row", 2) },
    { name: "no-items", when: p => p.variant === "normal", action: action("row", 4) },
    { name: "start", when: p => p.variant === "normal", action: action("submit", 0) },
    { name: "reclaim-removed-item", when: p => p.variant === "ready", action: action("reward", 1) },
    { name: "claimed-pokemon", when: p => p.variant === "ready", action: action("reward", 2) },
    { name: "fixed-rules", when: p => p.variant === "active", action: action("row", 2) },
    { name: "surrender", when: p => p.variant === "active", action: action("submit", 0) }
  ], measure: { reference: measure(selectors.reference), native: measure(selectors.native) }, layout: { reference: layout(selectors.reference), native: layout(selectors.native) }, verifyNative: mobile
});
