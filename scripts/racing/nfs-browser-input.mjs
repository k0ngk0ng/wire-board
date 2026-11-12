// Native CDP input and passive compositor/network observations for an owned
// agent-browser target. Never dispatches synthetic keyboard/touch DOM events or
// writes gameplay state. Optional Gamepad API simulation replaces only the
// browser hardware boundary; production mapping, sampling and HTTP stay real.
import {writeFile,rename,appendFile} from 'node:fs/promises';
import {resolve,sep} from 'node:path';
const [endpoint, base, rawOptions] = process.argv.slice(2);
const options = JSON.parse(rawOptions), socket = new WebSocket(endpoint);
if(options.progressPath&&!resolve(options.progressPath).startsWith(process.cwd()+sep))throw Error('progress evidence must remain in repository');
let progressWrite=Promise.resolve();const pendingShots=[],lapShots=new Set(),circuitRows=new Map();
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0, session, screen = false, pixelTrial = null;
const pending = new Map(), packets = [], states = [], requests = [], exceptions = [], frameEvents = [], pixelFrames = [];
const sent = new Map(), ackMS = [], held = new Map();
const wsFrames=[],wsHandshakes=[];let wsObserveStartMS=Date.now(),measurementWindow=null;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
  const id = ++serial, timer = setTimeout(() => { pending.delete(id); reject(Error(`CDP timeout: ${method}`)); }, method==='Runtime.evaluate'&&params.awaitPromise?(options.runtimeTimeoutMS??90000):20000);
  pending.set(id, { resolve: (value) => { clearTimeout(timer); resolve(value); }, reject: (error) => { clearTimeout(timer); reject(error); } });
  socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
});
const send = (method, params = {}) => call(method, params, session);
socket.onclose = () => { for (const p of pending.values()) p.reject(Error("CDP closed")); pending.clear(); };
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data);
  if (message.id) {
    const p = pending.get(message.id); if (!p) return;
    pending.delete(message.id); message.error ? p.reject(Error(JSON.stringify(message.error))) : p.resolve(message.result); return;
  }
  const p = message.params;
  if(message.method==='Network.webSocketHandshakeResponseReceived'){
    const headers=Object.fromEntries(Object.entries(p.response.headers).filter(([key])=>['upgrade','connection','sec-websocket-extensions','sec-websocket-protocol'].includes(key.toLowerCase())));
    const extensions=Object.entries(headers).find(([key])=>key.toLowerCase()==='sec-websocket-extensions')?.[1]??null;
    wsHandshakes.push({at:Date.now(),status:p.response.status,statusText:p.response.statusText,headers,extensions,perMessageDeflateNegotiated:typeof extensions==='string'&&extensions.includes('permessage-deflate')});
  }
  if(message.method==='Runtime.bindingCalled'&&p.name==='__nfsCircuitProgress'){
    const value=JSON.parse(p.payload);
    if(value.rows?.length){const leg=value.leg??Number(/lap-(\d+)/.exec(value.stage||'')?.[1]);if(Number.isFinite(leg)){const rows=circuitRows.get(leg)??[];rows.push(...value.rows);circuitRows.set(leg,rows)}}
    if(options.progressPath)progressWrite=progressWrite.then(async()=>{await appendFile(options.progressPath+'.journal.jsonl',p.payload+'\n');const summary={...value};delete summary.rows;await writeFile(options.progressPath+'.tmp',JSON.stringify(summary)+'\n');await rename(options.progressPath+'.tmp',options.progressPath)}).catch(e=>exceptions.push({error:e.message}));
    if(options.mode!=='offroad-circuit'&&value.transition&&Number.isInteger(value.lap)&&value.lap>=0&&value.lap<=3&&!lapShots.has(value.lap)){
      lapShots.add(value.lap);pendingShots.push(send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false}).then(async r=>{screenshots.push({name:options.track+'-lap-'+value.lap,data:r.data});if(options.progressPath)await writeFile(resolve(options.progressPath,'..',options.track+'-live-lap-'+value.lap+'.png'),Uint8Array.from(atob(r.data),c=>c.charCodeAt(0)))}).catch(e=>exceptions.push({error:e.message})));
    }
  }
  if (message.method === "Page.screencastFrame") {
    const entry = { timestamp: p.metadata.timestamp * 1000, receivedAt: Date.now(), metadata: p.metadata };
    frameEvents.push(entry);
    if (pixelTrial && pixelTrial.frames.length < 16) {
      const frame = { ...entry, data: p.data, trial: pixelTrial.index };
      pixelTrial.frames.push(frame); pixelFrames.push(frame);
    }
    void send("Page.screencastFrameAck", { sessionId: p.sessionId }).catch((error) => exceptions.push({ error: error.message }));
  }
  if (message.method === "Runtime.exceptionThrown") exceptions.push({ at: Date.now(), ...p.exceptionDetails });
  if (message.method === "Network.requestWillBeSent") requests.push({ at: Date.now(), url: p.request.url, method: p.request.method });
  if (message.method === "Network.webSocketFrameSent" || message.method === "Network.webSocketFrameReceived") {
    const direction = message.method.endsWith("Sent") ? "sent" : "received";
    const frame={at:Date.now(),direction,payloadBytes:p.response.opcode===2?Buffer.from(p.response.payloadData,'base64').byteLength:Buffer.byteLength(p.response.payloadData,'utf8'),type:p.response.opcode===2?'binary':'unparsed-text'};wsFrames.push(frame);
    try {
      const packet = JSON.parse(p.response.payloadData), at = Date.now();
      frame.type=packet.type??'json-without-type';
      if (packet.type === "input") { sent.set(packet.nonce, at); packets.push({ at, direction, ...packet }); }
      else if (packet.type === "ack") { const start = sent.get(packet.nonce); if (start !== undefined) ackMS.push(at - start); packets.push({ at, direction, ...packet }); }
      else if (packet.type === "network") packets.push({ at, direction, ...packet });
      else if (packet.type === "state" || packet.type === "game") {
        const state = typeof packet.state === "string" ? JSON.parse(packet.state) : packet.state;
        const view = packet.type === "game" ? packet.game?.data : state?.room?.game?.data;
        if (view?.cars&&(options.mode!=='offroad-circuit'||!states.length||at-states.at(-1).at>=900||view.phase!==states.at(-1).phase)) states.push({ at, bytes: Buffer.byteLength(JSON.stringify(view)), serverAt: view.serverAt,
          phase: view.phase, epoch: view.epoch, cars: view.cars,props:view.props,events:view.events,
          staticCounts: Object.fromEntries(["points", "road", "barriers", "props"].map((key) => [key, view.track?.[key]?.length ?? 0])) });
      }
    } catch { /* Binary protocol/other traffic is outside these observations. */ }
  }
};
const evaluate = async (expression) => {
  const result = await send("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
  if (result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails));
  return result.result.value;
};
const key = async (value, down, repeat = false) => {
  const functionKey = /^F([1-9]|1[0-2])$/.test(value);
  const named = {Escape:27,Enter:13,Tab:9,Backspace:8};
  const code = value === " " ? "Space" : value in named || value.startsWith("Arrow") || functionKey ? value : "Key" + value.toUpperCase();
  const virtual = value === " " ? 32 : named[value] ?? (functionKey ? 111 + Number(value.slice(1)) : ({ ArrowLeft: 37, ArrowUp: 38, ArrowRight: 39, ArrowDown: 40 }[value] ?? value.toUpperCase().charCodeAt(0)));
  if (down) held.set(value, true); else held.delete(value);
  const timestamp = Date.now() / 1000;
  await send("Input.dispatchKeyEvent", { type: down ? "keyDown" : "keyUp", key: value, code, windowsVirtualKeyCode: virtual, modifiers: 0, autoRepeat: repeat, timestamp });
  return timestamp * 1000;
};
const gamepad = (action) => evaluate(`(()=>{
  if(!window.__nfsGamepadProbe){const original=Object.getOwnPropertyDescriptor(navigator,'getGamepads');
    const probe={pads:[],original,restore:()=>{if(original)Object.defineProperty(navigator,'getGamepads',original);else delete navigator.getGamepads;delete window.__nfsGamepadProbe}};
    Object.defineProperty(navigator,'getGamepads',{configurable:true,value:()=>probe.pads});window.__nfsGamepadProbe=probe;}
  const probe=window.__nfsGamepadProbe,action=${JSON.stringify(action)},index=2;
  if(action.connected===false){probe.pads=[];return {simulation:true,connected:false}};
  const values=action.buttons??{},axes=action.axes??[0,0,0,0];
  const pad={id:'NFS acceptance standard Gamepad API simulation',index,mapping:'standard',connected:true,timestamp:performance.now(),
    buttons:Array.from({length:24},(_,i)=>({value:values[i]??0,pressed:(values[i]??0)>=.5,touched:(values[i]??0)>0})),axes};
  probe.pads=[null,null,pad];return {simulation:true,connected:true,index,buttons:values,axes}})()`);
