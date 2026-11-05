// Activate only this verifier's exact loopback URL. Optional hidden control
// dispatches a real trusted CDP click while the owned page is backgrounded.
const [endpoint, url, control] = process.argv.slice(2);
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, {once:true}); socket.addEventListener('error', reject, {once:true}); });
let sequence = 0;
const pending = new Map();
socket.addEventListener('message', event => {
 const response = JSON.parse(event.data);
 if (!response.id) return;
 const request = pending.get(response.id);pending.delete(response.id);
 if (!request) return;
 response.error ? request.reject(Error(JSON.stringify(response.error))) : request.resolve(response.result);
});
const call = (method, params = {}, sessionId) => new Promise((resolve,reject) => { const id = ++sequence;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(sessionId?{sessionId}:{})})); });
const {targetInfos} = await call('Target.getTargets');
const target = targetInfos.find(item => item.type === 'page' && item.url === url);
if (!target) throw Error('Owned native-media verifier target was not found');
const {sessionId} = await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
const evaluate = expression => call('Runtime.evaluate',{expression,returnByValue:true},sessionId).then(result => result.result.value);
const state = () => evaluate('({hidden:document.hidden,visibility:document.visibilityState,focused:document.hasFocus(),href:location.href,readyState:document.readyState})');
const wait = async (predicate, message) => { for(let i=0;i<200;i++){if(await predicate())return;await new Promise(resolve=>setTimeout(resolve,20));}throw Error(message); };
const result = {targetId:target.targetId,before:await state()};
await call('Target.activateTarget',{targetId:target.targetId});
await call('Page.bringToFront',{},sessionId);
await wait(async()=>{const value=await state();return !value.hidden&&value.visibility==='visible';},'Owned native-media page did not become visible after activation');
if (control === 'hidden-control') {
 const other = await call('Target.createTarget',{url:'about:blank',background:false});
 try {
  await call('Target.activateTarget',{targetId:other.targetId});
  await wait(async()=>{const value=await state();return value.hidden&&value.visibility==='hidden';},'Native hidden control did not background the owned verifier');
  const before = await state();
  const point = await evaluate('(()=>{const r=document.querySelector("#start").getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()');
  await evaluate('window.__hiddenClickObserved=[];document.querySelector("#start").addEventListener("click",event=>window.__hiddenClickObserved.push({trusted:event.isTrusted,hidden:document.hidden,visibility:document.visibilityState}),{capture:true})');
  await call('Input.dispatchMouseEvent',{type:'mousePressed',button:'left',clickCount:1,...point},sessionId);
  await call('Input.dispatchMouseEvent',{type:'mouseReleased',button:'left',clickCount:1,...point},sessionId);
  await wait(()=>evaluate('!!window.yuyuMediaAPI?.error || !!window.yuyuMediaAPI?.complete'),'Hidden control did not reach the media verifier handler');
  result.hiddenControl = {before,after:await state(),probe:await evaluate('({error:yuyuMediaAPI.error,events:yuyuMediaAPI.events,state:yuyuMediaAPI.snapshot(),click:window.__hiddenClickObserved})')};
  if(!result.hiddenControl.probe.click.some(event=>event.trusted&&event.hidden))throw Error('Hidden control did not deliver a real trusted background click');
  if(result.hiddenControl.probe.error!=='Error: independent music fixture failed'||result.hiddenControl.probe.events.length!==0)throw Error('Hidden control did not reproduce rejection before first media request');
 } finally {
  await call('Target.closeTarget',{targetId:other.targetId});
  await call('Target.activateTarget',{targetId:target.targetId});
  await call('Page.bringToFront',{},sessionId);
 }
}
await call('Target.activateTarget',{targetId:target.targetId});
await call('Page.bringToFront',{},sessionId);
await wait(async()=>{const value=await state();return !value.hidden&&value.visibility==='visible';},'Owned native-media page became hidden before gesture');
result.after = await state();
console.log(JSON.stringify(result));
await call('Target.detachFromTarget',{sessionId});socket.close();
