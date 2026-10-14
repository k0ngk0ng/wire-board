// Background and restore the owned game tab around a known server event.
const [endpoint,prefix,hideAt,showAt]=process.argv.slice(2);
const socket=new WebSocket(endpoint);await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let serial=0;const pending=new Map();socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(new Error(JSON.stringify(m.error))):p.resolve(m.result)};
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}))});
let temporary;
try{const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('Owned page not found');const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
 await new Promise(resolve=>setTimeout(resolve,Math.max(0,+hideAt-Date.now())));
 temporary=(await call('Target.createTarget',{url:'about:blank',background:false})).targetId;const extra=(await call('Target.attachToTarget',{targetId:temporary,flatten:true})).sessionId;await call('Page.bringToFront',{},extra);
 const hidden=(await call('Runtime.evaluate',{expression:'document.hidden',returnByValue:true},sessionId)).result.value;if(!hidden)throw Error('Game tab was not actually hidden');
 await new Promise(resolve=>setTimeout(resolve,Math.max(0,+showAt-Date.now())));await call('Page.bringToFront',{},sessionId);await call('Target.closeTarget',{targetId:temporary});temporary=undefined;
 await call('Target.detachFromTarget',{sessionId});console.log(JSON.stringify({hidden:true,restoredAt:Date.now()}));
}finally{if(temporary)await call('Target.closeTarget',{targetId:temporary});socket.close()}
