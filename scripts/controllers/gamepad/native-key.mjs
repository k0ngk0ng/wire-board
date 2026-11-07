// Native keyboard events for coexistence/shortcut tests; no DOM event injection.
// windowsVirtualKeyCode is standardized by CDP; do not reuse it as a macOS
// hardware keycode, which can trigger unrelated native default actions.
const [endpoint, origin, raw] = process.argv.slice(2),
  step = JSON.parse(raw);
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});
let id = 0;
const pending = new Map();
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data),
    task = pending.get(message.id);
  if (!task) return;
  pending.delete(message.id);
  message.error
    ? task.reject(Error(JSON.stringify(message.error)))
    : task.resolve(message.result);
};
const call = (method, params = {}, sessionId) =>
  new Promise((resolve, reject) => {
    const n = ++id;
    pending.set(n, { resolve, reject });
    socket.send(JSON.stringify({ id: n, method, params, sessionId }));
  });
let sessionId;
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(
    (t) => t.type === "page" && t.url.startsWith(origin),
  );
  if (!target) throw Error("Owned page missing");
  ({ sessionId } = await call("Target.attachToTarget", {
    targetId: target.targetId,
    flatten: true,
  }));
  await call(
    "Input.dispatchKeyEvent",
    {
      type: step.down ? "keyDown" : "keyUp",
      key: step.key,
      code: step.code,
      windowsVirtualKeyCode: step.vk,
      modifiers: step.modifiers ?? 0,
    },
    sessionId,
  );
  console.log(JSON.stringify({ nativeKeyboard: true, ...step }));
} finally {
  if (sessionId) await call("Target.detachFromTarget", { sessionId });
  socket.close();
}
