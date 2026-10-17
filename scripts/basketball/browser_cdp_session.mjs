// A persistent inspector connection for bounded assertions/native input on the
// exact fixture page. Browser lifecycle remains owned by agent-browser.
import { createInterface } from 'node:readline';
const endpoint=new URL(process.argv[2]),page=new URL(process.argv[3]);
if(!['127.0.0.1','localhost'].includes(endpoint.hostname)||!['127.0.0.1','localhost'].includes(page.hostname))throw Error('Local owned fixture browser required');
const targets=await(await fetch(`http://${endpoint.host}/json/list`)).json();
const target=targets.find(item=>item.type==='page'&&item.url===page.href);
if(!target)throw Error('Owned fixture page is no longer present');
const socket=new WebSocket(target.webSocketDebuggerUrl), pending=new Map();let id=0;
socket.addEventListener('message',event=>{
 const response=JSON.parse(event.data),p=pending.get(response.id);if(!p)return;
 pending.delete(response.id);clearTimeout(p.timeout);
 if(response.error||response.result?.exceptionDetails)p.reject(Error(JSON.stringify(response.error??response.result.exceptionDetails)));else p.resolve(response.result);
});
await new Promise((resolve,reject)=>{socket.addEventListener('open',resolve,{once:true});socket.addEventListener('error',reject,{once:true});});
const call=(method,params={})=>new Promise((resolve,reject)=>{
 const key=++id,timeout=setTimeout(()=>{pending.delete(key);reject(Error('CDP request timed out: '+method));},5000);
 pending.set(key,{resolve,reject,timeout});socket.send(JSON.stringify({id:key,method,params}));
});
await call('Page.bringToFront');
process.stdout.write('{"ready":true}\n');
const lines=createInterface({input:process.stdin,crlfDelay:Infinity});
for await(const line of lines){
 try{const request=JSON.parse(line);const result=await call(request.method,request.params);process.stdout.write(JSON.stringify({result})+'\n');}
 catch(error){process.stdout.write(JSON.stringify({error:String(error)})+'\n');}
}
socket.close();
