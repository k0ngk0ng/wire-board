// Keep the owned CDP session alive until passive logging exists after reload.
import { readFile } from "node:fs/promises";
const [endpoint, url] = process.argv.slice(2);
if (!/^http:\/\/127\.0\.0\.1:\d+\/$/.test(url))
  throw Error("Owned loopback URL required");
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  socket.addEventListener("open", resolve, { once: true });
  socket.addEventListener("error", reject, { once: true });
});
let id = 0;
const pending = new Map();
socket.addEventListener("message", (event) => {
  const value = JSON.parse(event.data), item = pending.get(value.id);
  if (!item) return;
  pending.delete(value.id);
  value.error ? item.reject(Error(JSON.stringify(value.error))) : item.resolve(value.result);
});
const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
  const next = ++id;
  pending.set(next, { resolve, reject });
  socket.send(JSON.stringify({ id: next, method, params, ...(sessionId ? { sessionId } : {}) }));
});
const { targetInfos } = await call("Target.getTargets");
const target = targetInfos.find((t) => t.type === "page" && t.url === url);
if (!target) throw Error("Owned tab missing");
const { sessionId } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true });
try {
  await call("Page.enable", {}, sessionId);
  const before = await call("Runtime.evaluate", { expression: "performance.timeOrigin", returnByValue: true }, sessionId);
  const timeOrigin = before.result.value;
  const source = await readFile(new URL("./platform_app_observer.js", import.meta.url), "utf8");
  await call("Page.addScriptToEvaluateOnNewDocument", { source }, sessionId);
  await call("Page.reload", {}, sessionId);
  const deadline = performance.now() + 25000;
  let ready = false;
  while (performance.now() < deadline) {
    const result = await call("Runtime.evaluate", { expression: "performance.timeOrigin!=="+JSON.stringify(timeOrigin)+"&&document.readyState==='complete'&&!!window.yuyuAppObservation", returnByValue: true }, sessionId);
    if (result.result?.value === true) { ready = true; break; }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  if (!ready) throw Error("Reload did not initialize passive observer and actual game");
  console.log("Actual reload completed with passive observer");
} finally {
  await call("Target.detachFromTarget", { sessionId });
  socket.close();
}
