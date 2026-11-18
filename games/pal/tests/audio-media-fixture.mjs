import fs from "node:fs/promises";
import { createRequire } from "node:module";

const root = new URL("../../../", import.meta.url);
const out = new URL(".local/pal/audio-media-browser/", root);
await fs.mkdir(out, { recursive: true });
const { build } = createRequire(new URL("web/package.json", root))("esbuild");
const source = `import {GameAudio} from "./web/src/audio";
const media=[],events=new WeakMap();
const platform=new GameAudio(()=>{const element=new Audio();media.push(element);const counts={playing:0};events.set(element,counts);element.addEventListener("playing",()=>counts.playing++);return element});
const audio=platform.channel("pal-source-acceptance");
const status=document.getElementById("status");
const wait=async(check,label)=>{const until=performance.now()+10000;while(!check()){if(performance.now()>until)throw Error("timeout: "+label);await new Promise(r=>setTimeout(r,10))}};
const live=key=>media.findLast(element=>element.isConnected&&element.dataset.platformTrack===key);
const check=(value,label)=>{if(!value)throw Error(label)};
async function lifecycle(index){
const path=index.music[9].path,key="lifecycle",id="life-1",url="/pal/audio-v1/"+path;
check(await audio.track(key,[url],{playbackID:id,loop:true,startOffset:2}),"lifecycle start");
let element=live(key);await wait(()=>element.currentTime>2.02,"offset advances");const before=element.currentTime;
check(await audio.track(key,[url],{playbackID:id,loop:true,startOffset:0}),"same ID retained");check(live(key)===element&&element.currentTime>=before,"same ID restarted");
audio.pauseTrack(key);const paused=element.currentTime;await new Promise(r=>setTimeout(r,100));check(element.paused&&Math.abs(element.currentTime-paused)<0.02,"pause changed position");
check(await audio.resumeTrack(key),"resume failed");await wait(()=>element.currentTime>paused+0.02,"resume advances");
check(audio.seekTrack(key,4),"seek rejected");await wait(()=>element.currentTime>=4,"seek applied");
check(await audio.restoreTrack(key,id,{startOffset:5,paused:true}),"paused restore rejected");await wait(()=>Math.abs(element.currentTime-5)<0.1,"restore position");check(element.paused,"restore started output");
check(await audio.resumeTrack(key),"restore resume rejected");await wait(()=>element.currentTime>5.02,"restored output advances");
platform.setEnabled(false);check(element.paused||!element.isConnected,"mute left output");check(!(await audio.track(key,[url],{playbackID:id,loop:true})),"muted old request played");
platform.setEnabled(true);check(!(await audio.track(key,[url],{playbackID:id,loop:true})),"unmute replayed stale request");
check(await audio.track(key,[url],{playbackID:"life-2",loop:true,startOffset:6}),"fresh music request failed");element=live(key);await wait(()=>element.currentTime>6.02,"fresh output advances");
audio.stopTrack(key);check(element.paused&&!element.isConnected,"stop left native output");
return {passed:true,sameIdentityRetained:true,pauseResume:true,seek:true,pausedRestore:true,muteStops:true,unmuteDoesNotReplay:true,stopReleases:true};
}
async function run(){
check(await platform.unlock(),"gesture unlock failed");
const index=await(await fetch("/pal/audio-v1/index.json")).json(),sources=[];
const rows=[...Object.values(index.sounds),...Object.values(index.music),...Object.values(index.cd)];
for(const row of rows){const key="source",id=row.path;const started=await audio.track(key,["/pal/audio-v1/"+row.path],{playbackID:id});check(started,"source rejected "+id);const element=live(key);await wait(()=>events.get(element).playing>0||element.ended,"native playing "+id);await wait(()=>element.currentTime>0||element.ended,"native clock "+id);const expected=row.samples/row.sampleRate;await wait(()=>Number.isFinite(element.duration)&&Math.abs(element.duration-expected)<0.06,"duration mismatch "+id+" "+element.duration+" "+expected);sources.push({path:id,played:events.get(element).playing>0,advanced:element.currentTime>0,duration:element.duration,expectedDuration:expected,codec:index.files[row.path].mime});audio.stopTrack(key);status.textContent=sources.length+"/458"}
const state=await lifecycle(index);platform.dispose();
return {passed:true,sources,lifecycle:state,touch:matchMedia("(pointer:coarse)").matches,viewport:[innerWidth,innerHeight],overflow:document.documentElement.scrollWidth>innerWidth};
}
document.getElementById("start").addEventListener("click",()=>{window.audioMediaDone=false;window.audioMediaFailure=null;run().then(result=>{window.audioMediaResult=result;window.audioMediaDone=true;status.textContent="458/458"}).catch(error=>{window.audioMediaFailure=error.message;window.audioMediaResult={passed:false,error:error.message};window.audioMediaDone=true;status.textContent=error.message})});
window.audioMediaReady=true;`;
await build({ stdin: { contents: source, loader: "ts", resolveDir: root.pathname }, absWorkingDir: root.pathname, nodePaths: [new URL("web/node_modules", root).pathname], bundle: true, outfile: new URL("fixture.js", out).pathname, format: "esm", sourcemap: false });
await fs.writeFile(new URL("index.html", out), '<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>仙剑音源验证</title><style>html,body{margin:0;background:#151719;color:#eee;font:16px system-ui}main{padding:24px;overflow-wrap:anywhere}button{padding:12px 20px}audio{display:none}</style></head><body><main><button id="start">开始验证</button><p id="status">0/458</p></main><script type="module" src="/fixture.js"></script></body></html>');
