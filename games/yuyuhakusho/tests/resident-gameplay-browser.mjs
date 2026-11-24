import path from "node:path";
import {pathToFileURL} from 'node:url';
import {createServer} from 'node:http';
import {Readable} from 'node:stream';
const {webkit}=await import(pathToFileURL(process.argv[3]));
import fs from 'node:fs/promises';
import assert from 'node:assert/strict';
const root=process.cwd();
const out='.local/yuyuhakusho/resident-assets/gameplay';
await fs.mkdir(out,{recursive:true});
process.env.TMPDIR=path.join(root,'.local/yuyuhakusho/resident-assets/tmp');
await fs.mkdir(process.env.TMPDIR,{recursive:true});
const before = `(()=>{if(window.recoveryTrace)return;const log=window.recoveryTrace={phase:'setup',recoveries:[],checks:[],frames:[],calls:[],lifecycle:[]};const descriptor=Object.getOwnPropertyDescriptor(WebSocket.prototype,'onmessage');Object.defineProperty(WebSocket.prototype,'onmessage',{...descriptor,set(fn){descriptor.set.call(this,typeof fn!=='function'?fn:function(event){const at=performance.now();log.lastWire=at;try{return fn.call(this,event)}finally{const ms=performance.now()-at;if(ms>10)log.calls.push({phase:log.phase,kind:'wire',ms,bytes:event.data.length})}})}});const focus=HTMLElement.prototype.focus;HTMLElement.prototype.focus=function(...args){const at=performance.now();try{return focus.apply(this,args)}finally{if(this.matches('[data-yuyu-board]'))log.calls.push({phase:log.phase,kind:'focus',ms:performance.now()-at})}};for(const type of ['resize','blur','focus','focusin','visibilitychange'])addEventListener(type,event=>log.lifecycle.push({type,at:performance.now(),target:event.target?.tagName,phase:log.phase}),true);let last=0;const tick=now=>{const b=document.querySelector('[data-yuyu-board]');if(b)log.frames.push({phase:log.phase,now,span:now-last,frame:b.dataset.frame,connection:b.dataset.connection});last=now;requestAnimationFrame(tick)};requestAnimationFrame(tick)})()`;
const browser=await webkit.launch({headless:false,executablePath:process.argv[4]});
const context=await browser.newContext({viewport:{width:1174,height:744},deviceScaleFactor:3,isMobile:true,hasTouch:true});
const page=await context.newPage();
await page.addInitScript(before);
await page.addInitScript(()=>{window.mediaProbe={plays:[],loads:[],activations:[]};const ids=new WeakMap();let nextID=0;const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(...args){const at=performance.now();if(!ids.has(this))ids.set(this,++nextID);const row={id:ids.get(this),at,phase:window.recoveryTrace?.phase,stream:!!this.srcObject,source:this.src.split('/').at(-1),activation:navigator.userActivation?.isActive,paused:this.paused};mediaProbe.plays.push(row);return play.apply(this,args).then(value=>{row.ok=true;row.ms=performance.now()-at;return value},error=>{row.error=String(error);row.ms=performance.now()-at;throw error})};const load=HTMLMediaElement.prototype.load;HTMLMediaElement.prototype.load=function(...args){mediaProbe.loads.push({at:performance.now(),phase:window.recoveryTrace?.phase});return load.apply(this,args)};for(const type of ['pointerdown','pointerup','touchend'])window.addEventListener(type,e=>mediaProbe.activations.push({type,phase:window.recoveryTrace?.phase,trusted:e.isTrusted,activation:navigator.userActivation?.isActive}),true)});

