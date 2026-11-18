import assert from "node:assert/strict";

const [endpoint, pageURL, selector] = process.argv.slice(2);
assert.equal(new URL(endpoint).hostname, "127.0.0.1");
assert.equal(new URL(pageURL).hostname, "127.0.0.1");
const socket = new WebSocket(endpoint);
const pending = new Map();
let sequence = 0;
socket.addEventListener("message", event => {
  const value = JSON.parse(event.data);
  const entry = pending.get(value.id);
  if (!entry) return;
  pending.delete(value.id);
  if (value.error) entry.reject(Error(JSON.stringify(value.error)));
  else entry.resolve(value.result);
});
await new Promise((resolve, reject) => { socket.addEventListener("open", resolve, { once: true }); socket.addEventListener("error", reject, { once: true }); });
const request = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
  const id = ++sequence;
  pending.set(id, { resolve, reject });
  socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
});
try {
  const { targetInfos } = await request("Target.getTargets");
  const target = targetInfos.find(target => target.type === "page" && target.url === pageURL);
  assert.ok(target, "Owned DOM oracle page missing");
  const { sessionId } = await request("Target.attachToTarget", { targetId: target.targetId, flatten: true });
  await request("DOM.enable", {}, sessionId);
  await request("CSS.enable", {}, sessionId);
  const { root } = await request("DOM.getDocument", {}, sessionId);
  const { nodeId } = await request("DOM.querySelector", { nodeId: root.nodeId, selector }, sessionId);
  assert.ok(nodeId);
  const { fonts } = await request("CSS.getPlatformFontsForNode", { nodeId }, sessionId);
  process.stdout.write(JSON.stringify(fonts) + "\n");
  await request("Target.detachFromTarget", { sessionId });
} finally { socket.close(); }
