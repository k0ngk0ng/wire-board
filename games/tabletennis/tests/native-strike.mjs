// Native CDP input into agent-browser's owned real application page. Public
// HTTP snapshots supply timing only; no callbacks or game state are fabricated.
const [wsURL, pageURL, raw] = process.argv.slice(2),
  options = JSON.parse(raw),
  ws = new WebSocket(wsURL);
let id = 0;
const pending = new Map();
await new Promise((r, j) => {
  ws.onopen = r;
  ws.onerror = j;
});
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.id) {
    const p = pending.get(m.id);
    pending.delete(m.id);
    m.error ? p.reject(m.error) : p.resolve(m.result);
  }
};
const call = (method, params = {}, sessionId) =>
  new Promise((resolve, reject) => {
    const key = ++id;
    pending.set(key, { resolve, reject });
    ws.send(JSON.stringify({ id: key, method, params, sessionId }));
  });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(
    (x) => x.type === "page" && x.url.startsWith(pageURL),
  );
  if (!target) throw Error("owned page missing");
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
    if (r.exceptionDetails) throw Error(JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  if (options.latency)
    await call(
      "Network.emulateNetworkConditions",
      {
        offline: false,
        latency: options.latency,
        downloadThroughput: -1,
        uploadThroughput: -1,
      },
      sessionId,
    );
  if (options.touch)
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: true, maxTouchPoints: 2 },
      sessionId,
    );
  let state,
    before,
    targetX,
    targetY,
    directionKey = null;
  const directionKeys = {
    KeyA: ["a", 65],
    KeyD: ["d", 68],
    KeyW: ["w", 87],
    KeyS: ["s", 83],
  };
  const setDirection = async (next) => {
    if (directionKey === next) return;
    if (directionKey)
      await call(
        "Input.dispatchKeyEvent",
        {
          type: "keyUp",
          key: directionKeys[directionKey][0],
          code: directionKey,
          windowsVirtualKeyCode: directionKeys[directionKey][1],
        },
        sessionId,
      );
    directionKey = next;
    if (next)
      await call(
        "Input.dispatchKeyEvent",
        {
          type: "rawKeyDown",
          key: directionKeys[next][0],
          code: next,
          windowsVirtualKeyCode: directionKeys[next][1],
        },
        sessionId,
      );
  };
  const until = Date.now() + 25000;
  while (Date.now() < until) {
    const visual = await evaluate(
      'document.querySelector(".tt-scene canvas").__tableTennisFrame',
    );
    const now = visual.at;
    let aligned = true;
    if (options.keyboard && visual.inputEnabled) {
      const tolerance = 35;
      const difference = (visual.contact?.x ?? visual.ball.x) - visual.racket.x;
      const vertical = (visual.contact?.y ?? visual.ball.y) - visual.racket.y;
      aligned = Math.abs(difference) < tolerance && Math.abs(vertical) < 20;
      await setDirection(
        Math.abs(difference) >= tolerance
          ? difference < 0
            ? "KeyA"
            : "KeyD"
          : Math.abs(vertical) >= 20
            ? vertical < 0
              ? "KeyW"
              : "KeyS"
            : null,
      );
    }
    if (
      visual.inputEnabled &&
      aligned &&
      (visual.phase === "serve" ||
        (!options.serveOnly &&
          now >= visual.controls.openAt + 190 &&
          now < visual.controls.closeAt - 100))
    ) {
      targetX = visual.contact?.x ?? visual.ball.x;
      targetY = visual.contact?.y ?? visual.ball.y;
      before = {
        rally: visual.rally,
        shot: visual.shot,
        seq: visual.controls.seq,
        phase: visual.phase,
        now,
      };
      break;
    }
    await sleep(15);
  }
  await setDirection(null);
  if (!before) throw Error("no own strike window");
  const rect = await evaluate(
    'document.querySelector(".tt-scene canvas").getBoundingClientRect().toJSON()',
  );
  const dy = options.kind === "chop" ? 65 : -65;
  const x =
      rect.x +
      Math.max(16, Math.min(rect.width - 16, targetX ?? rect.width * 0.5)),
    y =
      rect.y +
      Math.max(
        40,
        Math.min(rect.height - 40, (targetY ?? rect.height * 0.63) - dy / 2),
      );
  const trace = [];
  await evaluate(
    'window.__ttSamples=[];window.__ttSampling=true;(function sample(t){if(!window.__ttSampling||window.__ttSamples.length>=200)return;window.__ttSamples.push({time:t,wall:Date.now(),...document.querySelector(".tt-scene canvas").__tableTennisFrame});requestAnimationFrame(sample)})(performance.now())',
  );
  const inputBegan = Date.now();
  if (options.keyboard) {
    const key = options.kind === "chop" ? "j" : "k",
      code = options.kind === "chop" ? "KeyJ" : "KeyK",
      virtual = options.kind === "chop" ? 74 : 75;
    await call(
      "Input.dispatchKeyEvent",
      {
        type: "rawKeyDown",
        key,
        code,
        windowsVirtualKeyCode: virtual,
        nativeVirtualKeyCode: virtual,
      },
      sessionId,
    );
    await sleep(40);
    await call(
      "Input.dispatchKeyEvent",
      {
        type: "keyUp",
        key,
        code,
        windowsVirtualKeyCode: virtual,
        nativeVirtualKeyCode: virtual,
      },
      sessionId,
    );
    trace.push({ key, at: Date.now() });
  } else {
    for (let i = 0; i <= 6; i++) {
      const point = {
        x: x + ((options.dx ?? 10) * i) / 6,
        y: y + (dy * i) / 6,
      };
      if (options.touch) {
        await call(
          "Input.dispatchTouchEvent",
          {
            type: i === 0 ? "touchStart" : "touchMove",
            touchPoints: [
              { ...point, radiusX: 4, radiusY: 4, force: 1, id: 0 },
            ],
          },
          sessionId,
        );
      } else {
        await call(
          "Input.dispatchMouseEvent",
          {
            type: i === 0 ? "mouseMoved" : "mouseMoved",
            ...point,
            button: "none",
            buttons: 0,
          },
          sessionId,
        );
      }
      trace.push({ ...point, at: Date.now() });
      await sleep(options.stepDelay ?? 10);
    }
  }
  const releaseAt = Date.now();
  let measuredRTT = 0;
  let after, actingSeat;
  for (let i = 0; i < 70; i++) {
    const requestBegan = Date.now();
    const s = await evaluate('fetch("/api/state").then(r=>r.json())');
    measuredRTT = Date.now() - requestBegan;
    if (s.room.game.data.controls.seq > before.seq) {
      after = s.room.game.data;
      actingSeat = s.room.you;
      const paddle = after.paddles[s.room.you];
      if (paddle.result === "miss")
        throw Error(
          "native racket missed the ball: " +
            JSON.stringify({ before, paddle, trace }),
        );
      break;
    }
    await sleep(20);
  }
  if (!after)
    throw Error(
      "native swing was not accepted: " +
        JSON.stringify(await evaluate("document.body.innerText")),
    );
  if (options.touch)
    await call(
      "Input.dispatchTouchEvent",
      { type: "touchEnd", touchPoints: [] },
      sessionId,
    );
  await sleep(200);
  const frames = await evaluate("window.__ttSampling=false;window.__ttSamples");
  const launch =
    frames
      .map((f) => f.authorityFlight)
      .find(
        (f) => f?.lastHitter === actingSeat && f.shot === before.shot + 1,
      ) ?? null;
  const hitFrame = frames.find(
    (f) =>
      f.swingAge !== null &&
      f.swingAge >= 0 &&
      f.swingAge <= f.wall - inputBegan + 20 &&
      f.wall >= inputBegan,
  );
  if (!hitFrame?.stroke || hitFrame.stroke.kind !== options.kind)
    throw Error(
      "wrong live stroke: " +
        JSON.stringify({
          expected: options.kind,
          actual: hitFrame?.stroke,
          before,
        }),
    );
  const firstFeedback = frames.find(
    (f) =>
      f.wall >= inputBegan &&
      (f.gestureDown ||
        (f.swingAge !== null &&
          f.swingAge >= 0 &&
          f.swingAge <= f.wall - inputBegan + 20)),
  );
  const receipt = frames.find((f) => f.controls.seq > before.seq);
  if (!firstFeedback)
    throw Error("native input produced no local paddle feedback");
  const feedbackMS = firstFeedback.wall - inputBegan;
  if (feedbackMS > 120)
    throw Error("local paddle feedback exceeded 120ms: " + feedbackMS);
  console.log(
    JSON.stringify({
      passed: true,
      launch,
      noMouseButtons: !options.keyboard && !options.touch,
      acceptedBeforeTouchEnd: !!options.touch,
      measuredRTT,
      inputBegan,
      feedbackMS,
      stroke: hitFrame.stroke,
      swingDuration: hitFrame.swingDuration,
      feedbackBeforeReceipt: !!receipt && firstFeedback.wall < receipt.wall,
      releaseAt,
      before,
      after: {
        rally: after.rally,
        shot: after.shot,
        seq: after.controls.seq,
        phase: after.phase,
      },
      inputs: trace,
      frames,
    }),
  );
} finally {
  ws.close();
}
