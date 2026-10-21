// Failure inventory: source-confirmed result must continue its reward; stopping
// prediction at confirmation freezes it; joins/resumes must paint current state
// without replaying old pictures or audio; incomplete asset preload must fail
// before play; all runtime snapshots must remain restorable after projection.
import { loadBasketballWasm } from '../../games/basketball/web/wasm';
import { BasketballRenderer, loadBasketballAssets } from '../../games/basketball/web/renderer';
import { BasketballNetworkScene, type BasketballNetworkView } from '../../games/basketball/web/network-scene';
import type { BasketballRuntimeState, BasketballRuntimeView } from '../../games/basketball/web/client';
const checks:string[]=[];const check=(s:string,v:boolean)=>{if(!v)throw Error(s);checks.push(s)};
(async()=>{
 const fixture=await (await fetch('earned-reward.json')).json();
 const [engine,assets]=await Promise.all([loadBasketballWasm('./runtime'),loadBasketballAssets('/assets',{postResult:true})]);
 check('post-result textures preloaded',!!assets.postResult);
 const canvas=document.querySelector('canvas')!;
 const renderer=new BasketballRenderer(canvas,assets,projection=>engine.request({op:'projectPresentation',projection}));
 let drawn=0,postDrawn=0,lastFrame=-1;
 const render=(frame:any)=>{lastFrame=frame.frame;if(!frame.display.HasVisible)return;const d=frame.display;renderer.resetCourt(d.Visible.court);renderer.commitCourtEvents([...(d.CourtColumns??[]),...(d.CourtPatches??[]).map((patch:any)=>({patch}))]);renderer.render(d.Visible);drawn++;if(d.Visible.postResult)postDrawn++};
 const restore=(state:BasketballRuntimeState)=>engine.request<BasketballRuntimeView>({op:'restore',checkpoint:{version:1,state}});
 let running=restore(fixture.start);const start=running.state.videoFrames;
 for(let i=0;i<60;i++){running=engine.request({op:'advance',session:running.session,ticks:1});render({frame:running.state.videoFrames,display:running.state.display})}
 check('confirmed reward advances 60 actual Go video frames',running.state.videoFrames===start+60);
 check('Wasm reward agrees with native full state',JSON.stringify(running.state)===JSON.stringify(fixture.middle));
 check('actual reward snapshots reach renderer',postDrawn>0);
 const atMiddle=canvas.getContext('2d')!.getImageData(0,0,256,240).data;
 check('reward frame is visible and not uniform',atMiddle.some((v,i)=>i%4<3&&v!==atMiddle[i%4]));
 let animationFrames=60;
 while(!running.state.presentation?.complete&&animationFrames<600){running=engine.request({op:'advance',session:running.session,ticks:1});render({frame:running.state.videoFrames,display:running.state.display});animationFrames++}
 check('reward completes at native handoff',JSON.stringify(running.state)===JSON.stringify(fixture.finish));
 const terminal=running.state.videoFrames;
 running=engine.request({op:'advance',session:running.session,ticks:120});
 check('completed reward stays parked',running.state.videoFrames===terminal&&!(running.soundIDs?.length));
 engine.request({op:'dispose',session:running.session});
 const view=(state:BasketballRuntimeState):BasketballNetworkView=>({epoch:'earned-reward',serverAt:1,phase:'playing',canInput:true,runtime:state,inputs:Array.from({length:4},()=>({sequence:0,buttons:0,appliedButtons:0,automated:false})),videoInputs:[],events:[{sequence:1,videoFrame:state.videoFrames-1,ids:[19]}],soundSequence:1,scores:[0,0]});
 let sounds=0;const local=new BasketballNetworkScene({engine,controller:0,onFrame:render,onSound:()=>sounds++});
 local.update(view(fixture.middle),0,0);for(let n=1;n<=6;n++)local.tick(n*1000/60,()=>0);
 check('local prediction continues confirmed presentation',local.predictedState!.videoFrames===fixture.middle.videoFrames+6);
 check('join does not replay reward sounds',sounds===0);local.dispose();
 const spectator=new BasketballNetworkScene({engine,onFrame:render,onSound:()=>sounds++});
 spectator.update({...view(fixture.start),canInput:false},0,0);
 spectator.update({...view(fixture.middle),canInput:false,videoInputs:fixture.inputs},1,100);
 for(let n=0;n<60;n++)spectator.tick(100+n*1000/60);
 check('spectator replays post-confirmation video history to exact frame',lastFrame===fixture.middle.videoFrames);
 check('spectator replay matches native reward state',JSON.stringify(spectator.lastReplayState)===JSON.stringify(fixture.middle));spectator.dispose();
 const restored=new BasketballNetworkScene({engine,onFrame:render,onSound:()=>sounds++});
 restored.update({...view(fixture.finish),canInput:false},0,0);const shown=drawn;restored.tick(100);
 check('terminal reconnect paints retained final image once',drawn===shown&&lastFrame===terminal&&sounds===0);restored.dispose();
 // Controlled source ending starts after result confirmation; it is not an
 // earned fourteen-win campaign. All Go ticks run, and every phase transition
 // plus every thirtieth frame is painted through the production renderer.
 const ending=await(await fetch('ending.json')).json();
 let endingRun=restore(ending.initial),endingTicks=0,endingDrawn=0,endingPicture:any;
 const confirm=new Set<number>(ending.confirmFrames);const phases=new Set<number>();let previousPhase=-1;
 while(!endingRun.state.presentation?.complete&&endingTicks<16000){
  const prior=endingRun.state as any;
  if(confirm.has(prior.videoFrames+1))endingPicture=structuredClone(prior.display);
  endingRun=engine.request({op:'advance',session:endingRun.session,ticks:1,held:[confirm.has(prior.videoFrames+1)?16:0,0,0,0]});
  endingTicks++;
  const phase=(endingRun.state.presentation as any)?.video?.Scene?.Transition?.Match?.Phase;
  if(phase!==undefined)phases.add(phase);
  if(endingTicks%30===0||phase!==previousPhase||endingRun.state.presentation?.complete){render({frame:endingRun.state.videoFrames,display:endingRun.state.display});endingDrawn++}
  previousPhase=phase;
  if(endingTicks%120===0)await new Promise(resolve=>setTimeout(resolve,0));
 }
 check('controlled ending reaches native terminal state',JSON.stringify(endingRun.state)===JSON.stringify(ending.final));
 check('controlled ending paints every source scene phase',phases.size>=10&&endingDrawn>300);
 check('ending confirmation picture retained',!!endingPicture);
 engine.request({op:'dispose',session:endingRun.session});
 (window as any).showEnding=()=>{render({frame:endingPicture.frame??0,display:endingPicture});document.querySelector('output')!.textContent=`Controlled ending: ${endingTicks} video frames, ${endingDrawn} sampled frames`;};
 // Keep an active reward frame for inspectable desktop/mobile screenshots.
 render({frame:fixture.middle.videoFrames,display:fixture.middle.display});
 (window as any).result={status:'passed',checks,animationFrames,postDrawn,endingTicks,endingDrawn,endingPhases:[...phases],endingScope:ending.scope,scope:fixture.scope};document.querySelector('output')!.textContent=`${checks.length} checks passed; ${animationFrames} actual video frames`;
})().catch(error=>{(window as any).result={status:'failed',error:String(error),checks}});
