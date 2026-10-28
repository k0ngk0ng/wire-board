// Only attach to this acceptance's own page; dispatch native multi-touch events.
const [url,pageURL,input]=process.argv.slice(2);
const socket=new WebSocket(url);
await new Promise((resolve,reject)=>{socket.addEventListener('open',resolve,{once:true});socket.addEventListener('error',reject,{once:true})});
let id=0;const waiting=new Map();
socket.addEventListener('message',e=>{const r=JSON.parse(e.data);if(r.id){const p=waiting.get(r.id);waiting.delete(r.id);r.error?p.reject(Error(JSON.stringify(r.error))):p.resolve(r.result)}});
function call(method,params={},sessionId){return new Promise((resolve,reject)=>{const n=++id;waiting.set(n,{resolve,reject});socket.send(JSON.stringify({id:n,method,params,...(sessionId?{sessionId}:{})}))})}
const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url===pageURL);
if(!target)throw Error('Acceptance target missing');
const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
await call('Page.bringToFront',{},sessionId);
await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
const observations=[];
for(const action of JSON.parse(input)){
 const {expect,method='Input.dispatchTouchEvent',...event}=action;
 if(method==='acceptance:hide'){
  const other=await call('Target.createTarget',{url:'about:blank',background:false});
  try {
   await call('Target.activateTarget',{targetId:other.targetId});
   let hidden=false;
   for(let n=0;n<50;n++){
    hidden=(await call('Runtime.evaluate',{expression:'document.hidden',returnByValue:true},sessionId)).result.value;
    if(hidden)break;
    await new Promise(resolve=>setTimeout(resolve,20));
   }
   if(!hidden)throw Error('Original page did not become hidden');
   const held=(await call('Runtime.evaluate',{expression:'window.mdTest.sample()',returnByValue:true},sessionId)).result.value;
   if(held.length)throw Error('Hidden page retained input: '+JSON.stringify(held));
  } finally {
   await call('Target.closeTarget',{targetId:other.targetId});
   await call('Target.activateTarget',{targetId:target.targetId});
  }
 } else await call(method,event,sessionId);
 const read=await call('Runtime.evaluate',{expression:'window.mdTest.sample()',returnByValue:true},sessionId);
 observations.push({event:event.type??method,held:read.result.value});
 if(JSON.stringify(read.result.value)!==JSON.stringify(expect)){
  const detail=await call('Runtime.evaluate',{expression:'({keys:window.mdTest.keys,hidden:document.hidden,focused:document.hasFocus()})',returnByValue:true},sessionId);
  throw Error(JSON.stringify({got:read.result.value,expect,detail:detail.result.value}));
 }
}
console.log(JSON.stringify(observations));await call('Target.detachFromTarget',{sessionId});socket.close();
