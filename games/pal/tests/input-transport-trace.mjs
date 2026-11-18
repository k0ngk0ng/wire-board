// Observe only an explicitly owned local page and delay the first HTTP input
// response, without changing game state or native input events. The connection
// may recover to WebSocket while that HTTP acknowledgement is still pending.
const [endpoint, prefix] = process.argv.slice(2);
if (!endpoint || !/^http:\/\/127\.0\.0\.1:\d+$/.test(prefix))
  throw Error("Expected owned local browser endpoint and fixture URL");
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0, sessionId, delayed = false;
let holdWork = Promise.resolve(), holding = false;
const pending = new Map(), events = [], timers = new Set();
const call = (method, params = {}, sid = sessionId) => new Promise((resolve, reject) => {
  const id = ++serial;
  pending.set(id, { resolve, reject });
  socket.send(JSON.stringify({ id, method, params, ...(sid ? { sessionId: sid } : {}) }));
});
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data);
  if (message.id) {
    const request = pending.get(message.id); pending.delete(message.id);
    message.error ? request.reject(Error(JSON.stringify(message.error))) : request.resolve(message.result);
    return;
  }
  if (["Network.webSocketFrameSent", "Network.requestWillBeSent"].includes(message.method)) {
    try {
      const payload = JSON.parse(message.params.response?.payloadData ?? message.params.request.postData);
      if (payload.action?.type === "input") events.push({ at: Date.now(), transport: message.method, action: payload.action });
    } catch {}
  }
  if (message.method === "Fetch.requestPaused") {
    let action;
    try { action = JSON.parse(message.params.request.postData).action; } catch {}
    const delay = !delayed && action?.type === "input" ? 800 : 0;
    if (delay) { delayed = true; events.push({ at: Date.now(), delayedResponseMS: delay }); }
    const timer = setTimeout(() => {
      timers.delete(timer);
      void call("Fetch.continueResponse", { requestId: message.params.requestId }).then(() => { if (delay) events.push({ at: Date.now(), delayedResponseContinued: true }); }).catch(error => events.push({ error: String(error) }));
    }, delay);
    timers.add(timer);
  }
};
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(target => target.type === "page" && target.url.startsWith(prefix));
  if (!target) throw Error("Owned fixture page is missing");
  ({ sessionId } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }));
  await call("Network.enable");
  await call("Page.enable");
  await call("Fetch.enable", { patterns: [{ urlPattern: prefix + "/api/rooms/*", requestStage: "Response" }] });
  await call("Page.addScriptToEvaluateOnNewDocument", {
    source: 'window.palNativeKeyEvents=[];for(const kind of ["keydown","keyup"])window.addEventListener(kind,event=>{if(event.code.startsWith("Arrow"))palNativeKeyEvents.push({kind,code:event.code,repeat:event.repeat,at:performance.timeOrigin+performance.now()})},true)',
  });
  process.on("SIGUSR2", () => console.log(JSON.stringify({ events, delayed })));
  process.stdin.on("data", data => {
    if (data.toString().trim() !== "hold-left" || holding) return;
    holding = true;
    // Native CDP key events supply physical code and logical key, recording
    // their held duration alongside the observed network inputs.
    const key = { code: "ArrowLeft", key: "ArrowLeft", windowsVirtualKeyCode: 37 };
    holdWork = (async () => {
      try {
        await call("Input.dispatchKeyEvent", { type: "rawKeyDown", ...key });
        await new Promise(resolve => setTimeout(resolve, 350));
      } finally {
        await call("Input.dispatchKeyEvent", { type: "keyUp", ...key });
        holding = false;
      }
    })().catch(error => events.push({ error: String(error) }));
  });
  console.log("ready");
  await new Promise(resolve => { process.once("SIGTERM", resolve); process.once("SIGINT", resolve); });
  console.log(JSON.stringify({ events, delayed }));
} finally {
  await holdWork;
  process.stdin.destroy();
  for (const timer of timers) clearTimeout(timer);
  if (sessionId) {
    await call("Fetch.disable");
    await call("Target.detachFromTarget", { sessionId }, null);
  }
  socket.close();
}
