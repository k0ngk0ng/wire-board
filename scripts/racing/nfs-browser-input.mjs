// Native CDP input and passive compositor/network observations for an owned
// agent-browser target. Never dispatches synthetic keyboard/touch DOM events or
// writes gameplay state. Optional Gamepad API simulation replaces only the
// browser hardware boundary; production mapping, sampling and HTTP stay real.
const [endpoint, base, rawOptions] = process.argv.slice(2);
const options = JSON.parse(rawOptions), socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0, session, screen = false, pixelTrial = null;
const pending = new Map(), packets = [], states = [], requests = [], exceptions = [], frameEvents = [], pixelFrames = [];
const sent = new Map(), ackMS = [], held = new Map();
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
  const id = ++serial, timer = setTimeout(() => { pending.delete(id); reject(Error(`CDP timeout: ${method}`)); }, method==='Runtime.evaluate'&&params.awaitPromise?90000:20000);
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
    try {
      const packet = JSON.parse(p.response.payloadData), at = Date.now();
      if (packet.type === "input") { sent.set(packet.nonce, at); packets.push({ at, direction, ...packet }); }
      else if (packet.type === "ack") { const start = sent.get(packet.nonce); if (start !== undefined) ackMS.push(at - start); packets.push({ at, direction, ...packet }); }
      else if (packet.type === "network") packets.push({ at, direction, ...packet });
      else if (packet.type === "state" || packet.type === "game") {
        const state = typeof packet.state === "string" ? JSON.parse(packet.state) : packet.state;
        const view = packet.type === "game" ? packet.game?.data : state?.room?.game?.data;
        if (view?.cars) states.push({ at, bytes: Buffer.byteLength(JSON.stringify(view)), serverAt: view.serverAt,
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
const sample = () => evaluate(`(async()=>{const state=await(await fetch('/api/state')).json(),c=document.querySelector('.racing-chase'),controls=document.querySelector('.racing-controls');
  return {room:state.room,network:state.network,canvas:c?{...c.dataset,rect:c.getBoundingClientRect().toJSON()}:null,
  board:document.querySelector('.racing-board')?{...document.querySelector('.racing-board').dataset}:null,
  hud:document.querySelector('.racing-hud')?{rect:document.querySelector('.racing-hud').getBoundingClientRect().toJSON(),display:getComputedStyle(document.querySelector('.racing-hud')).display,visibility:getComputedStyle(document.querySelector('.racing-hud')).visibility}:null,
  map:document.querySelector('canvas[aria-label="赛车赛道"]')?{...document.querySelector('canvas[aria-label="赛车赛道"]').dataset}:null,
  audio:await Promise.all([...document.querySelectorAll('audio[data-platform-loop]')].map(async a=>{const source=a.currentSrc;let sourceSHA256=null;if(source.startsWith('data:audio/flac;base64,')){const bytes=Uint8Array.from(atob(source.split(',')[1]),c=>c.charCodeAt(0));sourceSHA256=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(n=>n.toString(16).padStart(2,'0')).join('')};return {key:a.dataset.platformLoop,src:source.slice(0,100),sourceLength:source.length,sourceSHA256,paused:a.paused,readyState:a.readyState,time:a.currentTime,rate:a.playbackRate,volume:a.volume,muted:a.muted,error:a.error?.code??null}})),
  visibility:document.visibilityState,focus:document.hasFocus(),
  pointer:{coarse:matchMedia('(pointer:coarse)').matches,touchPoints:navigator.maxTouchPoints},
  layout:(()=>{const visible=e=>{if(!e)return false;const r=e.getBoundingClientRect(),s=getComputedStyle(e);return r.width>0&&r.height>0&&s.display!=='none'&&s.visibility!=='hidden'},box=e=>e?e.getBoundingClientRect().toJSON():null;
    const stage=document.querySelector('.racing-stage'),board=document.querySelector('.racing-board'),panel=document.querySelector('.racing-controls'),style=panel?getComputedStyle(panel):null;
    return {width:innerWidth,height:innerHeight,scrollY,canvas:box(c),stage:box(stage),board:box(board),coarse:matchMedia('(pointer:coarse)').matches,
      controls:[...document.querySelectorAll('.racing-board button,.racing-wheel,.racing-skins,.racing-skin-picker,.racing-settings')].filter(visible).map(e=>({label:e.getAttribute('aria-label')||e.textContent.trim(),class:e.className,box:box(e),withinStage:!!stage?.contains(e)})),
      touch:[...document.querySelectorAll('.racing-steering button,.racing-wheel,.racing-pedals button')].filter(visible).map(e=>({label:e.getAttribute('aria-label'),box:box(e)})),
      overlays:[...document.querySelectorAll('.racing-hud span,.racing-hud strong,.racing-speedometer,.racing-route-button,.racing-steering button,.racing-wheel,.racing-pedals button')].filter(visible).map(e=>({label:e.getAttribute('aria-label')||e.textContent.trim(),rect:box(e)})),
      panel:panel?{withinStage:!!stage?.contains(panel),background:style.backgroundColor,backgroundImage:style.backgroundImage,position:style.position}:null,
      paragraphs:[...document.querySelectorAll('.racing-board p,.racing-view-controls span,.racing-wheel small')].filter(visible).map(e=>e.textContent.trim())}})(),
  controls:controls?{...controls.dataset}:null,activeClass:document.activeElement?.getAttribute('class'),selected:getSelection().toString(),
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
  if (options.width) await send("Emulation.setDeviceMetricsOverride", { width: options.width, height: options.height ?? 1000, deviceScaleFactor: 1, mobile: !!options.touch });
  await send("Emulation.setTouchEmulationEnabled", { enabled: !!options.touch, maxTouchPoints: 5 });
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
    const rows = await evaluate(`new Promise(resolve=>{const start=performance.now(),rows=[];const take=()=>{const c=document.querySelector('.racing-chase');
      rows.push({at:performance.now(),frame:Number(c?.dataset.frameCounter),renderedAt:Number(c?.dataset.renderedAt),x:Number(c?.dataset.renderX),y:Number(c?.dataset.renderY),snapshotAt:Number(c?.dataset.snapshotAt)});
      if(performance.now()-start<${duration})requestAnimationFrame(take);else resolve(rows)};requestAnimationFrame(take)})`);
    const unique = rows.filter((row, index) => index === 0 || row.frame !== rows[index - 1].frame);
    const intervals = unique.slice(1).map((row, index) => row.at - unique[index].at);
    const elapsed = rows.at(-1).at - rows[0].at;
    output = { before, after: await sample(), rows,
      metrics: { elapsed, rafSamples: rows.length, actualDrawUpdates: unique.length - 1, fps: (unique.length - 1) * 1000 / elapsed,
        p50MS: percentile(intervals, .5), p95MS: percentile(intervals, .95), p99MS: percentile(intervals, .99),
        moved: Math.hypot(rows.at(-1).x - rows[0].x, rows.at(-1).y - rows[0].y) },
      method: "Every display RAF sampled, counted only changed renderer frameCounter; total count and elapsed time retained, not merely intervals of drawn frames." };
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
    const trials = [];
    for (let index = 0; index < (options.trials ?? 8); index++) {
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
    await send("Page.stopScreencast"); screen = false; await key("b", false);
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
    output = { rect, rectSource: options.validationRect ? "supplied-harness-validation" : "renderer-projected", trials: metrics, metrics: { trials: metrics.length, visibleTrials: latencies.length, p50MS: percentile(latencies, .5), p95MS: percentile(latencies, .95), maxMS: Math.max(...latencies) },
      method: "Native steering at rest with B/front camera; actual compositor PNG pixels inside projected own-vehicle bounds; preinput noise calibrated; native page event time and compositor frame timestamps share epoch clock. Dataset/ACK are separate observations." };
  } else throw Error("Unknown probe mode: " + options.mode);
  if(options.screenshot)screenshots.push({name:options.screenshot,data:(await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})).data});
  output = { ...output, screenshots, pixelFrames, packets, states, requests, exceptions, frameEvents, ackMS,touchInputs,guardProbes,safetyDOM,safetyEvents:await evaluate('window.__nfsSafetyProbe?.events??[]'),
    ackMetrics: { count: ackMS.length, p50MS: percentile(ackMS, .5), p95MS: percentile(ackMS, .95),
      statuses: packets.filter((packet) => packet.type === "ack").reduce((counts, packet) => ({ ...counts, [packet.status]: (counts[packet.status] ?? 0) + 1 }), {}) } };
} finally {
  if (session) {
    try {await send('Input.dispatchTouchEvent',{type:'touchCancel',touchPoints:[]});await evaluate('window.__nfsSafetyProbe?.stop?.()');}catch{}
    if(options.touchPixels){try{await send('Input.dispatchTouchEvent',{type:'touchCancel',touchPoints:[]});}catch{}}
    for (const value of held.keys()) { try { await key(value, false); } catch {} }
    if (screen) { try { await send("Page.stopScreencast"); } catch {} }
    try { await evaluate("if(!window.__nfsNativeProbe?.early)window.__nfsNativeProbe?.stop?.()"); } catch {}
    try { await call("Target.detachFromTarget", { sessionId: session }); } catch {}
  }
  socket.close();
}
console.log(JSON.stringify(output));