const sample = () => evaluate(`(async()=>{const state=await(await fetch('/api/state',{signal:AbortSignal.timeout(5000)})).json(),c=document.querySelector('.racing-chase'),controls=document.querySelector('.racing-controls');
  return {room:state.room,network:state.network,canvas:c?{...c.dataset,rect:c.getBoundingClientRect().toJSON()}:null,
  board:document.querySelector('.racing-board')?{...document.querySelector('.racing-board').dataset}:null,
  hud:document.querySelector('.racing-hud')?{rect:document.querySelector('.racing-hud').getBoundingClientRect().toJSON(),display:getComputedStyle(document.querySelector('.racing-hud')).display,visibility:getComputedStyle(document.querySelector('.racing-hud')).visibility}:null,
  gauges:(()=>{const g=document.querySelector('.racing-gauges');if(!g)return null;const visible=e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e);return r.width>0&&r.height>0&&s.visibility!=='hidden'&&s.display!=='none'},hasBackground=color=>{const n=color.match(/[\\d.]+/g)?.map(Number)??[];return n.length===3||(n.length>=4&&n[3]>0)},colors=e=>{const s=getComputedStyle(e),svg=e.closest('svg[data-racing-dial]');if(svg){const face=svg.querySelector('.racing-dial-face');return {foreground:s.fill,background:face?getComputedStyle(face).fill:'transparent',source:'actual SVG dial face'}}for(let p=e;p;p=p.parentElement){const bg=getComputedStyle(p).backgroundColor;if(hasBackground(bg))return {foreground:s.color,background:bg,source:'nearest CSS painted background'}}return {foreground:s.color,background:'transparent',source:'no painted background'}};const gear=g.querySelector('[data-racing-gear]');let gearPaint=null,gearText=null;if(gear&&visible(gear)){const range=document.createRange();range.selectNodeContents(gear);gearText=range.getBoundingClientRect().toJSON();gearPaint=hasBackground(getComputedStyle(gear).backgroundColor)?gear.getBoundingClientRect().toJSON():gearText}
    return {rect:g.getBoundingClientRect().toJSON(),mode:g.dataset.mode,hidden:g.hidden,label:g.getAttribute('aria-label'),
      gear:gear?{text:gear.textContent,rect:gear.getBoundingClientRect().toJSON(),paintRect:gearPaint,textRect:gearText,fontSize:parseFloat(getComputedStyle(gear).fontSize),colors:colors(gear)}:null,
      readouts:[...g.querySelectorAll('[data-racing-speed],[data-racing-rpm],[data-racing-gear]')].filter(visible).map(e=>({kind:e.hasAttribute('data-racing-speed')?'speed':e.hasAttribute('data-racing-rpm')?'rpm':'gear',text:e.textContent,rect:e.getBoundingClientRect().toJSON(),colors:colors(e)})),
      dials:[...g.querySelectorAll('svg[data-racing-dial]')].filter(visible).map(svg=>{const rim=svg.querySelector('.racing-dial-rim'),m=svg.getScreenCTM();if(!rim||!m)return {kind:svg.dataset.racingDial,rect:svg.getBoundingClientRect().toJSON()};const c=new DOMPoint(rim.cx.baseVal.value,rim.cy.baseVal.value).matrixTransform(m),r=(rim.r.baseVal.value+parseFloat(getComputedStyle(rim).strokeWidth)/2)*Math.hypot(m.a,m.b);return {kind:svg.dataset.racingDial,rect:svg.getBoundingClientRect().toJSON(),circle:{cx:c.x,cy:c.y,r}}}),
      needles:[...g.querySelectorAll('[data-racing-speed-needle],[data-racing-rpm-needle]')].map(e=>({kind:e.hasAttribute('data-racing-speed-needle')?'speed':'rpm',transform:e.getAttribute('transform'),value:Number(e.getAttribute('data-value'))}))}})(),
  miniMap:document.querySelector('.racing-route-button')?.getBoundingClientRect().toJSON()??null,
  miniMapLabel:document.querySelector('.racing-route-button')?.innerText.trim()??'',miniMapAria:document.querySelector('.racing-route-button')?.getAttribute('aria-label')??null,
  chatToggle:document.querySelector('.chat-toggle')?.getBoundingClientRect().toJSON()??null,
  map:document.querySelector('canvas[aria-label="赛车赛道"]')?{...document.querySelector('canvas[aria-label="赛车赛道"]').dataset}:null,
  audio:await Promise.all([...document.querySelectorAll('audio[data-platform-loop]')].map(async a=>{const source=a.currentSrc;let sourceSHA256=null;if(source.startsWith('data:audio/flac;base64,')){const bytes=Uint8Array.from(atob(source.split(',')[1]),c=>c.charCodeAt(0));sourceSHA256=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(n=>n.toString(16).padStart(2,'0')).join('')};return {key:a.dataset.platformLoop,src:source.slice(0,100),sourceLength:source.length,sourceSHA256,paused:a.paused,readyState:a.readyState,time:a.currentTime,rate:a.playbackRate,volume:a.volume,muted:a.muted,error:a.error?.code??null}})),
  continuous:[...document.querySelectorAll('audio[data-platform-continuous]')].map(a=>({key:a.dataset.platformContinuous,owner:a.dataset.continuousOwner,state:a.dataset.continuousState,context:a.dataset.contextState,voices:JSON.parse(a.dataset.continuousVoices||'[]'),paused:a.paused,muted:a.muted,volume:a.volume,rate:a.playbackRate,loop:a.loop,readyState:a.readyState,time:a.currentTime,tracks:a.srcObject instanceof MediaStream?a.srcObject.getAudioTracks().map(t=>({id:t.id,readyState:t.readyState,enabled:t.enabled,muted:t.muted})):[],error:a.error?.code??null})),
  visibility:document.visibilityState,focus:document.hasFocus(),
  pointer:{coarse:matchMedia('(pointer:coarse)').matches,touchPoints:navigator.maxTouchPoints},
  layout:(()=>{const visible=e=>{if(!e)return false;const r=e.getBoundingClientRect(),s=getComputedStyle(e);return r.width>0&&r.height>0&&s.display!=='none'&&s.visibility!=='hidden'},box=e=>e?e.getBoundingClientRect().toJSON():null;
    const stage=document.querySelector('.racing-stage'),board=document.querySelector('.racing-board'),panel=document.querySelector('.racing-controls'),style=panel?getComputedStyle(panel):null;
    return {width:innerWidth,height:innerHeight,scrollY,canvas:box(c),stage:box(stage),board:box(board),coarse:matchMedia('(pointer:coarse)').matches,
      controls:[...document.querySelectorAll('.racing-board button,.racing-wheel,.racing-skins,.racing-skin-picker,.racing-settings')].filter(visible).map(e=>({label:e.getAttribute('aria-label')||e.textContent.trim(),class:e.className,box:box(e),withinStage:!!stage?.contains(e)})),
      touch:[...document.querySelectorAll('.racing-steering button,.racing-wheel,.racing-pedals button')].filter(visible).map(e=>({label:e.getAttribute('aria-label'),box:box(e)})),
      overlays:[...document.querySelectorAll('.racing-hud span,.racing-hud strong,.racing-speedometer,.racing-route-button,.racing-steering button,.racing-wheel,.racing-pedals button')].filter(visible).map(e=>({label:e.getAttribute('aria-label')||e.textContent.trim(),rect:box(e)})),
      hudParts:[...document.querySelectorAll('.racing-hud span,.racing-hud strong')].filter(visible).map(e=>({label:e.textContent.trim(),rect:box(e)})),
      panel:panel?{withinStage:!!stage?.contains(panel),background:style.backgroundColor,backgroundImage:style.backgroundImage,position:style.position}:null,
      paragraphs:[...document.querySelectorAll('.racing-board p,.racing-view-controls span,.racing-wheel small')].filter(visible).map(e=>e.textContent.trim())}})(),
  controls:controls?{...controls.dataset}:null,activeClass:document.activeElement?.getAttribute('class'),selected:getSelection().toString(),
  notices:[...document.querySelectorAll('.racing-error,.racing-network,.notice,.racing-status')].filter(e=>e.getClientRects().length).map(e=>e.textContent.trim()),
  safety:window.__nfsSafetyProbe?{activeTouches:window.__nfsSafetyProbe.activeTouches,blockPointerRelease:window.__nfsSafetyProbe.blockPointerRelease,blockedReleases:window.__nfsSafetyProbe.events.filter(e=>e.blocked).length}:null,
  viewport:innerWidth,modal:document.querySelector('[role=dialog]')?.getAttribute('aria-label')}})()`);
