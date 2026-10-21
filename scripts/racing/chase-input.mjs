// Native CDP gestures. Passive state/pose reads only; never patches gameplay.
const [endpoint,base,raw]=process.argv.slice(2),options=JSON.parse(raw),ws=new WebSocket(endpoint);
await new Promise((resolve,reject)=>{ws.onopen=resolve;ws.onerror=reject});let id=0,sessionId;const pending=new Map(),packets=[];
ws.onclose=()=>{for(const p of pending.values())p.reject(Error("CDP connection closed"));pending.clear()};
ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==='Network.webSocketFrameSent'||m.method==='Network.webSocketFrameReceived'){try{const v=JSON.parse(m.params.response.payloadData);if(v.type==='input'||v.type==='ack')packets.push({...v,observedAt:Date.now()})}catch{}}if(!m.id)return;const p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const n=++id;const timer=setTimeout(()=>{pending.delete(n);reject(Error('CDP timeout '+method+' '+(params.expression??params.key??'').slice(0,100)))},15000);pending.set(n,{resolve:r=>{clearTimeout(timer);resolve(r)},reject:e=>{clearTimeout(timer);reject(e)}});ws.send(JSON.stringify({id:n,method,params,sessionId}))});
try{
 const {targetInfos}=await call('Target.getTargets'),target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(base));if(!target)throw Error('owned page missing');({sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));
 const send=(method,params)=>call(method,params,sessionId),evaluate=async expression=>(await send('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true})).result.value;
 await send('Network.enable',{});await send('Emulation.setDeviceMetricsOverride',{width:options.width??1280,height:options.height??1000,deviceScaleFactor:1,mobile:!!options.touch});await send('Emulation.setTouchEmulationEnabled',{enabled:!!options.touch,maxTouchPoints:5});
 const read=()=>evaluate(`(async()=>{const room=(await (await fetch('/api/state')).json()).room,c=document.querySelector('.racing-chase'),controls=document.querySelector('.racing-controls');return {room,canvas:c?{...c.dataset,rect:{width:c.getBoundingClientRect().width,height:c.getBoundingClientRect().height}}:null,controls:controls?{...controls.dataset}:null,activeClass:document.activeElement?.getAttribute("class"),selected:getSelection().toString(),viewport:innerWidth,modal:document.querySelector('[role=dialog]')?.getAttribute('aria-label')}})()`);
 const samples=[{name:'before',data:await read()}],screenshots=[],motion=[];
 for(const action of options.actions){
  console.error(JSON.stringify({step:action,at:Date.now()}));
  if(action.motion){motion.push(...await evaluate(`new Promise(resolve=>{const rows=[];const take=()=>{const c=document.querySelector('.racing-chase');rows.push({at:performance.now(),snapshot:c.dataset.snapshotAt,x:Number(c.dataset.otherX),y:Number(c.dataset.otherY)});if(rows.length<20)requestAnimationFrame(take);else resolve(rows)};requestAnimationFrame(take)})`));continue}
  if(action.reduced!==undefined){await send('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-motion',value:action.reduced?'reduce':'no-preference'}]});continue}
  if(action.wait){await new Promise(r=>setTimeout(r,action.wait));continue}
  if(action.read){samples.push({name:action.read,data:await read()});continue}
  if(action.shot){screenshots.push({name:action.shot,data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});continue}
  if(action.disconnect){await evaluate("fetch('/_test/disconnect',{method:'POST'}).then(r=>r.json())");continue}
  if(action.scrollTo){await evaluate(`document.querySelector(${JSON.stringify(action.scrollTo)}).scrollIntoView({block:'center'})`);continue}
  if(action.key){const key=action.key,code=key===' '? 'Space':key.startsWith('Arrow')?key:'Key'+key.toUpperCase(),vk=key===' '?32:key.startsWith('Arrow')?{ArrowLeft:37,ArrowUp:38,ArrowRight:39,ArrowDown:40}[key]:key.toUpperCase().charCodeAt(0);await send('Input.dispatchKeyEvent',{type:action.down?'keyDown':'keyUp',key,code,windowsVirtualKeyCode:vk,modifiers:0,autoRepeat:!!action.repeat});continue}
  if(action.touch){const points=[];for(const p of action.points??[]){const xy=await evaluate(`(()=>{const e=document.querySelector(${JSON.stringify(p.selector)}),r=e.getBoundingClientRect();return {x:r.x+r.width*${p.x??.5},y:r.y+r.height*${p.y??.5}}})()`);points.push({...xy,id:p.id,radiusX:4,radiusY:4,force:1})}await send('Input.dispatchTouchEvent',{type:action.touch,touchPoints:points});continue}
  if(action.wheel){const xy=await evaluate(`(()=>{const r=document.querySelector(${JSON.stringify(action.wheel)}).getBoundingClientRect();return {x:r.x+r.width*.5,y:r.y+r.height*.5}})()`);await send('Input.dispatchMouseEvent',{type:'mouseWheel',...xy,deltaX:0,deltaY:action.delta??-120});continue}
 }
 await new Promise(r=>setTimeout(r,600));samples.push({name:'after',data:await read()});console.log(JSON.stringify({samples,packets,screenshots,motion}));
}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});ws.close()}
