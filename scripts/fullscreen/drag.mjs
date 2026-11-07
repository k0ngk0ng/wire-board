// Trusted pointer events in an owned local acceptance page only.
const [endpoint,origin,selector,mode,rawX,rawY,end='up']=process.argv.slice(2);
if(!/^http:\/\/127\.0\.0\.1:\d+\/?$/.test(origin))throw Error('Owned loopback target required');
const dx=Number(rawX),dy=Number(rawY);if(!Number.isFinite(dx)||!Number.isFinite(dy))throw Error('Invalid drag');
const socket=new WebSocket(endpoint);await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0,sid;const pending=new Map();
socket.onmessage=({data})=>{const message=JSON.parse(data),item=pending.get(message.id);if(!item)return;pending.delete(message.id);message.error?item.reject(Error(JSON.stringify(message.error))):item.resolve(message.result)};
const call=(method,params={})=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(!method.startsWith('Target.')&&sid?{sessionId:sid}:{})}))});
const evaluate=expression=>call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true}).then(r=>{if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));return r.result.value});
try{
 const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(origin));if(!target)throw Error('Owned page missing');
 ({sessionId:sid}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));
 await call('Emulation.setTouchEmulationEnabled',{enabled:mode==='touch',maxTouchPoints:5});
 const before=await evaluate(`(()=>{const e=document.querySelector(${JSON.stringify(selector)}),r=e.getBoundingClientRect();if(!e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)))throw Error('Drag handle obscured');window.nativeDragEvents=[];for(const type of ['pointerdown','pointermove','pointerup','pointercancel','click'])e.addEventListener(type,event=>nativeDragEvents.push({type,trusted:event.isTrusted,pointerType:event.pointerType}),{once:type==='click'});return {x:r.x,y:r.y,w:r.width,h:r.height}})()`);
 const start={x:before.x+before.w/2,y:before.y+before.h/2};
 if(mode==='touch')await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{...start,id:1}]});
 else await call('Input.dispatchMouseEvent',{type:'mousePressed',...start,button:'left',buttons:1,clickCount:1});
 for(let i=1;i<=8;i++){
  const point={x:start.x+dx*i/8,y:start.y+dy*i/8};
  if(mode==='touch')await call('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{...point,id:1}]});
  else await call('Input.dispatchMouseEvent',{type:'mouseMoved',...point,button:'left',buttons:1});
  await new Promise(resolve=>setTimeout(resolve,20));
 }
 if(mode==='touch')await call('Input.dispatchTouchEvent',{type:end==='cancel'?'touchCancel':'touchEnd',touchPoints:[]});
 else await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:start.x+dx,y:start.y+dy,button:'left',buttons:0,clickCount:1});
 await new Promise(resolve=>setTimeout(resolve,100));
 const after=await evaluate(`(()=>{const r=document.querySelector(${JSON.stringify(selector)}).getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,panel:!!document.querySelector('.chat-panel'),events:nativeDragEvents}})()`);
 console.log(JSON.stringify({mode,end,before,after,native:true}));
}finally{if(sid)await call('Target.detachFromTarget',{sessionId:sid});socket.close()}
