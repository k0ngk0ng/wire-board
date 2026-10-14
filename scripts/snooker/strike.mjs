// Native keyboard charge/release in the already owned agent-browser page.
// All shots still cross the production canvas handlers and authenticated API.
const [endpoint,prefix,angle='0',power='.35']=process.argv.slice(2);
const socket=new WebSocket(endpoint);await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0;const pending=new Map();socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(new Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
try {
 const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('Owned page not found');
 const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
 const key=async(code,key,virtual,type)=>call('Input.dispatchKeyEvent',{type,code,key,windowsVirtualKeyCode:virtual},sessionId);
 const aim=+angle===-90?['ArrowUp','ArrowUp',38]:['Home','Home',36];
 await key(...aim,'keyDown');await key(...aim,'keyUp');
 await key('Space',' ',32,'keyDown');
 await new Promise(resolve=>setTimeout(resolve,120+(+power-.02)/.98*1600));
 await key('Space',' ',32,'keyUp');
 await call('Target.detachFromTarget',{sessionId});
} finally {socket.close()}
