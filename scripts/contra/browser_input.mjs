// Native keyboard/mouse/multitouch on the acceptance-owned browser target.
const [endpoint, origin, raw] = process.argv.slice(2),
  options = JSON.parse(raw);
const ws = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  ws.onopen = resolve;
  ws.onerror = reject;
});
let sequence = 0,
  sessionId;
const pending = new Map();
const network = [];
const gamePackets = [];
let receivedBytes = 0,
  receivedFrames = 0;
ws.onmessage = ({ data }) => {
  const v = JSON.parse(data),
    p = pending.get(v.id);
  if (v.method === "Network.webSocketFrameReceived") {
    const raw = v.params.response.payloadData;
    receivedBytes += new TextEncoder().encode(raw).length;
    receivedFrames++;
    try {
      const packet = JSON.parse(raw);
      if (packet.type === "ack") network.push(v);
      if (packet.type === "game") {
        const view = packet.game?.data;
        gamePackets.push({
          at: v.params.timestamp,
          bytes: new TextEncoder().encode(raw).length,
          protocol: view?.protocol,
          revision: view?.revision,
          frames: view?.frames?.length,
          presentation: !!view?.snapshot?.presentation,
          audio: !!view?.snapshot?.audio,
          staticStages: !!view?.bundle?.stages,
        });
      }
    } catch {}
  } else if (v.method === "Network.webSocketFrameSent") network.push(v);
  if (!p) return;
  pending.delete(v.id);
  if (v.error) p.reject(Error(JSON.stringify(v.error)));
  else p.resolve(v.result);
};
const call = (method, params = {}) =>
  new Promise((resolve, reject) => {
    const id = ++sequence;
    pending.set(id, { resolve, reject });
    ws.send(
      JSON.stringify({
        id,
        method,
        params,
        sessionId: method.startsWith("Target.") ? undefined : sessionId,
      }),
    );
  });
const js = async (expression) =>
  (
    await call("Runtime.evaluate", {
      expression,
      returnByValue: true,
      awaitPromise: true,
    })
  ).result.value;
const key = async (code, type) =>
  call("Input.dispatchKeyEvent", {
    type,
    code,
    key: code === "Space" ? " " : code.replace("Key", "").toLowerCase(),
    windowsVirtualKeyCode:
      code === "Space" ? 32 : code.replace("Key", "").charCodeAt(0),
    nativeVirtualKeyCode:
      code === "Space" ? 32 : code.replace("Key", "").charCodeAt(0),
  });
let points = [],
  down = false;
