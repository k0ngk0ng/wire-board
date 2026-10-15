// Reload only the explicitly named agent-browser page without cached media.
// Useful after installing a CDN-failure route; no second browser is launched.
const [endpoint,prefix]=process.argv.slice(2);
if(!endpoint||!prefix)throw Error("Expected owned browser endpoint and URL");
const socket=new WebSocket(endpoint);
await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0,loaded;const pending=new Map();
socket.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==="Page.loadEventFired")loaded?.();if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
let sessionId,timer;
try{
 const {targetInfos}=await call("Target.getTargets");const target=targetInfos.find(t=>t.type==="page"&&t.url.startsWith(prefix));if(!target)throw Error("Owned page missing");
 ({sessionId}=await call("Target.attachToTarget",{targetId:target.targetId,flatten:true}));
 await call("Network.setCacheDisabled",{cacheDisabled:true},sessionId);await call("Network.clearBrowserCache",{},sessionId);await call("Page.enable",{},sessionId);
 const loading=new Promise((resolve,reject)=>{loaded=resolve;timer=setTimeout(()=>reject(Error("Uncached reload timed out")),15000)});
 await call("Page.reload",{ignoreCache:true},sessionId);await loading;
}finally{clearTimeout(timer);if(sessionId)await call("Target.detachFromTarget",{sessionId});socket.close()}
