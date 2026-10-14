// Network/CPU conditions in the existing agent-browser-owned page only.
const [endpoint,prefix,latency='250',rate='4']=process.argv.slice(2);
const socket=new WebSocket(endpoint);await new Promise((ok,no)=>{socket.onopen=ok;socket.onerror=no});
let serial=0;const pending=new Map();socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.no(Error(JSON.stringify(m.error))):p.ok(m.result)};
const call=(method,params={},sessionId)=>new Promise((ok,no)=>{const id=++serial;pending.set(id,{ok,no});socket.send(JSON.stringify({id,method,params,sessionId}))});
try {
 const {targetInfos}=await call('Target.getTargets');const page=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!page)throw Error('Owned page missing');
 const {sessionId}=await call('Target.attachToTarget',{targetId:page.targetId,flatten:true});
 await call('Network.enable',{},sessionId);
 await call('Network.emulateNetworkConditions',{offline:false,latency:+latency,downloadThroughput:1048576,uploadThroughput:1048576,connectionType:'cellular4g'},sessionId);
 await call('Emulation.setCPUThrottlingRate',{rate:+rate},sessionId);
 await call('Target.detachFromTarget',{sessionId});
} finally {socket.close()}
