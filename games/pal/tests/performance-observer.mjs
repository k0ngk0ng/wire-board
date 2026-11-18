import fs from "node:fs/promises";
const [endpoint, prefix, probe, heldResponseMS, ...assetURLs] = process.argv.slice(2);
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(prefix)) throw Error("Expected owned local page");
if (probe && !["selection-performance-page.js", "continuous-performance-page.js", "dense-battle-page.js", "summon-battle-page.js"].includes(probe)) throw Error("Unknown performance probe");
const summon = probe === "summon-battle-page.js";
const saveSummon = summon && process.env.PAL_OBSERVE_SUMMON_SAVE === "1";
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0, sessionId;
let saveRequested = false;
const pending = new Map(), events = [], sends = new Map(), connections = new Set();
const call = (method, params = {}, sid = sessionId) => new Promise((resolve, reject) => {
  const id = ++serial; pending.set(id, { resolve, reject });
  socket.send(JSON.stringify({ id, method, params, ...(sid ? { sessionId: sid } : {}) }));
});
async function touchSave() {
  const reply=await call("Runtime.evaluate",{expression:`(()=>{
    const b=document.querySelector("[data-room-fullscreen] [data-save-game]");
    if(!b||b.disabled)return null;
    const r=b.getBoundingClientRect(),x=r.x+r.width/2,y=r.y+r.height/2;
    return document.elementFromPoint(x,y)?.closest("[data-save-game]")===b?{x,y,id:1}:null;
  })()`,returnByValue:true});
  if(!reply.result.value)throw Error("visible original save button not ready");
  await call("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:[reply.result.value]});
  await call("Input.dispatchTouchEvent",{type:"touchEnd",touchPoints:[]});
  events.push({at:Date.now(),type:"save-dispatched"});
}
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data);
  if (message.id) {
    const request = pending.get(message.id); pending.delete(message.id);
    message.error ? request.reject(Error(JSON.stringify(message.error))) : request.resolve(message.result);
    return;
  }
  const p = message.params, now = Date.now();
  if (message.method === "Network.webSocketHandshakeResponseReceived") connections.add(p.requestId);
  if (message.method === "Network.webSocketClosed") connections.delete(p.requestId);
  if (message.method === "Fetch.requestPaused") {
    events.push({at:now,type:"effect-response-held",url:p.request.url,status:p.responseStatusCode});
    setTimeout(() => {
      events.push({at:Date.now(),type:"effect-response-released",url:p.request.url});
      call("Fetch.continueResponse",{requestId:p.requestId}).catch(error=>{
        events.push({at:Date.now(),type:"effect-response-error",error:String(error)});
      });
    },Number(heldResponseMS));
  }
  if (message.method === "Network.webSocketFrameSent") {
    try {
      const value = JSON.parse(p.response.payloadData);
      if (value.type === "input") {
        sends.set(value.nonce, now);
        events.push({ at: now, type: "sent", seq: value.action.seq, action: value.action, nonce: value.nonce });
      }
    } catch {}
  }
  if (message.method === "Network.webSocketFrameReceived") {
    try {
      const value = JSON.parse(p.response.payloadData);
      const frame = value.game?.data ?? value.state?.room?.game?.data;
      const record = { at: now, type: value.type, bytes: Buffer.byteLength(p.response.payloadData) };
      if (frame) Object.assign(record, { epoch: frame.epoch, seq: frame.seq, appliedSeq: frame.appliedSeq, x: frame.x, y: frame.y, phase: frame.phase, ...(probe ? { discardedSeq: frame.discardedSeq, menuSelected: frame.menu?.selected, battleSelected: frame.battle?.selected, battleState: frame.battle?.state, battleMenu: frame.battle?.menu } : {}) });
      if (value.type === "network") {
        record.connectionRTTMS = value.self?.rttMs;
        record.connectionSampledAt = value.self?.sampledAt;
      }
      if (value.type === "ack") Object.assign(record, { nonce: value.nonce, status: value.status, ackMS: sends.has(value.nonce) ? now - sends.get(value.nonce) : undefined });
      if (probe === "continuous-performance-page.js" && frame) {
        record.fieldFrame = frame.field?.frame;
        record.arrowFrames = frame.raster?.nodes.filter(n => n[0] === 1 && n[5] === 2 && n[6] === 0 && (n[7] === 68 || n[7] === 69) && n[8] === 172 && n[9] === 106).map(n => n[7]);
      }
      if ((probe === "dense-battle-page.js" || summon) && frame) {
        record.battlePhase = frame.battle?.phase;
        record.current = frame.battle?.current;
        record.cash = frame.cash;
        record.controlAt = frame.controlAt;
        record.enemies = frame.battle?.enemies.map(enemy => enemy.object);
        record.partyHP = frame.party?.map(player => player.hp);
        record.nodes = frame.raster?.nodes.length;
        record.effects = frame.raster?.nodes.filter(node=>node[0]===1&&node[5]===8).map(node=>node.slice(6,10));
        record.version = value.version;
        if (summon) {
          record.loading = !!value.state?.room?.loading || !!value.loading;
          record.roomID = value.roomId ?? value.state?.room?.id;
          record.serverInstance = value.serverInstance ?? value.state?.serverInstance;
          record.clockOffsetMS = value.clockOffsetMs ?? value.state?.room?.clockOffsetMs ?? 0;
          record.serverNow = value.serverNow ?? value.state?.serverNow;
          record.partyMP = frame.party?.map(player=>player.mp);
          record.spell = frame.battle?.object;
          record.sprites = frame.raster?.nodes.filter(node=>node[0]===1&&node[5]===6&&node[6]===14).map(node=>node.slice(6,10));
          if(saveSummon&&!saveRequested&&value.type==="game"&&record.effects?.some(n=>n[0]===44&&n[1]===6)){
            saveRequested=true;
            events.push({at:now,type:"save-trigger",frame:6});
            touchSave().catch(error=>events.push({at:Date.now(),type:"save-error",error:String(error)}));
          }
        }
      }
      events.push(record);
    } catch {}
  }
  if (events.length > 10000) events.splice(0, events.length - 10000);
};
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(target => target.type === "page" && target.url.startsWith(prefix));
  if (!target) throw Error("Owned performance page missing");
  ({ sessionId } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }));
  await call("Page.enable"); await call("Network.enable");
  if (heldResponseMS) {
    if ((!summon&&probe!=="dense-battle-page.js") || !/^[1-9]\d{1,2}$/.test(heldResponseMS) || assetURLs.length!==(summon?2:1) || !assetURLs.every(url=>/^https:\/\/cdn\.ichenj\.com\/bga\/.+\/pal\/visual-v1\/objects\/[a-f0-9]{64}\.json$/.test(url)) || new Set(assetURLs).size!==assetURLs.length)
      throw Error("Expected published spell metadata URLs for the delayed-response case");
    await call("Fetch.enable",{patterns:assetURLs.map(url=>({urlPattern:url,requestStage:"Response"}))});
  }
  await call("Page.addScriptToEvaluateOnNewDocument", { source: await fs.readFile(new URL("./" + (probe ?? "performance-page.js"), import.meta.url), "utf8") });
  process.on("SIGUSR2", () => console.log(JSON.stringify({ events, connections: connections.size })));
  console.log("ready");
  await new Promise(resolve => { process.once("SIGTERM", resolve); process.once("SIGINT", resolve); });
  console.log(JSON.stringify({ events, connections: connections.size }));
} finally {
  if (sessionId) await call("Target.detachFromTarget", { sessionId }, null);
  socket.close();
}