try {
  const { targetInfos } = await call("Target.getTargets");
  const target =
    targetInfos.find(
      (t) => t.type === "page" && [origin, origin + "/"].includes(t.url),
    ) ??
    targetInfos.find(
      (t) =>
        t.type === "page" &&
        t.url.startsWith(origin) &&
        !t.url.includes("/idle.html"),
    );
  if (!target) throw Error("acceptance target missing");
  ({ sessionId } = await call("Target.attachToTarget", {
    targetId: target.targetId,
    flatten: true,
  }));
  if (options.observeOnly) {
    console.log(
      JSON.stringify(
        await js(
          '({hidden:document.hidden,surface:{...document.querySelector("[data-contra-runtime]")?.dataset},media:[...document.querySelectorAll("audio[data-platform-track]")].map(m=>({key:m.dataset.platformTrack,src:m.currentSrc,position:m.currentTime,rate:m.playbackRate,paused:m.paused,muted:m.muted}))})',
        ),
      ),
    );
    ws.close();
    process.exit(0);
  }
  if (options.clickAfterHold || options.captureNetwork)
    await call("Network.enable");
  const startedAt = Date.now();
  const mediaStart = options.captureNetwork
    ? await js(
        `(()=>{if(!window.__contraMedia){window.__contraMedia=[];const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(...args){const row={at:performance.now(),src:this.currentSrc||this.src,offset:this.currentTime,paused:this.paused};window.__contraMedia.push(row);const promise=play.apply(this,args);promise?.catch(error=>{row.error=error.name});return promise;}}return window.__contraMedia.length})()`,
      )
    : 0;
  const cursor = ["revision", "visualFrame"].includes(options.cursor)
    ? options.cursor
    : "frame";
  const snapshot = options.platform
    ? '({...document.querySelector("[data-contra-runtime]").dataset})'
    : "window.contraBrowser.snapshot";
  const failure = options.platform
    ? 'document.querySelector(".contra-board-error")?.textContent'
    : "window.contraBrowser.error";
  const initialSnapshot = await js(snapshot);
  if (options.measureVisual)
    await js(`(()=>{
    const root=document.querySelector('[data-contra-runtime]'),canvas=root.querySelector('canvas'),ctx=canvas.getContext('2d'),actor=${options.actor ?? 0};
    const initial=JSON.parse(root.dataset.visualPlayers)[actor],pixels=ctx.getImageData(0,0,256,240).data;
    // Player colors come from verified stage-one NES palette entries 0x12/0x16.
    // Restrict the scan to the standing player, excluding HUD and water.
    const color=actor===0?[32,56,239]:[219,40,0];
    const blue=bytes=>{let total=0,count=0;for(let y=Math.max(24,initial.y-28);y<Math.min(190,initial.y+20);y++)for(let x=Math.max(0,initial.x-24);x<Math.min(256,initial.x+32);x++){const i=(y*256+x)*4;if(bytes[i]===color[0]&&bytes[i+1]===color[1]&&bytes[i+2]===color[2]){total+=x;count++}}return count?{x:total/count,count}:null};
    const baseline=blue(pixels);
    const report=window.__contraVisualLatency={initial,baseline,initialAuthority:root.dataset.revision,keyAt:null,firstPaint:null};
    const pressed=e=>{if(e.isTrusted&&report.keyAt===null)report.keyAt=performance.now()};
    window.addEventListener('keydown',pressed,true);window.addEventListener('pointerdown',pressed,true);
    const observe=()=>{const player=JSON.parse(root.dataset.visualPlayers)[actor];
      if(report.keyAt!==null&&player.x!==initial.x){
        const next=ctx.getImageData(0,0,256,240).data;let changed=0;for(let i=0;i<next.length;i++)if(next[i]!==pixels[i])changed++;
        const observed=blue(next);if(changed&&baseline&&observed&&Math.abs(observed.x-baseline.x)>.5){report.firstPaint={at:performance.now(),changedBytes:changed,observed,player,authority:root.dataset.revision,visual:root.dataset.visualRevision,predicted:root.dataset.predicted};window.removeEventListener('keydown',pressed,true);window.removeEventListener('pointerdown',pressed,true);return;}
      }
      if(report.keyAt===null||performance.now()-report.keyAt<1000)requestAnimationFrame(observe);else {window.removeEventListener('keydown',pressed,true);window.removeEventListener('pointerdown',pressed,true);}
    };requestAnimationFrame(observe);return true;
  })()`);
  const before = Number(initialSnapshot[cursor]);
  const lives = (value) =>
    typeof value.lives === "string"
      ? JSON.parse(value.lives)
      : value.players?.map((player) => player.lives);
  if (options.keys)
    for (const code of options.keys)
      await key(code, options.releaseOnly ? "keyUp" : "keyDown");
  if (options.touch) {
    await call("Emulation.setTouchEmulationEnabled", {
      enabled: true,
      maxTouchPoints: 5,
    });
    points = await js(
      `(${JSON.stringify(options.touch)}).map((item,index)=>{const r=document.querySelector(item.selector).getBoundingClientRect();return {x:r.left+r.width*(item.x??.5),y:r.top+r.height*(item.y??.5),id:index+1,radiusX:2,radiusY:2,force:1}})`,
    );
    await call("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: points,
    });
    down = true;
  }
  if (options.mouse) {
    const p = await js(
      `(()=>{const r=document.querySelector(${JSON.stringify(options.mouse)}).getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2}})()`,
    );
    points = [p];
    await call("Input.dispatchMouseEvent", {
      type: "mousePressed",
      ...p,
      button: "left",
      buttons: 1,
      clickCount: 1,
    });
    down = true;
  }
  const end = Date.now() + (options.timeoutMS ?? 8000);
  while (Date.now() < end) {
    const state = await js(`({snapshot:${snapshot},error:${failure}})`);
    if (state.error) throw Error(state.error);
    if (
      options.stopOnLifeDrop !== undefined &&
      lives(state.snapshot)[options.stopOnLifeDrop] <
        lives(initialSnapshot)[options.stopOnLifeDrop]
    )
      break;
    if (
      Number(state.snapshot[cursor]) >=
      before + (options.releaseOnly ? 0 : (options.frames ?? 12))
    )
      break;
    await new Promise((r) => setTimeout(r, 15));
  }
  const after = await js(snapshot);
  const lifeDropped =
    options.stopOnLifeDrop !== undefined &&
    lives(after)[options.stopOnLifeDrop] <
      lives(initialSnapshot)[options.stopOnLifeDrop];
  if (
    !lifeDropped &&
    Number(after[cursor]) <
      before + (options.releaseOnly ? 0 : (options.frames ?? 12))
  )
    throw Error("authoritative frames stopped");
  if (options.stopOnLifeDrop !== undefined && !lifeDropped)
    throw Error("natural movement did not reach a visible life-loss boundary");
  if (options.clickAfterHold) {
    const point = await js(
      `(()=>{const r=document.querySelector(${JSON.stringify(options.clickAfterHold)}).getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2}})()`,
    );
    await call("Input.dispatchMouseEvent", {
      type: "mousePressed",
      ...point,
      button: "left",
      buttons: 1,
      clickCount: 1,
    });
    await call("Input.dispatchMouseEvent", {
      type: "mouseReleased",
      ...point,
      button: "left",
      buttons: 0,
      clickCount: 1,
    });
    for (const code of options.keys ?? []) await key(code, "keyUp");
    await new Promise((resolve) => setTimeout(resolve, 700));
  }
  const media = options.captureNetwork
    ? await js(`window.__contraMedia.slice(${mediaStart})`)
    : [];
  console.log(
    JSON.stringify({
      before,
      initial: initialSnapshot,
      after,
      options,
      network,
      receivedBytes,
      receivedFrames,
      gamePackets,
      observedMS: Date.now() - startedAt,
      media,
      visualLatency: options.measureVisual
        ? await js("window.__contraVisualLatency")
        : undefined,
    }),
  );
} finally {
  if (sessionId) {
    if (options.keys && !options.hold)
      for (const code of options.keys) await key(code, "keyUp").catch(() => {});
    if (down && options.touch)
      await call("Input.dispatchTouchEvent", {
        type: "touchEnd",
        touchPoints: [],
      }).catch(() => {});
    if (down && options.mouse)
      await call("Input.dispatchMouseEvent", {
        type: "mouseReleased",
        ...points[0],
        button: "left",
        buttons: 0,
        clickCount: 1,
      }).catch(() => {});
  }
  ws.close();
}
