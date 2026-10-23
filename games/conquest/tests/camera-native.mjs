// Native browser input against the explicitly selected camera test page.
// No page scripts dispatch synthetic pointer/touch events.
const [endpoint, prefix, mode, ...raw] = process.argv.slice(2);
const [x, y] = raw.map(Number);
if (
  !endpoint ||
  !prefix ||
  ![
    "pinch",
    "drag",
    "mouse",
    "cancel",
    "tap",
    "mini",
    "mouse-path",
    "mouse-text",
    "mouse-outside",
    "mouse-tap",
    "mouse-fit",
    "mouse-fit-text",
    "touch-fit",
  ].includes(mode) ||
  ![x, y].every(Number.isFinite)
)
  throw Error("invalid camera input request");
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});
let id = 0;
const pending = new Map();
socket.onmessage = ({ data }) => {
  const m = JSON.parse(data);
  if (!m.id) return;
  const p = pending.get(m.id);
  pending.delete(m.id);
  m.error ? p.reject(Error(JSON.stringify(m.error))) : p.resolve(m.result);
};
const call = (method, params = {}, sessionId) =>
  new Promise((resolve, reject) => {
    const next = ++id;
    pending.set(next, { resolve, reject });
    socket.send(JSON.stringify({ id: next, method, params, sessionId }));
  });
const delay = (ms) => new Promise((r) => setTimeout(r, ms));
let sessionId;
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(
    (t) => t.type === "page" && t.url.startsWith(prefix),
  );
  if (!target) throw Error("owned camera page missing");
  ({ sessionId } = await call("Target.attachToTarget", {
    targetId: target.targetId,
    flatten: true,
  }));
  const evaluate = async (expression) =>
    (
      await call(
        "Runtime.evaluate",
        { expression, returnByValue: true, awaitPromise: true },
        sessionId,
      )
    ).result.value;
  const snapshot = () =>
    evaluate(
      `(()=>{const s=document.querySelector('#scene').transform.baseVal.consolidate().matrix;return {...window.cameraEvidence,frames:[...window.cameraEvidence.frames],camera:{x:s.e,y:s.f,scale:s.a},scrollY}})()`,
    );
  const before = await snapshot();
  const startTarget = await evaluate(
    `(()=>{const e=document.elementFromPoint(${x},${y});return e?{tag:e.tagName,region:e.getAttribute('data-region'),label:e.getAttribute('data-label')}:null})()`,
  );
  let released = null,
    postRelease = null;
  if (mode.startsWith("mouse")) {
    const rect = await evaluate(
      'document.querySelector("#camera-viewport").getBoundingClientRect().toJSON()',
    );
    const end =
      mode === "mouse-outside"
        ? { x: rect.right - 30, y: rect.bottom + 80 }
        : mode === "mouse-tap"
          ? { x, y }
          : { x: x + 48, y: y + 32 };
    await call(
      "Input.dispatchMouseEvent",
      { type: "mousePressed", x, y, button: "left", buttons: 1, clickCount: 1 },
      sessionId,
    );
    for (let i = 1; i <= (mode === "mouse-tap" ? 0 : 40); i++) {
      await call(
        "Input.dispatchMouseEvent",
        {
          type: "mouseMoved",
          x: x + ((end.x - x) * i) / 40,
          y: y + ((end.y - y) * i) / 40,
          button: "left",
          buttons: 1,
        },
        sessionId,
      );
      await delay(8);
    }
    await call(
      "Input.dispatchMouseEvent",
      {
        type: "mouseReleased",
        x: end.x,
        y: end.y,
        button: "left",
        buttons: 0,
        clickCount: 1,
      },
      sessionId,
    );
    await evaluate(
      "new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",
    );
    released = await snapshot();
    await call(
      "Input.dispatchMouseEvent",
      {
        type: "mouseMoved",
        x: rect.left + rect.width / 2,
        y: rect.top + rect.height / 2,
        button: "none",
        buttons: 0,
      },
      sessionId,
    );
    await evaluate(
      "new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",
    );
    postRelease = await snapshot();
  } else {
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: true, maxTouchPoints: 5 },
      sessionId,
    );
    const touch = (type, points) =>
      call(
        "Input.dispatchTouchEvent",
        {
          type,
          touchPoints: points.map((p) => ({
            radiusX: 2,
            radiusY: 2,
            force: 1,
            ...p,
          })),
        },
        sessionId,
      );
    await touch("touchStart", [{ x, y, id: 1 }]);
    if (mode === "pinch") {
      await touch("touchStart", [
        { x, y, id: 1 },
        { x: x + 40, y: y + 8, id: 2 },
      ]);
      for (let i = 1; i <= 30; i++) {
        await touch("touchMove", [
          { x: x - i * 1.5, y: y + i * 0.25, id: 1 },
          { x: x + 40 + i * 1.5, y: y + 8 + i * 0.5, id: 2 },
        ]);
        await delay(8);
      }
      await touch("touchEnd", [{ x: x - 45, y: y + 7.5, id: 1 }]);
      for (let i = 1; i <= 8; i++) {
        await touch("touchMove", [
          { x: x - 45 + i * 2, y: y + 7.5 + i, id: 1 },
        ]);
        await delay(8);
      }
    } else if (mode !== "tap") {
      for (let i = 1; i <= 40; i++) {
        await touch("touchMove", [{ x: x + i * 1.2, y: y + i * 0.8, id: 1 }]);
        await delay(8);
      }
    }
    await touch(mode === "cancel" ? "touchCancel" : "touchEnd", []);
  }
  await evaluate(
    "new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))",
  );
  await delay(80);
  const after = await snapshot();
  console.log(
    JSON.stringify({
      mode,
      startTarget,
      released,
      postRelease,
      before,
      after,
      renderDelta: after.renders - before.renders,
      clickDelta: after.clicks - before.clicks,
    }),
  );
} finally {
  if (sessionId) {
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: false },
      sessionId,
    );
    await call("Target.detachFromTarget", { sessionId });
  }
  socket.close();
}
