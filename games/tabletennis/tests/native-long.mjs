// Natural long return: real touch is placed on a visible near-court ball.
import assert from "node:assert/strict";
const [url, pageURL, raw = "{}"] = process.argv.slice(2),
  ws = new WebSocket(url),
  pending = new Map();
const options = JSON.parse(raw),
  touchEnabled = options.touch !== false;
let serial = 0;
await new Promise((r, j) => {
  ws.onopen = r;
  ws.onerror = j;
});
ws.onmessage = ({ data }) => {
  const m = JSON.parse(data),
    p = pending.get(m.id);
  if (p) {
    pending.delete(m.id);
    m.error ? p[1](m.error) : p[0](m.result);
  }
};
const call = (method, params = {}, sessionId) =>
  new Promise((r, j) => {
    const id = ++serial;
    pending.set(id, [r, j]);
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
      { expression, awaitPromise: true, returnByValue: true },
      sessionId,
    );
    assert.ok(!r.exceptionDetails, JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  await call(
    "Emulation.setTouchEmulationEnabled",
    { enabled: touchEnabled, maxTouchPoints: 1 },
    sessionId,
  );
  const frame = () =>
    evaluate('document.querySelector(".tt-scene canvas").__tableTennisFrame');
  const rect = await evaluate(
    'document.querySelector(".tt-scene canvas").getBoundingClientRect().toJSON()',
  );
  const touch = async (type, x, y) =>
    touchEnabled
      ? call(
          "Input.dispatchTouchEvent",
          {
            type,
            touchPoints:
              type === "touchEnd"
                ? []
                : [{ x, y, id: 0, radiusX: 4, radiusY: 4, force: 1 }],
          },
          sessionId,
        )
      : call(
          "Input.dispatchMouseEvent",
          {
            type: "mouseMoved",
            x: type === "touchEnd" ? 2 : x,
            y: type === "touchEnd" ? 2 : y,
            button: "none",
            buttons: 0,
          },
          sessionId,
        );
  let before;
  const until = Date.now() + 70000,
    warmups = [];
  while (Date.now() < until) {
    const f = await frame();
    if (f.inputEnabled && f.phase === "serve") {
      const x = rect.x + f.contact.x,
        y = rect.y + f.contact.y + 60;
      for (let i = 0; i <= 8; i++) {
        await touch(i ? "touchMove" : "touchStart", x, y - i * 15);
        await sleep(30);
      }
      await touch("touchEnd");
      warmups.push({ at: f.at, phase: f.phase });
      await sleep(200);
      continue;
    }
    if (
      f.inputEnabled &&
      f.phase === "rally" &&
      f.authorityFlight?.landingDepth >= 0.9 &&
      f.ball.world.z >= 1.8 &&
      f.ball.world.z <= 1.96
    ) {
      before = f;
      await touch("touchStart", rect.x + f.ball.x, rect.y + f.ball.y);
      if (!touchEnabled) {
        await sleep(10);
        await touch("touchMove", rect.x + f.ball.x, rect.y + f.ball.y - 12);
      }
      break;
    }
    await sleep(8);
  }
  assert.ok(before, "no natural long-ball opportunity");
  let after;
  for (let i = 0; i < 50; i++) {
    const room = await evaluate(
      'fetch("/api/state").then(r=>r.json()).then(s=>s.room)',
    );
    const hit = room.game.data.events.find(
      (e) => e.type === "hit" && e.seat === room.you && e.at >= before.at - 5,
    );
    if (hit) {
      after = { hit, view: room.game.data, seat: room.you };
      break;
    }
    await sleep(20);
  }
  await touch("touchEnd");
  assert.ok(after, "touch on near-court ball was not returned");
  assert.ok(Math.abs(after.hit.p.z) > 1.7, after.hit);
  assert.equal(after.hit.kind, touchEnabled ? "block" : "drive");
  console.log(
    JSON.stringify({
      passed: true,
      touch: touchEnabled,
      before,
      after,
      warmups,
    }),
  );
} finally {
  ws.close();
}
