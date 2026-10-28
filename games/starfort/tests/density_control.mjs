// Set CPU pacing only on the explicitly owned agent-browser page.
const [endpoint,prefix,rawRate]=process.argv.slice(2),rate=Number(rawRate);
if(!endpoint||!prefix||![1,4].includes(rate))throw Error('Expected owned CDP endpoint, URL prefix, rate 1 or 4');
const socket=new WebSocket(endpoint);await new Promise((r,j)=>{socket.onopen=r;socket.onerror=j});
let serial=0;const pending=new Map();socket.onmessage=({data})=>{const m=JSON.parse(data),p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
let sessionId;try{const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('Owned page missing');({sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));await call('Emulation.setCPUThrottlingRate',{rate},sessionId);console.log('READY');await new Promise(resolve=>process.stdin.once('data',resolve));await call('Emulation.setCPUThrottlingRate',{rate:1},sessionId);}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});socket.close();}
