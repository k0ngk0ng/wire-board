// CDP CPU sampling on one exact owned loopback page; no application mutation.
import {writeFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
const [endpoint,url,targetFile,seconds='5']=process.argv.slice(2),root=fileURLToPath(new URL('../../',import.meta.url));
if(!/^http:\/\/127\.0\.0\.1:\d+\/$/.test(url)||!targetFile.startsWith(root)||Number(seconds)>15)throw Error('Owned loopback page and workspace profile path required');
const socket=new WebSocket(endpoint);await new Promise((resolve,reject)=>{socket.addEventListener('open',resolve,{once:true});socket.addEventListener('error',reject,{once:true});});
let next=0;const pending=new Map();socket.addEventListener('message',event=>{const value=JSON.parse(event.data),item=pending.get(value.id);if(!item)return;pending.delete(value.id);value.error?item.reject(Error(JSON.stringify(value.error))):item.resolve(value.result);});
const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++next;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,...(sessionId?{sessionId}:{})}));});
const {targetInfos}=await call('Target.getTargets'),target=targetInfos.find(value=>value.type==='page'&&value.url===url);if(!target)throw Error('Owned Board page not found');
const {sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
await call('Target.activateTarget',{targetId:target.targetId});await call('Page.bringToFront',{},sessionId);
await call('Profiler.enable',{},sessionId);await call('Profiler.setSamplingInterval',{interval:1000},sessionId);await call('Profiler.start',{},sessionId);
await new Promise(resolve=>setTimeout(resolve,Number(seconds)*1000));
const result=await call('Profiler.stop',{},sessionId);await writeFile(targetFile,JSON.stringify(result.profile)+'\n');
await call('Profiler.disable',{},sessionId);await call('Target.detachFromTarget',{sessionId});socket.close();console.log(JSON.stringify({profile:targetFile,samples:result.profile.samples?.length??0,seconds:Number(seconds)}));
