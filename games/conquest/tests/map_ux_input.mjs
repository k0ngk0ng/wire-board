// Native, paced input into the agent-browser-owned page. This file never calls
// game handlers, changes game state, or launches a separate browser.
const [endpoint, prefix, encoded] = process.argv.slice(2);
const task = JSON.parse(encoded);
if (!endpoint || !prefix || !["drag", "pinch", "pinch-to-pan", "add-finger", "cancel", "mouse-drag", "mouse-hover", "mouse-tap", "wheel", "tap", "hold", "doubletap"].includes(task.mode)) throw Error("Invalid owned-page input request");
const { x, y, dx = 0, dy = 0, start = 22, end = 65, steps = 48, pace = 16, cpu = 1, latency = 0, rejectMove = false, holdMS = 750 } = task;
if (![x,y,dx,dy,start,end,steps,pace,cpu,latency,holdMS].every(Number.isFinite) || steps < 1 || steps > 120 || pace < 0 || pace > 100 || latency<0 || latency>1000 || holdMS<0 || holdMS>3000) throw Error("Invalid input coordinates or pacing");
const socket = new WebSocket(endpoint);
await new Promise((resolve,reject) => {socket.onopen=resolve;socket.onerror=reject});
let serial=0, sessionId, rejectionResult=null, rejectReply, rejectFailure;
const rejection = new Promise((resolve,reject)=>{rejectReply=resolve;rejectFailure=reject});
const pending = new Map();
socket.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==="Fetch.requestPaused"){void route(m.params).catch(rejectFailure);return}if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},session)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId:session}))});
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function route({requestId,request}) {
  let body;
  try {body=JSON.parse(request.postData??"null")} catch {}
  if(rejectMove&&!rejectionResult&&request.method==="POST"&&body?.type==="action"&&body?.action?.type==="move") {
    rejectionResult={status:400,url:request.url,action:body.action,scope:"single explicit test-network refusal; request never mutates server state"};
    await call("Fetch.fulfillRequest",{requestId,responseCode:400,responseHeaders:[{name:"Content-Type",value:"application/json"}],body:Buffer.from(JSON.stringify({error:"验收拒绝一次派兵"})).toString("base64")},sessionId);
    await call("Fetch.disable",{},sessionId);rejectReply(rejectionResult);
  } else await call("Fetch.continueRequest",{requestId},sessionId);
}
try {
  const {targetInfos}=await call("Target.getTargets");
  const target=targetInfos.find(t=>t.type==="page"&&t.url.startsWith(prefix));
  if(!target)throw Error("No owned browser page at expected origin");
  ({sessionId}=await call("Target.attachToTarget",{targetId:target.targetId,flatten:true}));
  await call("Emulation.setCPUThrottlingRate",{rate:cpu},sessionId);
  if(latency){await call("Network.enable",{},sessionId);await call("Network.emulateNetworkConditions",{offline:false,latency,downloadThroughput:-1,uploadThroughput:-1},sessionId)}
  if(rejectMove)await call("Fetch.enable",{patterns:[{urlPattern:prefix+"/api/rooms/*",requestStage:"Request"}]},sessionId);
  if(task.mode==="mouse-hover") {
    await call("Input.dispatchMouseEvent",{type:"mouseMoved",buttons:0,x,y},sessionId);
  } else if(task.mode==="mouse-tap") {
    await call("Input.dispatchMouseEvent",{type:"mouseMoved",buttons:0,x,y},sessionId);
    await call("Input.dispatchMouseEvent",{type:"mousePressed",button:"left",buttons:1,x,y,clickCount:1},sessionId);
    await call("Input.dispatchMouseEvent",{type:"mouseReleased",button:"left",buttons:0,x,y,clickCount:1},sessionId);
  } else if(task.mode==="wheel") {
    await call("Input.dispatchMouseEvent",{type:"mouseWheel",x,y,deltaX:0,deltaY:dy},sessionId);
  } else if(task.mode==="mouse-drag") {
    await call("Input.dispatchMouseEvent",{type:"mousePressed",button:"left",buttons:1,x,y,clickCount:1},sessionId);
    for(let i=1;i<=steps;i++){await sleep(pace);await call("Input.dispatchMouseEvent",{type:"mouseMoved",button:"left",buttons:1,x:x+dx*i/steps,y:y+dy*i/steps},sessionId)}
    await call("Input.dispatchMouseEvent",{type:"mouseReleased",button:"left",buttons:0,x:x+dx,y:y+dy,clickCount:1},sessionId);
  } else {
    await call("Emulation.setTouchEmulationEnabled",{enabled:true,maxTouchPoints:5},sessionId);
    const two=task.mode==="pinch"||task.mode==="pinch-to-pan"||task.mode==="add-finger";
    const points=i=>two?[{x:x-(start+(end-start)*i/steps)+dx*i/steps,y:y+dy*i/steps,id:1},{x:x+(start+(end-start)*i/steps)+dx*i/steps,y:y+dy*i/steps,id:2}]:[{x:x+dx*i/steps,y:y+dy*i/steps,id:1}];
    if(task.mode==="add-finger") {
      await call("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:[{x:x-start+15,y:y+10,id:1}]},sessionId);
      for(let i=1;i<=10;i++){await sleep(pace);await call("Input.dispatchTouchEvent",{type:"touchMove",touchPoints:[{x:x-start+15*(1-i/10),y:y+10*(1-i/10),id:1}]},sessionId)}
    }
    await call("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:points(0)},sessionId);
    if(task.mode==="hold")await sleep(holdMS);
    if(task.mode!=="tap"&&task.mode!=="doubletap"&&task.mode!=="hold")for(let i=1;i<=steps;i++){await sleep(pace);await call("Input.dispatchTouchEvent",{type:"touchMove",touchPoints:points(i)},sessionId)}
    if(task.mode==="pinch-to-pan"||task.mode==="add-finger") {
      const p=points(steps)[0];
      await call("Input.dispatchTouchEvent",{type:"touchEnd",touchPoints:[p]},sessionId);
      for(let i=1;i<=12;i++){await sleep(pace);await call("Input.dispatchTouchEvent",{type:"touchMove",touchPoints:[{...p,x:p.x-30*i/12,y:p.y-20*i/12}]},sessionId)}
    }
    await call("Input.dispatchTouchEvent",{type:task.mode==="cancel"?"touchCancel":"touchEnd",touchPoints:[]},sessionId);
    if(task.mode==="doubletap") {
      await sleep(20);
      await call("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:[{x,y,id:1}]},sessionId);
      await call("Input.dispatchTouchEvent",{type:"touchEnd",touchPoints:[]},sessionId);
    }
  }
  await call("Runtime.evaluate",{expression:"new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",awaitPromise:true},sessionId);
  if(rejectMove){let timeout;try{await Promise.race([rejection,new Promise((_,reject)=>{timeout=setTimeout(()=>reject(Error("the expected move request was never sent")),8000)})])}finally{clearTimeout(timeout)}}
  console.log(JSON.stringify({native:true,...task,...(rejectionResult?{rejection:rejectionResult}:{})}));
} finally {
  if(sessionId){if(rejectMove)await call("Fetch.disable",{},sessionId);if(latency)await call("Network.emulateNetworkConditions",{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1},sessionId);await call("Emulation.setCPUThrottlingRate",{rate:1},sessionId);await call("Emulation.setTouchEmulationEnabled",{enabled:false},sessionId);await call("Target.detachFromTarget",{sessionId})}
  socket.close();
}
