// Trusted browser input confined to the exact owned loopback acceptance page.
const [endpoint,url,raw]=process.argv.slice(2);
if(!/^http:\/\/127\.0\.0\.1:\d+\/$/.test(url))throw Error('Exact owned loopback URL required');
const socket=new WebSocket(endpoint);
await new Promise((resolve,reject)=>{socket.addEventListener('open',resolve,{once:true});socket.addEventListener('error',reject,{once:true});});
let next=0;const pending=new Map();
socket.addEventListener('message',event=>{const response=JSON.parse(event.data);const item=pending.get(response.id);if(!item)return;pending.delete(response.id);response.error?item.reject(Error(JSON.stringify(response.error))):item.resolve(response.result);});
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++next;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(sessionId?{sessionId}:{})}));});
const {targetInfos}=await call('Target.getTargets'),target=targetInfos.find(value=>value.type==='page'&&value.url===url);
if(!target)throw Error('Owned app target missing');
const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
const evaluate=expression=>call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true},sessionId).then(value=>{if(value.exceptionDetails)throw Error(JSON.stringify(value.exceptionDetails));return value.result.value;});
await call('Target.activateTarget',{targetId:target.targetId});await call('Page.bringToFront',{},sessionId);
await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
if(await evaluate('document.hidden'))throw Error('Owned app page is hidden');
const start=await evaluate('performance.now()');
const before=await evaluate('({paint:yuyuAppObservation.paints.at(-1),view:yuyuAppObservation.latestView,board:{...document.querySelector("[data-yuyu-board]").dataset}})');
for(const action of JSON.parse(raw)){
 const {method='Input.dispatchKeyEvent',wait=0,...params}=action;
 if(Object.keys(params).length)await call(method,params,sessionId);
 if(wait)await new Promise(resolve=>setTimeout(resolve,wait));
}
const result=await evaluate('(()=>{const o=yuyuAppObservation,t='+start+';return {start:t,now:performance.now(),hidden:document.hidden,board:{...document.querySelector("[data-yuyu-board]").dataset},keys:o.keys.filter(x=>x.at>=t),paints:o.paints.filter(x=>x.at>=t),inputs:o.inputs.filter(x=>x.at>=t),acks:o.acks.filter(x=>x.at>=t),views:o.views.filter(x=>x.at>=t)}})()');
console.log(JSON.stringify({before,...result}));
await call('Target.detachFromTarget',{sessionId});socket.close();
