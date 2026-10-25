// Native two-dimensional pointer path. All measurements use the actual rendered
// paddle centre and real HTTP state, never fabricated callbacks or snapshots.
import assert from "node:assert/strict";
const [url, pageURL, raw = "{}"] = process.argv.slice(2),
  options = JSON.parse(raw),
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
  const rect = await evaluate(
    'document.querySelector(".tt-scene canvas").getBoundingClientRect().toJSON()',
  );
  if (options.touch)
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: true, maxTouchPoints: 2 },
      sessionId,
    );
  const points = [
      [0.08, 0.12],
      [0.88, 0.12],
      [0.88, 0.84],
      [0.14, 0.84],
      [0.5, 0.5],
      [0.9, 0.92],
      [0.08, 0.08],
    ],
    trace = [];
  let previous = points[0],
    started = false;
  for (const target of points) {
    for (let step = 0; step <= 8; step++) {
      const x = previous[0] + ((target[0] - previous[0]) * step) / 8,
        y = previous[1] + ((target[1] - previous[1]) * step) / 8;
      const point = { x: rect.x + x * rect.width, y: rect.y + y * rect.height };
      if (options.touch)
        await call(
          "Input.dispatchTouchEvent",
          {
            type: started ? "touchMove" : "touchStart",
            touchPoints: [
              { ...point, radiusX: 4, radiusY: 4, force: 1, id: 0 },
            ],
          },
          sessionId,
        );
      else
        await call(
          "Input.dispatchMouseEvent",
          { type: "mouseMoved", ...point, button: "none", buttons: 0 },
          sessionId,
        );
      started = true;
      await sleep(24);
      const f = await evaluate(
        'document.querySelector(".tt-scene canvas").__tableTennisFrame',
      );
      const error = Math.hypot(
        f.racket.x - x * rect.width,
        f.racket.y - y * rect.height,
      );
      trace.push({ x: x * rect.width, y: y * rect.height, error, frame: f });
      assert.ok(error < 2, JSON.stringify(trace.at(-1)));
    }
    previous = target;
  }
  assert.equal(
    await evaluate(
      'getComputedStyle(document.querySelector(".tt-scene canvas")).cursor',
    ),
    "none",
  );
  const radii = trace.map((v) => v.frame.racket.radius);
  assert.ok(
    Math.max(...radii) - Math.min(...radii) < 0.1,
    "paddle size changed with depth",
  );
  if (options.touch)
    await call(
      "Input.dispatchTouchEvent",
      { type: "touchEnd", touchPoints: [] },
      sessionId,
    );
  else
    await call(
      "Input.dispatchMouseEvent",
      { type: "mouseMoved", x: 2, y: 2, button: "none", buttons: 0 },
      sessionId,
    );
  const room = await evaluate(
    'fetch("/api/state").then(r=>r.json()).then(s=>s.room)',
  );
  assert.ok(
    room.game.data.paddles[room.you].motionSeq >= 3,
    "movement did not reach the server",
  );
  console.log(
    JSON.stringify({
      passed: true,
      touch: !!options.touch,
      maxPixelError: Math.max(...trace.map((x) => x.error)),
      motionSeq: room.game.data.paddles[room.you].motionSeq,
      trace,
    }),
  );
} finally {
  ws.close();
}