const screenshots = [], samples = [], touchInputs=[],guardProbes=[],safetyDOM=[];
const targetPoint=(selector,allowUnderlay=false)=>evaluate(`new Promise((resolve,reject)=>{const start=performance.now(),take=()=>{const e=document.querySelector(${JSON.stringify(selector)}),r=e?.getBoundingClientRect();if(r?.width>0&&r?.height>0){const x=r.x+r.width*.5,y=r.y+r.height*.5;if(!${!!allowUnderlay}&&!e.contains(document.elementFromPoint(x,y))){reject(Error('native target covered/offscreen'));return}resolve({x,y});return}if(performance.now()-start>3000){reject(Error('native target not visible'));return}requestAnimationFrame(take)};take()})`);
function percentile(rows, quantile) { const ordered = rows.slice().sort((a, b) => a - b); return ordered[Math.max(0, Math.ceil(ordered.length * quantile) - 1)] ?? null; }
let output;
try {
  const { targetInfos } = await call("Target.getTargets");
  const targets = targetInfos.filter((target) => target.type === "page" && target.url.startsWith(base));
  if (targets.length !== 1) throw Error("Exactly one owned fixture tab is required: " + JSON.stringify(targets.map((target) => target.url)));
  ({ sessionId: session } = await call("Target.attachToTarget", { targetId: targets[0].targetId, flatten: true }));
  await send("Page.enable"); await send("Runtime.enable"); await send("Network.enable"); await send("Page.bringToFront");
  wsObserveStartMS=Date.now();
  if (options.width) await send("Emulation.setDeviceMetricsOverride", { width: options.width, height: options.height ?? 1000, deviceScaleFactor: 1, mobile: !!options.touch });
  await send("Emulation.setTouchEmulationEnabled", { enabled: !!options.touch, maxTouchPoints: 5 });
  if(options.observeSkids)await evaluate("(()=>{const rows=[],p={rows,last:''};p.timer=setInterval(()=>{const c=document.querySelector('.racing-chase'),q=c?.dataset.skidSample;if(q&&q!==p.last){p.last=q;rows.push({at:Date.now(),frame:Number(c.dataset.frameCounter),quad:JSON.parse(q)})}},20);window.__nfsSkidPixels=p;return true})()");
  await evaluate(`(()=>{if(window.__nfsNativeProbe?.early){window.__nfsNativeProbe.events.length=0;return true}window.__nfsNativeProbe?.stop?.();const events=[];const listener=e=>events.push({key:e.key,type:e.type,at:performance.timeOrigin+e.timeStamp,received:performance.timeOrigin+performance.now()});
    addEventListener('keydown',listener,true);addEventListener('keyup',listener,true);window.__nfsNativeProbe={events,stop:()=>{removeEventListener('keydown',listener,true);removeEventListener('keyup',listener,true)}};return true})()`);
  if (options.mode === "sequence") {
    samples.push({ name: "before", data: await sample() });
    for (const action of options.actions ?? []) {
      if (action.wait) { await sleep(action.wait); continue; }
      if(action.safetyStart){await evaluate(`(()=>{window.__nfsSafetyProbe?.stop?.();const events=[],p={events,activeTouches:[],blockPointerRelease:false};const types=['touchstart','touchmove','touchend','touchcancel','contextmenu','selectstart','selectionchange'];
        const listen=e=>{if('touches'in e)p.activeTouches=[...e.touches].map(t=>({id:t.identifier,label:t.target instanceof Element?t.target.closest('[aria-label]')?.getAttribute('aria-label'):null}));const row={type:e.type,trusted:e.isTrusted,at:performance.timeOrigin+e.timeStamp,activeTouches:p.activeTouches.map(t=>({...t})),label:e.target instanceof Element?e.target.closest('[aria-label]')?.getAttribute('aria-label'):null,defaultPrevented:e.defaultPrevented};setTimeout(()=>{row.defaultPrevented=e.defaultPrevented;row.selected=getSelection().toString();events.push(row);if(events.length>1000)events.splice(0,events.length-1000)},0)};
        for(const type of types)addEventListener(type,listen,{capture:true,passive:true});p.stop=()=>{p.blockPointerRelease=false;for(const type of types)removeEventListener(type,listen,true)};window.__nfsSafetyProbe=p;return true})()`);continue;}
      if(action.dropPointerReleases!==undefined){await evaluate(`window.__nfsSafetyProbe.blockPointerRelease=${!!action.dropPointerReleases}`);continue;}
      if(action.guardProbe){guardProbes.push(...await evaluate(`(()=>{return ${JSON.stringify(action.guardProbe)}.flatMap(selector=>{const e=document.querySelector(selector);if(!e)throw Error('missing guard target '+selector);return ['contextmenu','selectstart'].map(type=>{const event=new Event(type,{bubbles:true,cancelable:true});const accepted=e.dispatchEvent(event);return {selector,type,trusted:event.isTrusted,defaultPrevented:event.defaultPrevented,accepted,kind:'explicit synthetic cancelable guard-boundary probe; not native iOS'}})})})()`));continue;}
      if(action.selectEditorAll){await send('Input.dispatchKeyEvent',{type:'keyDown',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:4,commands:['selectAll']});await send('Input.dispatchKeyEvent',{type:'keyUp',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:4});continue;}
      if(action.nativeClick||action.contextMenu){const target=action.nativeClick||action.contextMenu,p=await targetPoint(typeof target==='string'?target:target.selector,target.allowUnderlay),button=action.contextMenu?'right':'left',buttons=action.contextMenu?2:1;await send('Input.dispatchMouseEvent',{type:'mousePressed',...p,button,buttons,clickCount:1});await send('Input.dispatchMouseEvent',{type:'mouseReleased',...p,button,buttons:0,clickCount:1});continue;}
      if(action.safetyDOM){safetyDOM.push(await evaluate(`(async()=>{const root=document.querySelector('.racing-board'),nodes=[root,...root.querySelectorAll('*')],held=[...root.querySelectorAll('.racing-steering button,.racing-pedals button,button[aria-label="喇叭"],button[aria-label="向后看"]')];
        const calloutSupported=CSS.supports('-webkit-touch-callout','none');const violations=nodes.filter(e=>!e.closest('input,textarea,select,[contenteditable=true]')).map(e=>{const s=getComputedStyle(e);return {tag:e.tagName,class:e.className,label:e.getAttribute('aria-label'),userSelect:s.userSelect,webkitUserSelect:s.webkitUserSelect,callout:s.getPropertyValue('-webkit-touch-callout')}}).filter(r=>r.userSelect!=='none'||(calloutSupported&&r.callout!=='none'));
        const css=[];for(const sheet of [...document.styleSheets])if(sheet.href){const text=await(await fetch(sheet.href)).text();const blocks=text.match(/[^{}]+\\{[^{}]*-webkit-touch-callout\\s*:\\s*none[^{}]*\\}/g)||[];for(const block of blocks){const selector=block.slice(0,block.indexOf('{')).trim();let matches=false;try{matches=root.matches(selector)}catch{}if(matches)css.push({href:new URL(sheet.href).pathname,rule:block.slice(0,1200)})}}
        return {checkedNodes:nodes.length,calloutSupported,violations,rootProtectionCSS:css,held:held.map(e=>({label:e.getAttribute('aria-label'),marked:e.hasAttribute('data-game-hold-control')})),clickOnly:[...root.querySelectorAll('button[aria-label="切换视角"],button[aria-label="查看赛道地图"],button[aria-label="重置赛车"]')].map(e=>({label:e.getAttribute('aria-label'),marked:e.hasAttribute('data-game-hold-control')}))}})()`));continue;}
      if (action.waitFor === 'resettable') {
        const deadline=Date.now()+8000;let ready=false;
        while(Date.now()<deadline){ready=await evaluate("fetch('/api/state').then(r=>r.json()).then(s=>{const d=s.room.game.data,c=d.cars.find(c=>c.seat===s.room.you);return d.controls.canReset&&c.speed<=55})");if(ready)break;await sleep(80)}
        if(!ready)throw Error('Actual brake input did not reach authoritative resettable speed');continue;
      }
      if (action.key) { await key(action.key, !!action.down, !!action.repeat); continue; }
      if (action.gamepad) { await gamepad(action.gamepad); continue; }
      if (action.gamepadStop) { await evaluate("window.__nfsGamepadProbe?.restore()"); continue; }
      if (action.focus) { await evaluate(`new Promise((resolve,reject)=>{const start=performance.now(),find=()=>{const e=document.querySelector(${JSON.stringify(action.focus)}),r=e?.getBoundingClientRect();if(r?.width>0&&r?.height>0){e.focus({preventScroll:true});resolve(true);return}if(performance.now()-start>3000){reject(Error('focus target did not become visible'));return}requestAnimationFrame(find)};find()})`); continue; }
      if (action.read) { samples.push({ name: action.read, data: await sample() }); continue; }
      if (action.shot) { screenshots.push({ name: action.shot, data: (await send("Page.captureScreenshot", { format: "png", captureBeyondViewport: false })).data }); continue; }
      if (action.scrollTo) { await evaluate(`document.querySelector(${JSON.stringify(action.scrollTo)}).scrollIntoView({block:'center'})`); continue; }
      if(action.viewport){
        await send('Emulation.setDeviceMetricsOverride',{...action.viewport,deviceScaleFactor:1,mobile:true,screenOrientation:{type:action.viewport.width>action.viewport.height?'landscapePrimary':'portraitPrimary',angle:action.viewport.width>action.viewport.height?90:0}});
        continue;
      }
      if (action.reduced !== undefined) { await send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-reduced-motion", value: action.reduced ? "reduce" : "no-preference" }] }); continue; }
      if (action.headers) { await send("Network.setExtraHTTPHeaders", { headers: action.headers }); continue; }
      if (action.reload) { await send("Page.reload", { ignoreCache: false }); continue; }
      if (action.disconnect) { await evaluate("fetch('/_test/disconnect',{method:'POST'}).then(r=>r.json())"); continue; }
      if (action.freeze) {
        // Stop this renderer's actual JS/input heartbeat. The authenticated
        // production HTTP clock keeps advancing, observed outside the frozen
        // page. Cookie values are never included in output.
        const {cookies}=await send('Network.getCookies',{urls:[base]});
        const cookie=cookies.map(row=>row.name+'='+row.value).join('; ');
        await send('Page.setWebLifecycleState',{state:'frozen'});
        try {await sleep(action.milliseconds??1200);const response=await fetch(base+'/api/state',{headers:{Cookie:cookie}});
          samples.push({name:'frozen-authority',data:{status:response.status,state:await response.json(),method:'actual page lifecycle frozen; external authenticated production HTTP; real clock'}});
        } finally {await send('Page.setWebLifecycleState',{state:'active'});}
        await sleep(150);continue;
      }
      if (action.offline !== undefined) { await send("Network.emulateNetworkConditions", { offline: action.offline, latency: 0, downloadThroughput: -1, uploadThroughput: -1 }); continue; }
      if (action.blur) {
        const { targetId } = await call("Target.createTarget", { url: "about:blank" });
        const { sessionId: temporary } = await call("Target.attachToTarget", { targetId, flatten: true });
        try {
          await call("Page.bringToFront", {}, temporary); await sleep(action.milliseconds ?? 350);
          samples.push({ name: "blurred", data: await sample() });
        } finally {
          await call("Target.closeTarget", { targetId });
          await send("Page.bringToFront");
        }
        await sleep(150); samples.push({ name: "returned", data: await sample() }); continue;
      }
      if (action.touch) {
        const points = [];
        for (const p of action.points ?? []) {
          const xy = await evaluate(`new Promise((resolve,reject)=>{const start=performance.now(),find=()=>{const e=document.querySelector(${JSON.stringify(p.selector)}),r=e?.getBoundingClientRect();if(r?.width>0&&r?.height>0&&getComputedStyle(e).visibility!=='hidden'){const x=r.x+r.width*${p.x??.5},y=r.y+r.height*${p.y??.5},hit=document.elementFromPoint(x,y);resolve({x,y,target:hit?.getAttribute('aria-label')||hit?.className||hit?.tagName,expected:e.contains(hit)});return};if(performance.now()-start>3000){reject(Error('touch target did not become visible'));return}requestAnimationFrame(find)};find()})`);
          touchInputs.push({selector:p.selector,id:p.id,...xy,allowUnderlay:!!p.allowUnderlay,at:Date.now()});
          if(!xy.expected&&!p.allowUnderlay)throw Error('Actual touch target is covered/offscreen: '+JSON.stringify(touchInputs.at(-1)));
          points.push({ x:xy.x,y:xy.y,id: p.id, radiusX: 4, radiusY: 4, force: 1 });
        }
        await send("Input.dispatchTouchEvent", { type: action.touch, touchPoints: points }); continue;
      }
      if (action.wheel) {
        const xy = await evaluate(`(()=>{const r=document.querySelector(${JSON.stringify(action.wheel)}).getBoundingClientRect();return {x:r.x+r.width*.5,y:r.y+r.height*.5}})()`);
        await send("Input.dispatchMouseEvent", { type: "mouseWheel", ...xy, deltaX: 0, deltaY: action.delta ?? -180 }); continue;
      }
      if (action.drag) {
        const r = await evaluate(`document.querySelector(${JSON.stringify(action.drag)}).getBoundingClientRect().toJSON()`);
        const x = r.x + r.width * .5, y = r.y + r.height * .5;
        await send("Input.dispatchMouseEvent", { type: "mousePressed", x, y, button: "left", buttons: 1, clickCount: 1 });
        await send("Input.dispatchMouseEvent", { type: "mouseMoved", x: x + (action.dx ?? 40), y: y + (action.dy ?? 20), button: "left", buttons: 1 });
        await send("Input.dispatchMouseEvent", { type: "mouseReleased", x: x + (action.dx ?? 40), y: y + (action.dy ?? 20), button: "left", buttons: 0, clickCount: 1 }); continue;
      }
      throw Error("Unknown native action: " + JSON.stringify(action));
    }
    await sleep(250); samples.push({ name: "after", data: await sample() });
    output = { samples };
  } else if(options.mode==='offroad-circuit') {
    await send('Runtime.addBinding',{name:'__nfsCircuitProgress'});
    const stage=async name=>evaluate(`__nfsCircuitProgress(JSON.stringify({track:${JSON.stringify(options.track)},at:Date.now(),stage:${JSON.stringify(name)}}))`);
    const lapShot=async lap=>{await stage('capture-lap-'+lap);const r=await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});screenshots.push({name:options.track+'-lap-'+lap,data:r.data});if(options.progressPath)await writeFile(resolve(options.progressPath,'..',options.track+'-live-lap-'+lap+'.png'),Uint8Array.from(atob(r.data),c=>c.charCodeAt(0)))};
    await stage('initial-sample');
    await evaluate("document.querySelector('.racing-chase').focus({preventScroll:true})");
    const before=await sample();
    const drive=async(stopLap,excursionDone)=>evaluate(`(async()=>{const track=${JSON.stringify(options.track)},route=await(await fetch('/racing/v3/tracks/'+track+'.route.json',{signal:AbortSignal.timeout(8000)})).json(),zone=${JSON.stringify(options.zone)},requested=${options.outside??20},normalSpeed=${options.speed??14},stopLap=${stopLap},started=performance.now(),rows=[],transitions=[],warnings=[],n=route.points.length,arcs=[0],clamp=(v,a,b)=>Math.max(a,Math.min(b,v));let phase=${excursionDone?4:0},maxOutside=0,returned=${!!excursionDone},lastLap=-1,lastArc=null,travelled=0,waypointIndex=0,journalAt=0;
      const readState=async()=>{for(let attempt=1;attempt<=3;attempt++){try{const r=await fetch('/api/state',{signal:AbortSignal.timeout(3000)});if(!r.ok)throw Error('GET state status '+r.status);return await r.json()}catch(e){const warning={at:Date.now(),attempt,error:String(e)};warnings.push(warning);__nfsCircuitProgress(JSON.stringify({track,at:Date.now(),stage:'GET-state-retry',warning}));if(attempt===3)throw e;await new Promise(r=>setTimeout(r,100))}}};
      for(let i=0;i<n;i++){const a=route.points[i],b=route.points[(i+1)%n];arcs.push(arcs.at(-1)+Math.hypot(b.x-a.x,b.z-a.z,b.y-a.y))}const total=arcs.at(-1);
      const roadAt=c=>{const i=((c.routeIndex??0)+n)%n,a=route.points[i],b=route.points[(i+1)%n],dx=b.x-a.x,dz=b.z-a.z,den=dx*dx+dz*dz,u=den?clamp(((c.x/10-a.x)*dx+(c.y/10-a.z)*dz)/den,0,1):0,x=a.x+dx*u,z=a.z+dz*u,rx=a.rx+(b.rx-a.rx)*u,rz=a.rz+(b.rz-a.rz)*u,r=Math.hypot(rx,rz)||1,side=((c.x/10-x)*rx+(c.y/10-z)*rz)/r,left=a.leftWidth+(b.leftWidth-a.leftWidth)*u,right=a.rightWidth+(b.rightWidth-a.rightWidth)*u;return {index:i,fraction:u,side,left,right,outside:Math.max(0,side-right,-side-left),arc:arcs[i]+(arcs[i+1]-arcs[i])*u}};
      const hud=()=>[...document.querySelectorAll('.racing-hud>div')].find(e=>e.querySelector('span')?.textContent==='圈数')?.querySelector('strong')?.textContent.replace(/\\s/g,'');
      while(performance.now()-started<${options.maxSeconds??1160}*1000){const state=await readState(),room=state.room,d=room.game.data,c=d.cars.find(c=>c.seat===room.you),road=roadAt(c);if(room.seats[room.you].autoPlay)throw Error('circuit test unexpectedly became autoplay');maxOutside=Math.max(maxOutside,road.outside);if(lastArc!==null){let delta=road.arc-lastArc;if(delta<-total/2)delta+=total;if(delta>total/2)delta-=total;travelled+=delta}lastArc=road.arc;
        if(phase===0&&road.index>=zone.start&&road.index<=zone.start+3)phase=1;
        if(zone.waypoints?.length&&phase===1){while(waypointIndex<zone.waypoints.length&&Math.hypot(c.x/10-zone.waypoints[waypointIndex].x,c.y/10-zone.waypoints[waypointIndex].z)<(zone.waypoints[waypointIndex].radius??2.5))waypointIndex++;if(waypointIndex===zone.waypoints.length)phase=3}
        else if(!zone.waypoints?.length){if(phase===1&&road.outside>=requested)phase=2;if(phase===2&&road.index>=zone.end-4)phase=3}
        if(phase===3&&road.outside<.3&&road.index>=zone.end-3){phase=4;returned=true}
        let changed=c.lap!==lastLap;if(changed){const expected=Math.min(c.lap+1,3)+'/3',deadline=performance.now()+3000;while(hud()!==expected&&performance.now()<deadline)await new Promise(r=>setTimeout(r,30));if(hud()!==expected)throw Error('actual lap HUD disagrees with authority '+JSON.stringify({lap:c.lap,hud:hud(),expected}));lastLap=c.lap;transitions.push({at:Date.now(),serverAt:d.serverAt,lap:c.lap,hud:hud(),car:c})}
        const row={at:Date.now(),serverAt:d.serverAt,car:c,road,phase,waypointIndex,hud:hud(),maximumOutsideM:maxOutside,travelledM:travelled,events:d.events};rows.push(row);if(changed||rows.length%10===0){__nfsCircuitProgress(JSON.stringify({track,at:row.at,stage:'driving-to-lap-'+stopLap,elapsedMS:performance.now()-started,lap:c.lap,hud:row.hud,routeIndex:road.index,phase,waypointIndex,maximumOutsideM:maxOutside,travelledM:travelled,speed:c.speed/10,transition:changed,finishAt:c.finishAt,rows:rows.slice(journalAt,Math.max(journalAt,rows.length-1))}));journalAt=Math.max(journalAt,rows.length-1)}
        if(c.finishAt||d.phase==='finished'||(c.lap>=stopLap&&(stopLap>=3||c.speed>80))){__nfsCircuitProgress(JSON.stringify({track,at:Date.now(),stage:'small-return-to-lap-'+stopLap,leg:stopLap,lap:c.lap,hud:hud(),rows:rows.slice(journalAt)}));return {transitions,maximumOutsideM:maxOutside,returned,phase,travelledM:travelled,elapsedMS:performance.now()-started,completed:c.lap>=stopLap,final:d}}
        let index=(road.index+3)%n,lane=-2,wanted=normalSpeed;if(phase===1){index=zone.start+3;wanted=4}if(phase===2){index=Math.min(road.index+3,zone.end-2);wanted=4}if(phase===3){index=zone.end-1;wanted=4}
        if(phase===1||phase===2){const p=route.points[index],width=zone.side<0?p.leftWidth:p.rightWidth,extra=zone.extra??requested;lane=zone.side*(width+extra+1)}
        if(phase===0||phase===4){for(let k=0;k<20;k++){const a=route.points[(road.index+k)%n],b=route.points[(road.index+k+1)%n],e=route.points[(road.index+k+2)%n],dx=b.x-a.x,dz=b.z-a.z,ex=e.x-b.x,ez=e.z-b.z,l1=Math.hypot(dx,dz),l2=Math.hypot(ex,ez),turn=Math.abs(Math.atan2(dx*ez-dz*ex,dx*ex+dz*ez)),curvature=turn/Math.max(.1,(l1+l2)/2);if(curvature>1e-5)wanted=Math.min(wanted,Math.sqrt(4/curvature+2*6*k*4.6))}}
        const p=route.points[index],r=Math.hypot(p.rx,p.rz)||1,waypoint=phase===1?zone.waypoints?.[waypointIndex]:null,gx=waypoint?.x??p.x+p.rx/r*lane,gz=waypoint?.z??p.z+p.rz/r*lane;if(waypoint)wanted=waypoint.speed??4;const dx=gx-c.x/10,dz=gz-c.y/10,diff=Math.atan2(Math.sin(Math.atan2(dz,dx)-c.angle),Math.cos(Math.atan2(dz,dx)-c.angle)),steer=clamp(Math.atan2(2*2.272*Math.sin(diff),Math.max(4,Math.hypot(dx,dz)))/Math.atan(2.272/11),-1,1),pressure=clamp((c.speed/10-wanted-.5)/3,0,1),throttle=pressure?0:clamp(.18+(wanted-c.speed/10)/4,0,1),action={type:'drive',epoch:d.epoch,seq:c.seq+1,steer,throttle,brake:pressure>0,brakeAmount:pressure,drift:false};row.input={action,status:'pending'};let response;try{response=await fetch('/api/rooms/'+room.id,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'action',version:room.version,nonce:crypto.randomUUID(),action}),signal:AbortSignal.timeout(5000)})}catch(e){row.input.error=String(e);__nfsCircuitProgress(JSON.stringify({track,at:Date.now(),stage:'drive-POST-outcome-unknown-no-retry',rows:rows.slice(journalAt)}));throw e}row.input.status=response.status;if(![200,409].includes(response.status))throw Error('ordinary circuit input rejected '+response.status);await new Promise(r=>setTimeout(r,100))}
      __nfsCircuitProgress(JSON.stringify({track,at:Date.now(),stage:'small-return-incomplete-lap-'+stopLap,leg:stopLap,rows:rows.slice(journalAt)}));return {transitions,maximumOutsideM:maxOutside,returned,phase,travelledM:travelled,completed:false,elapsedMS:performance.now()-started}})()`);
    await lapShot(0);await stage('first-driving');const first=await drive(1,false);first.rows=circuitRows.get(1)??[];await lapShot(1);
    if(!first.completed){await progressWrite;await Promise.all(pendingShots);output={before,first,after:await sample(),incomplete:'ordinary offroad first lap did not complete'};}
    else {
    await stage('lap1-pre-reset-sample');const preReset=await sample();await evaluate("document.querySelector('.racing-chase').focus({preventScroll:true})");await stage('native-R-1');await key('r',true);await key('r',false);await sleep(180);const reset=await sample();
    await stage('native-R-rapid-repeat');await key('r',true);await key('r',false);await sleep(180);const repeated=await sample();
    const resetUntil=reset.room.game.data.cars[reset.room.you].resetUntil;
    await stage('wait-first-2s-reset-animation');let waitDeadline=Date.now()+8000;while((await sample()).room.game.data.serverAt<resetUntil){if(Date.now()>waitDeadline)throw Error('first reset animation exceeded bounded wait');await sleep(60)}
    await stage('native-R-after-2s');await key('r',true);await key('r',false);await sleep(180);const secondReset=await sample();
    const nextUntil=secondReset.room.game.data.cars[secondReset.room.you].resetUntil;await stage('wait-second-2s-reset-animation');waitDeadline=Date.now()+8000;while((await sample()).room.game.data.serverAt<nextUntil){if(Date.now()>waitDeadline)throw Error('second reset animation exceeded bounded wait');await sleep(60)}
    screenshots.push({name:options.track+'-native-reset-after-lap1',data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});
    await stage('actual-page-reload');await send('Page.reload');const readyDeadline=Date.now()+90000;let reloadedReady=false;while(Date.now()<readyDeadline){try{if(await evaluate("document.querySelector('.racing-chase')?.dataset.classic==='3'&&!document.querySelector('[data-game-loading]')")){reloadedReady=true;break}}catch{}await sleep(150)}if(!reloadedReady)throw Error('actual page reload never initialized classic canvas');await sleep(250);const restored=await sample();
    await evaluate("document.querySelector('.racing-chase').focus({preventScroll:true})");
    await stage('second-driving');const middle=await drive(2,false);middle.rows=circuitRows.get(2)??[];await lapShot(2);
    await stage('third-driving');const last=await drive(3,true);last.rows=circuitRows.get(3)??[];await lapShot(3);
    const second={...last,rows:[...middle.rows,...last.rows],transitions:[...middle.transitions,...last.transitions],maximumOutsideM:Math.max(middle.maximumOutsideM,last.maximumOutsideM),returned:middle.returned&&last.returned,travelledM:middle.travelledM+last.travelledM,elapsedMS:middle.elapsedMS+last.elapsedMS};await progressWrite;await Promise.all(pendingShots);
    output={before,first,preReset,reset,repeated,secondReset,restored,second,after:await sample(),inputMethod:'ordinary authenticated HTTP drive on human seat with actual offroad excursion; native R high-speed/repeat/2s recovery and real reload; no autoplay, BotAction, snapshot or clock writes'};
    }
  } else if(options.mode==='excursion-drive') {
    const before=await sample();
    const result=await evaluate(`(async()=>{const track=${JSON.stringify(options.track??'uk')},route=await(await fetch('/racing/v3/tracks/'+track+'.route.json')).json(),rows=[],started=performance.now(),limit=${options.seconds??55}*1000,plan=${JSON.stringify(options.excursions??[])},speed=${options.speed??14},clamp=(v,a,b)=>Math.max(a,Math.min(b,v));let maximumOutside=0,returned=false,reached=false,returnAt=null,completed=false;
      const measure=(c)=>{let best=null;for(let k=-12;k<=12;k++){const i=((c.routeIndex??0)+k+route.points.length)%route.points.length,a=route.points[i],b=route.points[(i+1)%route.points.length],dx=b.x-a.x,dz=b.z-a.z,den=dx*dx+dz*dz,u=den?clamp(((c.x/10-a.x)*dx+(c.y/10-a.z)*dz)/den,0,1):0,x=a.x+dx*u,z=a.z+dz*u,rx=a.rx+(b.rx-a.rx)*u,rz=a.rz+(b.rz-a.rz)*u,n=Math.hypot(rx,rz)||1,side=((c.x/10-x)*rx+(c.y/10-z)*rz)/n,left=a.leftWidth+(b.leftWidth-a.leftWidth)*u,right=a.rightWidth+(b.rightWidth-a.rightWidth)*u,distance=Math.hypot(c.x/10-x,c.y/10-z),outside=Math.max(0,side<0?-side-left:side-right);if(!best||distance<best.distance)best={i,side,left,right,outside,distance}}return best};
      while(performance.now()-started<limit){const state=await(await fetch('/api/state')).json(),room=state.room,d=room.game.data,c=d.cars.find(c=>c.seat===room.you),m=measure(c);maximumOutside=Math.max(maximumOutside,m.outside);const active=plan.find(p=>(p.lap===undefined||p.lap===c.lap)&&c.routeIndex>=p.start&&c.routeIndex<=p.end),goalIndex=((c.routeIndex??0)+Math.max(3,Math.min(7,Math.round(c.speed/10*.3))))%route.points.length,p=route.points[goalIndex],normal=Math.hypot(p.rx,p.rz)||1;let lane=-Math.min(2,Math.max(.1,p.leftWidth-1.2));
        if(active){const side=active.side??-1,width=side<0?p.leftWidth:p.rightWidth,ramp=clamp((c.routeIndex-active.start)/6,0,1);lane=side*(width+active.outside)*ramp;if(m.outside>=active.outside-.25)reached=true}
        if(reached&&!active&&m.outside<.7&&!returned){returned=true;returnAt={at:Date.now(),serverAt:d.serverAt,car:c,road:m}}
        const hud=[...document.querySelectorAll('.racing-hud>div')].find(e=>e.querySelector('span')?.textContent==='圈数')?.querySelector('strong')?.textContent.trim();rows.push({at:Date.now(),serverAt:d.serverAt,car:c,road:m,hud,events:d.events,activeExcursion:active??null});
        if(c.finishAt||(${!!options.stopAfterReturn}&&returned&&d.serverAt-returnAt.serverAt>5000)){completed=true;break}
        const x=p.x+p.rx/normal*lane,z=p.z+p.rz/normal*lane,dx=x-c.x/10,dz=z-c.y/10,diff=Math.atan2(Math.sin(Math.atan2(dz,dx)-c.angle),Math.cos(Math.atan2(dz,dx)-c.angle)),steer=clamp(Math.atan2(2*2.272*Math.sin(diff),Math.max(4,Math.hypot(dx,dz)))/Math.atan(2.272/11),-1,1),wanted=active?Math.min(speed,8):Math.abs(diff)>.7?6:speed,v=c.speed/10,pressure=clamp((v-wanted-.5)/3,0,1),throttle=pressure?0:clamp(.18+(wanted-v)/4,0,1),action={type:'drive',epoch:d.epoch,seq:c.seq+1,steer,throttle,brake:pressure>0,brakeAmount:pressure,drift:false};
        const response=await fetch('/api/rooms/'+room.id,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'action',version:room.version,nonce:crypto.randomUUID(),action})});rows.at(-1).input={action,status:response.status};if(![200,409].includes(response.status))throw Error('ordinary excursion input rejected '+response.status);await new Promise(r=>setTimeout(r,100))}
      return {rows,maximumOutside,reached,returned,returnAt,completed,elapsedMS:performance.now()-started,plan}})()`);
    screenshots.push({name:'actual-state',data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});output={before,...result,after:await sample(),inputMethod:'ordinary authenticated HTTP action/drive from original grid with declared lateral route targets; no snapshot/clock writes'};
  } else if(options.mode==='lap-shoulder-reset') {
    const before=await sample();
    const result=await evaluate(`(async()=>{const route=await(await fetch('/racing/v3/tracks/uk.route.json')).json(),rows=[],start=performance.now(),clamp=(v,a,b)=>Math.max(a,Math.min(b,v));let stopped=false;
      while(performance.now()-start<60000){const state=await(await fetch('/api/state')).json(),room=state.room,d=room.game.data,c=d.cars.find(c=>c.seat===room.you),index=c.routeIndex,i=(index+3)%route.points.length,p=route.points[i],r=Math.hypot(p.rx,p.rz),lane=i>=20&&i<=35?-p.leftWidth-1.8:Math.max(-p.leftWidth+1.2,Math.min(p.rightWidth-1.2,-2)),x=p.x+p.rx/r*lane,z=p.z+p.rz/r*lane,dx=x-c.x/10,dz=z-c.y/10,diff=Math.atan2(Math.sin(Math.atan2(dz,dx)-c.angle),Math.cos(Math.atan2(dz,dx)-c.angle)),steer=clamp(Math.atan2(2*2.272*Math.sin(diff),Math.max(4,Math.hypot(dx,dz)))/Math.atan(2.272/11),-1,1),braking=index>=65&&index<85,v=c.speed/10,wanted=Math.abs(diff)>.7?6:12,pressure=braking?1:clamp((v-wanted-.5)/3,0,1),throttle=pressure?0:clamp(.18+(wanted-v)/4,0,1);
        rows.push({at:Date.now(),serverAt:d.serverAt,car:c});if(braking&&c.speed<3){stopped=true;break}
        const response=await fetch('/api/rooms/'+room.id,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'action',version:room.version,nonce:crypto.randomUUID(),action:{type:'drive',epoch:d.epoch,seq:c.seq+1,steer,throttle,brake:pressure>0,brakeAmount:pressure,drift:false}})});if(![200,409].includes(response.status))throw Error('normal lap drive rejected '+response.status);await new Promise(r=>setTimeout(r,100))}
      return {rows,stopped,elapsedMS:performance.now()-start}})()`);
    screenshots.push({name:'stopped-before-reset',data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});
    output={before,...result,after:await sample(),inputMethod:'normal authenticated drive through UK shoulder from original grid, real clock; no state/clock mutation'};
  } else if(options.mode==='props-drive') {
    const before=await sample();
    await evaluate(`(()=>{const p={frames:[],driving:true};window.__nfsPropsProbe=p;const tick=()=>{const c=document.querySelector('.racing-chase');p.frames.push({at:Date.now(),frame:c?.dataset.frameCounter,broken:c?.dataset.propsBroken,animations:c?.dataset.propAnimations});if(p.frames.length>6000)p.frames.shift();if(p.driving)requestAnimationFrame(tick)};requestAnimationFrame(tick);return true})()`);
    const drive=options.observeOnly?evaluate(`new Promise(resolve=>setTimeout(()=>{window.__nfsPropsProbe.driving=false;resolve({frames:window.__nfsPropsProbe.frames,rows:[],hit:null})},${options.milliseconds??18000}))`):evaluate(`(async()=>{const rows=[],started=performance.now(),index=${options.propIndex??1},target=${JSON.stringify(options.target??[320.48711373,131.324340898])},wanted=${options.speed??4},passDistance=${options.passDistance??0},wall=${!!options.wall};
      const route=${options.followRoute?"await(await fetch('/racing/v3/tracks/uk.route.json')).json()":"null"};
      const clamp=(v,a,b)=>Math.max(a,Math.min(b,v));let room,hit=null,direction=null,minDistance=Infinity,maximumSpeed=0;
      while(performance.now()-started<45000){const state=await(await fetch('/api/state')).json();room=state.room;const d=room.game.data,c=d.cars.find(c=>c.seat===room.you),bits=Uint8Array.from(atob(d.props.broken),x=>x.charCodeAt(0));
        if(!direction){const dx=target[0]-c.x/10,dz=target[1]-c.y/10,n=Math.hypot(dx,dz);direction=[dx/n,dz/n]}
        const distance=Math.hypot(c.x/10-target[0],c.y/10-target[1]),progress=(c.x/10-target[0])*direction[0]+(c.y/10-target[1])*direction[1];minDistance=Math.min(minDistance,distance);maximumSpeed=Math.max(maximumSpeed,c.speed/10);
        rows.push({at:Date.now(),serverAt:d.serverAt,car:c,props:d.props,events:d.events,distance,progress});
        const completed=passDistance?progress>=passDistance&&minDistance<1.35:wall?maximumSpeed>3&&d.events.some(e=>e.type==='collision'&&e.seat===room.you):(bits[index>>3]&(1<<(index&7)));
        if(completed){hit=rows.at(-1);break}
        let goal=target;if(route&&((25-(c.routeIndex??0)+route.points.length)%route.points.length)>8){const p=route.points[((c.routeIndex??0)+6)%route.points.length];goal=[p.x,p.z]}else if(passDistance&&progress>0)goal=[target[0]+direction[0]*(passDistance+10),target[1]+direction[1]*(passDistance+10)];
        const dx=goal[0]-c.x/10,dz=goal[1]-c.y/10,diff=Math.atan2(Math.sin(Math.atan2(dz,dx)-c.angle),Math.cos(Math.atan2(dz,dx)-c.angle));const delta=Math.atan2(2*2.272*Math.sin(diff),Math.max(4,Math.hypot(dx,dz))),steer=clamp(delta/Math.atan(2.272/11),-1,1),v=c.speed/10,pressure=clamp((v-wanted-.5)/3,0,1),throttle=pressure>0?0:clamp(.18+(wanted-v)/4,0,1);
        const action={type:'drive',epoch:d.epoch,seq:c.seq+1,steer,throttle,brake:pressure>0,brakeAmount:pressure,drift:false};const response=await fetch('/api/rooms/'+room.id,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'action',version:room.version,nonce:crypto.randomUUID(),action})});const result=await response.json();rows.at(-1).input={action,status:response.status};if(![200,409].includes(response.status))throw Error('normal drive rejected '+JSON.stringify(result));await new Promise(r=>setTimeout(r,100));}
      if(room){const state=await(await fetch('/api/state')).json(),r=state.room,d=r.game.data,c=d.cars.find(c=>c.seat===r.you);await fetch('/api/rooms/'+r.id,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'action',version:r.version,nonce:crypto.randomUUID(),action:{type:'drive',epoch:d.epoch,seq:c.seq+1,steer:0,throttle:0,brake:true,brakeAmount:1,drift:false}})})}
      await new Promise(r=>setTimeout(r,1900));window.__nfsPropsProbe.driving=false;return {rows,hit,elapsedMS:performance.now()-started,minDistance,maximumSpeed,completion:${JSON.stringify(options.wall?'fixed-wall-contact':options.passDistance?'passed-broken-position':'target-prop-broken')},frames:window.__nfsPropsProbe.frames}})()`);
    let done=false,result,error;drive.then(v=>{result=v;done=true},e=>{error=e;done=true});let impactShot=false;
    screenshots.push({name:'before',data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});
    while(!done){const frame=await evaluate('window.__nfsPropsProbe.frames.at(-1)');if(!impactShot&&frame?.animations&&JSON.parse(frame.animations).some(p=>p.phase==='animating')){screenshots.push({name:'impact',data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});impactShot=true}await sleep(50)}
    if(error)throw error;screenshots.push({name:'after',data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});output={before,...result,after:await sample(),inputMethod:options.observeOnly?'passive independent observer; no driving input':'ordinary authenticated HTTP action/drive closed loop, original Start position, real clock; no snapshot mutation'};
  } else if (options.mode === "performance") {
    const duration = options.milliseconds ?? 6000;
    const before = await sample();
    measurementWindow={start:Date.now()};const rows = await evaluate(`new Promise(resolve=>{const start=performance.now(),rows=[];const take=()=>{const c=document.querySelector('.racing-chase');
      rows.push({at:performance.now(),frame:Number(c?.dataset.frameCounter),renderedAt:Number(c?.dataset.renderedAt),x:Number(c?.dataset.renderX),y:Number(c?.dataset.renderY),snapshotAt:Number(c?.dataset.snapshotAt)});
      if(performance.now()-start<${duration})requestAnimationFrame(take);else resolve(rows)};requestAnimationFrame(take)})`);
    measurementWindow.end=Date.now();const unique = rows.filter((row, index) => index === 0 || row.frame !== rows[index - 1].frame);
    const intervals = unique.slice(1).map((row, index) => row.at - unique[index].at);
    const elapsed = rows.at(-1).at - rows[0].at;
    output = { before, after: await sample(), rows,
      metrics: { elapsed, rafSamples: rows.length, actualDrawUpdates: unique.length - 1, fps: (unique.length - 1) * 1000 / elapsed,
        p50MS: percentile(intervals, .5), p95MS: percentile(intervals, .95), p99MS: percentile(intervals, .99),
        moved: Math.hypot(rows.at(-1).x - rows[0].x, rows.at(-1).y - rows[0].y) },
      method: "Every display RAF sampled, counted only changed renderer frameCounter; total count and elapsed time retained, not merely intervals of drawn frames." };
  } else if(options.mode==='audio-container') {
    const before=await sample();
    const result=await evaluate(`(async()=>{const a=document.querySelector('audio[data-platform-loop="racing-engine-ldia-41"]');if(!a)throw Error('existing platform LDIA41 lane missing');const original={src:a.currentSrc,rate:a.playbackRate,defaultRate:a.defaultPlaybackRate,volume:a.volume,loop:a.loop},results=[],events=[];
      const types=['seeking','seeked','waiting','playing','pause','ratechange'];const listener=e=>{if(e.target===a)events.push({at:Date.now(),type:e.type,trusted:e.isTrusted,time:a.currentTime,rate:a.playbackRate,volume:a.volume,duration:a.duration})};for(const t of types)a.addEventListener(t,listener);
      const digest=async src=>{const bytes=Uint8Array.from(atob(src.split(',')[1]),c=>c.charCodeAt(0));return [...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(n=>n.toString(16).padStart(2,'0')).join('')};
      try{for(const part of [{label:'original-flac',src:original.src,expected:${JSON.stringify(options.flacSHA256)}},{label:'same-pcm-wav',src:${JSON.stringify(options.wavSource)},expected:${JSON.stringify(options.wavSHA256)}}]){if(await digest(part.src)!==part.expected)throw Error('controlled container bytes differ');a.pause();a.defaultPlaybackRate=original.rate;a.src=part.src;a.loop=true;a.load();await new Promise((resolve,reject)=>{if(a.readyState>=3)resolve();else{a.addEventListener('canplay',resolve,{once:true});a.addEventListener('error',()=>reject(Error('controlled source decode failed')),{once:true})}});a.playbackRate=original.rate;await a.play();const stream=a.captureStream(),recorder=new MediaRecorder(stream,{mimeType:'audio/webm;codecs=opus',audioBitsPerSecond:128000}),chunks=[],startedAt=Date.now(),rows=[];recorder.ondataavailable=e=>{if(e.data.size)chunks.push(e.data)};const stopped=new Promise(resolve=>recorder.onstop=resolve);recorder.start(100);
        const actualSourceSHA256=await digest(a.currentSrc);if(actualSourceSHA256!==part.expected)throw Error('platform changed controlled source before capture');while(Date.now()-startedAt<11000){rows.push({at:Date.now(),time:a.currentTime,rate:a.playbackRate,volume:a.volume,duration:a.duration,paused:a.paused,muted:a.muted,sourceKind:a.currentSrc.slice(0,24)});await new Promise(r=>setTimeout(r,20))}if(await digest(a.currentSrc)!==part.expected)throw Error('platform changed controlled source during capture');recorder.stop();await Promise.race([stopped,new Promise(r=>setTimeout(r,2000))]);for(const t of stream.getTracks())t.stop();const bytes=new Uint8Array(await new Blob(chunks).arrayBuffer());let text='';for(let i=0;i<bytes.length;i+=16384)text+=String.fromCharCode(...bytes.subarray(i,i+16384));results.push({label:part.label,key:'racing-engine-ldia-41',startedAt,stoppedAt:Date.now(),mime:'audio/webm;codecs=opus',sourceSHA256:part.expected,actualSourceSHA256,rows,data:btoa(text)})}}
      finally{a.pause();a.defaultPlaybackRate=original.defaultRate;a.src=original.src;a.loop=original.loop;a.load();a.playbackRate=original.rate;a.volume=original.volume;await a.play().catch(()=>{});for(const t of types)a.removeEventListener(t,listener)}
      return {audioArtifacts:results,mediaEvents:events,scope:'controlled same existing native lane source-container experiment; fixed observed idle rate/gain; not ordinary driving or production loading acceptance',originalRate:original.rate,originalGain:original.volume,elementStillSame:a===document.querySelector('audio[data-platform-loop="racing-engine-ldia-41"]')}})()`);
    output={before,...result,after:await sample()};
  } else if(options.mode==='audio-continuous') {
    const before=await sample();
    const recording=evaluate(`(async()=>{const media=[...document.querySelectorAll('audio[data-platform-continuous]')];if(media.length!==1)throw Error('expected unique platform continuous native sink, got '+media.length);const a=media[0];if(!(a.srcObject instanceof MediaStream))throw Error('continuous sink has no actual mixed MediaStream');const originals=a.srcObject.getAudioTracks();if(originals.length!==1)throw Error('continuous sink must contain one mixed audio track');const stream=new MediaStream(originals.map(t=>t.clone())),started=Date.now(),chunks=[],events=[],rows=[],errors=[],mime='audio/webm;codecs=opus',recorder=new MediaRecorder(stream,{mimeType:mime,audioBitsPerSecond:192000});
      const types=['play','playing','pause','waiting','stalled','ended','error','volumechange','ratechange','seeking','seeked'];const listener=e=>events.push({at:Date.now(),type:e.type,trusted:e.isTrusted,time:a.currentTime,volume:a.volume,muted:a.muted,rate:a.playbackRate,paused:a.paused});for(const type of types)a.addEventListener(type,listener);
      recorder.ondataavailable=e=>{if(e.data.size)chunks.push(e.data)};recorder.onerror=e=>errors.push({at:Date.now(),type:'recorder-error',error:String(e.error)});const stopped=new Promise(resolve=>recorder.onstop=resolve);recorder.start(100);window.__nfsContinuousCaptureProbe={rows,events,errors,recorder,media:a,stream};
      try{while(Date.now()-started<${options.milliseconds??36000}){rows.push({at:Date.now(),time:a.currentTime,master:a.volume,muted:a.muted,paused:a.paused,ready:a.readyState,rate:a.playbackRate,loop:a.loop,connected:a.isConnected,owner:a.dataset.continuousOwner,state:a.dataset.continuousState,context:a.dataset.contextState,voices:JSON.parse(a.dataset.continuousVoices||'[]'),nativeSinks:document.querySelectorAll('audio[data-platform-continuous]').length,fallbacks:[...document.querySelectorAll('audio[data-continuous-fallback]')].map(x=>({...x.dataset})),sourceTracks:originals.map(t=>({id:t.id,readyState:t.readyState,enabled:t.enabled,muted:t.muted}))});await new Promise(r=>setTimeout(r,40))}}
      finally{if(recorder.state!=='inactive')recorder.stop();await new Promise(resolve=>{const timer=setTimeout(()=>{errors.push({at:Date.now(),type:'recorder-stop-timeout'});resolve()},3000);stopped.then(()=>{clearTimeout(timer);resolve()})});for(const t of stream.getTracks())t.stop();for(const type of types)a.removeEventListener(type,listener)}
      const bytes=new Uint8Array(await new Blob(chunks,{type:mime}).arrayBuffer());let data='';for(let i=0;i<bytes.length;i+=16384)data+=String.fromCharCode(...bytes.subarray(i,i+16384));return {audioArtifacts:[{id:1,key:a.dataset.platformContinuous,startedAt:started,stoppedAt:Date.now(),mime,data:btoa(data)}],mediaTimeline:rows,mediaEvents:events,captureErrors:errors,startedAt:started,stoppedAt:Date.now(),captureMethod:'clone actual unique platform continuous HTMLAudio.srcObject mixed audio track -> MediaRecorder; stream already includes voice gain/pitch; native master envelope retained separately, no per-voice reconstruction',originalTrackStillLive:originals.every(t=>t.readyState==='live'),elementStillSame:a===document.querySelector('audio[data-platform-continuous]')}})()`);
    recording.catch(()=>{});
    if(!options.captureOnly){await sleep(8000);await key('w',true);await key(' ',true);await sleep(8000);await key(' ',false);await sleep(7000);await key('w',false);await key('s',true);for(let i=0;i<40;i++){const state=await sample(),car=state.room.game.data.cars.find(c=>c.seat===state.room.you);if(car.speed<2)break;await sleep(100)}await key('s',false)}
    output={before,...await recording,after:await sample(),captureOnly:!!options.captureOnly,controlProtocol:options.captureOnly?'stable idle only; no driving inputs':'native 8s idle, 8s W+Space stationary load, 7s W acceleration, S brake to stop, remaining idle',hardwareMutedByLaunchFlag:true};
  } else if(options.mode==='audio-output') {
    const before=await sample();
    process.stderr.write('audio-capture: begin\n');
    const recording=evaluate(`(async()=>{const started=Date.now(),entries=[],events=[],rows=[],errors=[],seen=new WeakMap(),mediaIDs=new WeakMap();let serial=0,mediaSerial=0;
      const eventTypes=['playing','play','pause','seeking','seeked','waiting','stalled','ended','error','volumechange','ratechange'];
      const listen=e=>{const a=e.target;if(a instanceof HTMLMediaElement&&a.dataset.platformLoop?.startsWith('racing-engine-'))events.push({at:Date.now(),type:e.type,trusted:e.isTrusted,key:a.dataset.platformLoop,id:seen.get(a)?.id,time:a.currentTime,volume:a.volume,rate:a.playbackRate,paused:a.paused})};for(const type of eventTypes)addEventListener(type,listen,true);
      window.__nfsAudioCaptureProbe={entries,events,rows,errors};
      const capture=a=>{let entry;try{const stream=a.captureStream();if(!stream.getAudioTracks().length)return;const id=++serial,key=a.dataset.platformLoop,chunks=[],mime='audio/webm;codecs=opus',recorder=new MediaRecorder(stream,{mimeType:mime,audioBitsPerSecond:128000});entry={id,key,startedAt:Date.now(),mime,chunks,recorder,media:a,stream};seen.set(a,entry);entries.push(entry);recorder.ondataavailable=e=>{if(e.data.size)chunks.push(e.data)};entry.done=new Promise(resolve=>{entry.resolve=resolve;recorder.onstop=()=>{entry.stoppedAt=Date.now();for(const t of stream.getTracks())t.stop();resolve()}});recorder.onerror=e=>{errors.push({at:Date.now(),key,id,type:'recorder-error',error:String(e.error)});entry.resolve()};for(const track of stream.getAudioTracks())track.addEventListener('ended',()=>{events.push({at:Date.now(),type:'capture-track-ended',key,id,requestedByCapture:!!entry.ending});if(recorder.state!=='inactive')recorder.stop()});recorder.start(100)}catch(e){errors.push({at:Date.now(),key:a.dataset.platformLoop,type:'capture-start',error:String(e)});if(entry)entry.resolve();seen.set(a,{failed:true})}};
      while(Date.now()-started<${options.milliseconds??28000}){const voices=[...document.querySelectorAll('audio[data-platform-loop^="racing-engine-"]')];for(const a of voices){if(!mediaIDs.has(a))mediaIDs.set(a,++mediaSerial);const old=seen.get(a);if(old?.recorder&&old.recorder.state!=='inactive'&&a.volume<=.003){old.ending='gain-below-capture-threshold';old.recorder.stop();seen.delete(a)}if(!seen.has(a)&&!a.paused&&a.readyState>=3&&a.volume>.005)capture(a)}rows.push({at:Date.now(),voices:voices.map(a=>({id:seen.get(a)?.id,nativeId:mediaIDs.get(a),key:a.dataset.platformLoop,time:a.currentTime,volume:a.volume,rate:a.playbackRate,paused:a.paused,ready:a.readyState,connected:a.isConnected}))});for(const entry of entries)if(!entry.media.isConnected&&entry.recorder.state!=='inactive'){entry.ending='media-detached';entry.recorder.stop()}await new Promise(r=>setTimeout(r,40))}
      for(const entry of entries)if(entry.recorder.state!=='inactive'){entry.ending='capture-duration';entry.recorder.stop()}await Promise.race([Promise.all(entries.map(e=>e.done)),new Promise(resolve=>setTimeout(()=>{for(const e of entries)if(!e.stoppedAt)errors.push({at:Date.now(),id:e.id,key:e.key,type:'recorder-stop-timeout',chunks:e.chunks.length,state:e.recorder.state,tracks:e.stream.getAudioTracks().map(t=>({readyState:t.readyState,muted:t.muted,enabled:t.enabled}))});resolve()},3000))]);for(const type of eventTypes)removeEventListener(type,listen,true);
      const clips=[];for(const e of entries){const bytes=new Uint8Array(await new Blob(e.chunks,{type:e.mime}).arrayBuffer());let data='';for(let i=0;i<bytes.length;i+=16384)data+=String.fromCharCode(...bytes.subarray(i,i+16384));clips.push({id:e.id,key:e.key,startedAt:e.startedAt,stoppedAt:e.stoppedAt,mime:e.mime,data:btoa(data)})}return {audioArtifacts:clips,mediaTimeline:rows,mediaEvents:events,captureErrors:errors,startedAt:started,stoppedAt:Date.now(),captureMethod:'actual HTMLMediaElement.captureStream per voice -> MediaRecorder; no substitute player or source synthesis'}})()`);
    recording.catch(()=>{});
    if(!options.captureOnly){await sleep(8000);process.stderr.write('audio-capture: idle complete\n');await key('w',true);if(options.stationaryLoadMS){await key(' ',true);await sleep(options.stationaryLoadMS);await key(' ',false)}await sleep(7000);await key('w',false);await key('s',true);
      for(let i=0;i<40;i++){const state=await sample(),car=state.room.game.data.cars.find(c=>c.seat===state.room.you);if(car.speed<2)break;await sleep(100)}await key('s',false);process.stderr.write('audio-capture: driving complete\n');}
    const captured=await recording;process.stderr.write('audio-capture: recording returned\n');output={before,...captured,after:await sample(),hardwareMutedByLaunchFlag:true};
  } else if (options.mode === "audio") {
    const before = await sample();
    const rows = await evaluate(`new Promise(resolve=>{const start=performance.now(),rows=[];const take=()=>{rows.push({at:Date.now(),voices:[...document.querySelectorAll('audio[data-platform-loop^="racing-engine-"]')].map(a=>({key:a.dataset.platformLoop,paused:a.paused,readyState:a.readyState,time:a.currentTime,rate:a.playbackRate,volume:a.volume,muted:a.muted,error:a.error?.code??null}))});
      if(performance.now()-start<${options.milliseconds ?? 12000})setTimeout(take,80);else resolve(rows)};take()})`);
    output = { before, rows, after: await sample(), mediaEvents: await evaluate("window.__nfsMediaProbe?.events??[]") };
  } else if (options.mode === "pixels") {
    if (!await evaluate("window.__nfsNativeProbe?.early===true")) throw Error("Native-event observer must be installed before application controllers (AGENT_BROWSER_INIT_SCRIPTS)");
    const initial = await sample(), own = initial.room.game.data.cars.find((car) => car.seat === initial.room.you);
    if (!own || own.speed > 2) throw Error("Front-wheel pixel timing requires the controlled car at rest");
    await evaluate("document.querySelector('.racing-chase').focus({preventScroll:true})");
    await key("b", true); await sleep(500);
    // A manually supplied rectangle is only for validating this measurement
    // tool against legacy pages. Remake acceptance requires renderer projection.
    const rect = await evaluate(`(()=>{const c=document.querySelector('.racing-chase'),r=c.getBoundingClientRect(),v=${options.validationRect ? JSON.stringify(options.validationRect) : "JSON.parse(c.dataset.vehicleRect||'null')"};if(!v)throw Error('actual projected vehicleRect missing');return {x:r.x+v.x,y:r.y+v.y,width:v.width,height:v.height,viewportWidth:innerWidth,viewportHeight:innerHeight}})()`);
    if (rect.width < 20 || rect.height < 20 || rect.x < 0 || rect.y < 0 || rect.x + rect.width > rect.viewportWidth || rect.y + rect.height > rect.viewportHeight) throw Error("Vehicle pixel box is invalid/offscreen: " + JSON.stringify(rect));
    await send("Page.startScreencast", { format: "png", everyNthFrame: 1, maxWidth: rect.viewportWidth, maxHeight: rect.viewportHeight }); screen = true;
    const trials = [],preInputSettling=[];measurementWindow={start:Date.now()};
    for (let index = 0; index < (options.trials ?? 8); index++) {
      const settled=await evaluate(`new Promise((resolve,reject)=>{const start=performance.now(),rows=[];let stableAt=null,lastFrame=-1,draws=0;const take=()=>{const c=document.querySelector('.racing-chase'),steering=Number(c?.dataset.steering),frame=Number(c?.dataset.frameCounter),at=performance.now();rows.push({at,steering,frame});if(Number.isFinite(steering)&&Math.abs(steering)<=.002){if(stableAt===null){stableAt=at;draws=0}if(frame!==lastFrame)draws++;if(at-stableAt>=250&&draws>=6){resolve({elapsedMS:at-start,stableMS:at-stableAt,draws,rows});return}}else{stableAt=null;draws=0}lastFrame=frame;if(at-start>=5000){reject(Error('actual rendered steering did not settle before new causal input'));return}requestAnimationFrame(take)};requestAnimationFrame(take)})`);
      preInputSettling.push({index,...settled});
      pixelTrial = { index, frames: [] };
      await sleep(130);
      if (pixelTrial.frames.length < 1) {
        const data = (await send("Page.captureScreenshot", { format: "png", captureBeyondViewport: false })).data;
        const baseline = { data, timestamp: Date.now(), receivedAt: Date.now(), metadata: {}, trial: index };
        pixelTrial.frames.push(baseline); pixelFrames.push(baseline);
      }
      const baselineCount = pixelTrial.frames.length;
      const label = index % 2 ? "向左转向" : "向右转向";
      let downAt;
      if(options.touchPixels){
        const point=await evaluate(`(()=>{const e=document.querySelector('button[aria-label="${label}"]'),r=e.getBoundingClientRect(),x=r.x+r.width*.5,y=r.y+r.height*.5;if(!r.width||!e.contains(document.elementFromPoint(x,y)))throw Error('actual thumb target is not visible');return {x,y}})()`);
        downAt=Date.now();
        await send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{...point,id:1,radiusX:4,radiusY:4,force:1}],timestamp:downAt/1000});
      }else downAt=await key(index % 2 ? "a" : "d",true);
      await sleep(180);
      const event = await evaluate(options.touchPixels?`window.__nfsNativeProbe.events.filter(e=>e.type==='pointerdown'&&e.label===${JSON.stringify(label)}).at(-1)`:`window.__nfsNativeProbe.events.filter(e=>e.type==='keydown'&&e.key===${JSON.stringify(index % 2 ? "a" : "d")}).at(-1)`);
      const state = await evaluate(`(()=>{const c=document.querySelector('.racing-chase');return {steering:Number(c.dataset.steering),renderedAt:Number(c.dataset.renderedAt),frameCounter:Number(c.dataset.frameCounter)}})()`);
      trials.push({ index, downAt, event, state, baselineCount, frames: pixelTrial.frames.slice() });
      pixelTrial = null;
      if(options.touchPixels)await send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
      else await key(index % 2 ? "a" : "d", false);
      await sleep(350);
    }
    measurementWindow.end=Date.now();await send("Page.stopScreencast"); screen = false; await key("b", false);
    const metrics = [];
    for (const trial of trials) {
      const before = trial.frames.slice(0, trial.baselineCount), after = trial.frames.slice(trial.baselineCount);
      if (!trial.event || !before.length) throw Error("Missing native event/baseline for pixel trial");
      const comparisons = await evaluate(`(async()=>{const rect=${JSON.stringify(rect)},frames=${JSON.stringify(trial.frames.map((frame) => ({ data: frame.data, timestamp: frame.timestamp })))},baselineCount=${trial.baselineCount};
        const images=[];for(const frame of frames){const bytes=Uint8Array.from(atob(frame.data),c=>c.charCodeAt(0)),image=await createImageBitmap(new Blob([bytes],{type:'image/png'}));const c=new OffscreenCanvas(Math.ceil(rect.width),Math.ceil(rect.height)),ctx=c.getContext('2d');
        const sx=image.width/rect.viewportWidth,sy=image.height/rect.viewportHeight;ctx.drawImage(image,rect.x*sx,rect.y*sy,rect.width*sx,rect.height*sy,0,0,c.width,c.height);images.push(ctx.getImageData(0,0,c.width,c.height).data);image.close();}
        const difference=(a,b)=>{let sum=0,changed=0;for(let i=0;i<a.length;i+=4){let delta=0;for(let c=0;c<3;c++){const v=Math.abs(a[i+c]-b[i+c]);sum+=v;delta+=v;}if(delta>=24)changed++;}return {mean:sum/(a.length/4*3),changed,pixels:a.length/4}};
        const baseline=images[baselineCount-1];return {noise:images.slice(0,baselineCount-1).map(a=>difference(a,baseline)),after:images.slice(baselineCount).map((image,i)=>({...difference(image,baseline),timestamp:frames[baselineCount+i].timestamp}))}})()`);
      const noiseMean = Math.max(0, ...comparisons.noise.map((row) => row.mean));
      const noiseChanged = Math.max(0, ...comparisons.noise.map((row) => row.changed));
      const visible = comparisons.after.find((row) => row.timestamp >= trial.event.at && row.mean > Math.max(.03, noiseMean * 2.5 + .03) && row.changed > Math.max(10, noiseChanged * 2 + 4));
      metrics.push({ index: trial.index, nativeEvent: trial.event, protocolSendAt: trial.downAt, steeringAfter: trial.state.steering,
        noiseMean, noiseChanged, comparisons: comparisons.after, firstVisibleAt: visible?.timestamp ?? null,
        latencyMS: visible ? visible.timestamp - trial.event.at : null });
    }
    const latencies = metrics.map((metric) => metric.latencyMS).filter((value) => value !== null);
    output = { rect, rectSource: options.validationRect ? "supplied-harness-validation" : "renderer-projected",preInputSettling, trials: metrics, metrics: { trials: metrics.length, visibleTrials: latencies.length, p50MS: percentile(latencies, .5), p95MS: percentile(latencies, .95), maxMS: Math.max(...latencies) },
      method: "Native steering at rest with B/front camera; before each new input, actual rendered wheel steering must settle for 250ms and six draws (bounded 5s); actual compositor PNG pixels inside projected own-vehicle bounds; unchanged preinput-noise formula; all eight sent trials retained; native page event time and compositor frame timestamps share epoch clock. Dataset/ACK are separate observations." };
  } else throw Error("Unknown probe mode: " + options.mode);
  if(options.observeSkids)output.skidObservations=await evaluate("(()=>{const p=window.__nfsSkidPixels;clearInterval(p.timer);return p.rows})()");
  if(options.skidPixelShot){
    const row=output.samples.find(r=>r.name===options.skidPixelShot.sample)?.data,shot=screenshots.find(r=>r.name===options.skidPixelShot.shot);
    if(!row||!shot)throw Error('skid pixel proof lacks actual sampled pose or PNG');
    output.skidPixelProof=await evaluate(`(async()=>{const s=${JSON.stringify(row)},rows=${JSON.stringify(output.skidObservations)},raw=${JSON.stringify(shot.data)},camera=JSON.parse(s.canvas.camera),vehicle=JSON.parse(s.canvas.vehicleRect),r=s.canvas.rect,eye=Object.values(camera.eye),target=Object.values(camera.target),sub=(a,b)=>a.map((v,i)=>v-b[i]),dot=(a,b)=>a.reduce((v,x,i)=>v+x*b[i],0),cross=(a,b)=>[a[1]*b[2]-a[2]*b[1],a[2]*b[0]-a[0]*b[2],a[0]*b[1]-a[1]*b[0]],unit=a=>{const l=Math.hypot(...a);return a.map(v=>v/l)},forward=unit(sub(target,eye)),right=unit(cross(forward,[0,1,0])),up=cross(right,forward),tangent=Math.tan(camera.fov*Math.PI/360),project=p=>{const v=sub(p,eye),depth=dot(v,forward);if(depth<=camera.near)return null;return {x:r.width*(1+dot(v,right)/(depth*tangent*r.width/r.height))/2,y:r.height*(1-dot(v,up)/(depth*tangent))/2}};
      const bytes=Uint8Array.from(atob(raw),c=>c.charCodeAt(0)),bitmap=await createImageBitmap(new Blob([bytes],{type:'image/png'})),canvas=new OffscreenCanvas(bitmap.width,bitmap.height),ctx=canvas.getContext('2d');ctx.drawImage(bitmap,0,0);const pixels=ctx.getImageData(0,0,canvas.width,canvas.height).data,scale=bitmap.width/innerWidth,within=(p,rect)=>p.x>=rect.x&&p.x<=rect.x+rect.width&&p.y>=rect.y&&p.y<=rect.y+rect.height,light=p=>{let sum=0,count=0;const x=Math.round((r.x+p.x)*scale),y=Math.round((r.y+p.y)*scale);for(let dy=-1;dy<=1;dy++)for(let dx=-1;dx<=1;dx++){const at=((y+dy)*canvas.width+x+dx)*4;if(at>=0&&at+2<pixels.length){sum+=.2126*pixels[at]+.7152*pixels[at+1]+.0722*pixels[at+2];count++}}return sum/count},points=[];
      for(const entry of rows){const q=entry.quad;if(q.born>Number(s.canvas.renderedAt)||q.born<Number(s.canvas.renderedAt)-15000||q.seat!==s.room.you)continue;const v=q.vertices,a=project(v.slice(0,3)),b=project(v.slice(3,6)),d=project(v.slice(6,9)),e=project(v.slice(15,18));if(!a||!b||!d||!e)continue;const centre={x:(a.x+b.x+d.x+e.x)/4,y:(a.y+b.y+d.y+e.y)/4},width=Math.hypot(a.x-b.x,a.y-b.y);if(width<1||within(centre,vehicle)||centre.x<15||centre.x>r.width-15||centre.y<r.height*.45||centre.y>r.height-8)continue;
        const dx=(b.x-a.x)/width,dy=(b.y-a.y)/width,offset=Math.max(4,width*1.7),left={x:centre.x-dx*offset,y:centre.y-dy*offset},right={x:centre.x+dx*offset,y:centre.y+dy*offset},middle=light(centre),neighbour=(light(left)+light(right))/2;points.push({slot:q.slot,wheel:q.wheel,born:q.born,surface:q.surface,centre,widthPixels:width,markLuminance:middle,adjacentLuminance:neighbour,darkening:neighbour-middle})}
      bitmap.close();return {method:'actual observed world skid quad vertices independently projected with sampled camera into compositor PNG; 3x3 centre luminance compared to both adjacent ground samples; own vehicle excluded',samples:points,darkerSamples:points.filter(p=>p.darkening>8).length,maximumDarkening:Math.max(0,...points.map(p=>p.darkening))}})()`);
  }
  if(options.screenshot)screenshots.push({name:options.screenshot,data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});
  if(options.compareShots){
    const [a,b]=options.compareShots.map(name=>screenshots.find(s=>s.name===name));if(!a||!b)throw Error('comparison screenshot missing');
    output.pictureComparison=await evaluate(`(async()=>{const r=document.querySelector('.racing-chase').getBoundingClientRect(),images=[];for(const data of ${JSON.stringify([a.data,b.data])}){const bytes=Uint8Array.from(atob(data),c=>c.charCodeAt(0)),bitmap=await createImageBitmap(new Blob([bytes],{type:'image/png'})),x=bitmap.width/innerWidth,y=bitmap.height/innerHeight,canvas=new OffscreenCanvas(Math.max(1,Math.floor(r.width*x)),Math.max(1,Math.floor(r.height*.5*y))),ctx=canvas.getContext('2d');ctx.drawImage(bitmap,r.x*x,(r.y+r.height*.5)*y,r.width*x,r.height*.5*y,0,0,canvas.width,canvas.height);images.push(ctx.getImageData(0,0,canvas.width,canvas.height).data);bitmap.close()}let sum=0,changed=0;for(let i=0;i<images[0].length;i+=4){let delta=0;for(let c=0;c<3;c++){const value=Math.abs(images[0][i+c]-images[1][i+c]);sum+=value;delta+=value}if(delta>=24)changed++}return {method:'actual compositor PNG lower half of visible cockpit canvas; original meshes also bound by GLB tests',names:${JSON.stringify(options.compareShots)},pixels:images[0].length/4,changedPixels:changed,meanChannelDifference:sum/(images[0].length/4*3)}})()`);
  }
  const trafficStart=measurementWindow?.start??wsObserveStartMS,trafficEnd=measurementWindow?.end??Date.now(),trafficDurationMS=Math.max(1,trafficEnd-trafficStart),traffic=wsFrames.filter(f=>f.at>=trafficStart&&f.at<=trafficEnd);
  const directions=Object.fromEntries(['sent','received'].map(direction=>{const frames=traffic.filter(f=>f.direction===direction),sizes=frames.map(f=>f.payloadBytes),payloadBytes=sizes.reduce((n,v)=>n+v,0),types={};for(const f of frames){const count=types[f.type]??={messages:0,payloadBytes:0};count.messages++;count.payloadBytes+=f.payloadBytes}return[direction,{messages:frames.length,payloadBytes,bytesPerSecond:payloadBytes*1000/trafficDurationMS,messagesPerSecond:frames.length*1000/trafficDurationMS,p50PayloadBytes:percentile(sizes,.5),p95PayloadBytes:percentile(sizes,.95),maximumPayloadBytes:Math.max(0,...sizes),types}]}));
  const stateIntervals=Object.fromEntries(['game','state','combined'].map(type=>{const frames=traffic.filter(f=>f.direction==='received'&&(type==='combined'?['game','state'].includes(f.type):f.type===type)),intervals=frames.slice(1).map((f,i)=>f.at-frames[i].at);return[type,{messages:frames.length,intervals:intervals.length,p50MS:percentile(intervals,.5),p95MS:percentile(intervals,.95),maximumMS:Math.max(0,...intervals)}]}));
  output = { ...output, wsHandshakes,wsPayloadSummary:{method:'CDP WebSocket application payload bytes from original payloadData, UTF-8 text or decoded binary; excludes WebSocket/TCP/TLS headers and compressed wire-size claims',scope:measurementWindow?'exact native performance/pixel observation window after scene ready; setup/bootstrap are outside this window':'entire helper observation interval; may include setup/reconnect/bootstrap and is not automatically a steady-race rate',startAt:trafficStart,endAt:trafficEnd,durationMS:trafficDurationMS,directions,stateIntervals},wsPayloadFrames:traffic,
    screenshots, pixelFrames, packets, states, requests, exceptions, frameEvents, ackMS,touchInputs,guardProbes,safetyDOM,safetyEvents:await evaluate('window.__nfsSafetyProbe?.events??[]'),
    ackMetrics: { count: ackMS.length, p50MS: percentile(ackMS, .5), p95MS: percentile(ackMS, .95),
      statuses: packets.filter((packet) => packet.type === "ack").reduce((counts, packet) => ({ ...counts, [packet.status]: (counts[packet.status] ?? 0) + 1 }), {}) } };
} finally {
  if (session) {
    try {await send('Input.dispatchTouchEvent',{type:'touchCancel',touchPoints:[]});await evaluate('window.__nfsSafetyProbe?.stop?.()');}catch{}
    if(options.observeSkids){try{await evaluate('clearInterval(window.__nfsSkidPixels?.timer)')}catch{}}
    if(options.touchPixels){try{await send('Input.dispatchTouchEvent',{type:'touchCancel',touchPoints:[]});}catch{}}
    for (const value of held.keys()) { try { await key(value, false); } catch {} }
    if (screen) { try { await send("Page.stopScreencast"); } catch {} }
    try { await evaluate("if(!window.__nfsNativeProbe?.early)window.__nfsNativeProbe?.stop?.()"); } catch {}
    try { await call("Target.detachFromTarget", { sessionId: session }); } catch {}
  }
  socket.close();
}
console.log(JSON.stringify(output));
