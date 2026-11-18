const [endpoint, prefix, mode, ...values] = process.argv.slice(2);
const [x, y, toX, toY] = values.map(Number);
if (!endpoint || !prefix || !["tap", "hold", "hold-cancel", "drag", "cancel", "two", "pinch"].includes(mode) || values.length !== 4 || ![x,y,toX,toY].every(Number.isFinite)) throw Error("Expected owned CDP endpoint, URL, gesture and four coordinates");
const socket = new WebSocket(endpoint);
await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0, sessionId;
const pending=new Map();
socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={})=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(sessionId?{sessionId}:{})}))});
try {
  const {targetInfos}=await call("Target.getTargets");
  const target=targetInfos.find(t=>t.type==="page"&&t.url.startsWith(prefix));
  if(!target)throw Error("No matching page in the owned browser");
  ({sessionId}=await call("Target.attachToTarget",{targetId:target.targetId,flatten:true}));
  const points=i=>mode==="two"||mode==="pinch"?[{x:x-18-(mode==="pinch"?i*4:0),y,id:1},{x:x+18+(mode==="pinch"?i*4:0),y,id:2}]:[{x:x+(toX-x)*i/8,y:y+(toY-y)*i/8,id:1}];
  await call("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:points(0)});
  if(mode==="hold"||mode==="hold-cancel")await new Promise(r=>setTimeout(r,510));
  if(["drag","cancel","pinch"].includes(mode))for(let i=1;i<=8;i++)await call("Input.dispatchTouchEvent",{type:"touchMove",touchPoints:points(i)});
  await call("Input.dispatchTouchEvent",{type:mode==="cancel"||mode==="hold-cancel"?"touchCancel":"touchEnd",touchPoints:[]});
  await call("Runtime.evaluate",{expression:"new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",awaitPromise:true});
} finally {
  if(sessionId){const attached=sessionId;sessionId=undefined;await call("Target.detachFromTarget",{sessionId:attached});}
  socket.close();
}
