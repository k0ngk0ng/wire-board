// Native input and visible Canvas observations in an agent-browser-owned tab.
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
const [endpoint, prefix, raw] = process.argv.slice(2),
  options = JSON.parse(raw);
const out = path.resolve(options.out);
if (path.relative(process.cwd(), out).startsWith(".."))
  throw Error("Output must stay in checkout");
mkdirSync(out, { recursive: true });
const socket = new WebSocket(endpoint);
await new Promise((r, j) => {
  socket.onopen = r;
  socket.onerror = j;
});
let serial = 0,
  sessionId,
  initScript;
const pending = new Map(),
  network = [];
const presented = [];
socket.onmessage = ({ data }) => {
  const m = JSON.parse(data);
  if (m.method === "Page.screencastFrame") {
    presented.push({
      at: m.params.metadata.timestamp * 1000,
      sha256: createHash("sha256").update(m.params.data).digest("hex"),
    });
    void call(
      "Page.screencastFrameAck",
      { sessionId: m.params.sessionId },
      sessionId,
    ).catch(() => {});
  }
  if (m.method === "Network.webSocketClosed")
    network.push({
      direction: "closed",
      at: Date.now(),
      bytes: 0,
      packet: { type: "closed" },
    });
  if (
    m.method === "Network.webSocketFrameReceived" ||
    m.method === "Network.webSocketFrameSent"
  ) {
    try {
      network.push({
        direction: m.method.endsWith("Received") ? "received" : "sent",
        at: Date.now(),
        bytes: Buffer.byteLength(m.params.response.payloadData),
        packet: JSON.parse(m.params.response.payloadData),
      });
    } catch {}
  }
  if (!m.id) return;
  const p = pending.get(m.id);
  pending.delete(m.id);
  m.error ? p.j(Error(JSON.stringify(m.error))) : p.r(m.result);
};
const call = (method, params = {}, session) =>
  new Promise((r, j) => {
    const id = ++serial;
    pending.set(id, { r, j });
    socket.send(JSON.stringify({ id, method, params, sessionId: session }));
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
const delay = (ms) => new Promise((r) => setTimeout(r, ms));
try {
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(
    (t) => t.type === "page" && t.url.startsWith(prefix),
  );
  if (!target) throw Error("Owned page missing");
  ({ sessionId } = await call("Target.attachToTarget", {
    targetId: target.targetId,
    flatten: true,
  }));
  await call("Network.enable", {}, sessionId);
  await call("Page.enable", {}, sessionId);
  const oldOrigin = await evaluate("performance.timeOrigin");
  const source = `(()=>{window.__escapeNative=[];let watching=false;const watch=(e)=>{if(!e.isTrusted||watching)return;const cv=document.querySelector('[data-escape-canvas]'),f=cv?.__escapeFrame;if(!f)return;watching=true;const entry={nativeAt:e.timeStamp,type:e.type,before:{...f},trusted:e.isTrusted};window.__escapeNative.push(entry);const end=performance.now()+1500;const step=()=>{const n=cv.__escapeFrame;if(n&&Math.hypot(n.x-f.x,n.y-f.y)>.08){entry.first={...n};entry.latencyMs=n.at-e.timeStamp;watching=false;return}if(performance.now()<end)requestAnimationFrame(step);else{entry.failed=true;watching=false}};requestAnimationFrame(step)};window.addEventListener('keydown',e=>{if(['ArrowRight','ArrowLeft'].includes(e.code))watch(e)},true);window.addEventListener('pointerdown',e=>{if(e.target.closest?.('.escape-direction-pad'))watch(e)},true);${options.mode === "gamepad" ? `window.__escapePad={id:'Escape acceptance virtual standard',index:0,connected:true,mapping:'standard',timestamp:performance.now(),axes:[0,0,0,0],buttons:Array.from({length:17},()=>({pressed:false,touched:false,value:0}))};Object.defineProperty(navigator,'getGamepads',{configurable:true,value:()=>[window.__escapePad]});` : ""}})()`;
  ({ identifier: initScript } = await call(
    "Page.addScriptToEvaluateOnNewDocument",
    { source },
    sessionId,
  ));
  await call("Page.reload", {}, sessionId);
  const until = Date.now() + 25000;
  while (
    !(await evaluate(
      `performance.timeOrigin!==${oldOrigin} && Array.isArray(window.__escapeNative) && !!document.querySelector('[data-escape-board][data-ready="true"]')`,
    ))
  ) {
    if (Date.now() > until) throw Error("Production room did not become ready");
    await delay(100);
  }
  await evaluate(
    `document.querySelector('[data-escape-canvas]').scrollIntoView({block:'center'});document.querySelector('[data-escape-canvas]').focus({preventScroll:true})`,
  );
  await delay(600);
  const capture = async (name) => {
    const { data } = await call(
      "Page.captureScreenshot",
      { format: "png" },
      sessionId,
    );
    writeFileSync(path.join(out, name + ".png"), Buffer.from(data, "base64"));
  };
  await capture("before");
  await call(
    "Page.startScreencast",
    {
      format: "jpeg",
      quality: 25,
      maxWidth: 800,
      maxHeight: 600,
      everyNthFrame: 1,
    },
    sessionId,
  );
  if (options.mode === "touch")
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: true, maxTouchPoints: 5 },
      sessionId,
    );
  const native = options.mode !== "observe" && options.mode !== "gamepad";
  let virtualGamepad;
  if (options.mode === "gamepad") {
    const before = await evaluate(
      "document.querySelector('[data-escape-canvas]').__escapeFrame",
    );
    await evaluate(
      `(async()=>{for(let i=0;i<90;i++){window.__escapePad.axes[0]=.4+(i%11)*.045;window.__escapePad.timestamp=performance.now();await new Promise(r=>setTimeout(r,20))}})()`,
    );
    await capture("after");
    await evaluate(
      "window.__escapePad.axes[0]=0;window.__escapePad.timestamp=performance.now()",
    );
    await delay(800);
    virtualGamepad = {
      kind: "virtual Gamepad API, no physical hardware",
      before,
      after: await evaluate(
        "document.querySelector('[data-escape-canvas]').__escapeFrame",
      ),
    };
  }
  let reconnect;
  if (options.mode === "reconnect") {
    const key = (type, code) =>
      call(
        "Input.dispatchKeyEvent",
        {
          type,
          key: code,
          code,
          windowsVirtualKeyCode: code === "ArrowRight" ? 39 : 37,
          nativeVirtualKeyCode: code === "ArrowRight" ? 39 : 37,
        },
        sessionId,
      );
    await key("keyDown", "ArrowRight");
    await delay(120);
    await capture("after");
    const cutAt = Date.now();
    const fault = await evaluate(
      `fetch('/_test/disconnect',{method:'POST'}).then(r=>r.json())`,
    );
    const until = Date.now() + 10000;
    let bootstrap;
    while (!bootstrap) {
      bootstrap = network.find(
        (e) => e.at > cutAt && e.packet.type === "state",
      );
      if (!bootstrap && Date.now() > until)
        throw Error("Connection did not recover with an authoritative state");
      if (!bootstrap) await delay(30);
    }
    await delay(700);
    const quietUntil = Date.now();
    await key("keyUp", "ArrowRight");
    await delay(80);
    const freshAt = Date.now();
    await key("keyDown", "ArrowLeft");
    await delay(140);
    await key("keyUp", "ArrowLeft");
    reconnect = {
      cutAt,
      closed: fault.closed,
      bootstrapAt: bootstrap.at,
      quietUntil,
      freshAt,
    };
  }
  for (
    let i = 0;
    i < (native && options.mode !== "reconnect" ? (options.samples ?? 5) : 0);
    i++
  ) {
    const right = i % 2 === 0,
      code = right ? "ArrowRight" : "ArrowLeft",
      virtual = right ? 39 : 37;
    if (options.mode === "touch") {
      const p = await evaluate(
        `(()=>{const e=document.querySelector('[data-escape-board] button[aria-label="向${right ? "右" : "左"}移动"]');e.scrollIntoView({block:'nearest'});const r=e.getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()`,
      );
      await call(
        "Input.dispatchTouchEvent",
        {
          type: "touchStart",
          touchPoints: [{ ...p, id: 1, radiusX: 3, radiusY: 3, force: 1 }],
        },
        sessionId,
      );
      if (i === 0) {
        await delay(60);
        await capture("after");
        await delay(60);
      } else await delay(120);
      await call(
        "Input.dispatchTouchEvent",
        { type: i === 4 ? "touchCancel" : "touchEnd", touchPoints: [] },
        sessionId,
      );
    } else {
      await call(
        "Input.dispatchKeyEvent",
        {
          type: "keyDown",
          key: code,
          code,
          windowsVirtualKeyCode: virtual,
          nativeVirtualKeyCode: virtual,
        },
        sessionId,
      );
      if (i === 0) {
        await delay(60);
        await capture("after");
        await delay(60);
      } else await delay(120);
      await call(
        "Input.dispatchKeyEvent",
        {
          type: "keyUp",
          key: code,
          code,
          windowsVirtualKeyCode: virtual,
          nativeVirtualKeyCode: virtual,
        },
        sessionId,
      );
    }
    await delay(350);
  }
  await delay(options.mode === "observe" ? (options.observeMs ?? 5000) : 1100);
  const metrics = await evaluate(
    `({native:window.__escapeNative,frame:document.querySelector('[data-escape-canvas]').__escapeFrame,overflow:document.documentElement.scrollWidth>innerWidth+1,timeOrigin:performance.timeOrigin})`,
  );
  await call("Page.stopScreencast", {}, sessionId);
  const displayed = presented.filter(
    (f, i, a) => i > 0 && i < a.length - 1 && f.at > a[0].at + 200,
  );
  const displayedGaps = displayed
    .slice(1)
    .map((f, i) => f.at - displayed[i].at)
    .filter((n) => n > 0);
  for (const event of metrics.native ?? []) {
    if (event.first) {
      const frame = presented.find(
        (f) => f.at >= metrics.timeOrigin + event.first.at,
      );
      if (frame)
        event.presentedLatencyMs =
          frame.at - (metrics.timeOrigin + event.nativeAt);
    }
  }
  const inputs = network.filter(
    (e) => e.direction === "sent" && e.packet.type === "input",
  );
  const acks = network.filter(
    (e) => e.direction === "received" && e.packet.type === "ack",
  );
  const frames = network.filter(
    (e) => e.direction === "received" && e.packet.type === "game",
  );
  const sizes = frames.map((e) => e.bytes).sort((a, b) => a - b);
  const result = {
    ...metrics,
    presentation: {
      frames: displayed.length,
      fps:
        displayed.length > 1
          ? ((displayed.length - 1) * 1000) /
            (displayed.at(-1).at - displayed[0].at)
          : 0,
      gaps: displayedGaps,
      distinct: new Set(displayed.map((f) => f.sha256)).size,
      firstFrames: displayed.slice(0, 3),
    },
    inputPackets: inputs.length,
    reconnect,
    virtualGamepad,
    uiError: await evaluate(
      "document.querySelector('.escape-error')?.textContent??null",
    ),
    connectionClosures: network.filter((e) => e.packet.type === "closed")
      .length,
    serverProgress: frames.length
      ? {
          from: frames[0].packet.game.data.tick,
          to: frames.at(-1).packet.game.data.tick,
        }
      : null,
    ackMs: inputs.map((e) => {
      const ack = acks.find((a) => a.packet.nonce === e.packet.nonce);
      return ack ? ack.at - e.at : null;
    }),
    frameBytes: {
      count: sizes.length,
      median: sizes[Math.floor(sizes.length / 2)],
      p95: sizes[Math.floor(sizes.length * 0.95)],
    },
    network: network.filter((e) => e.packet.type === "network").slice(-2),
    protocolSamples: frames.slice(0, 4),
    inputs,
  };
  writeFileSync(
    path.join(out, "result.json"),
    JSON.stringify(result, null, 2) + "\n",
  );
  console.log(JSON.stringify(result));
} finally {
  if (sessionId) {
    if (initScript)
      await call(
        "Page.removeScriptToEvaluateOnNewDocument",
        { identifier: initScript },
        sessionId,
      );
    if (options.mode === "touch")
      await call(
        "Emulation.setTouchEmulationEnabled",
        { enabled: false },
        sessionId,
      );
    await call("Target.detachFromTarget", { sessionId });
  }
  socket.close();
}
