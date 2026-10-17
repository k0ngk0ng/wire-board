// Bounded native input/evaluation on the exact page of our owned browser.
// agent-browser still owns navigation, lifecycle and screenshot collection.
let source='';for await(const part of process.stdin)source+=part;
const request=JSON.parse(source), endpoint=new URL(request.endpoint), page=new URL(request.page);
if(!['127.0.0.1','localhost'].includes(endpoint.hostname)||!['127.0.0.1','localhost'].includes(page.hostname))throw Error('Local owned fixture browser required');
const targets=await(await fetch(`http://${endpoint.host}/json/list`)).json();
const target=targets.find(item=>item.type==='page'&&item.url===page.href);
if(!target)throw Error('Owned fixture page is no longer present');
const socket=new WebSocket(target.webSocketDebuggerUrl);
const result=await new Promise((resolve,reject)=>{
 const timeout=setTimeout(()=>{socket.close();reject(Error('CDP fixture request timed out'));},5000);
 const done=(error,value)=>{clearTimeout(timeout);socket.close();error?reject(error):resolve(value);};
 // Chrome may retain its startup tab in these isolated profiles. Activate
 // only our exact fixture before native input/evaluation to avoid a frozen
 // background renderer between short-lived inspector connections.
 socket.addEventListener('open',()=>socket.send(JSON.stringify({id:1,method:'Page.bringToFront'})));
 socket.addEventListener('error',()=>done(Error('Owned browser socket failed')));
 socket.addEventListener('message',event=>{const response=JSON.parse(event.data);if(response.error)return done(Error(JSON.stringify(response.error)));if(response.id===1){socket.send(JSON.stringify({id:2,method:request.method,params:request.params??{}}));return;}if(response.id!==2)return;if(response.result?.exceptionDetails)return done(Error(JSON.stringify(response.result.exceptionDetails)));done(null,response.result);});
});
process.stdout.write(JSON.stringify(result));
