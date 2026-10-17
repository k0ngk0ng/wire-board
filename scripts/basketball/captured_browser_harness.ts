// Two-input development acceptance host; this is not a platform room mode.
import { createElement } from 'react';
import { createRoot } from 'react-dom/client';
import { flushSync } from 'react-dom';
import { FCController } from '../../web/src/controllers/FCController';
import type { ControllerProfile } from '../../web/src/controllers/input';
import { BasketballInput } from '../../games/basketball/web/input';
import { BasketballRenderer, loadBasketballAssets } from '../../games/basketball/web/renderer';
import { loadBasketballWasm } from '../../games/basketball/web/wasm';
import type { BasketballRuntimeConfig, BasketballRuntimeView } from '../../games/basketball/web/client';

const cooperative=new URLSearchParams(location.search).has('cooperative');
if(cooperative)document.querySelector('#scope')!.textContent='两人合作测试配置 · 原版电脑参与';
const profiles: ControllerProfile[] = [
 {revision:0,turboHz:10,keyboard:{up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',select:'KeyF',start:'Space',b:'KeyJ',a:'KeyK',turboB:'KeyU',turboA:'KeyI'}},
 {revision:0,turboHz:10,keyboard:{up:'ArrowUp',down:'ArrowDown',left:'ArrowLeft',right:'ArrowRight',select:'Digit7',start:'Digit8',b:'Comma',a:'Period',turboB:'Digit9',turboA:'Digit0'}},
];
const root=document.querySelector<HTMLElement>('#game')!,canvas=document.querySelector<HTMLCanvasElement>('canvas')!;
const events:unknown[]=[];
for(const type of ['keydown','keyup','pointerdown','pointerup','pointercancel'])root.addEventListener(type,event=>{
 const e=event as KeyboardEvent & PointerEvent;
 events.push({type,trusted:e.isTrusted,code:e.code,pointerType:e.pointerType,pointerId:e.pointerId,button:(e.target as HTMLElement).closest('[data-controller-button]')?.getAttribute('data-controller-button')});
},true);
const engine=await loadBasketballWasm('/runtime'),assets=await loadBasketballAssets('/assets');
const renderer=new BasketballRenderer(canvas,assets);
let view:BasketballRuntimeView|undefined,courtKey='',imageKey='';
const trace:unknown[]=[];
const inputs=profiles.map((profile,controller)=>new BasketballInput(profile,root,buttons=>{
 if(view)view=engine.request({op:'cancelInput',session:view.session,controller,buttons});
}));
for(let i=0;i<2;i++){
 const pad=createRoot(document.querySelector(`#port-${i}`)!);
 const sink={pressPointer:(id:number,buttons:readonly string[])=>{root.focus({preventScroll:true});inputs[i].pressPointer(id,buttons)},releasePointer:(id:number,cancelled:boolean)=>inputs[i].releasePointer(id,cancelled)};
 flushSync(()=>pad.render(createElement(FCController,{profile:profiles[i],mode:'play',sink})));
}
function present(){
 if(!view)return;
 const d=view.state.display;
 if(!d.HasVisible){canvas.getContext('2d')!.clearRect(0,0,256,240);return;}
 const k=`${d.Visible.court}:${d.CourtEpoch}:${d.CourtSerial}`;
 if(k!==courtKey){renderer.resetCourt(d.Visible.court);renderer.commitCourtEvents([...(d.CourtColumns??[]),...(d.CourtPatches??[]).map(patch=>({patch}))]);courtKey=k;imageKey='';}
 const image=JSON.stringify(d.Visible);if(image!==imageKey){renderer.render(d.Visible);imageKey=image;}
 const w=(view.state.playing as any)?.world.Preparation.Actions;
 document.querySelector('#status')!.textContent=`视频帧 ${view.state.videoFrames} · ${w?.SpecialHolder>=0?'箱内玩家 '+(w.SpecialHolder+1)+' · 剩余 '+w.captureObject:'场上比赛'}`;
}
function summary(){
 const s=view!.state,w=(s.playing as any)?.world.Preparation.Actions;
 return{video:s.videoFrames,logic:s.logicSteps,capture:w?.captureObject??0,holder:w?.SpecialHolder,players:w?.actors.map((a:any)=>a.player),auxiliary:w?.auxiliary,display:s.display.Visible};
}
function advance(ticks:number){
 if(!Number.isInteger(ticks)||ticks<1||ticks>120)throw Error('Invalid video count');
 for(let i=0;i<ticks;i++){
  const held=[inputs[0].sample(),inputs[1].sample(),0,0];
  view=engine.request({op:'advance',session:view!.session,ticks:1,held});present();trace.push({...summary(),held});
 }
 return summary();
}
const start=document.querySelector<HTMLButtonElement>('#start')!;
start.addEventListener('click',()=>{
 if(view)engine.request({op:'dispose',session:view.session});
 inputs.forEach(i=>i.cancelLocal());trace.length=0;events.length=0;courtKey='';imageKey='';
 const config=engine.request<BasketballRuntimeConfig>({op:'defaults'});
 Object.assign(config.match,{Court:4,Mode:128,Seed:0x12345678,ControllerAssignments:cooperative?[0,1,-1,-1]:[0,1,2,3]});
 view=engine.request({op:'create',config});present();root.focus();
});
Object.assign(window,{captureHost:{advance,summary,events,trace,profiles,get view(){return view},save:()=>engine.request<any>({op:'snapshot',session:view!.session}).checkpoint,restore:(checkpoint:unknown)=>{
 const next=engine.request<BasketballRuntimeView>({op:'restore',checkpoint});
 engine.request({op:'dispose',session:view!.session});view=next;inputs.forEach(i=>i.cancelLocal());courtKey='';imageKey='';present();return summary();
}}});
start.disabled=false;
