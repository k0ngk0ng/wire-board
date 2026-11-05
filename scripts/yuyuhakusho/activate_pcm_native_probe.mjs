// CDP visibility control is confined to the exact owned loopback research URL.
const [endpoint, url, action = 'activate'] = process.argv.slice(2);
if (!/^http:\/\/127\.0\.0\.1:\d+\/$/.test(url)) throw Error('Probe URL must be exact loopback origin');
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => {socket.addEventListener('open', resolve, {once:true});socket.addEventListener('error', reject, {once:true});});
let next = 0;const pending = new Map();
socket.addEventListener('message', event => {const value=JSON.parse(event.data);if(!value.id)return;const item=pending.get(value.id);pending.delete(value.id);if(item)value.error?item.reject(Error(JSON.stringify(value.error))):item.resolve(value.result);});
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++next;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(sessionId?{sessionId}:{})}));});
const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(item=>item.type==='page'&&item.url===url);
if(!target)throw Error('Exact owned PCM probe target not found');
const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
const evaluate=expression=>call('Runtime.evaluate',{expression,returnByValue:true},sessionId).then(value=>value.result.value);
const state=()=>evaluate('({hidden:document.hidden,visibility:document.visibilityState,focused:document.hasFocus(),href:location.href})');
const before=await state();let blank;
if(action==='hide') {blank=await call('Target.createTarget',{url:url+'blank',background:false});await call('Target.activateTarget',{targetId:blank.targetId});}
else {await call('Target.activateTarget',{targetId:target.targetId});await call('Page.bringToFront',{},sessionId);}
let after;
for(let i=0;i<200;i++){after=await state();if(action==='hide'?after.hidden:!after.hidden)break;await new Promise(resolve=>setTimeout(resolve,20));}
if(action==='hide'?!after.hidden:after.hidden)throw Error('Actual probe visibility did not change');
let probe;
if(action==='hide'){
 for(let i=0;i<250;i++){
  if(await evaluate('!!window.pcmNativeProbe?.complete || !!window.pcmNativeProbe?.error'))break;
  await new Promise(resolve=>setTimeout(resolve,20));
 }
 probe=await evaluate('window.pcmNativeProbe?.result()');
}
let version={},commandLine={};try{version=await call('Browser.getVersion');commandLine=await call('Browser.getBrowserCommandLine');}catch{}
console.log(JSON.stringify({targetId:target.targetId,before,after,blankTarget:blank?.targetId,version,commandLine,probe}));
await call('Target.detachFromTarget',{sessionId});socket.close();
