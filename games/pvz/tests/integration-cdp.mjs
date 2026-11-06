// Native touch dispatch is not exposed by the installed browser CLI. Reuse its
// owned CDP browser, with a caller-selected page and only declared commands.
const [endpoint, origin, encoded] = process.argv.slice(2);
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => { socket.addEventListener("open", resolve, { once: true }); socket.addEventListener("error", reject, { once: true }); });
let serial = 0;
const pending = new Map();
socket.addEventListener("message", event => { const r = JSON.parse(event.data); if (r.id && pending.has(r.id)) { const { resolve, reject } = pending.get(r.id); pending.delete(r.id); r.error ? reject(Error(JSON.stringify(r.error))) : resolve(r.result); } });
function call(method, params = {}, sessionId) { return new Promise((resolve, reject) => { const id = ++serial; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) })); }); }
try {
 const { targetInfos } = await call("Target.getTargets");
 const pages = targetInfos.filter(t => t.type === "page" && t.url.startsWith(origin));
 if (pages.length !== 1) throw Error("Expected exactly one selected acceptance page");
 const { sessionId } = await call("Target.attachToTarget", { targetId: pages[0].targetId, flatten: true });
 const results = [];
 for (const command of JSON.parse(encoded)) results.push(await call(command.method, command.params, sessionId));
 console.log(JSON.stringify(results));
 await call("Target.detachFromTarget", { sessionId });
} finally { socket.close(); }
