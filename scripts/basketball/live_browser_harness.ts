import { BasketballClient, type BasketballRuntimeView, type BasketballRuntimeConfig } from '../../games/basketball/web/client';
import { loadBasketballWasm } from '../../games/basketball/web/wasm';
import { GameAudio } from '../../web/src/audio';
import type { ControllerProfile } from '../../web/src/controllers/input';
import { createElement } from 'react';
import { createRoot } from 'react-dom/client';
import { flushSync } from 'react-dom';
import { FCController } from '../../web/src/controllers/FCController';

const profile:ControllerProfile={revision:0,turboHz:10,keyboard:{up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',select:'KeyF',start:'Space',b:'KeyJ',a:'KeyK',turboB:'KeyU',turboA:'KeyI'}};
const root=document.querySelector<HTMLElement>('#game')!, canvas=document.querySelector('canvas')!;
const platformAudio=new GameAudio();platformAudio.mount();
const button=document.querySelector<HTMLButtonElement>('#start')!, error=document.querySelector<HTMLElement>('#error')!;
let client:BasketballClient|undefined,last=0,accumulator=0;
let currentProfile=profile;
const pad=createRoot(document.querySelector('#controls')!);
function renderPad(){flushSync(()=>pad.render(createElement(FCController,{profile:currentProfile,mode:'play',disabled:!client,sink:client?.controllerSink})));}
renderPad();
const manual=new URLSearchParams(location.search).has('manual');
const court=Number(new URLSearchParams(location.search).get('court')??'1');
if(![0,1,2,3,4,5,6].includes(court))throw Error('Court is not available in this development host');
document.querySelector('header p')!.textContent=court===1?'Go 运行验证 · 纽约场地':`Go 运行验证 · 球场 ${court}`;
let stopped=false;
const showError=(value:unknown)=>{stopped=true;error.textContent=String(value);};
function showPhase(view:BasketballRuntimeView){
 const state=view.state;
 document.querySelector('#status')!.textContent=state.outcome
  ? `比赛结束 · 比分 ${state.outcome.scores[0]} : ${state.outcome.scores[1]}`
  : state.awaitingNextPeriod ? '中场休息 · 按 START 开始下半场'
  : state.playing?.match.Phase===2 ? '已暂停 · 按 START 继续'
  : state.playing ? '比赛进行中 · 键位可在控制器设置中修改' : '准备开球';
}
button.addEventListener('click',async()=>{
 button.disabled=true;error.textContent='';
 try{
  await platformAudio.unlock();
  const engine=await loadBasketballWasm('/runtime'),config=engine.request<BasketballRuntimeConfig>({op:'defaults'});
  config.match.Court=court;
  client=await BasketballClient.create({root,canvas,profile,config,runtimeURL:'/runtime',assetsURL:'/assets',audio:platformAudio.channel('basketball-live'),audioOptions:{localBase:'/audio'},seed:0x12345678,onView:showPhase});
  renderPad();
  Object.assign(window,{live:{client,advance:(ticks:number)=>client!.advanceVideo(ticks),get view(){return client!.view;},save:()=>client!.checkpoint(),restore:(snapshot:unknown)=>client!.restore(snapshot),configure:(p:ControllerProfile)=>{client!.configure(p);currentProfile=structuredClone(p);renderPad();}}});
  root.focus();showPhase(client.view);
 }catch(value){showError(value);button.disabled=false;}
});
function frame(now:number){
 if(last===0)last=now;
 const elapsed=now-last;last=now;
 if(!manual&&client&&!stopped&&!document.hidden){
  // The local development host supplies 60 Hz video ticks. Game simulation,
  // original logical cadence and match timing all remain inside Go.
  if(elapsed>250){accumulator=0;client.cancelInput();}else accumulator+=elapsed;
  const ticks=Math.floor(accumulator/(1000/60));
  if(ticks){accumulator-=ticks*(1000/60);try{client.advanceVideo(ticks);}catch(value){showError(value);}}
 }
 requestAnimationFrame(frame);
}
document.addEventListener('visibilitychange',()=>{last=0;accumulator=0;if(document.hidden){client?.cancelInput();platformAudio.stop();}});
window.addEventListener('pagehide',()=>{client?.dispose();pad.unmount();platformAudio.dispose();});
requestAnimationFrame(frame);
