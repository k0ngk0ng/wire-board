// Native CDP events, confined to this acceptance's exact owned loopback page.
const [endpoint,url,raw]=process.argv.slice(2);
if(!/^http:\/\/127\.0\.0\.1:\d+\/$/.test(url))throw Error('Exact owned loopback URL required');
const socket=new WebSocket(endpoint);
await new Promise((resolve,reject)=>{socket.addEventListener('open',resolve,{once:true});socket.addEventListener('error',reject,{once:true});});
let next=0;const pending=new Map();
socket.addEventListener('message',event=>{const response=JSON.parse(event.data);const item=pending.get(response.id);if(!item)return;pending.delete(response.id);response.error?item.reject(Error(JSON.stringify(response.error))):item.resolve(response.result);});
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++next;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(sessionId?{sessionId}:{})}));});
const {targetInfos}=await call('Target.getTargets'),target=targetInfos.find(value=>value.type==='page'&&value.url===url);
if(!target)throw Error('Owned Board target not found');
const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
const evaluate=expression=>call('Runtime.evaluate',{expression,returnByValue:true},sessionId).then(value=>{if(value.exceptionDetails)throw Error(JSON.stringify(value.exceptionDetails));return value.result.value;});
await call('Target.activateTarget',{targetId:target.targetId});await call('Page.bringToFront',{},sessionId);
await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
if(await evaluate('document.hidden'))throw Error('Owned Board page is hidden');
const observations=[];
const pixelHash=()=>evaluate('(()=>{const c=document.querySelector("canvas"),p=c.getContext("2d").getImageData(0,0,c.width,c.height).data;let h=2166136261;for(let i=0;i<p.length;i++)h=Math.imul(h^p[i],16777619);return (h>>>0).toString(16)})()');
for(const action of JSON.parse(raw)){
 const {method='Input.dispatchKeyEvent',observeMove,observePaused,expectButtons,wait=0,...params}=action;
 const before=observeMove!==undefined?{...await evaluate('({x:Number(document.querySelector("[data-yuyu-board]").dataset.localX),pose:yuyuBoardProbe.lastStep?.players.find(p=>p.slot===0)?.Pose,action:yuyuBoardProbe.lastStep?.players.find(p=>p.slot===0)?.Action})'),pixelHash:await pixelHash()}:undefined;
 await call(method,params,sessionId);
 if(observeMove!==undefined){
  const started=Date.now();let observation,firstResponse;
  do{observation=await evaluate('({authority:yuyuBoardProbe.authority.players[0].x,local:Number(document.querySelector("[data-yuyu-board]").dataset.localX),pose:yuyuBoardProbe.lastStep?.players.find(p=>p.slot===0)?.Pose,action:yuyuBoardProbe.lastStep?.players.find(p=>p.slot===0)?.Action,at:performance.now(),inputAt:yuyuBoardProbe.firstInputAt,frame:document.querySelector("[data-yuyu-board]").dataset.frame})');if(!firstResponse&&(observation.local>before.x||observation.pose!==before.pose||observation.action!==before.action)){firstResponse={...observation,pixelHash:await pixelHash()};}if(observation.local>observeMove+5)break;await new Promise(resolve=>setTimeout(resolve,2));}while(Date.now()-started<1500);
  observation.before=before;observation.firstResponse=firstResponse;observation.pixelHash=await pixelHash();
  if(firstResponse)observation.firstVisibleResponseMs=firstResponse.at-firstResponse.inputAt;
  observations.push(observation);
 }else if(observePaused!==undefined){
  const started=Date.now();let state;
  do{state=await evaluate('({localPaused:yuyuBoardProbe.lastStep?.paused,authorityPhase:yuyuBoardProbe.authority.phase,held:yuyuBoardProbe.lastHeld,frame:document.querySelector("[data-yuyu-board]").dataset.frame})');if(state.localPaused===observePaused)break;await new Promise(resolve=>setTimeout(resolve,5));}while(Date.now()-started<350);
  observations.push({...state,elapsedMs:Date.now()-started});
 }else if(expectButtons!==undefined){
  const started=Date.now();let state;
  do{state=await evaluate('({buttons:yuyuBoardProbe.authority.inputs[0].appliedButtons,localX:document.querySelector("[data-yuyu-board]").dataset.localX,frame:document.querySelector("[data-yuyu-board]").dataset.frame})');if(state.buttons===expectButtons)break;await new Promise(resolve=>setTimeout(resolve,10));}while(Date.now()-started<2000);
  if(state.buttons!==expectButtons)throw Error(JSON.stringify({expectButtons,state}));observations.push({...state,event:params.type,native:true});
 }else if(wait)await new Promise(resolve=>setTimeout(resolve,wait));
}
console.log(JSON.stringify(observations));await call('Target.detachFromTarget',{sessionId});socket.close();
