import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, writeFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const base = process.argv[2];
assert.match(base ?? "", /^http:\/\/127\.0\.0\.1:\d+\/?$/);
const temporary = path.join(
  root,
  ".local/yuyuhakusho/recovery-loop/browser-temporary",
);
for (const name of ["profile", "config", "data", "cache"])
  await mkdir(path.join(temporary, name), { recursive: true });
const env = {
  ...process.env,
  TMPDIR: temporary,
  XDG_CONFIG_HOME: path.join(temporary, "config"),
  XDG_DATA_HOME: path.join(temporary, "data"),
  XDG_CACHE_HOME: path.join(temporary, "cache"),
  AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/yuyuhakusho/s"),
  AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
  AGENT_BROWSER_NO_VERSION_CHECK: "1",
};
const execute = promisify(execFile);
const session = (
  await execute(
    "agent-browser",
    ["session", "id", "--scope", "worktree", "--prefix", "recovery"],
    { cwd: root, env },
  )
).stdout.trim();
const browser = async (...args) => {
  const { stdout } = await execute(
    "agent-browser",
    ["--session", session, ...args, "--json"],
    { cwd: root, env, timeout: 30000 },
  );
  const value = JSON.parse(stdout);
  assert.ok(value.success, stdout);
  return value.data;
};
await browser("--profile", path.join(temporary, "profile"), "open", base);
const endpoint = await browser("get", "cdp-url");
const socket = new WebSocket(endpoint.cdpUrl);
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});
let serial = 0;
const pending = new Map();
socket.onmessage = ({ data }) => {
  const value = JSON.parse(data);
  const request = pending.get(value.id);
  if (!request) return;
  pending.delete(value.id);
  value.error
    ? request.reject(Error(JSON.stringify(value.error)))
    : request.resolve(value.result);
};
const call = (method, params = {}, sessionId) =>
  new Promise((resolve, reject) => {
    const id = ++serial;
    pending.set(id, { resolve, reject });
    socket.send(JSON.stringify({ id, method, params, sessionId }));
  });
const { targetInfos } = await call("Target.getTargets");
const { sessionId } = await call("Target.attachToTarget", {
  targetId: targetInfos.find(
    (value) => value.type === "page" && value.url.startsWith(base),
  ).targetId,
  flatten: true,
});
const evaluate = async (expression) => {
  const value = await call(
    "Runtime.evaluate",
    { expression, returnByValue: true, awaitPromise: true },
    sessionId,
  );
  assert.ok(!value.exceptionDetails, JSON.stringify(value.exceptionDetails));
  return value.result.value;
};
const api = async (suffix, body) =>
  evaluate(
    `(async()=>{const response=await fetch('/api'+${JSON.stringify(suffix)},{method:${body ? '"POST"' : '"GET"'},headers:{'Content-Type':'application/json'},body:${body ? JSON.stringify(JSON.stringify(body)) : "undefined"}});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value})()`,
  );
const command = async (type) => {
  const state = await api("/state");
  return api("/rooms/" + state.room.id, {
    type,
    version: state.room.version,
    nonce: crypto.randomUUID(),
  });
};
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const wait = async (name, predicate, timeout = 90000) => {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const value = await predicate();
    if (value) return value;
    await delay(100);
  }
  throw Error("Timed out: " + name);
};
const out = path.join(root, ".local/yuyuhakusho/recovery-loop/results");

await call("Page.enable", {}, sessionId);
const before =
  "(()=>{if(window.recoveryTrace)return;const log=window.recoveryTrace={phase:'setup',recoveries:[],checks:[],frames:[],calls:[]};const descriptor=Object.getOwnPropertyDescriptor(WebSocket.prototype,'onmessage');Object.defineProperty(WebSocket.prototype,'onmessage',{...descriptor,set(fn){descriptor.set.call(this,typeof fn!=='function'?fn:function(event){if(log.phase!=='setup'&&JSON.parse(event.data).type==='game'){const queue=this.recoveryQueue??=[];queue.push(event);if(!this.recoveryTimer)this.recoveryTimer=setTimeout(()=>{this.recoveryTimer=0;for(const next of queue.splice(0))fn.call(this,next)},150)}else setTimeout(()=>fn.call(this,event),60)})}});const send=WebSocket.prototype.send;WebSocket.prototype.send=function(data){setTimeout(()=>{if(this.readyState===1)send.call(this,data)},60)};let last=0;const tick=now=>{const b=document.querySelector('[data-yuyu-board]');if(b)log.frames.push({phase:log.phase,now,span:now-last,frame:b.dataset.frame,connection:b.dataset.connection});last=now;requestAnimationFrame(tick)};requestAnimationFrame(tick)})()";
