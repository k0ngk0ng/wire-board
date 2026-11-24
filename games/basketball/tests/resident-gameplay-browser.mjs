import path from "node:path";
import {pathToFileURL} from 'node:url';
const {webkit}=await import(pathToFileURL(process.argv[3]));
import fs from 'node:fs/promises';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {createRequire} from 'node:module';
const root=process.cwd();
const out='.local/basketball/verification/resident-assets';
await fs.mkdir(out,{recursive:true});
process.env.TMPDIR=path.join(root,'.local/basketball/verification/resident-assets/tmp');
await fs.mkdir(process.env.TMPDIR,{recursive:true});
const before = `(()=>{if(window.recoveryTrace)return;const log=window.recoveryTrace={phase:'setup',recoveries:[],checks:[],frames:[],calls:[],lifecycle:[]};const descriptor=Object.getOwnPropertyDescriptor(WebSocket.prototype,'onmessage');Object.defineProperty(WebSocket.prototype,'onmessage',{...descriptor,set(fn){descriptor.set.call(this,typeof fn!=='function'?fn:function(event){const at=performance.now();log.lastWire=at;try{return fn.call(this,event)}finally{const ms=performance.now()-at;if(ms>10)log.calls.push({phase:log.phase,kind:'wire',ms,bytes:event.data.length})}})}});const focus=HTMLElement.prototype.focus;HTMLElement.prototype.focus=function(...args){const at=performance.now();try{return focus.apply(this,args)}finally{if(this.matches('[data-yuyu-board]'))log.calls.push({phase:log.phase,kind:'focus',ms:performance.now()-at})}};for(const type of ['resize','blur','focus','focusin','visibilitychange'])addEventListener(type,event=>log.lifecycle.push({type,at:performance.now(),target:event.target?.tagName,phase:log.phase}),true);let last=0;const tick=now=>{const b=document.querySelector('[data-yuyu-board]');if(b)log.frames.push({phase:log.phase,now,span:now-last,frame:b.dataset.frame,connection:b.dataset.connection});last=now;requestAnimationFrame(tick)};requestAnimationFrame(tick)})()`.replaceAll('[data-yuyu-board]','[data-basketball-ready]').replace('frame:b.dataset.frame',"frame:b.querySelector('canvas')?.dataset.videoFrame").replace('connection:b.dataset.connection',"connection:b.querySelector('.basketball-status')?.textContent");
const browser=await webkit.launch({headless:false,executablePath:process.argv[4]});
const context=await browser.newContext({viewport:{width:1174,height:744},deviceScaleFactor:3,isMobile:true,hasTouch:true});
const page=await context.newPage();
await page.addInitScript(before);
await page.addInitScript(()=>{window.mediaProbe={plays:[],loads:[],activations:[]};const ids=new WeakMap();let nextID=0;const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(...args){const at=performance.now();if(!ids.has(this))ids.set(this,++nextID);const row={id:ids.get(this),at,phase:window.recoveryTrace?.phase,stream:!!this.srcObject,source:this.src.split('/').at(-1),activation:navigator.userActivation?.isActive,paused:this.paused};mediaProbe.plays.push(row);return play.apply(this,args).then(value=>{row.ok=true;row.ms=performance.now()-at;return value},error=>{row.error=String(error);row.ms=performance.now()-at;throw error})};const load=HTMLMediaElement.prototype.load;HTMLMediaElement.prototype.load=function(...args){mediaProbe.loads.push({at:performance.now(),phase:window.recoveryTrace?.phase});return load.apply(this,args)};for(const type of ['pointerdown','pointerup','touchend'])window.addEventListener(type,e=>mediaProbe.activations.push({type,phase:window.recoveryTrace?.phase,trusted:e.isTrusted,activation:navigator.userActivation?.isActive}),true)});

