// Native touch through the acceptance-owned browser, no synthetic DOM click.
const [endpoint, origin, selector, mode] = process.argv.slice(2);
const ws = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  ws.onopen = resolve;
  ws.onerror = reject;
});
let sequence = 0,
  sessionId;
const pending = new Map();
ws.onmessage = ({ data }) => {
  const message = JSON.parse(data),
    entry = pending.get(message.id);
  if (!entry) return;
  pending.delete(message.id);
  if (message.error) entry.reject(Error(JSON.stringify(message.error)));
  else entry.resolve(message.result);
};
const call = (method, params = {}, sid = sessionId) =>
  new Promise((resolve, reject) => {
    const id = ++sequence;
    pending.set(id, { resolve, reject });
    ws.send(
      JSON.stringify({
        id,
        method,
        params,
        sessionId: method.startsWith("Target.") ? undefined : sid,
      }),
    );
  });
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(
    (t) => t.type === "page" && t.url.startsWith(origin),
  );
  if (!target) throw Error("Acceptance page missing");
  ({ sessionId } = await call("Target.attachToTarget", {
    targetId: target.targetId,
    flatten: true,
  }));
  if (mode === "shot" || mode === "bomb") {
    await call("Input.dispatchKeyEvent", {
      type: "keyDown",
      key: " ",
      code: "Space",
      windowsVirtualKeyCode: 32,
      nativeVirtualKeyCode: 32,
    });
    await new Promise((resolve) => setTimeout(resolve, 300));
    const held = await call("Runtime.evaluate", {
      expression:
        mode === "shot"
          ? 'document.querySelector(".fortressblue-board").dataset.fortressblueCharging'
          : '"1"',
      returnByValue: true,
      awaitPromise: true,
    });
    await call("Input.dispatchKeyEvent", {
      type: "keyUp",
      key: " ",
      code: "Space",
      windowsVirtualKeyCode: 32,
      nativeVirtualKeyCode: 32,
    });
    if (held.result.value !== "1")
      throw Error("Native Space did not start charge");
    console.log(JSON.stringify({ nativeKeyboard: true, charge: true }));
  } else {
    await call("Emulation.setTouchEmulationEnabled", {
      enabled: mode !== "mouse",
      maxTouchPoints: 5,
    });
    const { result, exceptionDetails } = await call("Runtime.evaluate", {
      expression: `(async()=>{const e=document.querySelector(${JSON.stringify(selector)});e.scrollIntoView({block:'nearest'});await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const r=e.getBoundingClientRect(),x=r.x+r.width/2,y=r.y+r.height/2;if(!e.contains(document.elementFromPoint(x,y)))throw Error('Control obscured');return {x,y,id:1}})()`,
      returnByValue: true,
      awaitPromise: true,
    });
    if (exceptionDetails) throw Error(JSON.stringify(exceptionDetails));
    if (mode === "mouse") {
      await call("Input.dispatchMouseEvent", {
        type: "mousePressed",
        x: result.value.x,
        y: result.value.y,
        button: "left",
        buttons: 1,
        clickCount: 1,
      });
      await call("Input.dispatchMouseEvent", {
        type: "mouseReleased",
        x: result.value.x,
        y: result.value.y,
        button: "left",
        buttons: 0,
        clickCount: 1,
      });
    } else {
      await call("Input.dispatchTouchEvent", {
        type: "touchStart",
        touchPoints: [result.value],
      });
      await new Promise((resolve) => setTimeout(resolve, 80));
      await call("Input.dispatchTouchEvent", {
        type: "touchEnd",
        touchPoints: [],
      });
    }
    console.log(
      JSON.stringify({
        nativeTouch: mode !== "mouse",
        nativeMouse: mode === "mouse",
        target: selector,
        point: result.value,
      }),
    );
  }
} finally {
  if (sessionId)
    await call("Target.detachFromTarget", { sessionId }, undefined);
  ws.close();
}