const api=(route,body)=>page.evaluate(async({route,body})=>{const response=await fetch('/api'+route,{method:body?'POST':'GET',headers:{'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value},{route,body});
const command=async type=>{const room=(await api('/state')).room;return api('/rooms/'+room.id,{type,version:room.version,nonce:crypto.randomUUID()})};
const business=JSON.parse(await fs.readFile(path.join(root,'.local/business.json'),'utf8'));
const actual=process.env.CDN_ORIGIN||business.CDN_ORIGIN;
assert.ok(actual);
const resourceRequests=[];
const resourceServer=createServer(async(request,response)=>{
 try{
  if(!request.url.startsWith('/bga/yuyuhakusho/audio-')){response.writeHead(404);response.end();return}
  resourceRequests.push({path:request.url,range:request.headers.range||''});
  const source=await fetch(actual.replace(/\/$/,'')+request.url);
  const headers={'Content-Type':source.headers.get('Content-Type')||'audio/flac','Access-Control-Allow-Origin':'*','Access-Control-Expose-Headers':'Content-Length,Content-Range','Cache-Control':'no-store'};
  if(source.headers.get('Content-Length'))headers['Content-Length']=source.headers.get('Content-Length');
  response.writeHead(source.status,headers);Readable.fromWeb(source.body).pipe(response);
 }catch{response.writeHead(502);response.end()}
});
await new Promise(resolve=>resourceServer.listen(0,'127.0.0.1',resolve));
const resourceOrigin='http://127.0.0.1:'+resourceServer.address().port;
await page.route('**/assets/audio-release.json*',async route=>{const response=await route.fetch();const body=(await response.text()).replaceAll(actual,resourceOrigin);return route.fulfill({response,body})});
await page.route('**/game-load-manifest.json*',async route=>{const response=await route.fetch();const value=await response.json();value.assetBase=resourceOrigin+'/bga/';return route.fulfill({response,body:JSON.stringify(value)})});
const report={passed:false,cases:[],errors:[],resourceRequests};
page.on('pageerror',error=>report.errors.push(String(error)));
try{
 await page.goto(process.argv[2]);
 await page.waitForSelector('input');
 await api('/register',{name:'WebKitC'+Date.now().toString().slice(-8),password:'webkit-c-local-only',invite:'gamepad-browser'});
 await api('/rooms',{kind:'yuyuhakusho',capacity:4,options:{arena:2,characters:[3,5,3,2],teamBattle:false},name:'WebKit C only'});
 for(let i=0;i<3;i++)await command('add_bot');
 await page.reload();
 await page.evaluate(async root=>{
  const {NetworkClient}=await import('/@fs'+root+'/games/yuyuhakusho/web/network-client.ts');
  const {NetworkScene}=await import('/@fs'+root+'/games/yuyuhakusho/web/network-scene.ts');
  const initialize=NetworkClient.prototype.initialize;NetworkClient.prototype.initialize=async function(...args){window.recoveryClient=this;return initialize.apply(this,args)};
  const recover=NetworkClient.prototype.recover;NetworkClient.prototype.recover=function(reason){if(!this.recovering&&performance.now()>=this.retryAt)recoveryTrace.recoveries.push({at:performance.now(),phase:recoveryTrace.phase,reason:String(reason),frame:this.view?.frame,confirmed:String(this.scene?.confirmedFrame),predicted:String(this.scene?.predictedFrame),queue:this.transport.queue.length,history:this.transport.history.length,phaseNow:this.view?.phase,progressAge:performance.now()-this.transport.lastProgress,replay:this.view?.replay,stack:reason?.stack});return recover.call(this,reason)};
  const check=NetworkScene.prototype.check;NetworkScene.prototype.check=function(reply,view){try{return check.call(this,reply,view)}catch(error){recoveryTrace.checks.push({phase:recoveryTrace.phase,actual:{frame:reply.view?.frame,rules:reply.rulesDigest,presentation:reply.presentationDigest},expected:{frame:view.frame,rules:view.digest,presentation:view.presentationDigest}});throw error}};
 },root);
 await page.getByRole('button',{name:'我准备好了',exact:true}).tap();
 await page.getByRole('button',{name:'开始游戏',exact:true}).tap();
 await page.waitForFunction(()=>!!document.querySelector('[data-controller-button=c]:not(:disabled)'),undefined,{timeout:90000});
 await page.waitForFunction(()=>!document.querySelector('[data-game-loading]'));
 report.preloadedResourceRequests=resourceRequests.length;
 assert.ok(report.preloadedResourceRequests>=136,'actual original audio was not prepared before play');
 console.log('WebKit ready with complete resident audio');
 await page.evaluate(()=>{
  recoveryTrace.operations={};const original=Go.prototype._resume;Go.prototype._resume=function(...args){const command=this._pendingEvent?.args?.[0];const op=typeof command==='string'?JSON.parse(command).op:'timer';const start=performance.now();try{return original.apply(this,args)}finally{const ms=performance.now()-start;const phase=recoveryTrace.phase;const bucket=recoveryTrace.operations[phase]??={};const entry=bucket[op]??={count:0,ms:0,max:0};entry.count++;entry.ms+=ms;entry.max=Math.max(entry.max,ms)}};
  recoveryTrace.touches=[];for(const type of ['touchstart','touchend','pointerdown'])document.addEventListener(type,e=>recoveryTrace.touches.push({type,trusted:e.isTrusted,target:e.target?.dataset?.controllerButton,at:performance.now()}),true);
 });
 for(const kind of ['idle','rapid-c','keyboard-c','rapid-c-again','muted-c']){
  if(kind==='rapid-c-again')await page.evaluate(()=>recoveryClient.audio.suspend());
  if(kind==='muted-c'){await page.evaluate(()=>document.querySelector('[data-sound-trigger]').click());await page.locator('[data-sound-mute]').click({force:true})}
  await page.evaluate(kind=>recoveryTrace.phase=kind,kind);
  const start=(await api('/state')).room;
  if(start.status!=='playing'){report.terminalBefore=kind;break}
  if(kind==='idle')await page.waitForTimeout(8000);
  else for(let i=0;i<(kind==='rapid-c'?30:80);i++){
   if(kind==='keyboard-c'){await page.keyboard.down('l');await page.waitForTimeout(25);await page.keyboard.up('l')}
   else {const r=await page.locator('[data-controller-button=c]').boundingBox();await page.touchscreen.tap(r.x+r.width/2,r.y+r.height/2)}
   await page.waitForTimeout(75);
  }
  const end=(await api('/state')).room;
  const summary=await page.evaluate(kind=>{const f=recoveryTrace.frames.filter(v=>v.phase===kind);return {frames:f.length,rawCallbackRate:1000*(f.length-1)/(f.at(-1).now-f[0].now),maxGap:Math.max(...f.map(v=>v.span)),visibleFrames:new Set(f.map(v=>v.frame)).size,recoveries:recoveryTrace.recoveries.filter(v=>v.phase===kind),checks:recoveryTrace.checks.filter(v=>v.phase===kind),operations:recoveryTrace.operations[kind],focus:recoveryTrace.calls.filter(v=>v.phase===kind&&v.kind==='focus')}},kind);
  summary.streams=await page.evaluate(()=>[...document.querySelectorAll('audio[data-platform-spatial]')].map(m=>({currentTime:m.currentTime,paused:m.paused,error:m.error?.code,decoded:m.dataset.spatialBuffers,voices:m.dataset.spatialVoices})));summary.appliedSequences=end.game.data.inputs[0].appliedSequence-start.game.data.inputs[0].appliedSequence;
  summary.resourceNetworkRequests=resourceRequests.length-report.preloadedResourceRequests;assert.equal(summary.resourceNetworkRequests,0,'gameplay went back to the resource server');report.cases.push({kind,...summary});console.log(JSON.stringify(report.cases.at(-1)));
 }
 report.media=await page.evaluate(()=>mediaProbe);report.primers=report.media.plays.filter(p=>p.source.startsWith('wav;base64'));assert.ok(report.primers.length>0);assert.ok(report.primers.every(p=>p.ok&&!p.error),'Native unlock audio failed');const counts=Object.values(Object.groupBy(report.primers,p=>p.id)).map(p=>p.length);assert.ok(counts.every(count=>count===1),'Input restarted native unlock audio');report.resourcePolicyPassed=true;report.gameplayHealthy=report.errors.length===0&&report.cases.every(row=>!row.recoveries.length&&!row.checks.length);report.passed=true;
}catch(error){process.exitCode=1;report.error=String(error);report.text=await page.locator('body').innerText().catch(()=>null);console.error(error)}
finally{report.trace=await page.evaluate(()=>window.recoveryTrace).catch(()=>null);await page.screenshot({path:out+'/resident-game.png'}).catch(()=>{});await fs.writeFile(out+'/resident-game.json',JSON.stringify(report,null,2));await browser.close();await new Promise(resolve=>resourceServer.close(resolve))}
