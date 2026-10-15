// Real touch events inside an explicitly selected agent-browser-owned page.
// No game-specific state, synthetic DOM events, or second browser session.
const [endpoint, prefix, mode, ...coordinates] = process.argv.slice(2);
const [x, y, toX, toY] = coordinates.map(Number);
if (!endpoint || !prefix || !["tap", "drag", "cancel", "pinch"].includes(mode) || coordinates.length !== 4 || ![x,y,toX,toY].every(Number.isFinite)) throw Error("Expected CDP endpoint, owned URL, touch mode and four coordinates");
const socket = new WebSocket(endpoint);
await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0; const pending=new Map();
socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
let sessionId;
try {
  const {targetInfos}=await call("Target.getTargets");
  const target=targetInfos.find(t=>t.type==="page"&&t.url.startsWith(prefix));
  if(!target)throw Error("No matching page in the owned session");
  ({sessionId}=await call("Target.attachToTarget",{targetId:target.targetId,flatten:true}));
  await call("Emulation.setTouchEmulationEnabled",{enabled:true,maxTouchPoints:5},sessionId);
  const points=i=>mode==="pinch"?[{x:x-25-i*4,y,id:1},{x:x+25+i*4,y,id:2}]:[{x:x+(toX-x)*i/8,y:y+(toY-y)*i/8,id:1}];
  await call("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:points(0)},sessionId);
  if(mode!=="tap")for(let i=1;i<=8;i++)await call("Input.dispatchTouchEvent",{type:"touchMove",touchPoints:points(i)},sessionId);
  await call("Input.dispatchTouchEvent",{type:mode==="cancel"?"touchCancel":"touchEnd",touchPoints:[]},sessionId);
  await call("Runtime.evaluate",{expression:"new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",awaitPromise:true},sessionId);
} finally {
  if(sessionId){await call("Emulation.setTouchEmulationEnabled",{enabled:false},sessionId);await call("Target.detachFromTarget",{sessionId});}
  socket.close();
}
