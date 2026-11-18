import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { domSHA, verifyDOMOracle } from "./dom-oracle.mjs";

const research = new URL("../../../.local/pokepath/research/source/src/js/game/", import.meta.url);
const [game, area, publicCatalog] = await Promise.all([readFile(new URL("Game.js", research), "utf8"), readFile(new URL("core/Area.js", research), "utf8"), readFile(new URL("../assets/public/pokepath/v1/catalog.json", import.meta.url), "utf8")]);
const effect = game.slice(game.indexOf("if (this.main.mapEffects != 0)"), game.indexOf("\n\t}\n\n  \ttryDeployUnit"));
const map = area.slice(area.indexOf("this.main.game.canvasBackground.src = this.map.background"), area.indexOf("this.hitsReceived = 0", area.indexOf("this.main.game.canvasBackground.src = this.map.background")));
assert.ok(effect.includes("this.currentGlobalAlpha") && map.includes("8-1bis") && map.includes("effectEnabled"));
const reference = `
const {catalog,input,parameters}=window.oracleData;
const canvas=document.createElement('canvas');
Object.assign(canvas.style,{position:'absolute',left:'259px',top:'25px',transform:'none',outline:'none'});
document.querySelector('.section-scene-container').remove();document.body.append(canvas);
const ctx=canvas.getContext('2d',{willReadFrequently:true});
const original={canvas,ctx,canvasBackground:new Image(),canvasEffect:new Image(),effectTime:0,effectEnabled:false,main:{}};
const mapArea={main:{game:original}};
function changeRoute(){${map}}
function overlay(scaledDelta){${effect}}
let current,lastFlags;
async function paint(id,mode,flags={},delta=0){
 const route={...catalog.routes[id],effect:catalog.routes[id].effect||null};
 if(current!==id){canvas.width=route.width;canvas.height=route.height}
 if(current!==id||lastFlags!==flags){
  mapArea.map=route;mapArea.main.player={secrets:flags.secrets??{},secretItems:flags.secretItems??{}};
  changeRoute.call(mapArea);
  await original.canvasBackground.decode();if(original.effectEnabled)await original.canvasEffect.decode();
  current=id;lastFlags=flags;
 }
 ctx.drawImage(original.canvasBackground,0,0,canvas.width,canvas.height);
 original.main.mapEffects=mode;overlay.call(original,delta);
}
const measure=async()=>{const{width,height}=canvas, bytes=await crypto.subtle.digest('SHA-256',ctx.getImageData(0,0,width,height).data);return{width,height,sha256:Array.from(new Uint8Array(bytes)).map(byte=>byte.toString(16).padStart(2,'0')).join('')}};
window.renderMapStep=async index=>{const step=input.timeline[index], flags=step.flags??{};for(let index=0;index<step.frames;index++)await paint(step.route,step.mode,flags,step.delta);window.mapSurface=await measure()};
if(parameters.scene==='catalog'){
 const hashes=[],flags={};for(const id of input.routes){await paint(id,0,flags);hashes.push({id,...await measure()})};await paint(0,0,flags);window.mapSurface={routes:hashes};
}else if(parameters.scene==='secrets'){
 const results=[];for(const[id,flags]of input.secrets){await paint(id,0);const from=await measure();await paint(id,0,flags);const to=await measure();results.push({id,from,to})};window.mapSurface={secrets:results};
}else await window.renderMapStep(0);
`;
const timeline = [
  { label: "start", route: 0, mode: 1, frames: 1, delta: 0 },
  { label: "pulsing", route: 0, mode: 1, frames: 32, delta: 13 },
  { label: "full-mode-crossfade", route: 0, mode: 2, frames: 4, delta: 13 },
  { label: "disabled", route: 0, mode: 0, frames: 15, delta: 13 },
  { label: "resume-effect", route: 0, mode: 1, frames: 3, delta: 13 },
  { label: "paused-board-redraw", route: 0, mode: 1, frames: 5, delta: 0 },
  { label: "change-route-and-theme", route: 6, mode: 2, frames: 2, delta: 13, flags: { secretItems: { gengarite: true } } }
];
const routes = Object.values(JSON.parse(publicCatalog).routes).map(route => route.id).sort((a, b) => a - b);
assert.equal(routes.length, 47);
const secrets = [[6, { secretItems: { gengarite: true } }], [29, { secrets: { klefki: true } }], [39, { secretItems: { beedrillite: true } }]];
const invoke = index => async ({ evaluate }) => { await evaluate(`window.renderMapStep(${index})`); };
const probe = "window.mapSurface";
await verifyDOMOracle({
  name: "map-surface", command: "node games/pokepath/tests/map-surface-oracle.mjs",
  scope: "Original map draw and overlay fragments against production Canvas and asset loading. Each case records all actual Canvas RGBA pixels via SHA-256 at native route dimensions; screenshots sample the 720x624 viewport.",
  sources: [{ path: "game/Game.js", sha256: domSHA(game), version: "1.7.3" }, { path: "game/core/Area.js", sha256: domSHA(area), version: "1.7.3" }], styles: [], reference, native: 'import "./games/pokepath/tests/map-surface-fixture.mjs";', input: { routes, timeline, secrets },
  cases: ["catalog", "secrets", "effect"].map(scene => ({ language: 8, theme: "original", font: "PressStart2P", scene })), steps: [{ name: "initial" }, ...timeline.slice(1).map((step, index) => ({ name: step.label, when: p => p.scene === "effect", action: invoke(index + 1) }))], measure: { native: probe, reference: probe }
});
