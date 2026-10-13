// agent-browser owns the session. Supply explicit viewport coordinates because
// its mouse wheel command can target a different location from mouse move.
const [endpoint, prefix, x, y, deltaY] = process.argv.slice(2);
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});
let serial = 0;
const pending = new Map();
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data);
  if (!message.id) return;
  const task = pending.get(message.id);
  pending.delete(message.id);
  if (message.error) task.reject(new Error(JSON.stringify(message.error)));
  else task.resolve(message.result);
};
function call(method, params = {}, sessionId) {
  return new Promise((resolve, reject) => {
    const id = ++serial;
    pending.set(id, { resolve, reject });
    socket.send(JSON.stringify({ id, method, params, sessionId }));
  });
}
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(
    (t) => t.type === "page" && t.url.startsWith(prefix),
  );
  if (!target)
    throw new Error("No page in the owned browser session matches " + prefix);
  const { sessionId } = await call("Target.attachToTarget", {
    targetId: target.targetId,
    flatten: true,
  });
  await call(
    "Input.dispatchMouseEvent",
    { type: "mouseWheel", x: +x, y: +y, deltaX: 0, deltaY: +deltaY },
    sessionId,
  );
  await call("Target.detachFromTarget", { sessionId });
} finally {
  socket.close();
}
