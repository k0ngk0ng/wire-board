// Real CDP pointer movement/cancellation. Read-only diagnostics measure the
// production paddle; no game callback, clock or state is injected.
import assert from "node:assert/strict";
const [url, pageURL, touchFlag] = process.argv.slice(2);
const touch = touchFlag === "true",
  ws = new WebSocket(url),
  pending = new Map();
let serial = 0;
await new Promise((resolve, reject) => {
  ws.onopen = resolve;
  ws.onerror = reject;
});
ws.onmessage = ({ data }) => {
  const m = JSON.parse(data),
    p = pending.get(m.id);
  if (p) {
    pending.delete(m.id);
    m.error ? p.reject(m.error) : p.resolve(m.result);
  }
};
const call = (method, params = {}, sessionId) =>
  new Promise((resolve, reject) => {
    const id = ++serial;
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify({ id, method, params, sessionId }));
  });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
try {
  const { targetInfos } = await call("Target.getTargets"),
    target = targetInfos.find(
      (t) => t.type === "page" && t.url.startsWith(pageURL),
    );
  assert.ok(target);
  const { sessionId } = await call("Target.attachToTarget", {
    targetId: target.targetId,
    flatten: true,
  });
  const evaluate = async (expression) => {
    const r = await call(
      "Runtime.evaluate",
      { expression, returnByValue: true, awaitPromise: true },
      sessionId,
    );
    assert.ok(!r.exceptionDetails, JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  const frame = () =>
    evaluate('document.querySelector(".tt-scene canvas").__tableTennisFrame');
  const initial = await evaluate(
    'fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.controls.seq)',
  );
  let ready = false;
  for (let i = 0; i < 1500; i++) {
    const f = await frame();
    if (f.phase === "rally" && !f.inputEnabled) {
      ready = true;
      break;
    }
    await sleep(10);
  }
  assert.ok(ready, "natural rally required");
  if (touch)
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: true, maxTouchPoints: 2 },
      sessionId,
    );
  const rect = await evaluate(
      'document.querySelector(".tt-scene canvas").getBoundingClientRect().toJSON()',
    ),
    y = rect.y + rect.height * 0.63,
    trace = [];
  for (let i = 0; i < 3; i++) {
    const x = rect.x + rect.width * 0.5 + (i - 1) * 30;
    if (touch)
      await call(
        "Input.dispatchTouchEvent",
        {
          type: i ? "touchMove" : "touchStart",
          touchPoints: [{ x, y, radiusX: 4, radiusY: 4, force: 1, id: 0 }],
        },
        sessionId,
      );
    else {
      await call(
        "Input.dispatchMouseEvent",
        {
          type: "mouseMoved",
          x,
          y,
          button: i ? "left" : "none",
          buttons: i ? 1 : 0,
        },
        sessionId,
      );
      if (!i)
        await call(
          "Input.dispatchMouseEvent",
          {
            type: "mousePressed",
            x,
            y,
            button: "left",
            buttons: 1,
            clickCount: 1,
          },
          sessionId,
        );
    }
    await sleep(25);
    trace.push({ pointerX: x - rect.x, frame: await frame() });
  }
  assert.ok(
    trace[2].frame.racket.x - trace[0].frame.racket.x > 35,
    JSON.stringify(trace),
  );
  if (touch)
    await call(
      "Input.dispatchTouchEvent",
      { type: "touchCancel", touchPoints: [] },
      sessionId,
    );
  else {
    // ESC cancels the held gesture; release must not produce a delayed stroke.
    await call(
      "Input.dispatchKeyEvent",
      {
        type: "rawKeyDown",
        key: "Escape",
        code: "Escape",
        windowsVirtualKeyCode: 27,
      },
      sessionId,
    );
    await call(
      "Input.dispatchKeyEvent",
      {
        type: "keyUp",
        key: "Escape",
        code: "Escape",
        windowsVirtualKeyCode: 27,
      },
      sessionId,
    );
    await call(
      "Input.dispatchMouseEvent",
      {
        type: "mouseReleased",
        x: rect.x + rect.width * 0.5 + 30,
        y,
        button: "left",
        buttons: 0,
        clickCount: 1,
      },
      sessionId,
    );
  }
  await sleep(100);
  const after = await frame();
  assert.equal(after.gestureDown, false);
  const state = await evaluate(
    'fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data)',
  );
  if (touch)
    assert.equal(
      state.controls.seq,
      initial,
      "touch cancel must not submit a swing",
    );
  console.log(
    JSON.stringify({
      passed: true,
      touch,
      trace,
      after,
      initialSeq: initial,
      finalSeq: state.controls.seq,
    }),
  );
} finally {
  ws.close();
}
