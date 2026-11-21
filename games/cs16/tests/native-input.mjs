import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve, sep } from "node:path";
import { WireSelfObserver } from './wire-self-observer.mjs';

const input = JSON.parse(readFileSync(0, "utf8"));
const { endpoint, pageURL, options = {} } = input;
const output = resolve(options.output);
assert.ok(output.startsWith(resolve(".local/cs16") + sep));
mkdirSync(dirname(output), { recursive: true });
const socket = new WebSocket(endpoint);
const pending = new Map();
const report = { passed: false, scope: "trusted Chromium input to rendered WebGL pixel changes; excludes physical display scanout", touchSimulation: !!options.touch, network: { sentBytes: 0, receivedBytes: 0, sentMessages: 0, receivedMessages: 0, acks: [], applications: [], rtts: [], messages: [] } };
const sent = new Map();
const sequences = new Map();
const eventIDs = new Set();
const selfObserver = new WireSelfObserver();
let ordinal = 0;
let session;
let temporaryTarget;
await new Promise((accept, reject) => { socket.onopen = accept; socket.onerror = reject; });
socket.onmessage = event => {
  const message = JSON.parse(event.data);
  const selected = pending.get(message.id);
  if (selected) {
    pending.delete(message.id);
    clearTimeout(selected.timer);
    message.error ? selected.reject(message.error) : selected.accept(message.result);
  }
  if (message.sessionId !== session) return;
  if (message.method === "Network.webSocketFrameError") {
    report.network.socketErrors ??= [];
    report.network.socketErrors.push(message.params);
    return;
  }
  if (!["Network.webSocketFrameSent", "Network.webSocketFrameReceived"].includes(message.method)) return;
  const incoming = message.method.endsWith("Received");
  const frame = message.params.response;
  if (frame.opcode !== 1 && frame.opcode !== 2) {
    report.network.controlFrames = (report.network.controlFrames ?? 0) + 1;
    return;
  }
  const payload = frame.payloadData;
  const bytes = frame.opcode === 2 ? Buffer.from(payload, "base64").length : new TextEncoder().encode(payload).length;
  const at = message.params.timestamp * 1000;
  report.network.firstAt ??= at;
  report.network.lastAt = at;
  const second = String(Math.floor(at / 1000));
  report.network.buckets ??= {};
  report.network.buckets[second] ??= { receivedBytes: 0, sentBytes: 0, gameMessages: 0, shots: 0 };
  report.network.buckets[second][incoming ? "receivedBytes" : "sentBytes"] += bytes;
  report.network[incoming ? "receivedBytes" : "sentBytes"] += bytes;
  report.network[incoming ? "receivedMessages" : "sentMessages"]++;
  let body;
  try { body = JSON.parse(payload); } catch { return; }
  report.network.messageTypes ??= {};
  const classification = (incoming ? "in:" : "out:") + body.type;
  report.network.messageTypes[classification] = (report.network.messageTypes[classification] ?? 0) + 1;
  if (!incoming && body.nonce) sent.set(body.nonce, at);
  if (!incoming && body.action?.seq) sequences.set(`${body.serverInstance}:${body.action.epoch}:${body.action.life}:${body.action.seq}`, { seq: body.action.seq, at });
  if (!incoming && body.action?.type === "input") report.network.lastInput = { at, action: body.action };
  if (!incoming && body.action?.select) {
    report.network.selects ??= [];
    report.network.selects.push({ at, seq: body.action.seq, item: body.action.select, nonce: body.nonce });
  }
  if (incoming && body.type === "ack" && sent.has(body.nonce)) report.network.acks.push({ at, nonce: body.nonce, milliseconds: at - sent.get(body.nonce), status: body.status });
  if (incoming && body.type === "network" && typeof body.self?.rttMs === "number") report.network.rtts.push(body.self.rttMs);
  const data = body.game?.data ?? body.room?.game?.data ?? body.state?.room?.game?.data;
  if (incoming && data) {
    report.network.gameMessageSizes ??= [];
    report.network.gameMessageSizes.push(bytes);
    report.network.buckets[second].gameMessages++;
    report.network.epochs ??= [];
    if (!report.network.epochs.includes(data.epoch)) report.network.epochs.push(data.epoch);
    if (data.map) report.network.mapPackets = (report.network.mapPackets ?? 0) + 1;
    const leaked = [];
    const inspect = (value, location) => {
      if (!value || typeof value !== "object") return;
      for (const [key, child] of Object.entries(value)) {
        if (["history", "seed", "botPath", "botGoal", "pending", "queuedFrame"].includes(key)) leaked.push(location + "." + key);
        inspect(child, location + "." + key);
      }
    };
    inspect(data, "game.data");
    if (leaked.length) report.network.internalFieldLeaks = [...new Set([...(report.network.internalFieldLeaks ?? []), ...leaked])];
    for (const event of data.events ?? []) {
      const id = data.epoch + ":" + event.id;
      if (eventIDs.has(id)) continue;
      eventIDs.add(id);
      report.network.eventCounts ??= {};
      report.network.eventCounts[event.type] = (report.network.eventCounts[event.type] ?? 0) + 1;
      if (event.type === "shot" || event.type === "fire") {
        report.network.buckets[second].shots++;
        report.network.shooters ??= [];
        if (!report.network.shooters.includes(event.seat)) report.network.shooters.push(event.seat);
      }
    }
  }
  const observed = incoming && data ? selfObserver.observe(data) : undefined;
  const player = observed?.player;
  if (incoming && player) report.network.lastAuthority = { at, frame: data.frame, player, partial: observed.partial, epoch: observed.epoch, life: observed.life };
  if (incoming && player && observed.appliedObserved) for (const [key, value] of sequences) {
    if (key === `${body.serverInstance ?? body.state?.serverInstance}:${observed.epoch}:${observed.life}:${value.seq}` && value.seq <= player.appliedSeq) {
      report.network.applications.push({ seq: value.seq, milliseconds: at - value.at, frame: data.frame });
      sequences.delete(key);
    }
  }
  if (report.network.messages.length < 1000) report.network.messages.push({ incoming, at, bytes, type: body.type, frame: data?.frame, receivedSeq: player?.receivedSeq, appliedSeq: player?.appliedSeq, action: body.action });
};
const call = (method, params = {}, sessionId = session) => new Promise((accept, reject) => {
  const id = ++ordinal;
  const timer = setTimeout(() => { pending.delete(id); reject(Error("CDP timeout: " + method)); }, 15000);
  pending.set(id, { accept, reject, timer });
  socket.send(JSON.stringify({ id, method, params, sessionId }));
});
const wait = milliseconds => new Promise(accept => setTimeout(accept, milliseconds));
const evaluate = async expression => {
  const response = await call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
  assert.ok(!response.exceptionDetails, JSON.stringify(response.exceptionDetails));
  return response.result.value;
};
const percentile = (values, percent = 95) => values.length ? [...values].sort((a, b) => a - b)[Math.ceil(values.length * percent / 100) - 1] : null;
const view = () => evaluate("document.querySelector('[data-cs16-canvas]')?.__cs16Frame");
const budget = (condition, message) => {
  if (condition) return;
  report.performanceFailures ??= [];
  report.performanceFailures.push(message);
  if (!options.diagnostic) assert.ok(condition, message);
};
const clickSelector = async selector => {
  const area = await evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});if(!element)throw Error('missing native click target');element.scrollIntoView({block:'center'});return element.getBoundingClientRect().toJSON()})()`);
  const point = { x: area.x + area.width / 2, y: area.y + area.height / 2, button: "left", clickCount: 1 };
  await call("Input.dispatchMouseEvent", { type: "mousePressed", ...point, buttons: 1 });
  await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...point, buttons: 0 });
};
try {
  report.browser = await call("Browser.getVersion", {}, undefined);
  const targets = await call("Target.getTargets", {}, undefined);
  const target = targets.targetInfos.find(item => item.type === "page" && item.url.startsWith(pageURL));
  assert.ok(target, "production page not found");
  session = (await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }, undefined)).sessionId;
  await call("Page.bringToFront");
  await call("Emulation.setTouchEmulationEnabled", { enabled: !!options.touch, maxTouchPoints: options.touch ? 3 : 1 });
  if (options.touch) await call("Emulation.setEmitTouchEventsForMouse", { enabled: true, configuration: "mobile" });
  else await call("Emulation.setEmitTouchEventsForMouse", { enabled: false });
  await evaluate("document.querySelector('[data-cs16-board]').scrollIntoView({block:'center',inline:'nearest'});true");
  report.emulation = await evaluate("({touchPoints:navigator.maxTouchPoints,coarse:matchMedia('(pointer:coarse)').matches,width:innerWidth,height:innerHeight})");
  await call("Network.enable");
  const selfContext = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({seat:state.room.you,epoch:state.room.game.data.epoch,life:state.room.game.data.players.find(player=>player.seat===state.room.you)?.life}))");
  selfObserver.bind(selfContext);
  report.network.observerBootstrap = { ...selfContext, source: 'authenticated HTTP identity only; player fields are observed from raw socket frames' };
  await call("Performance.enable");
  if (options.observeOnly) {
    report.scope = options.denseExpected ? "natural ten-seat combat wire and authoritative clock observation" : "wire observation across transport loss and real page restore";
    const initial = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({epoch:state.room.game.data.epoch,frame:state.room.game.data.frame,finished:state.room.game.data.finished}))");
    if (options.readyFile) {
      const ready = resolve(options.readyFile);
      assert.ok(ready.startsWith(resolve(".local/cs16") + sep));
      writeFileSync(ready, JSON.stringify({ ready: true, initial }) + "\n");
    }
    const began = Date.now();
    const minimum = options.observeMS ?? 10000;
    const maximum = options.denseExpected ? 45000 : minimum;
    while (Date.now() - began < maximum) {
      await wait(250);
      const shots = (report.network.eventCounts?.shot ?? 0) + (report.network.eventCounts?.fire ?? 0);
      if (Date.now() - began >= minimum && (!options.denseExpected || shots >= 10 && (report.network.shooters?.length ?? 0) >= 2)) break;
    }
    const final = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({epoch:state.room.game.data.epoch,frame:state.room.game.data.frame,finished:state.room.game.data.finished}))");
    assert.equal(final.epoch, initial.epoch, "observed restore changed game epoch");
    assert.ok(final.finished || final.frame - initial.frame >= (Date.now() - began) * 70 / 1000, "observation stalled the authoritative rule clock");
    const shots = (report.network.eventCounts?.shot ?? 0) + (report.network.eventCounts?.fire ?? 0);
    if (options.denseExpected) assert.ok(shots >= 10 && (report.network.shooters?.length ?? 0) >= 2, "natural dense observation did not contain real multi-player shooting");
    assert.ok(!report.network.internalFieldLeaks?.length, "internal state leaked into wire: " + JSON.stringify(report.network.internalFieldLeaks));
    report.summary = { initial, final, observedMS: Date.now() - began, shots, shooters: report.network.shooters ?? [], mapPackets: report.network.mapPackets ?? 0 };
    report.functionalPassed = true;
    report.passed = true;
  } else if (options.backgroundOnly) {
    report.scope = "native held input followed by a real background tab, neutral input release and authenticated restore";
    await evaluate(`(() => {
      const observed={events:[],sent:[]};
      const listener=event=>{if(event.type==='blur'&&event.target!==window)return;const now=performance.now();const raw=event.timeStamp>1000000000000?event.timeStamp-performance.timeOrigin:event.timeStamp;observed.events.push({type:event.type,at:Number.isFinite(raw)&&raw>=0&&raw<=now?raw:now,dispatchedAt:now,hidden:document.hidden,focused:document.hasFocus(),trusted:event.isTrusted,windowTarget:event.target===window});};
      window.addEventListener('blur',listener,true);document.addEventListener('visibilitychange',listener,true);
      const original=WebSocket.prototype.send;
      const wrapped=function(data){try{if(typeof data==='string'){const body=JSON.parse(data);if(body.action?.type==='input')observed.sent.push({at:performance.now(),action:body.action});}}catch{}return Reflect.apply(original,this,[data]);};
      WebSocket.prototype.send=wrapped;
      window.__cs16Lifecycle=observed;
      window.__cs16LifecycleCleanup=()=>{window.removeEventListener('blur',listener,true);document.removeEventListener('visibilitychange',listener,true);if(WebSocket.prototype.send===wrapped)WebSocket.prototype.send=original;};
      return true;
    })()`);
    await evaluate("window.__cs16BackgroundCanvas=document.querySelector('[data-cs16-canvas]');document.querySelector('[data-cs16-board]').focus();true");
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyW", key: "w", windowsVirtualKeyCode: 87 });
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyJ", key: "j", windowsVirtualKeyCode: 74 });
    await wait(90);
    const timing = await call("Performance.getMetrics");
    const began = timing.metrics.find(metric => metric.name === "Timestamp").value * 1000;
    const targetRequestAt = Date.now();
    const blank = await call("Target.createTarget", { url: "about:blank", background: false }, undefined);
    const targetCreatedAt = Date.now();
    temporaryTarget = blank.targetId;
    await wait(150);
    const background = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({hidden:document.hidden,focused:document.hasFocus(),lifecycle:window.__cs16Lifecycle,player:state.room.game.data.players[state.room.you],seat:state.room.seats[state.room.you],epoch:state.room.game.data.epoch,frame:state.room.game.data.frame}))");
    const event = background.lifecycle.events.find(item => item.trusted && (item.type === "blur" || item.hidden));
    const release = event && background.lifecycle.sent.find(item => item.at >= event.at && item.action.moveX === 0 && item.action.moveY === 0 && !item.action.fire);
    const wireRelease = release && report.network.messages.find(item => !item.incoming && item.action?.seq === release.action.seq);
    report.backgroundObservation = { background, nativeEvent: event, release, wireRelease, createTargetMS: targetCreatedAt - targetRequestAt, preTargetMonotonicAt: began };
    assert.ok(background.hidden && !background.focused, "new tab did not actually background the game");
    assert.ok(event && release && wireRelease && release.at - event.at <= 50, "native blur/visibility event did not promptly send a neutral input");
    assert.ok(background.player.input.moveX === 0 && background.player.input.moveY === 0 && !background.player.input.fire && !background.seat.autoPlay, "background input remained held or human was taken over");
    await call("Target.closeTarget", { targetId: blank.targetId }, undefined);
    temporaryTarget = undefined;
    await call("Page.bringToFront");
    for (const [code, key, windowsVirtualKeyCode] of [["KeyW", "w", 87], ["KeyJ", "j", 74]]) await call("Input.dispatchKeyEvent", { type: "keyUp", code, key, windowsVirtualKeyCode });
    await evaluate("document.querySelector('[data-cs16-board]').focus();true");
    const foreground = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({hidden:document.hidden,focused:document.hasFocus(),sameCanvas:window.__cs16BackgroundCanvas===document.querySelector('[data-cs16-canvas]'),epoch:state.room.game.data.epoch,frame:state.room.game.data.frame}))");
    assert.ok(!foreground.hidden && foreground.focused && foreground.sameCanvas && foreground.epoch === background.epoch && foreground.frame >= background.frame, "foreground restore changed the canvas, game identity or clock");
    report.summary = { background, foreground, createTargetMS: targetCreatedAt - targetRequestAt, releaseDelayMS: release.at - event.at, nativeEvent: event, release: release.action, wireRelease };
    report.functionalPassed = true;
    report.passed = true;
  } else if (options.scrollOnly) {
    report.scope = "native Chromium touch scroll of the real buy menu, without state injection";
    const before = await evaluate("(()=>{const menu=document.querySelector('.cs16-buy-menu');return {rect:menu.getBoundingClientRect().toJSON(),top:menu.scrollTop,height:menu.clientHeight,total:menu.scrollHeight,windowY:scrollY}})()");
    assert.ok(before.total > before.height, "mobile buy menu did not create a real scroll range");
    for (let index = 0; index < 8; index++) {
      const state = await evaluate("(()=>{const menu=document.querySelector('.cs16-buy-menu');return {top:menu.scrollTop,height:menu.clientHeight,total:menu.scrollHeight}})()");
      if (state.top >= state.total - state.height - 2) break;
      const point = { x: before.rect.x + before.rect.width / 2, y: before.rect.bottom - 25, radiusX: 4, radiusY: 4, force: 1, id: 0 };
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [point] });
      for (let move = 1; move <= 8; move++) {
        await call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ ...point, y: point.y - Math.min(220, before.height - 65) * move / 8 }] });
        await wait(12);
      }
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      await wait(160);
    }
    const after = await evaluate("(()=>{const menu=document.querySelector('.cs16-buy-menu');return {rect:menu.getBoundingClientRect().toJSON(),top:menu.scrollTop,height:menu.clientHeight,total:menu.scrollHeight,windowY:scrollY,last:menu.querySelector('[data-cs16-buy=secammo]').getBoundingClientRect().toJSON()}})()");
    assert.ok(after.top > before.top + 50, "native touch drag did not scroll the buy menu");
    assert.ok(after.last.top >= after.rect.top && after.last.bottom <= after.rect.bottom + 2, "bottom ammunition row cannot be reached");
    assert.ok(Math.abs(after.windowY - before.windowY) <= 2, "buy scroll escaped into the room page");
    report.summary = { before, after, nativeScrolled: true, bottomReachable: true };
    if (options.buyItem) {
      const original = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>state.room.game.data.players[state.room.you])");
      const item = await evaluate(`document.querySelector('[data-cs16-buy=${options.buyItem}]').getBoundingClientRect().toJSON()`);
      const point = { x: item.x + item.width / 2, y: item.y + item.height / 2, radiusX: 4, radiusY: 4, force: 1, id: 0 };
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [point] });
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      await wait(150);
      const purchased = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>state.room.game.data.players[state.room.you])");
      assert.ok(purchased.life === original.life && purchased.reserve > original.reserve && purchased.money < original.money, "native bottom-row ammunition purchase was not applied");
      report.summary.purchase = { item: options.buyItem, beforeReserve: original.reserve, afterReserve: purchased.reserve, beforeMoney: original.money, afterMoney: purchased.money };
    }
    report.functionalPassed = true;
    report.passed = true;
  } else {
  await evaluate(`(() => {
    const canvas = document.querySelector('[data-cs16-canvas]');
    if (!canvas?.__cs16Frame) throw Error('production canvas is not ready');
    const gl = canvas.getContext('webgl2') || canvas.getContext('webgl');
    if (!gl) throw Error('real WebGL context is missing');
    const pixels = new Uint8Array(16 * 16 * 4);
    const hash = () => {
      let result = 2166136261;
      for (const [x, y] of [[canvas.width / 2,canvas.height / 2],[canvas.width / 2,canvas.height / 4],[canvas.width / 4,canvas.height / 3]]) {
        gl.readPixels(Math.max(0,Math.floor(x)-8),Math.max(0,Math.floor(y)-8),16,16,gl.RGBA,gl.UNSIGNED_BYTE,pixels);
        for (let index=0;index<pixels.length;index+=4) result=Math.imul(result^pixels[index]^pixels[index+1]^pixels[index+2],16777619);
      }
      return result>>>0;
    };
    const probe={events:[],frames:[],responses:[],longTasks:[],longTasksSupported:PerformanceObserver.supportedEntryTypes.includes('longtask'),running:true,pending:null,lastHash:null};
    const longTaskObserver=probe.longTasksSupported?new PerformanceObserver(list=>probe.longTasks.push(...list.getEntries().map(entry=>({at:entry.startTime,milliseconds:entry.duration})))):null;
    longTaskObserver?.observe({type:'longtask',buffered:false});
    const types=['keydown','keyup','pointerdown','pointermove','pointerup','pointercancel','touchstart','touchmove','touchend','touchcancel'];
    const listener=event=>{
      if(!event.isTrusted)return;
      const dispatchedAt=performance.now();
      const sourceAt=event.timeStamp>1000000000000?event.timeStamp-performance.timeOrigin:event.timeStamp;
      const at=Number.isFinite(sourceAt)&&sourceAt>=0&&sourceAt<=dispatchedAt?sourceAt:dispatchedAt;
      const entry={type:event.type,at,dispatchedAt,queueMS:dispatchedAt-at,trusted:event.isTrusted,key:event.code??null,pointerId:event.pointerId??null,target:event.target?.dataset?.cs16Action??event.target?.dataset?.cs16Stick??null};
      probe.events.push(entry);
      const item=probe.pending;
      if(item&&item.inputAt===null&&((item.kind==='key'&&event.type==='keydown')||(item.kind==='mouse'&&event.type==='pointermove')||(item.kind==='touch'&&(event.type==='pointermove'||event.type==='touchmove'))))item.inputAt=entry.at;
    };
    for(const type of types)document.addEventListener(type,listener,true);
    probe.arm=(kind,code)=>{probe.pending={kind,code,inputAt:null,before:structuredClone(canvas.__cs16Frame),hash:probe.lastHash,armedAt:performance.now()};};
    const loop=at=>{
      if(!probe.running)return;
      const frame=canvas.__cs16Frame;
      const pixelReadbackStartAt=performance.now();
      const currentHash=hash();
      const pixelReadbackEndAt=performance.now();
      if(probe.frames.length<2500)probe.frames.push({...frame,at,pixelHash:currentHash,pixelReadbackStartAt,pixelReadbackEndAt});
      const item=probe.pending;
      if(item?.kind==='key'&&item.inputAt===null){
        const entry=canvas.__cs16Metrics?.inputs?.find(entry=>entry.type==='keydown'&&entry.trusted&&entry.at>=item.armedAt);
        if(entry){item.inputAt=entry.createdAt??entry.at;probe.events.push({...entry,queueMS:(entry.dispatchedAt??entry.at)-(entry.createdAt??entry.at),key:item.code,source:'native-window-capture'});}
      }
      if(item?.kind==='mouse'&&item.inputAt===null){
        const entry=canvas.__cs16Metrics?.inputs?.find(entry=>entry.type==='mousemove'&&entry.trusted&&entry.at>=item.armedAt);
        if(entry){item.inputAt=entry.createdAt??entry.at;probe.events.push({...entry,queueMS:(entry.dispatchedAt??entry.at)-(entry.createdAt??entry.at),source:'native-locked-mouse'});}
      }
      if(item&&item.inputAt!==null&&frame&&frame.inputEnabled&&item.before.inputEnabled&&frame.life===item.before.life){
        const before=item.before;
        const changed=item.kind==='mouse'||item.code?.startsWith('Arrow')?frame.yaw!==before.yaw||frame.pitch!==before.pitch:frame.x!==before.x||frame.y!==before.y||frame.z!==before.z||item.kind==='touch'&&(frame.yaw!==before.yaw||frame.pitch!==before.pitch);
        if(changed&&item.hash!==null&&currentHash!==item.hash){const visibleAt=performance.now();probe.responses.push({kind:item.kind,inputAt:item.inputAt,visibleAt,milliseconds:visibleAt-item.inputAt,pixelReadbackStartAt,pixelReadbackEndAt,pixelsChanged:true,beforeHash:item.hash,afterHash:currentHash,before,after:structuredClone(frame)});probe.pending=null;}
      }
      probe.lastHash=currentHash;
      requestAnimationFrame(loop);
    };
    probe.stop=()=>{probe.running=false;longTaskObserver?.disconnect();for(const type of types)document.removeEventListener(type,listener,true);};
    window.__cs16NativeProbe=probe;
    requestAnimationFrame(loop);
    return true;
  })()`);
  await wait(50);
  const sample = async (kind, action, code) => {
    await evaluate(`window.__cs16NativeProbe.arm(${JSON.stringify(kind)},${JSON.stringify(code)});true`);
    await action();
    await wait(90);
  };
  if (options.touch) {
    const areas = await evaluate("[document.querySelector('[data-cs16-stick=move]'),document.querySelector('[data-cs16-action=fire]')].map(element=>element.getBoundingClientRect().toJSON())");
    report.touchHits = await evaluate("[document.querySelector('[data-cs16-stick=move]'),document.querySelector('[data-cs16-action=fire]')].map(element=>{const rect=element.getBoundingClientRect();const hit=document.elementFromPoint(rect.x+rect.width/2,rect.y+rect.height/2);return {target:element.dataset.cs16Action??element.dataset.cs16Stick,hit:hit?.tagName,owned:!!hit&&(element===hit||element.contains(hit)),rect:rect.toJSON()}})");
    assert.ok(report.touchHits.every(item => item.owned), "a primary touch target is covered by another control");
    assert.ok(areas[1].width >= 64 && areas[1].height >= 64, "primary fire target is smaller than 64px");
    const actionAreas = await evaluate("[...document.querySelectorAll('[data-cs16-action]')].map(element=>({id:element.dataset.cs16Action,rect:element.getBoundingClientRect().toJSON()}))");
    assert.ok(actionAreas.every(item => item.rect.width >= 44 && item.rect.height >= 44), "a critical touch action is smaller than 44px");
    const centers = areas.map((rect, id) => ({ x: rect.x + rect.width / 2, y: rect.y + rect.height / 2, radiusX: 4, radiusY: 4, force: 1, id }));
    const beforeTouch = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>state.room.game.data.players[state.room.you])");
    const lastAction = () => report.network.lastInput?.action;
    report.singleFingerReleases = [];
    for (let index = 0; index < (options.samples ?? 24); index++) {
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: centers });
      const moved = centers.map((point, id) => ({ ...point, x: point.x + (id ? 20 : index % 2 ? 24 : -24) }));
      await sample("touch", () => call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: moved }));
      const keep = index % 2 ? 0 : 1;
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [moved[1 - keep]] });
      await wait(60);
      const action = lastAction();
      if (index < 2) {
        assert.ok(action && (keep ? action.moveX === 0 && action.moveY === 0 && action.fire : (action.moveX !== 0 || action.moveY !== 0) && !action.fire), "one finger release cancelled or retained the wrong input: " + JSON.stringify(action));
        report.singleFingerReleases.push({ remaining: keep ? "fire" : "move", action });
      }
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      await wait(30);
    }
    const combined = report.network.messages.filter(item => !item.incoming && item.action?.fire && (item.action.moveX !== 0 || item.action.moveY !== 0)).map(item => item.action);
    const afterTouch = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>state.room.game.data.players[state.room.you])");
    report.twoThumbs = { combined, before: beforeTouch, after: afterTouch, lastInput: lastAction() };
    assert.ok(combined.length > 0 && new Set(combined.map(action => action.yaw)).size > 1, "two thumbs did not simultaneously move, aim and fire");
    assert.ok(beforeTouch.life === afterTouch.life && afterTouch.ammo < beforeTouch.ammo, "two-thumb fire was not applied by authority");
    assert.ok(!lastAction().fire && lastAction().moveX === 0 && lastAction().moveY === 0, "touch release retained an input");
    const crouchBox = await evaluate("document.querySelector('[data-cs16-action=crouch]').getBoundingClientRect().toJSON()");
    const crouchPoint = { x: crouchBox.x + crouchBox.width / 2, y: crouchBox.y + crouchBox.height / 2, radiusX: 4, radiusY: 4, force: 1, id: 0 };
    report.crouchToggle = [];
    for (const expected of [true, false]) {
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [crouchPoint] });
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      await wait(150);
      const player = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>state.room.game.data.players[state.room.you])");
      assert.equal(player.crouch, expected, "mobile crouch toggle did not persist its intended state");
      report.crouchToggle.push({ expected, actual: player.crouch });
    }
    report.touchRelease = await view();
    await evaluate(`(() => {
      const probe=window.__cs16NativeProbe;
      probe.guard={menus:[],selections:[]};
      const context=event=>queueMicrotask(()=>probe.guard.menus.push({trusted:event.isTrusted,prevented:event.defaultPrevented,target:event.target.tagName}));
      const selection=()=>{const text=getSelection()?.toString()??'';if(text)probe.guard.selections.push(text);};
      document.addEventListener('contextmenu',context,true);
      document.addEventListener('selectionchange',selection);
      const stop=probe.stop;
      probe.stop=()=>{stop();document.removeEventListener('contextmenu',context,true);document.removeEventListener('selectionchange',selection);};
      return true;
    })()`);
    const label = await evaluate("document.querySelector('[data-cs16-action=reload]').getBoundingClientRect().toJSON()");
    const held = { x: label.x + label.width / 2, y: label.y + label.height / 2, radiusX: 4, radiusY: 4, force: 1, id: 0 };
    await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [held] });
    await wait(720);
    await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    const canvasRect = await evaluate("document.querySelector('[data-cs16-canvas]').getBoundingClientRect().toJSON()");
    const right = { x: canvasRect.x + canvasRect.width / 2, y: canvasRect.y + canvasRect.height / 3, button: "right", clickCount: 1 };
    await call("Input.dispatchMouseEvent", { type: "mousePressed", ...right, buttons: 2 });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...right, buttons: 0 });
    await wait(60);
    report.nativeControlGuard = await evaluate("({...window.__cs16NativeProbe.guard,selection:getSelection()?.toString()??''})");
    assert.ok(!report.nativeControlGuard.selection && report.nativeControlGuard.selections.length === 0, "native long press selected game text");
    assert.ok(report.nativeControlGuard.menus.some(event => event.trusted && event.prevented), "native game context menu was not prevented");
  } else {
    await evaluate("document.querySelector('[data-cs16-board]').focus();true");
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "Digit2", key: "2", windowsVirtualKeyCode: 50 });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "Digit2", key: "2", windowsVirtualKeyCode: 50 });
    await wait(400);
    const keys = [{ code: "KeyW", key: "w", windowsVirtualKeyCode: 87 }, { code: "KeyS", key: "s", windowsVirtualKeyCode: 83 }, { code: "KeyA", key: "a", windowsVirtualKeyCode: 65 }, { code: "KeyD", key: "d", windowsVirtualKeyCode: 68 }, { code: "ArrowLeft", key: "ArrowLeft", windowsVirtualKeyCode: 37 }, { code: "ArrowRight", key: "ArrowRight", windowsVirtualKeyCode: 39 }];
    for (let index = 0; index < (options.samples ?? 12); index++) {
      const key = keys[index % keys.length];
      if (!key.code.startsWith("Arrow")) {
        let settled = false;
        for (let attempt = 0; attempt < 60; attempt++) {
          settled = await evaluate("(()=>{const frames=window.__cs16NativeProbe.frames.slice(-3);return frames.length===3&&frames.every(frame=>frame.x===frames[0].x&&frame.y===frames[0].y&&frame.z===frames[0].z&&frame.life===frames[0].life)})()");
          if (settled) break;
          await wait(25);
        }
        assert.ok(settled, "ongoing movement prevented an isolated native key response measurement");
      }
      await sample("key", () => call("Input.dispatchKeyEvent", { type: "keyDown", ...key }), key.code);
      await call("Input.dispatchKeyEvent", { type: "keyUp", ...key });
      await wait(35);
    }
    const rect = await evaluate("document.querySelector('[data-cs16-canvas]').getBoundingClientRect().toJSON()");
    const center = { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 };
    await call("Input.dispatchMouseEvent", { type: "mousePressed", ...center, button: "left", buttons: 1, clickCount: 1 });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...center, button: "left", buttons: 0, clickCount: 1 });
    for (let index = 0; index < 8; index++) await sample("mouse", () => call("Input.dispatchMouseEvent", { type: "mouseMoved", x: center.x + (index % 2 ? 24 : -24), y: center.y + (index % 3 ? 12 : -12), button: "none" }));
    report.pointerLocked = await evaluate("!!document.pointerLockElement");
    const before = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({you:state.room.you,data:state.room.game.data}))");
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyJ", key: "j", windowsVirtualKeyCode: 74 });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "KeyJ", key: "j", windowsVirtualKeyCode: 74 });
    await wait(250);
    const after = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>state.room.game.data)");
    const ownBefore = before.data.players[before.you];
    const ownAfter = after.players[before.you];
    report.quickTap = { before: ownBefore, after: ownAfter, interrupted: !ownBefore.alive || !ownAfter.alive || ownBefore.life !== ownAfter.life, events: after.events.filter(event => event.seat === before.you && event.id > before.data.eventId && ['fire', 'shot'].includes(event.type)) };
    if (!report.quickTap.interrupted && ownBefore.ammo > 0 && !['knife', 'c4', 'hegrenade', 'flashbang', 'smokegrenade'].includes(ownBefore.weapon)) assert.ok(ownAfter.ammo < ownBefore.ammo, "native short fire tap was not applied");
    const press = async (code, key, windowsVirtualKeyCode) => {
      await call("Input.dispatchKeyEvent", { type: "keyDown", code, key, windowsVirtualKeyCode });
      await call("Input.dispatchKeyEvent", { type: "keyUp", code, key, windowsVirtualKeyCode });
    };
    const displayedWeapon = async predicate => {
      for (let attempt = 0; attempt < 50; attempt++) {
        const state = await view();
        if (state && predicate(state.weapon)) return state;
        await wait(20);
      }
      throw Error("native weapon selection did not reach the local visible view");
    };
    const authoritativeWeapon = async (weapon, sequence = 0) => {
      for (let attempt = 0; attempt < 100; attempt++) {
        const state = report.network.lastAuthority;
        if (state?.player.weapon === weapon && state.player.appliedSeq >= sequence) return state;
        await wait(20);
      }
      throw Error("native weapon selection did not reach real server projection");
    };
    const beforeSelect = report.network.selects?.length ?? 0;
    await press("Digit3", "3", 51);
    const knife = await displayedWeapon(weapon => weapon === "knife");
    await press("KeyQ", "q", 81);
    const previous = await displayedWeapon(weapon => weapon === ownAfter.weapon);
    const rapid = (report.network.selects ?? []).slice(beforeSelect);
    assert.deepEqual(rapid.slice(0, 2).map(item => item.item), ["knife", ownAfter.weapon], "rapid Digit3 and Q did not send both ordered selections");
    const confirmed = await authoritativeWeapon(ownAfter.weapon, rapid[1].seq);
    const firstAck = report.network.acks.find(ack => ack.nonce === rapid[0].nonce);
    const secondAck = report.network.acks.find(ack => ack.nonce === rapid[1].nonce);
    assert.ok(firstAck?.status === 200 && secondAck?.status === 200 && firstAck.at <= secondAck.at, "both rapid selections were not accepted in order");
    assert.ok(rapid[1].at < firstAck.at, "rapid Q verification waited for the first selection ACK");
    await call("Input.dispatchMouseEvent", { type: "mouseWheel", ...center, deltaX: 0, deltaY: 120 });
    const wheel = await displayedWeapon(weapon => weapon !== previous.weapon);
    const wheelAuthority = await authoritativeWeapon(wheel.weapon);
    await press("Digit2", "2", 50);
    const sidearm = ownAfter.inventory.find(item => ["glock18", "usp", "p228", "deagle", "elite", "fiveseven"].includes(item.id));
    assert.ok(sidearm, "native slot two requires a legitimately owned sidearm");
    const pistol = await displayedWeapon(weapon => weapon === sidearm.id);
    const pistolAuthority = await authoritativeWeapon(pistol.weapon);
    report.legacyWeaponControls = { digit3: knife, quickSwap: previous, rapid, firstAck, secondAck, confirmed, wheel, wheelAuthority, digit2: pistol, pistolAuthority };
    const authorityWhere = async predicate => {
      for (let attempt = 0; attempt < 150; attempt++) {
        const state = report.network.lastAuthority;
        if (state && predicate(state)) return state;
        await wait(20);
      }
      throw Error("native modifier combination did not reach server projection");
    };
    const reloadSetup = await evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({source:'authenticated HTTP precondition only',player:state.room.game.data.players.find(player=>player.seat===state.room.you)}))");
    report.reloadSetup = { source: reloadSetup.source, ammo: reloadSetup.player.ammo, reserve: reloadSetup.player.reserve, wirePartial: pistolAuthority.partial };
    if (reloadSetup.player.reserve === 0) {
      assert.ok(options.allowAmmoPurchase !== false, "ground-map reload requires legitimately acquired reserve; spawn purchase is forbidden");
      await press("Period", ".", 190);
      await authorityWhere(state => state.player.reserve > 0);
    }
    const modifiers = await evaluate("({platform:navigator.platform,keyboardLockAvailable:typeof navigator.keyboard?.lock==='function',fullscreen:!!document.fullscreenElement,pointerLock:!!document.pointerLockElement})");
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "ControlLeft", key: "Control", windowsVirtualKeyCode: 17, modifiers: 2 });
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyW", key: "w", windowsVirtualKeyCode: 87, modifiers: 2 });
    const walking = await authorityWhere(state => state.player.input?.crouch && state.player.input?.moveY > 0);
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "KeyW", key: "w", windowsVirtualKeyCode: 87, modifiers: 2 });
    const reloadReady = await authorityWhere(state => state.player.nextAttack <= state.frame);
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyR", key: "r", windowsVirtualKeyCode: 82, modifiers: 2 });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "KeyR", key: "r", windowsVirtualKeyCode: 82, modifiers: 2 });
    const reloading = await authorityWhere(state => state.player.crouch && state.player.inventory?.some(item => item.id === state.player.weapon && item.reloadUntil > state.frame));
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "ControlLeft", key: "Control", windowsVirtualKeyCode: 17, modifiers: 0 });
    const released = await authorityWhere(state => !state.player.input?.crouch && state.player.input?.moveY === 0);
    await authorityWhere(state => !state.player.inventory?.some(item => item.id === state.player.weapon && item.reloadUntil > state.frame));
    report.controlCombos = { support: modifiers, boundary: "trusted DOM combinations on this detected browser/platform; system accelerators on other operating systems are not certified", crouchWalk: walking, reloadReady, crouchReload: reloading, released };
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "Escape", key: "Escape", windowsVirtualKeyCode: 27 });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "Escape", key: "Escape", windowsVirtualKeyCode: 27 });
    report.escapeReleased = await evaluate("!document.pointerLockElement");
    if (!report.escapeReleased) {
      report.driverPointerCleanup = true;
      await evaluate("document.exitPointerLock();true");
    }
    await wait(50);
    await clickSelector('.chat-toggle');
    await clickSelector('#table-chat-panel textarea');
    await evaluate("(()=>{window.__cs16EditorKeys=[];const handler=event=>{if(event.target.matches?.('#table-chat-panel textarea'))queueMicrotask(()=>window.__cs16EditorKeys.push({code:event.code,ctrl:event.ctrlKey,meta:event.metaKey,trusted:event.isTrusted,prevented:event.defaultPrevented}));};document.addEventListener('keydown',handler,true);window.__cs16EditorCleanup=()=>document.removeEventListener('keydown',handler,true);return true})()");
    await call("Input.insertText", { text: "cs16 edit shortcut" });
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "ControlLeft", key: "Control", windowsVirtualKeyCode: 17, modifiers: 2 });
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyA", key: "a", windowsVirtualKeyCode: 65, modifiers: 2 });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "KeyA", key: "a", windowsVirtualKeyCode: 65, modifiers: 2 });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "ControlLeft", key: "Control", windowsVirtualKeyCode: 17, modifiers: 0 });
    const mac = /Mac/.test(modifiers.platform);
    const modifier = { code: mac ? "MetaLeft" : "ControlLeft", key: mac ? "Meta" : "Control", windowsVirtualKeyCode: mac ? 91 : 17, modifiers: mac ? 4 : 2 };
    await call("Input.dispatchKeyEvent", { type: "keyDown", ...modifier });
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyA", key: "a", windowsVirtualKeyCode: 65, modifiers: modifier.modifiers, commands: ["selectAll"] });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "KeyA", key: "a", windowsVirtualKeyCode: 65, modifiers: modifier.modifiers });
    await call("Input.dispatchKeyEvent", { type: "keyUp", ...modifier, modifiers: 0 });
    report.editorShortcuts = await evaluate("(()=>{const editor=document.querySelector('#table-chat-panel textarea');return {keys:window.__cs16EditorKeys,selected:editor.selectionEnd-editor.selectionStart,length:editor.value.length}})()");
    report.editorShortcuts.driverCommand = "selectAll";
    report.editorShortcuts.driverBoundary = "plain textarea on this browser also requires the native CDP edit command for platform select-all";
    assert.ok(report.editorShortcuts.keys.some(item => item.code === "KeyA" && item.ctrl && item.trusted && !item.prevented), "editor Ctrl shortcut was swallowed by game controls");
    assert.ok(report.editorShortcuts.keys.filter(item => item.code === "KeyA").every(item => item.trusted && !item.prevented), "platform editor key was prevented by game controls");
    assert.equal(report.editorShortcuts.selected, report.editorShortcuts.length, "native platform select-all no longer works in chat editor");
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "Backspace", key: "Backspace", windowsVirtualKeyCode: 8 });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "Backspace", key: "Backspace", windowsVirtualKeyCode: 8 });
    await evaluate("window.__cs16EditorCleanup();true");
    await clickSelector('.chat-toggle');
  }
  await wait(options.observeMS ?? 1200);
  const observation = await evaluate("(()=>{const probe=window.__cs16NativeProbe;probe.stop();return {events:probe.events,responses:probe.responses,frames:probe.frames,longTasks:probe.longTasks,longTasksSupported:probe.longTasksSupported,final:document.querySelector('[data-cs16-canvas]').__cs16Frame,metrics:document.querySelector('[data-cs16-canvas]').__cs16Metrics}})()");
  Object.assign(report, observation);
  const frames = observation.frames;
  const intervals = frames.slice(1).map((entry, index) => entry.at - frames[index].at);
  const elapsed = frames.at(-1).at - frames[0].at;
  const changes = frames.slice(1).filter((entry, index) => entry.drawCount !== frames[index].drawCount).length;
  const visibleSteps = frames.slice(1).flatMap((entry, index) => {
    const previous = frames[index];
    return entry.inputEnabled && previous.inputEnabled && entry.life === previous.life && entry.at - previous.at <= 34 && [entry.x, entry.y, previous.x, previous.y].every(Number.isFinite) ? [Math.hypot(entry.x - previous.x, entry.y - previous.y) / 256] : [];
  });
  report.summary = { responses: observation.responses.length, keyboardResponses: observation.responses.filter(item => item.kind === "key").length, mouseResponses: observation.responses.filter(item => item.kind === "mouse").length, nativeDispatchQueueP95MS: percentile(observation.events.filter(item => typeof item.queueMS === "number").map(item => item.queueMS)), keyboardP95MS: percentile(observation.responses.filter(item => item.kind === "key").map(item => item.milliseconds)), mouseP95MS: percentile(observation.responses.filter(item => item.kind === "mouse").map(item => item.milliseconds)), responseP95MS: percentile(observation.responses.map(item => item.milliseconds)), frameP95MS: percentile(intervals), frameP99MS: percentile(intervals, 99), visibleFPS: changes * 1000 / elapsed, visibleUpdates: changes, elapsed, rttP95MS: percentile(report.network.rtts), ackP95MS: percentile(report.network.acks.map(item => item.milliseconds)), appliedP95MS: percentile(report.network.applications.map(item => item.milliseconds)), sentBytesPerSecond: report.network.sentBytes * 1000 / elapsed, receivedBytesPerSecond: report.network.receivedBytes * 1000 / elapsed };
  assert.ok(report.summary.responses >= (options.minimumResponses ?? (options.touch ? 20 : 12)), "too few real pixel responses: " + JSON.stringify(report.summary));
  report.summary.maxVisibleStepHL = Math.max(0, ...visibleSteps);
  report.summary.maxPredictionLeadFrames = Math.max(0, ...frames.map(entry => entry.predictedFrame - entry.serverFrame).filter(Number.isFinite));
  report.summary.maxPendingInputs = Math.max(0, ...frames.map(entry => entry.pending).filter(Number.isFinite));
  assert.ok(report.summary.maxPredictionLeadFrames <= 15 && report.summary.maxPendingInputs <= 64, "prediction or pending input bounds were exceeded");
  budget(report.summary.responseP95MS <= (options.touch ? 50 : 33), "input-to-pixel budget exceeded: " + JSON.stringify(report.summary));
  if (!options.touch) {
    assert.ok(report.pointerLocked, "native canvas click did not acquire relative mouse control");
    assert.ok(report.summary.keyboardResponses >= 6 && report.summary.mouseResponses >= 6, "keyboard or relative mouse responses are missing: " + JSON.stringify(report.summary));
    budget(report.summary.keyboardP95MS <= 33 && report.summary.mouseP95MS <= 33, "keyboard or mouse latency budget exceeded: " + JSON.stringify(report.summary));
  }
  budget(report.summary.frameP95MS <= 20, "display interval budget exceeded: " + JSON.stringify(report.summary));
  budget(report.summary.frameP99MS <= 33, "display tail interval budget exceeded: " + JSON.stringify(report.summary));
  budget(report.summary.visibleFPS >= 55, "visible update budget exceeded: " + JSON.stringify(report.summary));
  assert.ok(report.network.applications.length > 0, "no authoritative input application observed");
  report.functionalPassed = true;
  report.passed = !report.performanceFailures?.length;
  const photograph = async name => {
    const clip = await evaluate("({x:0,y:0,width:innerWidth,height:innerHeight,scale:1})");
    const picture = await call("Page.captureScreenshot", { format: "png", clip, captureBeyondViewport: true });
    const pixels = Buffer.from(picture.data, "base64");
    assert.equal(pixels.readUInt32BE(16), clip.width, "screenshot width does not match the observed viewport");
    assert.equal(pixels.readUInt32BE(20), clip.height, "screenshot height does not match the observed viewport");
    const file = output.replace(/\.json$/, "") + "-" + name + ".png";
    writeFileSync(file, pixels);
    report.screenshots ??= [];
    report.screenshots.push(file);
  };
  const layout = async name => {
    const audit = await evaluate(`(() => {
      const rect=element=>element.getBoundingClientRect().toJSON();
      const visible=element=>{const box=element.getBoundingClientRect();if(!box.width||!box.height||element.closest('[inert]')||document.fullscreenElement&&!document.fullscreenElement.contains(element))return false;const hit=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);return !!hit&&(hit===element||element.contains(hit));};
      const controls=[...document.querySelectorAll('[data-cs16-action],[data-fullscreen-toggle],.chat-toggle:not([aria-hidden=true])')].filter(visible).map(element=>({kind:element.dataset.cs16Action??(element.matches('.chat-toggle')?'platform-chat':'platform-fullscreen'),rect:rect(element)}));
      const hud=[...document.querySelectorAll('.cs16-vitals,.cs16-ammo,.cs16-match-hud,.cs16-inventory')].map(element=>({kind:element.className,text:element.textContent,rect:rect(element)}));
      const overlap=(a,b)=>a.left<b.right&&a.right>b.left&&a.top<b.bottom&&a.bottom>b.top;
      return {coarse:matchMedia('(pointer:coarse)').matches,width:innerWidth,height:innerHeight,canvas:rect(document.querySelector('[data-cs16-canvas]')),controls,hud,overlaps:hud.flatMap(item=>controls.filter(control=>overlap(item.rect,control.rect)).map(control=>({hud:item.kind,control:control.kind}))),assets:document.querySelector('[data-cs16-canvas]').__cs16Assets};
    })()`);
    report.nativeLayouts ??= [];
    report.nativeLayouts.push({ name, ...audit });
    if (options.touch) {
      assert.equal(audit.width, name === "landscape-controls" ? 760 : 320, "touch layout has the wrong viewport width");
      assert.equal(audit.height, name === "landscape-controls" ? 360 : 760, "touch layout has the wrong viewport height");
    }
    await photograph(name);
    if (audit.overlaps.length) {
      report.layoutFailures ??= [];
      report.layoutFailures.push({ name, overlaps: audit.overlaps });
      if (!options.diagnostic) throw Error("real touch layout overlaps critical HUD: " + JSON.stringify(audit.overlaps));
    }
  };
  const nativeClick = async selector => {
    const area = await evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});if(!element)throw Error('missing native click target');element.scrollIntoView({block:'center'});return element.getBoundingClientRect().toJSON()})()`);
    const point = { x: area.x + area.width / 2, y: area.y + area.height / 2, button: "left", clickCount: 1 };
    await call("Input.dispatchMouseEvent", { type: "mousePressed", ...point, buttons: 1 });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...point, buttons: 0 });
  };
  const waitMode = async expected => {
    for (let attempt = 0; attempt < 100; attempt++) {
      const current = await evaluate("document.querySelector('[data-room-fullscreen]')?.dataset.roomFullscreen??''");
      if (expected === null ? current !== "" : current === expected) return current;
      await wait(20);
    }
    throw Error("native fullscreen control did not reach requested mode");
  };
  const settle = async () => {
    const before = (await view()).drawCount;
    for (let attempt = 0; attempt < 100; attempt++) {
      if ((await view()).drawCount >= before + 3) return;
      await wait(20);
    }
    throw Error("rendering did not settle after visual layout change");
  };
  await wait(1100);
  await photograph(options.touch ? "portrait" : "desktop-idle");
  if (options.touch) {
    await evaluate("window.__cs16TouchCanvas=document.querySelector('[data-cs16-canvas]');true");
    await layout("portrait-controls");
    await nativeClick('button[aria-label="全屏"]');
    const nativeMode = await waitMode(null);
    await call("Emulation.setDeviceMetricsOverride", { width: 320, height: 760, deviceScaleFactor: 1, mobile: true });
    await settle();
    await layout("fullscreen-" + nativeMode);
    await nativeClick('button[aria-label="退出全屏"]');
    await waitMode("");
    await evaluate("(()=>{const surface=document.querySelector('[data-game-fullscreen-surface]');window.__cs16TouchFullscreen=Object.getOwnPropertyDescriptor(surface,'requestFullscreen');Object.defineProperty(surface,'requestFullscreen',{configurable:true,value:()=>Promise.reject(Error('acceptance fallback'))});return true})()");
    await nativeClick('button[aria-label="全屏"]');
    await waitMode("page");
    await call("Emulation.setDeviceMetricsOverride", { width: 320, height: 760, deviceScaleFactor: 1, mobile: true });
    await settle();
    await layout("fullscreen-page");
    await call("Emulation.setDeviceMetricsOverride", { width: 760, height: 360, deviceScaleFactor: 1, mobile: true });
    await settle();
    await layout("landscape-controls");
    await call("Emulation.setDeviceMetricsOverride", { width: 320, height: 760, deviceScaleFactor: 1, mobile: true });
    await settle();
    await nativeClick('button[aria-label="退出全屏"]');
    await waitMode("");
    await evaluate("(()=>{const surface=document.querySelector('[data-game-fullscreen-surface]');if(window.__cs16TouchFullscreen)Object.defineProperty(surface,'requestFullscreen',window.__cs16TouchFullscreen);else delete surface.requestFullscreen;return true})()");
    assert.ok(await evaluate("window.__cs16TouchCanvas===document.querySelector('[data-cs16-canvas]')"), "touch orientation or fullscreen remounted the canvas");
    report.passed = report.passed && !report.layoutFailures?.length;
  } else if (options.visualAudit) {
    const original = await evaluate("({width:innerWidth,height:innerHeight,ratio:devicePixelRatio})");
    await call("Emulation.setDeviceMetricsOverride", { width: 1024, height: 768, deviceScaleFactor: 1, mobile: false });
    await nativeClick('button[aria-label="全屏"]');
    await waitMode(null);
    await settle();
    await layout("desktop-4x3-fullscreen");
    await nativeClick('button[aria-label="退出全屏"]');
    await waitMode("");
    await call("Emulation.setDeviceMetricsOverride", { width: original.width, height: original.height, deviceScaleFactor: original.ratio, mobile: false });
    await settle();
    report.passed = report.passed && !report.layoutFailures?.length;
  }
  }
} catch (error) {
  report.passed = false;
  report.functionalPassed = false;
  report.error = String(error.stack ?? error);
  process.exitCode = 1;
} finally {
  if (session && options.touch && !options.observeOnly) await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] }).catch(() => {});
  if (session && options.backgroundOnly) await evaluate("window.__cs16LifecycleCleanup?.();true").catch(() => {});
  if (temporaryTarget) await call("Target.closeTarget", { targetId: temporaryTarget }, undefined).catch(() => {});
  if (session) await evaluate("if(document.pointerLockElement)document.exitPointerLock();true").catch(() => {});
  if (session && !report.frames && !options.scrollOnly && !options.backgroundOnly && !options.observeOnly) {
    const partial = await evaluate("(()=>{const probe=window.__cs16NativeProbe;if(!probe)return {};probe.stop();return {events:probe.events,responses:probe.responses,frames:probe.frames,longTasks:probe.longTasks,longTasksSupported:probe.longTasksSupported,final:document.querySelector('[data-cs16-canvas]')?.__cs16Frame}})()").catch(() => ({}));
    Object.assign(report, partial);
  }
  if (session) await call("Target.detachFromTarget", { sessionId: session }, undefined).catch(() => {});
  const wireMS = (report.network.lastAt ?? 0) - (report.network.firstAt ?? 0);
  const wireSizes = report.network.gameMessageSizes ?? [];
  report.wire = { durationMS: wireMS, receivedBytesPerSecond: wireMS > 0 ? report.network.receivedBytes * 1000 / wireMS : 0, sentBytesPerSecond: wireMS > 0 ? report.network.sentBytes * 1000 / wireMS : 0, gameMessagesPerSecond: wireMS > 0 ? wireSizes.length * 1000 / wireMS : 0, gameMessageBytesP95: percentile(wireSizes), maxGameMessageBytes: Math.max(0, ...wireSizes), peakOneSecondReceivedBytes: Math.max(0, ...Object.values(report.network.buckets ?? {}).map(bucket => bucket.receivedBytes)), eventCounts: report.network.eventCounts ?? {}, applicationPayloadOnly: true, internalFieldLeaks: report.network.internalFieldLeaks ?? [] };
  if (report.summary) report.summary.wire = report.wire;
  if (report.wire.internalFieldLeaks.length) {
    report.passed = false;
    report.functionalPassed = false;
    report.error = "internal scheduling or state leaked into public wire: " + JSON.stringify(report.wire.internalFieldLeaks);
    process.exitCode = 1;
  }
  writeFileSync(output, JSON.stringify(report, null, 2) + "\n");
  socket.close();
}
