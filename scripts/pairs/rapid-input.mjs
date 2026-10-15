// Native fast input in the existing agent-browser page; never dispatch DOM clicks.
import fs from 'node:fs';
const [endpoint,prefix,mode,input,gap='35']=process.argv.slice(2);
const points=JSON.parse(fs.readFileSync(input,'utf8'));
const socket=new WebSocket(endpoint);await new Promise((ok,no)=>{socket.onopen=ok;socket.onerror=no});
let serial=0;const pending=new Map();socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);clearTimeout(p.timer);m.error?p.no(Error(JSON.stringify(m.error))):p.ok(m.result)};
const call=(method,params={},sessionId)=>new Promise((ok,no)=>{const id=++serial;const timer=setTimeout(()=>{pending.delete(id);no(Error('CDP timeout: '+method))},8000);pending.set(id,{ok,no,timer});socket.send(JSON.stringify({id,method,params,sessionId}))});
try{
 const {targetInfos}=await call('Target.getTargets');const page=targetInfos.find(p=>p.type==='page'&&p.url.startsWith(prefix));if(!page)throw Error('Owned page missing');
 const {sessionId}=await call('Target.attachToTarget',{targetId:page.targetId,flatten:true});await call('Page.bringToFront',{},sessionId);
 await call('Network.enable',{},sessionId);
 await call('Network.emulateNetworkConditions',{offline:false,latency:180,downloadThroughput:1048576,uploadThroughput:1048576,connectionType:'cellular4g'},sessionId);
 await call('Emulation.setTouchEmulationEnabled',{enabled:mode!=='mouse',maxTouchPoints:5},sessionId);
 for(const [i,p] of points.entries()){
  if(mode==='mouse'){
   await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:p.x,y:p.y},sessionId);
   await call('Input.dispatchMouseEvent',{type:'mousePressed',x:p.x,y:p.y,button:'left',buttons:1,clickCount:1},sessionId);
   await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:p.x,y:p.y,button:'left',buttons:0,clickCount:1},sessionId);
  }else{
   await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:p.x,y:p.y,id:i+1}]},sessionId);
   if(p.to)await call('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:p.to.x,y:p.to.y,id:i+1}]},sessionId);
   await call('Input.dispatchTouchEvent',{type:mode==='cancel'?'touchCancel':'touchEnd',touchPoints:[]},sessionId);
  }
  await new Promise(r=>setTimeout(r,+gap));
 }
 await new Promise(r=>setTimeout(r,1600));
 await call('Emulation.setTouchEmulationEnabled',{enabled:false},sessionId);await call('Target.detachFromTarget',{sessionId});
}finally{socket.close()}
