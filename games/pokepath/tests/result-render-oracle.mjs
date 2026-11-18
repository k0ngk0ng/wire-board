import { referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const sources = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass("utils/GameScene.js", "GameScene", ["constructor"]),
  referenceDOMClass("game/scenes/FinalScene.js", "FinalScene", ["render", "update", "getText"]),
  referenceDOMClass("game/scenes/DefeatScene.js", "DefeatScene", ["render", "update", "getRetryWave", "getRetryText"])
]);
const team = [{ id: 9, species: "charmander" }, { id: 88, species: "pikachu" }, { id: 73, species: "spoink" }];
const input = { view: { progression: { settings: {}, challenge: { levelCap: 0, slotLimit: 0, toughEnemies: 0, draft: false, noItems: false, permadeath: false, mirror: false } }, stars: 450, team } };
const reference = `
 const {catalog,dictionary,parameters,input}=window.oracleData,text=dictionary.text,playSound=()=>{};
 const win=parameters.phase==='route_complete',practice=parameters.phase==='practice_complete',full=parameters.challenge==='full';
 const route=full&&win?21:0,wave=practice?1:Number(parameters.wave);
 const enemy=key=>{const item=catalog.enemies[key];return {...item,name:item.names,sprite:{base:'./'+item.sprite.base}}};
 const BOSS={0:enemy('shaymin'),21:enemy('heatran')};
 const CHALLENGES_LIST=['lvlCap','slotLimit','toughEnemies','draft','noItems','permadeath',...(win?['mirror']:[])];
 ${sources.map(source=>source.code).join("\n")}
 const world=win?new GameScene(440,230):new GameScene(340,300);
 const scene=Object.assign(Object.create((win?FinalScene:DefeatScene).prototype),world);
 const health=win||practice&&parameters.wave==='1'?12:0;
 const rules={lvlCap:50,slotLimit:6,toughEnemies:50,draft:true,noItems:false,permadeath:true,mirror:true};
 scene.main={lang:Number(parameters.language),uiTheme:parameters.theme==='halloween'?3:0,game:{chrono:{stop(){}}},area:{routeNumber:route,waveNumber:wave,map:{id:route,name:catalog.routes[route].names},inChallenge:full?rules:false,towers:[],isCustom:practice},player:{stars:input.view.stars,health:{[route]:health},obtainChallengeRibbonsFromObject(){}},team:{pokemon:input.view.team.map(pokemon=>({sprite:{base:'./'+catalog.species[pokemon.species].sprite.base}}))}};
 window.resultActions=[];
 scene.close=()=>window.resultActions.push({type:'continueRoute'});
 world.close=scene.close;
 scene.restart=()=>window.resultActions.push(practice?{type:'custom_retry'}:{type:'retry',mode:'restart'});
 scene.retry=()=>window.resultActions.push({type:'retry',mode:'retry'});
 if(!win){scene.header.removeChild(scene.closeButton);scene.saveWaveRetry=[25,50,75]}
 scene.render();scene.update();
 const viewport=document.createElement('div'),root=document.createElement('div'),screen=document.createElement('div');
 viewport.id='view-port';root.id='screen-root';screen.id='screen';root.style.transform='scale(1)';
 screen.append(document.querySelector('.section-scene-container'),scene.background);root.append(screen);viewport.append(root);document.body.append(viewport);
`;
const selectors = {
  reference: { outer: ".game-scene-window", title: ".game-scene-title", prompt: ".defeat-scene-prompt", text: ".defeat-scene-info", rows: ".final-challenge", hall: ".final-hof", image: '[class*="scene-image"]', close: ".game-scene-close-button", restart: ".defeat-restart-button", retry: ".defeat-retry-button" },
  native: { outer: ".pokepath-result", title: ".pokepath-result-title", prompt: ".pokepath-result-prompt", text: ".pokepath-result-text", rows: ".pokepath-result-rule", hall: ".pokepath-result-member", image: ".pokepath-result-portrait", close: "[data-result-continue]", restart: "[data-result-restart]", retry: "[data-result-retry]" }
};
const measure = keys => `(()=>{const s=${JSON.stringify(keys)},first=key=>document.querySelector(s[key]),all=key=>[...document.querySelectorAll(s[key])],text=node=>node?.innerText?.replace(/\\s+/g,' ').trim()??'',rect=element=>[element.getBoundingClientRect().width,element.getBoundingClientRect().height];const retry=first('retry');return {title:text(first('title')),prompt:text(first('prompt')),text:text(first('text')),rows:all('rows').map(element=>[text(element),getComputedStyle(element).color]),hall:all('hall').length,image:rect(first('image')),frame:rect(first('outer')),controls:[text(first('close')),text(first('restart')),text(retry),Boolean(retry&&(retry.disabled||getComputedStyle(retry).pointerEvents==='none'))],actions:window.resultActions}})()`;
const click = key => async ({ kind, browser, evaluate }) => {
  const selector = selectors[kind][key];
  const point = await evaluate(`(()=>{const node=document.querySelector(${JSON.stringify(selector)}),r=node.getBoundingClientRect();return [Math.round(r.x+r.width/2),Math.round(r.y+r.height/2)]})()`);
  await browser("mouse", "move", ...point.map(String));
  await browser("mouse", "down", "left"); await browser("mouse", "up", "left");
};
await verifyDOMOracle({
  name: "result-render", command: "node games/pokepath/tests/result-render-oracle.mjs",
  sources: sources.map(source => source.identity), styles: [], input, reference,
  native: 'import "./games/pokepath/tests/ResultFixture.tsx";',
  platformStyles: true,
  cases: [
    { language: "8", theme: "original", font: "PressStart2P", phase: "route_complete", wave: "100", challenge: "none" },
    { language: "0", theme: "classic", font: "Rubik", phase: "route_complete", wave: "100", challenge: "full" },
    { language: "8", theme: "original", font: "PressStart2P", phase: "defeat", wave: "4", challenge: "none" },
    { language: "2", theme: "classic", font: "Lexend", phase: "defeat", wave: "51", challenge: "none" },
    { language: "0", theme: "halloween", font: "Rubik", phase: "defeat", wave: "100", challenge: "full" },
    { language: "0", theme: "original", font: "PressStart2P", phase: "practice_complete", wave: "1", challenge: "none" },
    { language: "8", theme: "original", font: "PressStart2P", phase: "practice_complete", wave: "2", challenge: "none" }
  ],
  steps: [
    { name: "open" },
    { name: "confirm-shared-route", when: p => p.phase === "route_complete", action: click("close") },
    { name: "restart", when: p => p.phase !== "route_complete", action: click("restart") },
    { name: "retry", when: p => p.phase === "defeat", action: click("retry") }
  ],
  measure: { reference: measure(selectors.reference), native: measure(selectors.native) }
});
