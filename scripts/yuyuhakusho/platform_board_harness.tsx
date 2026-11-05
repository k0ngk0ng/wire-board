import {useRef,useState,useEffect} from 'react';
import {createRoot} from '../../web/node_modules/react-dom/client';
import {GameAudio} from '../../web/src/audio';
import {GameFullscreen,type GameFullscreenHandle} from '../../web/src/ui/room-fullscreen';
import {Board} from '../../games/yuyuhakusho/web/Board';
import {OptionsForm} from '../../games/yuyuhakusho/web/Options';
import {Preparation} from '../../games/yuyuhakusho/web/Preparation';
import {Cover} from '../../games/yuyuhakusho/web/Cover';
import type {GameRoom,BoardProps} from '../../web/src/game-api/module';
import type {NetworkView,GameOptions} from '../../games/yuyuhakusho/web/types';
const audio=new GameAudio();
const profile={revision:1,turboHz:10,keyboard:{up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',mode:'KeyF',start:'Space',b:'KeyJ',a:'KeyK',c:'KeyL',x:'KeyU',y:'KeyI',z:'KeyO'}};
type Timing={sequence?:number;sent:number;ack?:number;appliedAt?:number;bytes:number};
const probe={ready:false,loaded:0,progress:0,error:'',actions:[] as object[],prepare:[] as object[],delay:0,authority:undefined as NetworkView|undefined,firstInputAt:0,visibleAt:0,fullReady:false,testGate:false,publicGate:false,syncCount:0,syncDelayMS:0,gapUpdates:0,gapInjected:[] as {frame:string;at:number}[],pollMS:100,rpc:[] as {op:string;at:number;duration:number;frame?:string}[],fetches:[] as {url:string;at:number;callbackAt:number}[],timings:[] as Timing[],paints:[] as {at:number;frame:string}[],lastHeld:[] as number[],lastStep:undefined as {players:{slot:number;X:number;Y:number;Pose:number;Action:number}[]}|undefined};
Object.assign(window,{yuyuBoardProbe:probe});
// Test-only observation: preserve exact RPC arguments/results, including opaque
// checkpoint strings. No engine result or production action is substituted.
let runtimeCall:((command:string,...buffers:Uint8Array[])=>string)|undefined;
Object.defineProperty(window,'yuyuhakushoNetworkWasm',{configurable:true,get:()=>runtimeCall,set:(call:typeof runtimeCall)=>{runtimeCall=(command,...buffers)=>{const at=performance.now(),raw=call!(command,...buffers),duration=performance.now()-at,request=JSON.parse(command),result=JSON.parse(raw);probe.rpc.push({op:request.op,at,duration,frame:result.view?.frame});if(probe.rpc.length>10000)probe.rpc.shift();if(request.op==='step'&&result.ok){probe.lastStep=result.view;probe.lastHeld=request.held;}return raw;};}});
const originalPaint=CanvasRenderingContext2D.prototype.putImageData;
CanvasRenderingContext2D.prototype.putImageData=function(...args:Parameters<typeof originalPaint>){const result=Reflect.apply(originalPaint,this,args);queueMicrotask(()=>{const root=this.canvas.closest<HTMLElement>('[data-yuyu-board]');if(root){probe.paints.push({at:performance.now(),frame:root.dataset.frame??''});if(probe.paints.length>180)probe.paints.shift();}});return result;};
function Harness(){
 const [options,setOptions]=useState<GameOptions>({arena:0,teamBattle:false,characters:[3,9]});
 const [started,setStarted]=useState(false),[view,setView]=useState<NetworkView>(),[version,setVersion]=useState(1);
 const [clock,setClock]=useState({serverNow:0,receivedAt:performance.now(),epoch:1});
 const [publicGate,setPublicGate]=useState(false);probe.publicGate=publicGate;
 const fullscreen=useRef<GameFullscreenHandle>(null);
 const room:GameRoom={id:'board-harness',name:'原生Board验收',kind:'yuyuhakusho',host:'human',capacity:2,status:started?'playing':'waiting',version,you:0,options,loading:publicGate?{token:'fixture-public-loading',build:'fixture-build',frozenAt:0,ready:[probe.ready,false]}:null,seats:[{id:'human',name:'玩家',ready:true},{id:'bot',name:'电脑',ready:true,bot:true}]};
 async function update(){const r=await fetch('/fixture').then(r=>r.json());if(probe.gapUpdates>0){probe.gapUpdates--;probe.gapInjected.push({frame:r.data.view.frame,at:performance.now()});r.data.view={...r.data.view,resync:true,replay:{version:1,firstFrame:'0',frameCount:0,data:''}};}setView(r.data.view);setVersion(r.version);setClock({serverNow:r.serverNow,receivedAt:performance.now(),epoch:1});probe.authority=r.data.view;for(const timing of probe.timings)if(timing.sequence!==undefined&&timing.appliedAt===undefined&&r.data.view.inputs[0].appliedSequence>=timing.sequence)timing.appliedAt=performance.now();}
 useEffect(()=>{void update();if(!started)return;const timer=setInterval(()=>{void update();},probe.pollMS);return()=>clearInterval(timer);},[started]);
 const props:BoardProps<NetworkView>={room,data:view!,busy:false,assets:'',sound:false,audio:audio.channel('board-harness'),clock,inputWindow:8,act:async(action)=>{probe.actions.push(action);if(probe.publicGate)throw Error('请等待所有玩家加载完成');const body=JSON.stringify(action),timing:Timing={sequence:typeof action.sequence==='number'?action.sequence:undefined,sent:performance.now(),bytes:new TextEncoder().encode(body).length};probe.timings.push(timing);if(probe.timings.length>128)probe.timings.shift();const delay=probe.delay;await fetch('/act?delay='+delay,{method:'POST',headers:{'Content-Type':'application/json'},body}).then(async r=>{if(!r.ok)throw Error(await r.text());});timing.ack=performance.now();await update();},command:async()=>{},onReady:()=>{probe.ready=true;probe.loaded++;},onLoadProgress:(n)=>{probe.progress=n;},onLoadError:message=>{probe.error=message;}};
 return <main style={{maxWidth:850,margin:'auto',padding:12}}>
  <header id="platform-header">共用平台菜单</header>
  {!started&&<><div style={{width:240}}><Cover/></div><OptionsForm value={options} onChange={setOptions} capacity={2} disabled={false}/><Preparation room={room} options={options} host busy={false} assets="" prepare={action=>{probe.prepare.push(action);if(action.type==='arena')setOptions({...options,arena:Number(action.arena)});if(action.type==='character'){const next=[...options.characters];next[Number(action.seat??0)]=Number(action.character);setOptions({...options,characters:next});}}}/><button id="start-board" onClick={async()=>{await fetch('/restart',{method:'POST',body:JSON.stringify(options)});await update();setPublicGate(probe.testGate);setStarted(true);}}>开始Board验收</button></>}
  {started&&publicGate&&<button id="release-public-gate" onClick={()=>setPublicGate(false)}>确认另一位玩家加载完成</button>}
  {started&&view&&<><div id="room-actions"><button id="fullscreen" onClick={()=>fullscreen.current?.enter()}>全屏</button><button id="leave">离开房间</button></div><div id="public-players">{view.players.map(p=><span key={p.seat} data-public-seat={p.seat}>{room.seats[p.seat]?.name} {Math.max(0,p.health)}</span>)}</div><GameFullscreen ref={fullscreen} scope="board-harness" enabled={probe.ready} status={<div data-layout-public-hud>魔强统一战 · 对战中</div>} onReadyChange={value=>{probe.fullReady=value;}}><Board {...props}/></GameFullscreen></>}
 </main>;
}
window.addEventListener('keydown',event=>{if(event.isTrusted&&event.code==='KeyD')probe.firstInputAt=performance.now();});
const originalFetch=window.fetch.bind(window);
window.fetch=async(input,init)=>{if(String(input)==='/api/controllers?game=yuyuhakusho')return new Response(JSON.stringify({models:[],profiles:{'game:yuyuhakusho':profile}}),{headers:{'Content-Type':'application/json'}});const at=performance.now();let result;if(String(input)==='/api/rooms/board-harness/sync'){probe.syncCount++;if(probe.syncDelayMS)await new Promise(resolve=>setTimeout(resolve,probe.syncDelayMS));result=await originalFetch('/sync',init);}else result=await originalFetch(input,init);probe.fetches.push({url:String(input),at,callbackAt:performance.now()});if(probe.fetches.length>1000)probe.fetches.shift();return result;};
createRoot(document.getElementById('root')!).render(<Harness/>);
