import { mkdir, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { recordNativePixelInput, inspectNativePixelInput } from "./pixel-response.mjs";

const [endpoint, prefix, label = "native", durationValue = "8000", latencyValue = "0", selector = '[data-pokepath-action="speed"]'] = process.argv.slice(2);
const duration = Number(durationValue);
const latency = Number(latencyValue);
if (!endpoint || !prefix || !/^[-a-z0-9]+$/.test(label) || !Number.isInteger(duration) || duration < 1000 || duration > 30000 || !Number.isInteger(latency) || latency < 0 || latency > 1000) throw Error("Expected owned CDP endpoint, local page URL, label, duration, optional latency and control selector");
if (!["127.0.0.1", "localhost", "[::1]"].includes(new URL(prefix).hostname)) throw Error("Presentation acceptance requires an explicitly owned local test page");
const output = join(fileURLToPath(new URL("../../../", import.meta.url)), ".local/pokepath/verification");
await mkdir(output, { recursive: true });
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0;
let session;
const pending = new Map();
const inputRequests = new Map();
const acknowledgments = [];
const rtts = [];
const authorityFrames = [];
const network = [];
const listeners = new Set();
const nativeFrameErrors = [];
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data);
  if (message.id) {
    const request = pending.get(message.id);
    pending.delete(message.id);
    if (request) message.error ? request.reject(Error(JSON.stringify(message.error))) : request.resolve(message.result);
    return;
  }
  if (message.sessionId !== session) return;
  if (message.method === "Page.screencastFrame") void call("Page.screencastFrameAck", { sessionId: message.params.sessionId }).catch(error => nativeFrameErrors.push(String(error)));
  for (const listener of listeners) listener(message.method, message.params);
  const value = message.params;
  if (!["Network.webSocketFrameSent", "Network.webSocketFrameReceived"].includes(message.method)) return;
  const raw = value.response?.payloadData;
  if (typeof raw !== "string") return;
  try {
    const packet = JSON.parse(raw);
    const bytes = Buffer.byteLength(raw);
    network.push({ direction: message.method.endsWith("Sent") ? "out" : "in", at: value.timestamp, bytes, type: packet.type });
    if (message.method.endsWith("Sent") && packet.type === "input") inputRequests.set(packet.nonce, { at: value.timestamp, clientInputID: packet.action?.clientInputID });
    if (message.method.endsWith("Received")) {
      const input = inputRequests.get(packet.nonce);
      if (packet.type === "ack" && input) acknowledgments.push({ clientInputID: input.clientInputID, status: packet.status, elapsedMS: (value.timestamp - input.at) * 1000 });
      if (packet.type === "network" && Number.isFinite(packet.self?.rttMs)) rtts.push({ at: value.timestamp, rttMS: packet.self.rttMs, sampledAt: packet.self.sampledAt });
      if (packet.type === "game") authorityFrames.push({ at: value.timestamp, serverNow: packet.serverNow, clock: packet.game?.data?.clock, sequence: packet.game?.data?.sequence, snapshot: Boolean(packet.game?.data?.snapshot), bytes });
    }
  } catch {}
};
const call = (method, params = {}, target = session) => new Promise((resolve, reject) => {
  const id = ++serial;
  pending.set(id, { resolve, reject });
  socket.send(JSON.stringify({ id, method, params, ...(target ? { sessionId: target } : {}) }));
});
const evaluate = async expression => {
  const result = await call("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
  if (result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails));
  return result.result.value;
};
const quantiles = values => {
  const sorted = [...values].sort((a, b) => a - b);
  const at = fraction => sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * fraction))] ?? null;
  return { count: sorted.length, p50: at(0.5), p95: at(0.95), p99: at(0.99), max: sorted.at(-1) ?? null };
};
let report;
try {
  const { targetInfos } = await call("Target.getTargets", {}, null);
  const target = targetInfos.find(value => value.type === "page" && value.url.startsWith(prefix));
  if (!target) throw Error("No matching explicitly owned page");
  ({ sessionId: session } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }, null));
  await call("Runtime.enable");
  await call("Network.enable");
  await call("Performance.enable");
  const performanceMetrics = await call("Performance.getMetrics");
  const navigationStart = performanceMetrics.metrics.find(metric => metric.name === "NavigationStart")?.value;
  await evaluate("(()=>{for(const media of document.querySelectorAll('audio[data-platform-audio]')){media.muted=true;media.pause();}try{localStorage.setItem('wb_sound','off');dispatchEvent(new StorageEvent('storage',{key:'wb_sound',newValue:'off',storageArea:localStorage}));}catch{}return true;})()");
  await call("Network.emulateNetworkConditions", { offline: false, latency, downloadThroughput: -1, uploadThroughput: -1 });
  const room = await evaluate("fetch('/api/state').then(async response=>{if(!response.ok)throw Error('Authenticated platform state is unavailable');const state=await response.json();if(!state.user||state.room?.kind!=='pokepath'||state.room.status!=='playing')throw Error('An authenticated playing Pokepath room is required');return{kind:state.room.kind,players:state.room.seats.length,you:state.room.you,spectating:Boolean(state.room.spectating),waveActive:Boolean(state.room.game?.data?.waveActive),speed:state.room.game?.data?.speed,globalSound:localStorage.getItem('wb_sound')}})");
  await evaluate(`(()=>{
    if(window.__pokepathPresentationProbe)throw Error('Presentation probe is already active');
    const canvas=document.querySelector('.pokepath-board canvas');
    if(!canvas)throw Error('Native game canvas is not ready');
    const scratch=new OffscreenCanvas(120,104);
    const context=scratch.getContext('2d',{willReadFrequently:true});
    const probe={started:performance.now(),frames:[],inputs:[],costs:[],predictions:[],active:true,request:0};
    window.__pokepathPresentationProbe=probe;
    probe.input=event=>{if(event.isTrusted)probe.inputs.push({type:event.type,at:performance.now(),target:event.target.getAttribute?.('data-pokepath-action')??event.target.getAttribute?.('aria-label')??event.target.className});};
    for(const type of ['pointerdown','click','keydown'])addEventListener(type,probe.input,true);
    const sample=at=>{if(!probe.active)return;context.clearRect(0,0,120,104);context.drawImage(canvas,0,0,120,104);const bytes=context.getImageData(0,0,120,104).data;let hash=2166136261;for(let index=0;index<bytes.length;index+=4){hash=Math.imul(hash^bytes[index],16777619);hash=Math.imul(hash^bytes[index+1],16777619);hash=Math.imul(hash^bytes[index+2],16777619);}const prediction=probe.predictions.at(-1);probe.frames.push({at,hash:hash>>>0,moving:Boolean(prediction&&prediction.waveActive&&!prediction.paused&&prediction.visibleEnemies>0)});probe.request=requestAnimationFrame(sample);};
    probe.request=requestAnimationFrame(sample);
    return true;
  })()`);
  await new Promise(resolve => setTimeout(resolve, duration));
  await evaluate("(()=>{const probe=window.__pokepathPresentationProbe;probe.active=false;probe.sampleEnd=performance.now();cancelAnimationFrame(probe.request);return true})()");
  const rect = await evaluate(`(async()=>{const control=document.querySelector(${JSON.stringify(selector)});if(!control)return null;control.scrollIntoView({block:'nearest'});await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));const box=control.getBoundingClientRect();const hit=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);return{x:box.x,y:box.y,width:box.width,height:box.height,hit:Boolean(hit&&(hit===control||control.contains(hit))),disabled:Boolean(control.disabled)};})()`);
  let pixelTrace;
  if (rect?.hit && !rect.disabled && rect.width > 0 && rect.height > 0) {
    pixelTrace = await recordNativePixelInput({ call, evaluate, rect, listen: listener => { listeners.add(listener); return () => listeners.delete(listener); }, readInput: () => evaluate("window.__pokepathPresentationProbe.inputs.at(-1)") });
    if (nativeFrameErrors.length) throw Error(nativeFrameErrors.join("\n"));
  }
  const measured = await evaluate(`(()=>{const probe=window.__pokepathPresentationProbe;probe.active=false;cancelAnimationFrame(probe.request);for(const type of ['pointerdown','click','keydown'])removeEventListener(type,probe.input,true);const result={frames:probe.frames,inputs:probe.inputs,costs:probe.costs,predictions:probe.predictions,elapsedMS:probe.sampleEnd-probe.started,inputObservationMS:performance.now()-probe.sampleEnd};delete window.__pokepathPresentationProbe;return result;})()`);
  const pixelResponse = pixelTrace ? { selector, ...await inspectNativePixelInput({ trace: pixelTrace, output, label: `presentation-${label}` }) } : undefined;
  const deadline = Date.now() + 3000;
  while (acknowledgments.length < inputRequests.size && Date.now() < deadline) await new Promise(resolve => setTimeout(resolve, 10));
  const frameIntervals = measured.frames.slice(1).map((frame, index) => frame.at - measured.frames[index].at);
  const changed = measured.frames.filter((frame, index) => index > 0 && frame.hash !== measured.frames[index - 1].hash);
  const pixelIntervals = changed.slice(1).map((frame, index) => frame.at - changed[index].at);
  const movingIntervals = measured.frames.slice(1).map((frame, index) => ({ current: frame, previous: measured.frames[index] })).filter(pair => pair.current.moving && pair.previous.moving);
  const movingMS = movingIntervals.reduce((sum, pair) => sum + pair.current.at - pair.previous.at, 0);
  const movingChanges = movingIntervals.filter(pair => pair.current.hash !== pair.previous.hash).length;
  const arrivals = Number.isFinite(navigationStart) ? authorityFrames.map(frame => {
    const presented = measured.predictions.find(prediction => prediction.authoritySequence >= frame.sequence && navigationStart + prediction.nativeAt / 1000 >= frame.at);
    return presented ? (navigationStart + presented.nativeAt / 1000 - frame.at) * 1000 : undefined;
  }).filter(value => value !== undefined) : [];
  const costs = Object.fromEntries(["clone", "apply", "advance", "project"].map(phase => [phase, quantiles(measured.costs.filter(sample => sample.phase === phase).map(sample => sample.milliseconds))]));
  report = { command: `node games/pokepath/tests/presentation-browser.mjs ${endpoint} ${prefix} ${label} ${duration} ${latency} '${selector}'`, scope: "Explicitly owned native React/Canvas game in an authenticated platform room; no original program or iframe", room, configuredNetworkLatencyMS: latency, nativeRAF: quantiles(frameIntervals), actualCanvasChanges: { ...quantiles(pixelIntervals), changes: changed.length, perSecond: changed.length * 1000 / measured.elapsedMS, moving: { milliseconds: movingMS, changes: movingChanges, perSecond: movingMS ? movingChanges * 1000 / movingMS : null } }, pixelResponse: pixelResponse ?? { available: false, reason: "No visible enabled target or no changed target pixels were observed" }, inputACK: { ...quantiles(acknowledgments.map(sample => sample.elapsedMS)), timedOut: acknowledgments.length < inputRequests.size }, transportRTT: rtts.length ? { available: true, method: "Server-measured WebSocket protocol Ping/Pong, separate from application ACK", ...quantiles(rtts.map(sample => sample.rttMS)), samples: rtts } : { available: false, reason: "No protocol Ping/Pong RTT sample was observed; HTTP/action duration is not substituted" }, authorityToPresentationMS: { ...quantiles(arrivals), method: "CDP monotonic NavigationStart plus native performance.now at completed game paint" }, costs, authorityFrames, network, acknowledgments, measured };
  await writeFile(join(output, `presentation-${label}.json`), JSON.stringify(report, null, 2) + "\n");
  process.stdout.write(JSON.stringify({ nativeRAF: report.nativeRAF, actualCanvasChanges: report.actualCanvasChanges, pixelResponse: report.pixelResponse, inputACK: report.inputACK, transportRTT: report.transportRTT, authorityToPresentationMS: report.authorityToPresentationMS, costs }) + "\n");
} finally {
  if (session) {
    try { await evaluate("(()=>{const probe=window.__pokepathPresentationProbe;if(!probe)return;probe.active=false;cancelAnimationFrame(probe.request);for(const type of ['pointerdown','click','keydown'])removeEventListener(type,probe.input,true);delete window.__pokepathPresentationProbe;})()"); } catch {}
    try { await call("Network.emulateNetworkConditions", { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 }); } catch {}
    try { await call("Target.detachFromTarget", { sessionId: session }, null); } catch {}
  }
  socket.close();
}
