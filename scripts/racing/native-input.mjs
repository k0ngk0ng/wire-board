// Native CDP input to an owned agent-browser page. No DOM event dispatch or
// application state injection. Inspect data-* and captured requests only.
import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
import readline from 'node:readline';
const [endpoint,prefix,mode,output]=process.argv.slice(2);
const socket=new WebSocket(endpoint);await new Promise((ok,no)=>{socket.onopen=ok;socket.onerror=no});
let serial=0;const pending=new Map(),pageEvents=[];
socket.onmessage=({data})=>{const m=JSON.parse(data);if(!m.id){if(m.method==='Page.javascriptDialogOpening'||m.method==='Page.javascriptDialogClosed'||m.method==='Page.lifecycleEvent'){pageEvents.push({at:Date.now(),method:m.method,params:m.params});if(m.method.includes('Dialog'))console.error(JSON.stringify(pageEvents.at(-1)))}return;}const p=pending.get(m.id);if(!p)return;pending.delete(m.id);clearTimeout(p.timer);m.error?p.no(Error(JSON.stringify(m.error))):p.ok(m.result)};
const call=(method,params={},sessionId)=>new Promise((ok,no)=>{const id=++serial,timer=setTimeout(()=>{pending.delete(id);no(Error('CDP timeout '+method))},10000);pending.set(id,{ok,no,timer});socket.send(JSON.stringify({id,method,params,sessionId}))});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
let sessionId;
async function run(mode,output) {
 const samples=[];
 const {targetInfos}=await call('Target.getTargets');const page=targetInfos.find(p=>p.type==='page'&&p.url.startsWith(prefix));if(!page)throw Error('Owned racing page missing');
 if(!sessionId)({sessionId}=await call('Target.attachToTarget',{targetId:page.targetId,flatten:true}));await call('Page.bringToFront',{},sessionId);
 await call('Page.enable',{},sessionId);await call('Page.setLifecycleEventsEnabled',{enabled:true},sessionId);
 const evaluate=async expression=>(await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true},sessionId)).result.value;
 const sample=async label=>samples.push({label,...await evaluate(`(()=>{const c=document.querySelector('.racing-controls'),a=document.querySelector('canvas[aria-label="赛车赛道"]');return {controls:c?{...c.dataset}:null,board:{...document.querySelector('.racing-board')?.dataset},canvas:a?{...a.dataset,width:a.getBoundingClientRect().width,height:a.getBoundingClientRect().height}:null,requests:window.__racingRequests?.slice(-12),audio:window.__racingAudio?.slice(-12)}})()`)});
 const tab=(...args)=>JSON.parse(execFileSync('agent-browser',['tab',...args,'--json'],{env:process.env,encoding:'utf8',timeout:15000}));
 const restoreForeground=async()=>{
  await call('Page.bringToFront',{},sessionId);
  const foreground=await evaluate(`new Promise(resolve=>requestAnimationFrame(a=>requestAnimationFrame(b=>resolve({visible:document.visibilityState,focus:document.hasFocus(),a,b}))))`);
  if(foreground.visible!=='visible'||!foreground.focus||foreground.b<=foreground.a)throw Error('Owned page did not resume rendering: '+JSON.stringify(foreground));
 };
 const blurTab=async(label,ms=250)=>{
  const temporary='racing-blur-'+Date.now();const created=tab('new','--label',temporary,'about:blank');if(!created.success)throw Error(JSON.stringify(created));
  await sleep(ms);await sample(label);
  const switched=tab(page.targetId);if(!switched.success)throw Error(JSON.stringify(switched));
  await restoreForeground();
  const closed=tab('close',temporary);if(!closed.success)throw Error(JSON.stringify(closed));
  await restoreForeground();
 };
 const key=async (type,key,code,virtual)=>call('Input.dispatchKeyEvent',{type,key,code,windowsVirtualKeyCode:virtual},sessionId);
 const touch=async (type,points)=>call('Input.dispatchTouchEvent',{type,touchPoints:points},sessionId);
 await call('Network.enable',{},sessionId);
 if(mode==='keyboard'||mode==='slow-keyboard'||mode==='blur') {
  if(mode==='slow-keyboard')await call('Network.emulateNetworkConditions',{offline:false,latency:180,downloadThroughput:1048576,uploadThroughput:1048576,connectionType:'cellular4g'},sessionId);
  await key('keyDown','w','KeyW',87);await sleep(450);await sample('accelerate');
  if(mode==='slow-keyboard')for(let i=0;i<5;i++){await sleep(160);await sample('slow-snapshot-'+i)}
  for(const [label,k,code,v] of [['left','a','KeyA',65],['right','d','KeyD',68],['left-again','a','KeyA',65]]) {
   await key('keyDown',k,code,v);await sleep(90);await sample(label);await key('keyUp',k,code,v);await sleep(35);
  }
  await key('keyDown',' ','Space',32);await sleep(120);await sample('drift');await key('keyUp',' ','Space',32);
  if(mode==='blur') {
   await blurTab('blurred',300);
  }
  await key('keyUp','w','KeyW',87);await sleep(350);await sample('released');
  await call('Network.emulateNetworkConditions',{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1},sessionId);
 } else if(mode==='touch'||mode==='touch-cancel') {
  await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
  const rects=await evaluate(`(()=>{const rect=s=>{const r=document.querySelector(s).getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,width:r.width}};return {wheel:rect('[aria-label="方向盘"]'),drift:rect('button[aria-label="漂移"]'),brake:rect('button[aria-label="刹车"]')}})()`);
  const p={x:rects.wheel.x,y:rects.wheel.y,id:1},q={x:rects.drift.x,y:rects.drift.y,id:2},offset=Math.min(48,rects.wheel.width*.35);
  await touch('touchStart',[p]);await sleep(100);
  await touch('touchMove',[{...p,x:p.x-offset}]);await sleep(120);await sample('left');
  await touch('touchMove',[{...p,x:p.x+offset}]);await sleep(120);await sample('right');
  await touch('touchStart',[{...p,x:p.x+offset},q]);await sleep(180);await sample('two-finger-drift');
  // CDP touchEnd with an explicit point releases that contact, not its peers.
  await touch('touchEnd',[q]);await sleep(120);await sample('drift-released-wheel-held');
  await touch('touchMove',[{...p,x:p.x-offset}]);await sleep(120);await sample('left-again');
  await touch(mode==='touch-cancel'?'touchCancel':'touchEnd',[]);await sleep(250);await sample('released');
  const brake={x:rects.brake.x,y:rects.brake.y,id:3};await touch('touchStart',[brake]);await sleep(100);await sample('brake');await touch('touchEnd',[]);
  await call('Emulation.setTouchEmulationEnabled',{enabled:false},sessionId);
 } else if(mode==='diagnose') {
  const save=()=>fs.writeFileSync(output,JSON.stringify({mode,samples,pageEvents},null,2)+'\n');
  samples.push({stage:'attached',target:page.targetId});save();
  try { samples.push({stage:'dom',...await evaluate(`({url:location.href,online:navigator.onLine,visibility:document.visibilityState,focus:document.hasFocus(),activeElement:document.activeElement?.outerHTML.slice(0,350),timerSource:window.setTimeout.toString(),now:performance.now(),tick:document.querySelector('.racing-board')?.dataset.tick,requests:window.__racingRequests?.slice(-8)})`)});save(); } catch(e) {samples.push({stage:'dom',error:e.message});save();}
  await call('Performance.enable',{},sessionId);samples.push({stage:'performance-before',...await call('Performance.getMetrics',{},sessionId)});save();await sleep(1000);samples.push({stage:'performance-after',...await call('Performance.getMetrics',{},sessionId)});save();
  for(const [stage,expression] of [['microtask','Promise.resolve(1).then(v=>v+1)'],['timer','new Promise(resolve=>setTimeout(()=>resolve(performance.now()),0))'],['raf','new Promise(resolve=>requestAnimationFrame(resolve))']]) {
   try{samples.push({stage,value:await evaluate(expression)});}catch(e){samples.push({stage,error:e.message})}save();
  }
  try {samples.push({stage:'fetch',result:await evaluate(`Promise.all([['wrapped',window.fetch],['native',window.__racingNativeFetch||window.fetch]].map(async([name,f])=>{const t=performance.now();try{const r=await f('/api/state',{signal:AbortSignal.timeout(5000)});return {name,status:r.status,ms:performance.now()-t,phase:(await r.json()).room?.game?.data?.phase}}catch(e){return {name,error:e.name,message:e.message,ms:performance.now()-t}}}))`)});save();}catch(e){samples.push({stage:'fetch',error:e.message});save();}
 } else if(mode==='key-pedals') {
  for(const [label,keyName,code,virtual] of [['漂移',' ','Space',32],['刹车','Enter','Enter',13]]) {
   await evaluate(`document.querySelector('button[aria-label="${label}"]').focus()`);
   await key('keyDown',keyName,code,virtual);await sleep(150);await sample(label+'-held');
   await key('keyUp',keyName,code,virtual);await sleep(100);await sample(label+'-released');
  }
  await evaluate(`document.querySelector('button[aria-label="漂移"]').focus()`);await key('keyDown',' ','Space',32);await sleep(100);
  await blurTab('pedal-blurred',200);await key('keyUp',' ','Space',32);
  await evaluate(`document.querySelector('button[aria-label="跟车"]').focus()`);await key('keyDown',' ','Space',32);await key('keyUp',' ','Space',32);await sleep(100);await sample('follow-button-space');
 } else if(mode==='touch-recovery') {
  await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
  const p=await evaluate(`(()=>{const r=document.querySelector('[aria-label="方向盘"]').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,id:1}})()`);
  await touch('touchStart',[p]);await touch('touchEnd',[]);await sleep(200);await sample('auto-active');
  await blurTab('hidden');await sleep(250);await sample('visible-without-gesture');
  await touch('touchStart',[p]);await touch('touchEnd',[]);await sleep(200);await sample('wake-after-hidden');
  await call('Network.emulateNetworkConditions',{offline:true,latency:0,downloadThroughput:0,uploadThroughput:0},sessionId);await sleep(1400);await sample('offline');
  await call('Network.emulateNetworkConditions',{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1},sessionId);await sleep(2500);await sample('reconnected-without-gesture');
  await touch('touchStart',[p]);await touch('touchEnd',[]);await sleep(200);await sample('wake-after-reconnect');
  await call('Emulation.setTouchEmulationEnabled',{enabled:false},sessionId);
 } else if(mode==='camera') {
  const rect=await evaluate(`(()=>{const r=document.querySelector('canvas[aria-label="赛车赛道"]').getBoundingClientRect();return {x:r.x+r.width*.6,y:r.y+r.height*.5}})()`);
  // Establish a fixed camera with an actual short drag before measuring the
  // wheel anchor, so following a moving car cannot confound the coordinates.
  await call('Input.dispatchMouseEvent',{type:'mousePressed',x:rect.x,y:rect.y,button:'left',buttons:1,clickCount:1},sessionId);
  await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:rect.x+1,y:rect.y,button:'left',buttons:1},sessionId);
  await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:rect.x+1,y:rect.y,button:'left',buttons:0,clickCount:1},sessionId);await sleep(50);
  await sample('before');
  await call('Input.dispatchMouseEvent',{type:'mouseWheel',x:rect.x,y:rect.y,deltaX:0,deltaY:-180},sessionId);await sleep(150);await sample('wheel');
  await call('Input.dispatchMouseEvent',{type:'mousePressed',x:rect.x,y:rect.y,button:'left',buttons:1,clickCount:1},sessionId);
  await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:rect.x+45,y:rect.y+20,button:'left',buttons:1},sessionId);
  await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:rect.x+45,y:rect.y+20,button:'left',buttons:0,clickCount:1},sessionId);await sleep(150);await sample('dragged');
  const mini=await evaluate(`(()=>{const r=document.querySelector('[aria-label="赛道小地图"]').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})()`);
  await call('Input.dispatchMouseEvent',{type:'mousePressed',x:mini.x+mini.w*.25,y:mini.y+mini.h*.5,button:'left',buttons:1,clickCount:1},sessionId);await sleep(80);await sample('minimap-first');
  await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:mini.x+mini.w*.75,y:mini.y+mini.h*.5,button:'left',buttons:1},sessionId);await sleep(80);await sample('minimap-dragged');
  await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:mini.x+mini.w*.75,y:mini.y+mini.h*.5,button:'left',buttons:0,clickCount:1},sessionId);
 } else if(mode==='pinch') {
  await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
  const rect=await evaluate(`(()=>{const r=document.querySelector('canvas[aria-label="赛车赛道"]').getBoundingClientRect();return {x:r.x+r.width*.5,y:r.y+r.height*.5}})()`);
  await sample('before');
  await touch('touchStart',[{id:1,x:rect.x-30,y:rect.y},{id:2,x:rect.x+30,y:rect.y}]);
  await touch('touchMove',[{id:1,x:rect.x-45,y:rect.y},{id:2,x:rect.x+45,y:rect.y}]);await sleep(50);
  await touch('touchMove',[{id:1,x:rect.x-60,y:rect.y},{id:2,x:rect.x+60,y:rect.y}]);await sleep(120);await sample('pinched');await touch('touchEnd',[]);
  await call('Emulation.setTouchEmulationEnabled',{enabled:false},sessionId);
 } else if(mode==='offline') {
  await call('Network.emulateNetworkConditions',{offline:true,latency:0,downloadThroughput:0,uploadThroughput:0},sessionId);
  await sleep(1000);await sample('offline');
  await call('Network.emulateNetworkConditions',{offline:false,latency:180,downloadThroughput:1048576,uploadThroughput:1048576,connectionType:'cellular4g'},sessionId);
  await sleep(1800);await sample('reconnected');
  await call('Network.emulateNetworkConditions',{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1},sessionId);
 } else if(mode==='block-cdn') {
  const base=process.env.WIRE_BOARD_RACING_ASSETS;if(!base)throw Error('Explicit racing CDN base required');
  const urls=[base.replace(/\/$/,'')+'/racing/v1/*'];await call('Network.setBlockedURLs',{urls},sessionId);samples.push({blockedURLs:urls});
 } else if(mode==='motion') {
  for(let i=0;i<35;i++){await sample(String(i));await sleep(25)}
 } else throw Error('Unknown native input mode: '+mode);
 fs.writeFileSync(output,JSON.stringify({mode,samples,pageEvents},null,2)+'\n');console.log(JSON.stringify({mode,samples:samples.length,output}));
}
try {
 if(mode==='serve') {
  for await(const line of readline.createInterface({input:process.stdin,crlfDelay:Infinity})) {
   if(!line.trim())continue;
   const request=JSON.parse(line);
   try { await run(request.mode,request.output); }
   catch(error) { console.error(error);console.log(JSON.stringify({mode:request.mode,error:error.message})); }
  }
 } else await run(mode,output);
} finally { if(sessionId)await call('Target.detachFromTarget',{sessionId}).catch(()=>{});socket.close(); }
