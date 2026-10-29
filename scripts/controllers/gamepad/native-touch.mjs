// Native touch in an owned test page; game and input state are not replaced.
const [
  endpoint,
  origin,
  raw,
  selector = "[data-bubbleparty-canvas],[data-battlecity-canvas]",
] = process.argv.slice(2);
const steps = JSON.parse(raw),
  socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});
let serial = 0;
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
    const id = ++serial;
    pending.set(id, { resolve, reject });
    socket.send(JSON.stringify({ id, method, params, sessionId }));
  });
let sessionId;
let touching = false;
const samples = [];
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
    "Emulation.setTouchEmulationEnabled",
    { enabled: true, maxTouchPoints: 5 },
    sessionId,
  );
  for (const { type, touchPoints, wait = 0 } of steps) {
    if (
      !["touchStart", "touchMove", "touchEnd", "touchCancel"].includes(type) ||
      wait < 0 ||
      wait > 1000
    )
      throw Error("Invalid touch step");
    await call("Input.dispatchTouchEvent", { type, touchPoints }, sessionId);
    touching = type === "touchStart" || type === "touchMove";
    if (wait) await new Promise((r) => setTimeout(r, wait));
    const result = await call(
      "Runtime.evaluate",
      {
        expression:
          "({...document.querySelector(" +
          JSON.stringify(selector) +
          ")?.dataset})",
        returnByValue: true,
      },
      sessionId,
    );
    samples.push({ type, nativeTouch: true, render: result.result.value });
  }
  console.log(JSON.stringify(samples));
} finally {
  if (sessionId) {
    if (touching)
      await call(
        "Input.dispatchTouchEvent",
        { type: "touchCancel", touchPoints: [] },
        sessionId,
      );
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: false },
      sessionId,
    );
    await call("Target.detachFromTarget", { sessionId });
  }
  socket.close();
}