await call(
  "Page.addScriptToEvaluateOnNewDocument",
  { source: before },
  sessionId,
);
await browser("reload");
const report = {
  passed: false,
  cases: [],
  environment: {
    browser: "Chromium via agent-browser",
    nativeTouch: true,
    viewport: [951, 585],
    cpuSlowdown: Number(process.env.CPU ?? 2),
    arena: Number(process.env.ARENA ?? 10),
    websocketDelayPerDirectionMS: 60,
    gameDeliveryBatchMS: 150,
    measuredRTT: false,
  },
};
try {
  await api("/register", {
    name: "Recovery" + Date.now().toString().slice(-8),
    password: "local-test-only",
    invite: "gamepad-browser",
  });
  await api("/rooms", {
    kind: "yuyuhakusho",
    capacity: 4,
    options: {
      arena: Number(process.env.ARENA ?? 10),
      characters: [3, 10, 10, 10],
      teamBattle: false,
    },
    name: "Recovery diagnosis",
  });
  for (let i = 0; i < 3; i++) await command("add_bot");
  await call(
    "Emulation.setDeviceMetricsOverride",
    { width: 951, height: 585, deviceScaleFactor: 3, mobile: true },
    sessionId,
  );
  await call(
    "Emulation.setTouchEmulationEnabled",
    { enabled: true, maxTouchPoints: 5 },
    sessionId,
  );
  await browser("reload");
  await evaluate(before);
  await evaluate(
    `(async()=>{const {NetworkClient}=await import('/@fs'+${JSON.stringify(root)}+'games/yuyuhakusho/web/network-client.ts');const {NetworkScene}=await import('/@fs'+${JSON.stringify(root)}+'games/yuyuhakusho/web/network-scene.ts');const initialize=NetworkClient.prototype.initialize;NetworkClient.prototype.initialize=async function(...args){window.recoveryClient=this;return initialize.apply(this,args)};const recover=NetworkClient.prototype.recover;NetworkClient.prototype.recover=function(reason){if(!this.recovering&&performance.now()>=this.retryAt)recoveryTrace.recoveries.push({at:performance.now(),phase:recoveryTrace.phase,reason:String(reason),frame:this.view?.frame,confirmed:String(this.scene?.confirmedFrame),predicted:String(this.scene?.predictedFrame),queue:this.transport.queue.length,history:this.transport.history.length});return recover.call(this,reason)};const check=NetworkScene.prototype.check;NetworkScene.prototype.check=function(reply,view){try{return check.call(this,reply,view)}catch(error){recoveryTrace.checks.push({phase:recoveryTrace.phase,actual:{frame:reply.view?.frame,rules:reply.rulesDigest,presentation:reply.presentationDigest},expected:{frame:view.frame,rules:view.digest,presentation:view.presentationDigest},replay:view.replay});throw error}};return true})()`,
  );
  await browser(
    "wait",
    "--fn",
    "[...document.querySelectorAll('button')].some(b=>b.textContent.trim()==='我准备好了')",
  );
  await browser(
    "find",
    "role",
    "button",
    "click",
    "--name",
    "我准备好了",
    "--exact",
  );
  await browser(
    "wait",
    "--fn",
    "[...document.querySelectorAll('button')].some(b=>b.textContent.trim()==='开始游戏'&&!b.disabled)",
  );
  await browser(
    "find",
    "role",
    "button",
    "click",
    "--name",
    "开始游戏",
    "--exact",
  );
  await wait(
    "runtime",
    async () =>
      await evaluate(
        "!!document.querySelector('[data-controller-button=a]:not(:disabled)')",
      ),
  );
  console.log("ready");
  await delay(1500);
  console.log("client", await evaluate("!!window.recoveryClient"));
  await call(
    "Emulation.setCPUThrottlingRate",
    { rate: Number(process.env.CPU ?? 2) },
    sessionId,
  );
  await evaluate(
    `(()=>{recoveryTrace.operations={};const original=Go.prototype._resume;Go.prototype._resume=function(...args){const command=this._pendingEvent?.args?.[0];const op=typeof command==='string'?JSON.parse(command).op:'timer';const start=performance.now();try{return original.apply(this,args)}finally{const ms=performance.now()-start;const phase=recoveryTrace.phase;const bucket=recoveryTrace.operations[phase]??={};const entry=bucket[op]??={count:0,ms:0,max:0};entry.count++;entry.ms+=ms;entry.max=Math.max(entry.max,ms)}};return true})()`,
  );

  await evaluate(
    `recoveryTrace.touches=[];for(const type of ['touchstart','touchend','pointerdown'])document.addEventListener(type,e=>recoveryTrace.touches.push({type,trusted:e.isTrusted,target:e.target?.dataset?.controllerButton||e.target?.className,defaultPrevented:e.defaultPrevented}),true);true`,
  );
  report.geometry = await evaluate(
    `({width:innerWidth,height:innerHeight,screen:{width:screen.width,height:screen.height},buttons:[...document.querySelectorAll('[data-controller-button]')].map(b=>({id:b.dataset.controllerButton,rect:b.getBoundingClientRect().toJSON()}))})`,
  );

  const touch = async (type) => {
    const p = await evaluate(
      "(()=>{const r=document.querySelector('[data-controller-button=a]').getBoundingClientRect();return {id:1,x:r.x+r.width/2,y:r.y+r.height/2}})()",
    );
    return call(
      "Input.dispatchTouchEvent",
      { type, touchPoints: type === "touchEnd" ? [] : [p] },
      sessionId,
    );
  };
  for (const kind of ["idle", "held", "rapid-multi"]) {
    await evaluate(`recoveryTrace.phase=${JSON.stringify(kind)};true`);
    const startRoom = (await api("/state")).room;
    if (kind === "rapid-multi") {
      const positions = await evaluate(
        `['up','a','b','c','ac'].map((b,i)=>{const r=document.querySelector('[data-controller-button='+b+']').getBoundingClientRect();return {id:i+1,x:r.x+r.width/2,y:r.y+r.height/2}})`,
      );
      for (let i = 0; i < 80; i++) {
        await call(
          "Input.dispatchTouchEvent",
          { type: "touchStart", touchPoints: positions },
          sessionId,
        );
        await delay(25);
        await call(
          "Input.dispatchTouchEvent",
          { type: "touchEnd", touchPoints: [] },
          sessionId,
        );
        await delay(35);
      }
    } else if (kind === "idle") await delay(5000);
    else if (kind === "held") {
      await touch("touchStart");
      await delay(5000);
      await touch("touchEnd");
    } else if (kind === "multi-rapid") {
      const points = await evaluate(
        `['up','a','b','c','ac','abc'].map((b,i)=>{const r=document.querySelector('[data-controller-button='+b+']').getBoundingClientRect();return {id:i+1,x:r.x+r.width/2,y:r.y+r.height/2}})`,
      );
      for (let i = 0; i < 100; i++) {
        await call(
          "Input.dispatchTouchEvent",
          { type: "touchStart", touchPoints: points },
          sessionId,
        );
        await delay(25);
        await call(
          "Input.dispatchTouchEvent",
          { type: "touchEnd", touchPoints: [] },
          sessionId,
        );
        await delay(35);
      }
    } else
      for (let i = 0; i < 70; i++) {
        await touch("touchStart");
        await delay(30);
        await touch("touchEnd");
        await delay(60);
      }
    const summary = await evaluate(
      `(()=>{const f=recoveryTrace.frames.filter(v=>v.phase===${JSON.stringify(kind)});return {frames:f.length,fps:1000*(f.length-1)/(f.at(-1).now-f[0].now),maxGap:Math.max(...f.map(v=>v.span)),visibleFrames:new Set(f.map(v=>v.frame)).size,recoveries:recoveryTrace.recoveries.filter(v=>v.phase===${JSON.stringify(kind)}),checks:recoveryTrace.checks.filter(v=>v.phase===${JSON.stringify(kind)})}})()`,
    );
    const endRoom = (await api("/state")).room;
    summary.appliedSequences =
      endRoom.game.data.inputs[0].appliedSequence -
      startRoom.game.data.inputs[0].appliedSequence;
    summary.nativeTouches = await evaluate("recoveryTrace.touches.length");
    summary.operations = await evaluate(
      "recoveryTrace.operations[recoveryTrace.phase]",
    );
    report.cases.push({ kind, ...summary });
    console.log(JSON.stringify(report.cases.at(-1)));
  }
  await call("Emulation.setCPUThrottlingRate", { rate: 1 }, sessionId);
  report.reconciliation = await evaluate(`(async()=>{
    const client=window.recoveryClient;
    const source=client.scene;
    const calls=[];
    const wasm={load:source.wasm.load.bind(source.wasm),request(command,...buffers){calls.push(command.op);return source.wasm.request(command,...buffers)}};
    const scene=new source.constructor(wasm,source.asset,0);
    try {
      const first=await client.options.loadCheckpoint();
      scene.restore(first.data.checkpoint,first.data.view);
      scene.sample(0,undefined);
      const period=1000/59.92274340431231;
      scene.sample(period*12,undefined);
      const previous=scene.frame;
      await new Promise(resolve=>setTimeout(resolve,20));
      const next=await client.options.loadCheckpoint();
      if(BigInt(next.data.view.frame)>=previous)throw Error('Authority overtook render fixture');
      scene.accept(next.data.view,[]);
      calls.length=0;
      scene.sample(period*13,undefined);
      const result={calls:[...calls],draws:calls.filter(op=>op==='step').length,rendered:scene.renderedFrame,target:String(scene.frame),authority:next.data.view.frame};
      if(result.rendered!==result.target)throw Error('Reconciled final pixels were not rendered');
      return result;
    }finally{scene.dropSessions()}
  })()`);
  assert.equal(
    report.reconciliation.draws,
    1,
    "Correction constructed a historical image that the same RAF immediately replaces",
  );
  const geometry = () =>
    evaluate(
      `Object.fromEntries(['.yuyu-board','canvas','.yuyu-controller'].map(s=>[s,document.querySelector(s).getBoundingClientRect().toJSON()]))`,
    );
  report.beforeRecovery = await geometry();
  await evaluate(
    `(()=>{const c=window.recoveryClient;const original=c.options.loadCheckpoint;c.options.loadCheckpoint=async()=>{await new Promise(r=>setTimeout(r,1500));return original()};void c.recover(Error('E2E temporary disconnection'));return true})()`,
  );
  await wait("recovery status", () =>
    evaluate("!!document.querySelector('.yuyu-status')"),
  );
  report.duringRecovery = await geometry();
  report.status = await evaluate(
    "({text:document.querySelector('.yuyu-status').textContent,rect:document.querySelector('.yuyu-status').getBoundingClientRect().toJSON(),position:getComputedStyle(document.querySelector('.yuyu-status')).position})",
  );
  await browser(
    "screenshot",
    path.join(out, "recovery-" + (process.env.RUN ?? "run") + ".png"),
  );
  await wait("connection restored", () =>
    evaluate(
      `document.querySelector('[data-yuyu-board]')?.dataset.connection==='connected'`,
    ),
  );
  assert.equal(
    report.beforeRecovery.canvas.height,
    report.duringRecovery.canvas.height,
    "Recovery status reduced canvas height",
  );
  assert.deepEqual(
    report.beforeRecovery,
    report.duringRecovery,
    "Recovery moved game or controller",
  );
  assert.equal(
    report.status.position,
    "absolute",
    "Recovery status reserves a page row",
  );
  const rapid = report.cases.find((value) => value.kind === "rapid-multi");
  assert.ok(
    rapid.nativeTouches >= 1000,
    "Native multi-touch edges were not dispatched",
  );
  assert.ok(
    rapid.appliedSequences >= 100,
    "Server did not apply rapid input edges",
  );
  assert.deepEqual(
    rapid.recoveries,
    [],
    "Continuous game messages lost their replay history",
  );
  report.passed = true;
} catch (error) {
  report.error = String(error);
  process.exitCode = 1;
  console.error(error);
  report.text = await evaluate("document.body.innerText").catch(() => null);
} finally {
  report.trace = await evaluate("recoveryTrace").catch(() => null);
  await writeFile(
    path.join(out, "browser-" + (process.env.RUN ?? "run") + ".json"),
    JSON.stringify(report, null, 2),
  );
  await browser("screenshot", path.join(out, "diagnosis.png")).catch(() => {});
  socket.close();
  await browser("close").catch(() => {});
  await rm(temporary, { recursive: true, force: true });
}