const api=(route,body)=>page.evaluate(async({route,body})=>{const response=await fetch('/api'+route,{method:body?'POST':'GET',headers:{'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value},{route,body});
const command=async type=>{const room=(await api('/state')).room;return api('/rooms/'+room.id,{type,version:room.version,nonce:crypto.randomUUID()})};
const report={passed:false,cases:[],errors:[]};
page.on('pageerror',error=>report.errors.push(String(error)));
try{
 await page.goto(process.argv[2]);
 await page.waitForSelector('input');
 await api('/register',{name:'WebKitC'+Date.now().toString().slice(-8),password:'webkit-c-local-only',invite:'gamepad-browser'});
 await api('/rooms',{kind:'basketball',capacity:2,options:{mode:'versus'},name:'Basketball audio taps'});
 for(let i=0;i<1;i++)await command('add_bot');
 await page.reload();
 await page.getByRole('button',{name:'我准备好了',exact:true}).tap();
 await page.getByRole('button',{name:'开始游戏',exact:true}).tap();
 await page.waitForFunction(()=>!!document.querySelector('[data-controller-button=a]:not(:disabled)'),undefined,{timeout:90000});
 await page.waitForFunction(()=>!document.querySelector('[data-game-loading]'));
 report.resourceBefore=await page.evaluate(async()=>await(await fetch('/__resident-stats')).json());
 assert.ok(report.resourceBefore.count>30,'fixture did not observe actual cold resource requests');
 console.log('WebKit basketball ready with full resident resources');
 await page.evaluate(()=>{
  recoveryTrace.operations={};const original=Go.prototype._resume;Go.prototype._resume=function(...args){const command=this._pendingEvent?.args?.[0];const op=typeof command==='string'?JSON.parse(command).op:'timer';const start=performance.now();try{return original.apply(this,args)}finally{const ms=performance.now()-start;const phase=recoveryTrace.phase;const bucket=recoveryTrace.operations[phase]??={};const entry=bucket[op]??={count:0,ms:0,max:0};entry.count++;entry.ms+=ms;entry.max=Math.max(entry.max,ms)}};
  recoveryTrace.touches=[];for(const type of ['touchstart','touchend','pointerdown'])document.addEventListener(type,e=>recoveryTrace.touches.push({type,trusted:e.isTrusted,target:e.target?.dataset?.controllerButton,at:performance.now()}),true);
 });
 for(const kind of ['idle','rapid-a','muted-a']){
  if(kind==='muted-a'){await page.evaluate(()=>document.querySelector('[data-sound-trigger]').click());await page.locator('[data-sound-mute]').click({force:true})}
  await page.evaluate(kind=>recoveryTrace.phase=kind,kind);
  const start=(await api('/state')).room;
  if(start.status!=='playing'){report.terminalBefore=kind;break}
  if(kind==='idle')await page.waitForTimeout(8000);
  else for(let i=0;i<30;i++){
   if(kind==='keyboard-a'){await page.keyboard.down('k');await page.waitForTimeout(25);await page.keyboard.up('k')}
   else {const r=await page.locator('[data-controller-button=a]').boundingBox();await page.touchscreen.tap(r.x+r.width/2,r.y+r.height/2)}
   await page.waitForTimeout(75);
  }
  const end=(await api('/state')).room;
  const summary=await page.evaluate(kind=>{const f=recoveryTrace.frames.filter(v=>v.phase===kind);return {frames:f.length,rawCallbackRate:1000*(f.length-1)/(f.at(-1).now-f[0].now),maxGap:Math.max(...f.map(v=>v.span)),visibleFrames:new Set(f.map(v=>v.frame)).size,recoveries:recoveryTrace.recoveries.filter(v=>v.phase===kind),checks:recoveryTrace.checks.filter(v=>v.phase===kind),operations:recoveryTrace.operations[kind],focus:recoveryTrace.calls.filter(v=>v.phase===kind&&v.kind==='focus')}},kind);
  summary.receivedSequences=end.game.data.inputs[0].sequence-start.game.data.inputs[0].sequence;
  summary.resourceNetworkRequests=(await page.evaluate(async()=>await(await fetch('/__resident-stats')).json())).count-report.resourceBefore.count;assert.equal(summary.resourceNetworkRequests,0,'gameplay fetched a resource after readiness');report.cases.push({kind,...summary});console.log(JSON.stringify(report.cases.at(-1)));
 }
 report.media=await page.evaluate(()=>mediaProbe);report.primers=report.media.plays.filter(p=>p.source.startsWith('wav;base64'));assert.ok(report.primers.length>0);assert.ok(report.primers.every(p=>p.ok&&!p.error),'Native unlock audio failed');const counts=Object.values(Object.groupBy(report.primers,p=>p.id)).map(p=>p.length);assert.ok(counts.every(count=>count===1),'Input restarted native unlock audio');report.passed=true;
}catch(error){process.exitCode=1;report.error=String(error);report.text=await page.locator('body').innerText().catch(()=>null);console.error(error)}
finally{report.trace=await page.evaluate(()=>window.recoveryTrace).catch(()=>null);await page.screenshot({path:out+'/webkit-audio-'+(process.env.REFERENCE==='1'?'red':'green')+'.png'}).catch(()=>{});await fs.writeFile(out+'/webkit-audio-'+(process.env.REFERENCE==='1'?'red':'green')+'.json',JSON.stringify(report,null,2));await browser.close()}
