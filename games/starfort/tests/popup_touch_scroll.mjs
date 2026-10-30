// Native compositor touch scroll in an explicitly owned agent-browser page.
const [endpoint,prefix,rawX,rawY]=process.argv.slice(2),x=Number(rawX),y=Number(rawY);
if(!endpoint||!prefix||!Number.isFinite(x)||!Number.isFinite(y))throw Error('Expected owned browser endpoint, URL and coordinates');
const socket=new WebSocket(endpoint);await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0;const pending=new Map();socket.onmessage=({data})=>{const m=JSON.parse(data),p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
let sessionId;
try{
 const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('Owned page missing');
 ({sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));
 await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
 await call('Input.synthesizeScrollGesture',{x,y,yDistance:-120,speed:300,gestureSourceType:'touch'},sessionId);
 await call('Runtime.evaluate',{expression:'new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))',awaitPromise:true},sessionId);
}finally{if(sessionId){await call('Emulation.setTouchEmulationEnabled',{enabled:false},sessionId);await call('Target.detachFromTarget',{sessionId});}socket.close();}
