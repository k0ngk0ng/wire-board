const [endpoint, prefix, mode, ...coordinates] = process.argv.slice(2);
const [x, y, toX, toY] = coordinates.map(Number);
const bySelector=mode==="click-selector";
if (!endpoint || !prefix || (bySelector ? coordinates.length!==1 || !coordinates[0] : !["tap", "drag", "cancel", "pinch", "click"].includes(mode) || coordinates.length !== 4 || ![x,y,toX,toY].every(Number.isFinite))) throw Error("Expected CDP endpoint, owned URL, pointer mode, and a selector or four coordinates");
const socket = new WebSocket(endpoint);
await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0; const pending=new Map();
socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
let sessionId;
let restoreTouch=false;
try {
  const {targetInfos}=await call("Target.getTargets");
  const target=targetInfos.find(t=>t.type==="page"&&t.url.startsWith(prefix));
  if(!target)throw Error("No matching page in the owned session");
  ({sessionId}=await call("Target.attachToTarget",{targetId:target.targetId,flatten:true}));
  if(mode==="click"||bySelector) {
    await call("Target.activateTarget",{targetId:target.targetId});
    let point={x,y};
    if(bySelector){
      const selector=JSON.stringify(coordinates[0]);
      const hit=await call("Runtime.evaluate",{expression:`(async()=>{const target=document.querySelector(${selector});if(!target||target.matches(':disabled'))return {error:'Target missing or disabled'};target.scrollIntoView({block:'nearest',inline:'nearest'});await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));const r=target.getBoundingClientRect();const x=r.left+r.width/2,y=r.top+r.height/2;const hit=document.elementFromPoint(x,y);return {x,y,hit:target.contains(hit),cover:hit?.outerHTML.slice(0,180)}})()`,awaitPromise:true,returnByValue:true},sessionId);
      const geometry=hit.result.value;
      if(!geometry?.hit)throw Error(`Button ${coordinates[0]} is not reachable: ${JSON.stringify(geometry??hit.exceptionDetails)}`);
      point={x:geometry.x,y:geometry.y};
    }
    await call("Input.dispatchMouseEvent",{type:"mouseMoved",...point},sessionId);
    await call("Input.dispatchMouseEvent",{type:"mousePressed",...point,button:"left",buttons:1,clickCount:1},sessionId);
    await call("Input.dispatchMouseEvent",{type:"mouseReleased",...point,button:"left",buttons:0,clickCount:1},sessionId);
  } else {
    const previousTouch=!!(await call("Runtime.evaluate",{expression:"navigator.maxTouchPoints>0",returnByValue:true},sessionId)).result.value;
    if(!previousTouch){await call("Emulation.setTouchEmulationEnabled",{enabled:true,maxTouchPoints:5},sessionId);restoreTouch=true;}
    const points=i=>mode==="pinch"?[{x:x-25-i*4,y,id:1},{x:x+25+i*4,y,id:2}]:[{x:x+(toX-x)*i/8,y:y+(toY-y)*i/8,id:1}];
    await call("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:points(0)},sessionId);
    if(mode!=="tap")for(let i=1;i<=8;i++)await call("Input.dispatchTouchEvent",{type:"touchMove",touchPoints:points(i)},sessionId);
    await call("Input.dispatchTouchEvent",{type:mode==="cancel"?"touchCancel":"touchEnd",touchPoints:[]},sessionId);
  }
  await call("Runtime.evaluate",{expression:"new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",awaitPromise:true},sessionId);
} finally {
  if(sessionId){if(restoreTouch)await call("Emulation.setTouchEmulationEnabled",{enabled:false},sessionId);await call("Target.detachFromTarget",{sessionId});}
  socket.close();
}
