import readline from "node:readline";
const [endpoint,prefix]=process.argv.slice(2);
if(!endpoint||!/^http:\/\/127\.0\.0\.1:\d+$/.test(prefix))throw Error("Expected owned local page");
const socket=new WebSocket(endpoint);await new Promise((r,j)=>{socket.onopen=r;socket.onerror=j});
let serial=0,sessionId,targetId,backgroundTarget;
const pending=new Map(),events=[];
const call=(method,params={},sid=sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(sid?{sessionId:sid}:{})}));});
socket.onmessage=({data})=>{
 const m=JSON.parse(data);
 if(m.id){const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result);return;}
 if(m.method==='Network.webSocketFrameSent'){
  try{const v=JSON.parse(m.params.response.payloadData);if(v.type==='input')events.push({at:Date.now(),kind:'sent',action:v.action,nonce:v.nonce});}catch{}
 }
 if(m.method==='Network.webSocketFrameReceived'){
  try{const v=JSON.parse(m.params.response.payloadData);if(v.type==='ack')events.push({at:Date.now(),kind:'ack',status:v.status,nonce:v.nonce});if(v.type==='state')events.push({at:Date.now(),kind:'state',room:v.state.room?.id,loading:!!v.state.room?.loading});}catch{}
 }
 if(m.method==='Network.webSocketClosed')events.push({at:Date.now(),kind:'closed'});
 if(m.method==='Network.webSocketHandshakeResponseReceived')events.push({at:Date.now(),kind:'connected'});
};
const evaluate=async expression=>(await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true})).result.value;
const input=readline.createInterface({input:process.stdin});
try{
 const {targetInfos}=await call('Target.getTargets',{},null),target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));
 if(!target)throw Error('Owned lifecycle page missing');targetId=target.targetId;
 ({sessionId}=await call('Target.attachToTarget',{targetId,flatten:true},null));
 await call('Page.enable');await call('Network.enable');
 console.log('ready');
 for await(const line of input){
  const command=JSON.parse(line);let value;
  if(command.type==='key'){
   const key={ArrowDown:40,ArrowUp:38,ArrowLeft:37,ArrowRight:39,Enter:13}[command.code];if(!key)throw Error('Unsupported test key');
   await call('Input.dispatchKeyEvent',{type:command.down?'rawKeyDown':'keyUp',code:command.code,key:command.code,windowsVirtualKeyCode:key,autoRepeat:!!command.repeat});value={at:Date.now()};
  }else if(command.type==='background'){
   ({targetId:backgroundTarget}=await call('Target.createTarget',{url:'about:blank'},null));
   await call('Target.activateTarget',{targetId:backgroundTarget},null);
   value=await evaluate('({hidden:document.hidden,focused:document.hasFocus()})');
  }else if(command.type==='foreground'){
   await call('Target.activateTarget',{targetId},null);
   if(backgroundTarget){await call('Target.closeTarget',{targetId:backgroundTarget},null);backgroundTarget=undefined;}
   value=await evaluate('({hidden:document.hidden,focused:document.hasFocus()})');
  }else if(command.type==='trace')value={events,visibility:await evaluate('({hidden:document.hidden,focused:document.hasFocus()})')};
  else if(command.type==='status')value=await evaluate('({hidden:document.hidden,focused:document.hasFocus(),ready:document.querySelector("[data-pal-board]")?.dataset.palReady,error:document.querySelector(".pal-board-error")?.textContent,sameCanvas:!window.palLifecycleCanvas||window.palLifecycleCanvas===document.querySelector("[data-pal-board] canvas"),page:performance.timeOrigin,loading:!!document.querySelector("[data-game-loading]"),interruptions:window.palLifecycleInterruptions})');
  else if(command.type==='remember-canvas'){await evaluate(`(()=>{
   window.palLifecycleCanvas=document.querySelector("[data-pal-board] canvas");
   window.palLifecycleInterruptions=[];
   const observe=new MutationObserver(records=>{
    for(const r of records){
     if(r.type==="attributes"&&(r.oldValue==="false"||r.target.dataset.palReady==="false"))
      window.palLifecycleInterruptions.push({kind:"board-not-ready",at:Date.now()});
     for(const e of r.addedNodes)if(e instanceof Element)
      for(const [selector,kind] of [["[data-game-loading]","loading"],[".pal-board-error","board-error"]])
       if(e.matches(selector)||e.querySelector(selector))window.palLifecycleInterruptions.push({kind,at:Date.now()});
    }
   });
   observe.observe(document.body,{subtree:true,childList:true,attributes:true,attributeOldValue:true,attributeFilter:["data-pal-ready"]});
  })()`);value=true;}
  else if(command.type==='session-cookie')value=(await call('Network.getCookies',{urls:[prefix]})).cookies.find(c=>c.name==='wb_session')?.value;
  else throw Error('Unknown lifecycle command');
  console.log(JSON.stringify(value));
 }
}finally{
 if(backgroundTarget)await call('Target.closeTarget',{targetId:backgroundTarget},null);
 if(sessionId)await call('Target.detachFromTarget',{sessionId},null);
 socket.close();input.close();
}
