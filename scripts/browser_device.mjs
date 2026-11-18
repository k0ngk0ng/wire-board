// Apply native mobile metrics/touch to an explicitly owned browser page.
// The viewport command can reset touch emulation, so tests call this last.
const [endpoint,prefix,w,h]=process.argv.slice(2),width=Number(w),height=Number(h);
if(!endpoint||!prefix||!Number.isInteger(width)||!Number.isInteger(height)||width<240||height<160)throw Error("Expected owned CDP endpoint/URL and mobile dimensions");
const socket=new WebSocket(endpoint);await new Promise((r,j)=>{socket.onopen=r;socket.onerror=j});
let serial=0,sessionId;const pending=new Map();
socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sid)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId:sid}))});
try{
 const {targetInfos}=await call("Target.getTargets"),target=targetInfos.find(t=>t.type==="page"&&t.url.startsWith(prefix));if(!target)throw Error("No matching owned page");
 ({sessionId}=await call("Target.attachToTarget",{targetId:target.targetId,flatten:true}));
 await call("Emulation.setDeviceMetricsOverride",{width,height,deviceScaleFactor:1,mobile:true},sessionId);
 await call("Emulation.setTouchEmulationEnabled",{enabled:true,maxTouchPoints:5},sessionId);
 await call("Runtime.evaluate",{expression:"new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",awaitPromise:true},sessionId);
 if(process.argv.includes('--keep')){
  console.log('ready');
  await new Promise(resolve=>{process.once('SIGTERM',resolve);process.once('SIGINT',resolve)});
 }
}finally{if(sessionId)await call("Target.detachFromTarget",{sessionId});socket.close()}
