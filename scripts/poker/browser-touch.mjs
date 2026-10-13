// Native touch input in the session owned by agent-browser; no separate browser.
const [endpoint,prefix,startX,startY,endX,endY]=process.argv.slice(2);
const socket=new WebSocket(endpoint);await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0;const pending=new Map();socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(new Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
try{const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('Owned page not found');const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
 await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
 await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:+startX,y:+startY,id:1}]},sessionId);
 await call('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:+endX,y:+endY,id:1}]},sessionId);
 await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]},sessionId);
 await call('Emulation.setTouchEmulationEnabled',{enabled:false},sessionId);await call('Target.detachFromTarget',{sessionId});
}finally{socket.close()}
