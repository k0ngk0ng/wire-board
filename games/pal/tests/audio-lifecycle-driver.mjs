import readline from "node:readline";

const [endpoint, prefix] = process.argv.slice(2);
if (!endpoint || !/^http:\/\/127\.0\.0\.1:\d+$/.test(prefix)) throw Error("Expected owned local audio page");
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0, sessionId, targetId, backgroundTarget, operation;
const pending = new Map();
const call = (method, params = {}, sid = sessionId) => new Promise((resolve, reject) => {
  const id = ++serial;
  pending.set(id, { resolve, reject });
  socket.send(JSON.stringify({ id, method, params, ...(sid ? { sessionId: sid } : {}) }));
});
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data), request = pending.get(message.id);
  if (!request) return;
  pending.delete(message.id);
  if (message.error) request.reject(Error(JSON.stringify(message.error)));
  else request.resolve(message.result);
};
const evaluate = async expression => {
  const result = await call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
  if (result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails));
  return result.result.value;
};
const sample = () => evaluate(`(async()=>{const state=await(await fetch('/api/state')).json();return {at:performance.now(),hidden:document.hidden,focused:document.hasFocus(),state,events:window.palNativeAudio,media:[...document.querySelectorAll('audio[data-platform-track]')].map(m=>({track:m.dataset.platformTrack,time:m.currentTime,paused:m.paused,source:m.currentSrc}))}})()`);
const tap = async touch => {
  const point = await evaluate(`(()=>{const button=document.querySelector('[data-sound-mute]');if(!button||!button.getClientRects().length)throw Error('Visible platform mute button missing');const b=button.getBoundingClientRect();return {x:b.x+b.width/2,y:b.y+b.height/2}})()`);
  if (touch) {
    await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ ...point, radiusX: 1, radiusY: 1, force: 1, id: 1 }] });
    await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  } else {
    await call("Input.dispatchMouseEvent", { type: "mousePressed", ...point, button: "left", clickCount: 1 });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...point, button: "left", clickCount: 1 });
  }
};
const input = readline.createInterface({ input: process.stdin });
try {
  const { targetInfos } = await call("Target.getTargets", {}, null), target = targetInfos.find(value => value.type === "page" && value.url.startsWith(prefix));
  if (!target) throw Error("Owned audio page missing");
  targetId = target.targetId;
  ({ sessionId } = await call("Target.attachToTarget", { targetId, flatten: true }, null));
  console.log("ready");
  for await (const line of input) {
    const command = JSON.parse(line);
    let value;
    if (command.type === "status") value = await sample();
    else if (command.type === "arm-visibility" || command.type === "arm-mute") {
      operation = (async () => {
        const first = await evaluate(`new Promise((resolve,reject)=>{const timer=setTimeout(()=>{document.removeEventListener('playing',watch,true);reject(Error('Original sound did not start'))},45000);const watch=event=>{const m=event.target;if(m instanceof HTMLAudioElement&&m.dataset.platformTrack?.startsWith('pal-sfx-')){clearTimeout(timer);document.removeEventListener('playing',watch,true);resolve({at:performance.now(),time:m.currentTime,source:m.currentSrc})}};document.addEventListener('playing',watch,true);window.palAudioLifecycleArmed=true})`);
        const before = await sample();
        let quiet, returned;
        if (command.type === "arm-visibility") {
          ({ targetId: backgroundTarget } = await call("Target.createTarget", { url: "about:blank" }, null));
          await call("Target.activateTarget", { targetId: backgroundTarget }, null);
          quiet = await sample();
          await new Promise(resolve => setTimeout(resolve, 100));
          await call("Target.activateTarget", { targetId }, null);
          await call("Target.closeTarget", { targetId: backgroundTarget }, null);
          backgroundTarget = undefined;
          returned = await sample();
        } else {
          await tap(command.touch);
          quiet = await sample();
          await new Promise(resolve => setTimeout(resolve, 100));
          await tap(command.touch);
          returned = await sample();
        }
        return { first, before, quiet, returned };
      })().then(result => ({ result }), error => ({ error: String(error) }));
      value = await evaluate("window.palAudioLifecycleArmed===true");
    } else if (command.type === "result") value = await operation;
    else throw Error("Unknown audio lifecycle command");
    console.log(JSON.stringify(value));
  }
} finally {
  if (backgroundTarget) await call("Target.closeTarget", { targetId: backgroundTarget }, null);
  if (sessionId) await call("Target.detachFromTarget", { sessionId }, null);
  socket.close();
  input.close();
}
