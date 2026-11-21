import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve, sep } from "node:path";

const argumentsList = process.argv.slice(2);
const inputConfig = argumentsList[0] === "--stdin" ? JSON.parse(readFileSync(0, "utf8")) : { endpoint: argumentsList[0], pageURL: argumentsList[1], options: JSON.parse(argumentsList[2] ?? "{}") };
const { endpoint, pageURL, options } = inputConfig;
const outputFile = resolve(options.output ?? ".local/hotlinemiami/browser/native-last.json");
assert.ok(outputFile.startsWith(resolve(".local/hotlinemiami") + sep), "native report must remain in the game workspace");
mkdirSync(dirname(outputFile), { recursive: true });
const socket = new WebSocket(endpoint);
const requests = new Map();
const network = { sentBytes: 0, receivedBytes: 0, sentMessages: 0, receivedMessages: 0, largestMessage: 0, messageTypes: {}, messages: [], acks: [], applications: [] };
const media = { players: [], events: [], properties: [], messages: [], errors: [], captureBegan: 0 };
const sent = new Map();
const sentInputs = new Map();
let ordinal = 0;
let session;
let navigationStart = 0;
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});
socket.onmessage = ({ data }) => {
  const message = JSON.parse(data);
  const request = requests.get(message.id);
  if (request) {
    requests.delete(message.id);
    clearTimeout(request.timer);
    message.error ? request.reject(message.error) : request.resolve(message.result);
  }
  if (message.sessionId !== session) return;
  if (message.method?.startsWith("Media.")) {
    if (message.method === "Media.playersCreated") media.players.push(...message.params.players);
    if (message.method === "Media.playerEventsAdded" && media.events.length < 1200) media.events.push(...message.params.events.map(event => ({ playerId: message.params.playerId, timestamp: event.timestamp, value: event.value })));
    if (message.method === "Media.playerPropertiesChanged" && media.properties.length < 1200) media.properties.push(...message.params.properties.map(property => ({ playerId: message.params.playerId, ...property })));
    if (message.method === "Media.playerMessagesLogged" && media.messages.length < 200) media.messages.push(...message.params.messages.map(entry => ({ playerId: message.params.playerId, ...entry })));
    if (message.method === "Media.playerErrorsRaised" && media.errors.length < 200) media.errors.push(...message.params.errors.map(entry => ({ playerId: message.params.playerId, ...entry })));
    return;
  }
  if (!message.method?.startsWith("Network.webSocketFrame")) return;
  const incoming = message.method.endsWith("Received");
  const payload = message.params.response.payloadData;
  const bytes = new TextEncoder().encode(payload).length;
  if (incoming) { network.receivedBytes += bytes; network.receivedMessages++; }
  else { network.sentBytes += bytes; network.sentMessages++; }
  network.largestMessage = Math.max(network.largestMessage, bytes);
  let body;
  try { body = JSON.parse(payload); } catch { return; }
  const at = Math.round(message.params.timestamp * 1000000);
  const classification = `${incoming ? "in" : "out"}:${body.type ?? "unknown"}`;
  network.messageTypes[classification] = (network.messageTypes[classification] ?? 0) + 1;
  if (!incoming && body.nonce) sent.set(body.nonce, at);
  if (!incoming && body.action?.seq) sentInputs.set(body.action.seq, at);
  if (incoming && body.type === "ack" && sent.has(body.nonce)) network.acks.push({ microseconds: at - sent.get(body.nonce), status: body.status });
  const projectedPlayer = body.game?.data?.players?.[body.game?.data?.you];
  const authority = incoming && projectedPlayer ? { x: projectedPlayer.x, y: projectedPlayer.y, life: projectedPlayer.life, receivedSeq: projectedPlayer.receivedSeq, appliedSeq: projectedPlayer.appliedSeq } : undefined;
  if (authority) network.lastAuthority = { ...authority, frame: body.game.data.frame, at, localAt: message.params.timestamp * 1000 - navigationStart };
  if (incoming && projectedPlayer) for (const [sequence, began] of sentInputs) {
    if (sequence <= projectedPlayer.appliedSeq) {
      network.applications.push({ seq: sequence, microseconds: at - began, serverFrame: body.game.data.frame });
      sentInputs.delete(sequence);
    }
  }
  const action = !incoming && body.action ? Object.fromEntries(["seq", "life", "moveX", "moveY", "aim", "fire", "throw", "pickup", "finish", "interact"].map(key => [key, body.action[key]])) : undefined;
  if (action) network.lastInput = action;
  if (network.messages.length < 600) network.messages.push({ incoming, bytes, type: body.type, at, localAt: message.params.timestamp * 1000 - navigationStart, action, authority, serverFrame: body.game?.data?.frame, serverNow: body.serverNow, receivedSeq: body.game?.data?.players?.[body.game?.data?.you]?.receivedSeq, appliedSeq: body.game?.data?.players?.[body.game?.data?.you]?.appliedSeq });
};
const call = (method, params = {}, sessionId = session) => new Promise((resolve, reject) => {
  const id = ++ordinal;
  const timer = setTimeout(() => { requests.delete(id); reject(new Error(`CDP timeout ${method}`)); }, 10000);
  requests.set(id, { resolve, reject, timer });
  socket.send(JSON.stringify({ id, method, params, sessionId }));
});
const wait = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));
const evaluate = async (expression) => {
  const response = await call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
  assert.ok(!response.exceptionDetails, JSON.stringify(response.exceptionDetails));
  return response.result.value;
};
const percentile = (values, fraction) => values.length ? [...values].sort((a, b) => a - b)[Math.min(values.length - 1, Math.ceil(values.length * fraction) - 1)] : null;
const playable = async () => {
  for (let attempt = 0; attempt < 250; attempt++) {
    const enabled = await evaluate(`(() => {const frame=document.querySelector('[data-hotlinemiami-canvas]')?.__hotlineFrame;return !!frame?.inputEnabled&&!!frame.local?.alive&&frame.serverFrame>=frame.local.downUntil&&frame.serverFrame>=(frame.local.busyUntil??0)})()`);
    if (enabled) return;
    await wait(40);
  }
  throw new Error("player never returned to an actionable life");
};
try {
  const browserVersion = await call("Browser.getVersion", {}, undefined);
  const { targetInfos } = await call("Target.getTargets", {}, undefined);
  const target = targetInfos.find((entry) => entry.type === "page" && entry.url.startsWith(pageURL));
  assert.ok(target, "production room target missing");
  session = (await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }, undefined)).sessionId;
  await call("Page.bringToFront");
  await call("Performance.enable");
  const performanceMetrics = (await call("Performance.getMetrics")).metrics;
  navigationStart = (performanceMetrics.find(metric => metric.name === "NavigationStart")?.value ?? 0) * 1000;
  media.captureBegan = performanceMetrics.find(metric => metric.name === "Timestamp")?.value ?? 0;
  network.navigationStart = navigationStart;
  await call("Network.enable");
  await call("Media.enable").catch(() => {});
  const requiredAudio = options.requireAudio?.length ? await evaluate(`fetch('/hotlinemiami/manifest.json').then(response=>{if(!response.ok)throw Error('audio manifest missing');return response.json()}).then(manifest=>${JSON.stringify(options.requireAudio)}.map(id=>{const file=(manifest.sounds??manifest.audio??{})[id]?.file;if(!file)throw Error('required audio source missing: '+id);return {id,file}}))`) : [];
  await evaluate(`(() => {
    const canvas = document.querySelector('[data-hotlinemiami-canvas]');
    if (!canvas || !canvas.__hotlineFrame?.local) throw new Error('production canvas has not initialized');
    window.__hotlineNativeProbe?.stop?.();
    const probe = { events: [], responses: [], frames: [], pending: null, running: true };
    const hash = () => {
      const pixels = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data;
      let value = 2166136261;
      for (let index = 0; index < pixels.length; index += 16) value = Math.imul(value ^ pixels[index], 16777619);
      return value >>> 0;
    };
    probe.arm = (kind, x, y) => { probe.pending = { kind, x, y, before: structuredClone(canvas.__hotlineFrame.local), beforeLogical: structuredClone(canvas.__hotlineFrame.logical), beforeFlash: canvas.__hotlineFrame.attackFlashUntil ?? 0, hash: hash(), at: null, armedAt: performance.now() }; };
    const listener = (event) => {
      if (!event.isTrusted) return;
      const current = probe.pending;
      const at = performance.now();
      const entry = { type: event.type, at, trusted: event.isTrusted, key: event.code ?? null };
      probe.events.push(entry);
      if (current && current.at === null && ((current.kind === 'key' && event.type === 'keydown') || (current.kind === 'aim' && event.type === 'pointermove') || (current.kind === 'touch' && event.type === 'pointermove') || (current.kind === 'attack' && event.type === 'pointerdown'))) current.at = at;
    };
    const types = ['keydown', 'keyup', 'pointerdown', 'pointermove', 'pointerup', 'pointercancel'];
    for (const type of types) document.addEventListener(type, listener, true);
    const loop = (at) => {
      if (!probe.running) return;
      const frame = canvas.__hotlineFrame;
      if (probe.frames.length < 1500) probe.frames.push({ at, drawCount: frame?.drawCount ?? frame?.frame, x: frame?.local?.x, y: frame?.local?.y, logicalX: frame?.logical?.x, logicalY: frame?.logical?.y, cameraX: frame?.camera?.x, cameraY: frame?.camera?.y, cameraWidth: frame?.camera?.width, cameraHeight: frame?.camera?.height, viewportWidth: frame?.viewport?.width, viewportHeight: frame?.viewport?.height, serverFrame: frame?.serverFrame, predictedFrame: frame?.predictedFrame, receivedSeq: frame?.receivedSeq, appliedSeq: frame?.appliedSeq, inputWindow: frame?.inputWindow, pending: frame?.pending });
      const pending = probe.pending;
      const local = frame?.local;
      if (pending?.kind === 'key' && pending.at === null) {
        const source = window.__hotlineTrustedEvents ?? canvas.__hotlineMetrics?.inputs ?? [];
        const event = source.find(entry => entry.type === 'keydown' && entry.trusted && entry.at >= pending.armedAt);
        if (event) { pending.at = event.at; probe.events.push(event); }
      }
      if (pending?.at !== null && pending && local && local.life === pending.before.life) {
        const dx = local.x - pending.before.x;
        const dy = local.y - pending.before.y;
        const changed = pending.kind === 'attack' ? frame.attackFlashUntil > frame.now && frame.attackFlashUntil > pending.beforeFlash : pending.kind === 'aim' ? local.aim !== pending.before.aim : dx * pending.x + dy * pending.y > 0;
        if (changed) {
          const visibleAt = performance.now();
          const afterHash = hash();
          if (afterHash !== pending.hash) {
            probe.responses.push({ kind: pending.kind, inputAt: pending.at, visibleAt, milliseconds: visibleAt - pending.at, pixelsChanged: true, before: pending.before, after: structuredClone(local), beforeLogical: pending.beforeLogical, afterLogical: structuredClone(frame.logical) });
            probe.pending = null;
          }
        }
      }
      requestAnimationFrame(loop);
    };
    probe.stop = () => { probe.running = false; for (const type of types) document.removeEventListener(type, listener, true); };
    window.__hotlineNativeProbe = probe;
    requestAnimationFrame(loop);
    return true;
  })()`);
  const arm = (kind, x = 0, y = 0) => evaluate(`window.__hotlineNativeProbe.arm(${JSON.stringify(kind)},${x},${y});true`);
  const cameraAimChecks = [];
  if (options.touch) {
    await call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 2 });
    const areas = await evaluate(`['move','aim'].map(kind=>{const e=document.querySelector('[data-hotlinemiami-stick="'+kind+'"]');if(!e)throw new Error('missing '+kind+' stick');return e.getBoundingClientRect().toJSON()})`);
    const movement = areas[0];
    const aiming = areas[1];
    const center = { x: movement.x + movement.width / 2, y: movement.y + movement.height / 2 };
    const aimCenter = { x: aiming.x + aiming.width / 2, y: aiming.y + aiming.height / 2 };
    let responses = 0;
    const began = Date.now();
    for (let index = 0; index < (options.samples ?? 12) * 4 && responses < (options.samples ?? 12) && Date.now() - began < 30000; index++) {
      await playable();
      const direction = [[1, 0], [-1, 0], [0, 1], [0, -1]][index % 4];
      const previousResponses = await evaluate("window.__hotlineNativeProbe.responses.length");
      const point = { ...center, radiusX: 4, radiusY: 4, force: 1, id: 0 };
      const second = { ...aimCenter, radiusX: 4, radiusY: 4, force: 1, id: 1 };
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [point, second] });
      await arm("touch", direction[0], direction[1]);
      await call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ ...point, x: center.x + direction[0] * 24, y: center.y + direction[1] * 24 }, { ...second, x: aimCenter.x + 12 }] });
      await wait(100);
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      await wait(60);
      if (await evaluate("window.__hotlineNativeProbe.responses.length") > previousResponses) responses++;
    }
  } else {
    const keys = [{ code: "KeyD", key: "d", windowsVirtualKeyCode: 68, x: 1, y: 0 }, { code: "KeyA", key: "a", windowsVirtualKeyCode: 65, x: -1, y: 0 }, { code: "KeyS", key: "s", windowsVirtualKeyCode: 83, x: 0, y: 1 }, { code: "KeyW", key: "w", windowsVirtualKeyCode: 87, x: 0, y: -1 }];
    for (let index = 0; index < (options.samples ?? 16); index++) {
      await playable();
      const { x, y, ...key } = keys[index % keys.length];
      await arm("key", x, y);
      await call("Input.dispatchKeyEvent", { type: "keyDown", ...key });
      await wait(100);
      await call("Input.dispatchKeyEvent", { type: "keyUp", ...key });
      await wait(60);
    }
    const rectangle = await evaluate("document.querySelector('[data-hotlinemiami-canvas]').getBoundingClientRect().toJSON()");
    for (let index = 0; index < 8; index++) {
      await arm("aim");
      await call("Input.dispatchMouseEvent", { type: "mouseMoved", x: rectangle.x + rectangle.width * (index % 2 ? 0.2 : 0.8), y: rectangle.y + rectangle.height * (index % 3 ? 0.3 : 0.7), button: "none" });
      await wait(90);
    }
    await playable();
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyD", key: "d", windowsVirtualKeyCode: 68 });
    for (let index = 0; index < 8; index++) {
      await wait(17);
      const observed = await evaluate("(()=>{const canvas=document.querySelector('[data-hotlinemiami-canvas]');return {frame:canvas.__hotlineFrame,rect:canvas.getBoundingClientRect().toJSON()}})()");
      const frame = observed.frame;
      if (!frame.inputEnabled || !frame.local.alive) continue;
      const pointerX = rectangle.x + rectangle.width * 0.2;
      const pointerY = rectangle.y + rectangle.height * 0.3;
      const expected = (Math.round(Math.atan2(pointerY - observed.rect.y - frame.local.screenY, pointerX - observed.rect.x - frame.local.screenX) * 1024 / (Math.PI * 2)) + 1024) % 1024;
      const error = Math.min((frame.local.aim - expected + 1024) % 1024, (expected - frame.local.aim + 1024) % 1024);
      cameraAimChecks.push({ at: frame.now, life: frame.local.life, x: frame.local.x, y: frame.local.y, screenX: frame.local.screenX, screenY: frame.local.screenY, serverFrame: frame.serverFrame, predictedFrame: frame.predictedFrame, pointerX, pointerY, rect: observed.rect, expected, actual: frame.local.aim, error });
    }
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "KeyD", key: "d", windowsVirtualKeyCode: 68 });
  }
  let quickTap;
  let unarmed;
  const interruptedAttacks = [];
  for (let attempt = 0; attempt < 6; attempt++) {
    await playable();
    const before = await evaluate('fetch("/api/state").then(response=>response.json()).then(state=>({you:state.room.you,data:state.room.game.data}))');
    const player = before.data.players[before.you];
    const timing = await evaluate(`JSON.parse(window.hotlineMotion.weapons()).find(weapon=>weapon.id===${JSON.stringify(player.weapon)})??JSON.parse(window.hotlineMotion.unarmed())`);
    if (!player.alive) { interruptedAttacks.push({ life: player.life, reason: "authority-already-dead" }); continue; }
    if (player.alive && (timing.melee || player.ammo > 0)) {
      if (options.touch) {
        const rectangle = await evaluate("document.querySelector('[data-hotlinemiami-stick=aim]').getBoundingClientRect().toJSON()");
        const point = { x: rectangle.x + rectangle.width / 2, y: rectangle.y + rectangle.height / 2, radiusX: 4, radiusY: 4, force: 1, id: 1 };
        await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [point] });
        await call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ ...point, x: point.x + rectangle.width * 0.42 }] });
        await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      } else {
        const rectangle = await evaluate("document.querySelector('[data-hotlinemiami-canvas]').getBoundingClientRect().toJSON()");
        const point = { x: rectangle.x + rectangle.width * 0.8, y: rectangle.y + rectangle.height * 0.5, button: "left", clickCount: 1 };
        await call("Input.dispatchMouseEvent", { type: "mousePressed", ...point, buttons: 1 });
        await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...point, buttons: 0 });
      }
      await wait(240);
      const after = await evaluate('fetch("/api/state").then(response=>response.json()).then(state=>state.room.game.data)');
      const fires = after.events.filter((event) => event.type === "fire" && event.seat === before.you && event.id > before.data.eventId);
      const afterPlayer = after.players[before.you];
      quickTap = { beforeAmmo: player.ammo, beforeLife: player.life, melee: !!timing.melee, afterAmmo: afterPlayer.ammo, life: afterPlayer.life, alive: afterPlayer.alive, events: fires };
      if (fires.length === 0 && (!afterPlayer.alive || afterPlayer.life !== player.life || afterPlayer.downUntil > player.downUntil)) {
        interruptedAttacks.push(quickTap);
        quickTap = undefined;
        continue;
      }
    }
    break;
  }
  if (options.unarmed) {
    const rectangle = await evaluate("document.querySelector('[data-hotlinemiami-canvas]').getBoundingClientRect().toJSON()");
    const point = { x: rectangle.x + rectangle.width * 0.8, y: rectangle.y + rectangle.height * 0.5, clickCount: 1 };
    await call("Input.dispatchMouseEvent", { type: "mousePressed", ...point, button: "right", buttons: 2 });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...point, button: "right", buttons: 0 });
    let state;
    for (let attempt = 0; attempt < 100; attempt++) {
      state = await evaluate('fetch("/api/state").then(response=>response.json()).then(state=>({you:state.room.you,data:state.room.game.data}))');
      if (state.data.players[state.you].weapon === "" && state.data.players[state.you].ammo === 0) break;
      await wait(20);
    }
    assert.equal(state.data.players[state.you].weapon, "", "legal native throw did not release the weapon");
    await wait(150);
    await arm("attack");
    await call("Input.dispatchMouseEvent", { type: "mousePressed", ...point, button: "left", buttons: 1 });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...point, button: "left", buttons: 0 });
    await wait(240);
    const after = await evaluate('fetch("/api/state").then(response=>response.json()).then(state=>state.room.game.data)');
    unarmed = { weapon: after.players[state.you].weapon, ammo: after.players[state.you].ammo, events: after.events.filter(event => event.id > state.data.eventId && event.type === "fire" && event.seat === state.you && !event.weapon) };
  }
  let mobileAudit;
  if (options.mobileAudit) {
    await evaluate(`(() => {window.__hotlineMobileAudit={contextMenus:[],selections:[]};document.addEventListener('contextmenu',event=>{queueMicrotask(()=>window.__hotlineMobileAudit.contextMenus.push({trusted:event.isTrusted,prevented:event.defaultPrevented,target:event.target.tagName}));},true);document.addEventListener('selectionchange',()=>{const text=getSelection()?.toString()??'';if(text)window.__hotlineMobileAudit.selections.push(text);});return true})()`);
    const snapshot = () => evaluate(`(()=>{const canvas=document.querySelector('[data-hotlinemiami-canvas]');const hint=document.querySelector('[data-hotlinemiami-hint]');const ammo=[...document.querySelectorAll('.hm-ammo,.hm-ammo strong,.hm-ammo span')].map(element=>({tag:element.tagName,text:element.textContent,rect:element.getBoundingClientRect().toJSON()}));const toolbar=document.querySelector('.game-fullscreen-toolbar .room-fullscreen-toggle')?.getBoundingClientRect().toJSON();return {rect:canvas.getBoundingClientRect().toJSON(),local:canvas.__hotlineFrame.local,hint:hint?.dataset.hotlinemiamiHint,text:hint?.textContent,ammo,toolbar,ammoToolbarOverlap:!!toolbar&&ammo.some(item=>item.rect.left<toolbar.right&&item.rect.right>toolbar.left&&item.rect.top<toolbar.bottom&&item.rect.bottom>toolbar.top),scrollY}})()`);
    const photograph = async name => { const shot = await call("Page.captureScreenshot", { format: "png" }); writeFileSync(resolve(dirname(outputFile), name + ".png"), Buffer.from(shot.data, "base64")); };
    const clickButton = async label => {
      const rectangle = await evaluate(`(()=>{const element=document.querySelector('button[aria-label=${JSON.stringify(label)}]');element.scrollIntoView({block:'center'});return element.getBoundingClientRect().toJSON()})()`);
      const point = { x: rectangle.x + rectangle.width / 2, y: rectangle.y + rectangle.height / 2, button: "left", clickCount: 1 };
      await call("Input.dispatchMouseEvent", { type: "mousePressed", ...point, buttons: 1 });
      await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...point, buttons: 0 });
      await wait(120);
    };
    const geometryEqual = (left, right) => ["top", "height", "width"].every(key => left.rect[key] === right.rect[key]);
    const controls = await snapshot();
    assert.ok(controls.local.alive && controls.local.ammo > 0, "mobile ammunition scenario requires a live gun");
    const aim = await evaluate("document.querySelector('[data-hotlinemiami-stick=aim]').getBoundingClientRect().toJSON()");
    const firePoint = { x: aim.x + aim.width * 0.93, y: aim.y + aim.height / 2, radiusX: 4, radiusY: 4, force: 1, id: 0 };
    await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [firePoint] });
    let empty;
    for (let attempt = 0; attempt < 160; attempt++) {
      empty = await snapshot();
      if (empty.local.ammo === 0) break;
      await wait(40);
    }
    await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await wait(150);
    empty = await snapshot();
    assert.ok(empty.local.alive && empty.local.weapon && empty.local.ammo === 0 && empty.hint === "empty" && empty.text.includes("投掷空枪") && empty.text.includes("拾取"), JSON.stringify(empty));
    assert.ok(geometryEqual(controls, empty), "empty-gun hint moved the normal canvas");
    await photograph("mobile-empty-normal");
    await clickButton("全屏");
    const fullEmpty = await snapshot();
    assert.equal(fullEmpty.hint, "empty");
    await photograph("mobile-empty-fullscreen");
    const throwButton = await evaluate("document.querySelector('button[aria-label=投掷武器]').getBoundingClientRect().toJSON()");
    const throwPoint = { x: throwButton.x + throwButton.width / 2, y: throwButton.y + throwButton.height / 2, radiusX: 4, radiusY: 4, force: 1, id: 0 };
    await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [throwPoint] });
    await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    let fullUnarmed;
    for (let attempt = 0; attempt < 100; attempt++) {
      fullUnarmed = await snapshot();
      if (fullUnarmed.local.weapon === "") break;
      await wait(30);
    }
    assert.ok(fullUnarmed.local.weapon === "" && fullUnarmed.hint === "unarmed" && geometryEqual(fullEmpty, fullUnarmed), "unarmed hint moved the fullscreen canvas");
    const guards = [];
    for (const mode of ["fullscreen", "normal"]) {
      if (mode === "normal") {
        await clickButton("退出全屏");
        await evaluate(`scrollTo(0,${controls.scrollY});true`);
        await wait(120);
        assert.ok(geometryEqual(empty, await snapshot()), "unarmed hint moved the normal canvas after fullscreen");
      }
      for (const selector of ["[data-hotlinemiami-stick=move] .hm-stick-name", ".hm-bottom-line", "[data-hotlinemiami-canvas]"]) {
        const rectangle = await evaluate(`document.querySelector(${JSON.stringify(selector)}).getBoundingClientRect().toJSON()`);
        const point = { x: rectangle.x + rectangle.width / 2, y: rectangle.y + rectangle.height / 2, radiusX: 4, radiusY: 4, force: 1, id: 0 };
        await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [point] });
        await wait(720);
        await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
        await call("Input.dispatchMouseEvent", { type: "mousePressed", x: point.x, y: point.y, button: "right", buttons: 2, clickCount: 1 });
        await call("Input.dispatchMouseEvent", { type: "mouseReleased", x: point.x, y: point.y, button: "right", buttons: 0, clickCount: 1 });
        await wait(60);
        const state = await evaluate("({selection:getSelection()?.toString()??'',contexts:window.__hotlineMobileAudit.contextMenus.slice(-1),selections:window.__hotlineMobileAudit.selections})");
        assert.ok(!state.selection && state.selections.length === 0 && state.contexts[0]?.trusted && state.contexts[0]?.prevented, "native control selection/context menu was not suppressed: " + JSON.stringify(state));
        guards.push({ mode, selector, ...state });
      }
    }
    const sticks = await evaluate("['move','aim'].map(kind=>document.querySelector('[data-hotlinemiami-stick='+kind+']').getBoundingClientRect().toJSON())");
    const points = sticks.map((rect, index) => ({ x: rect.x + rect.width / 2 + (index ? 12 : 24), y: rect.y + rect.height / 2, radiusX: 4, radiusY: 4, force: 1, id: index }));
    await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: points });
    await wait(150);
    await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await wait(240);
    const released = await evaluate("[...document.querySelectorAll('.hm-stick-thumb')].map(element=>{const transform=new DOMMatrix(getComputedStyle(element).transform);return {x:transform.m41,y:transform.m42}})");
    const neutral = network.lastInput;
    writeFileSync(resolve(dirname(outputFile), "mobile-release.json"), JSON.stringify({ released, neutral, guards }));
    assert.ok(released.length === 2 && released.every(stick => stick.x === 0 && stick.y === 0) && neutral?.moveX === 0 && neutral?.moveY === 0 && !neutral?.fire, "multitouch release retained an input: " + JSON.stringify({ released, neutral }));
    mobileAudit = { controls, empty, fullEmpty, fullUnarmed, final: await snapshot(), guards, released, neutral };
  }
  await wait(options.observeMS ?? 2200);
  const observation = await evaluate(`(() => { const probe=window.__hotlineNativeProbe;probe.stop();const canvas=document.querySelector('[data-hotlinemiami-canvas]');return { events:probe.events,responses:probe.responses,frames:probe.frames,final:canvas.__hotlineFrame,metrics:canvas.__hotlineMetrics,rendered:canvas.width>0&&canvas.height>0 }; })()`);
  const responseTimes = observation.responses.map((entry) => entry.milliseconds);
  const intervals = observation.frames.slice(1).map((entry, index) => entry.at - observation.frames[index].at);
  const elapsed = observation.frames.at(-1).at - observation.frames[0].at;
  const counts = observation.frames.map((entry) => entry.drawCount);
  const changes = counts.slice(1).filter((count, index) => count !== counts[index]).length;
  const playback = [];
  for (const playerId of new Set(media.events.map(event => event.playerId))) {
    let source;
    for (const row of media.events.filter(event => event.playerId === playerId).sort((a, b) => a.timestamp - b.timestamp)) {
      let event;
      try { event = JSON.parse(row.value); } catch { continue; }
      if (event.event === "kLoad") {
        source = { playerId, url: event.url, loadedAt: row.timestamp, playingAt: [], duration: null };
        playback.push(source);
      } else if (source && event.event === "kPipelineStateChange" && event.pipeline_state === "kPlaying") source.playingAt.push(row.timestamp);
      else if (source && event.event === "kDurationChanged") source.duration = event.duration;
    }
  }
  const audio = { captureBegan: media.captureBegan, validPlayerAttribution: media.events.length > 0 && media.events.every(event => typeof event.playerId === "string" && event.playerId.length > 0), playback, requirements: requiredAudio.map(resource => ({ ...resource, playingPlayerIds: [...new Set(playback.filter(source => (resource.file.startsWith("https:") ? source.url === resource.file : new URL(source.url).pathname.endsWith("/" + resource.file)) && source.playingAt.some(at => at >= media.captureBegan)).map(source => source.playerId))] })) };
  const result = { ...observation, touch: !!options.touch, quickTap, interruptedAttacks, unarmed, cameraAimChecks, browserVersion, network, media, audio, measurement: "trusted native DOM event to rendered canvas pixel response; excludes physical display panel latency", summary: { responses: responseTimes.length, responseP95: percentile(responseTimes, 0.95), frameP95: percentile(intervals, 0.95), frameP99: percentile(intervals, 0.99), visibleFPS: changes * 1000 / elapsed, elapsed, visibleUpdates: changes, sentBytesPerSecond: network.sentBytes * 1000 / elapsed, receivedBytesPerSecond: network.receivedBytes * 1000 / elapsed, receivedMessagesPerSecond: network.receivedMessages * 1000 / elapsed, gameMessagesPerSecond: (network.messageTypes['in:game'] ?? 0) * 1000 / elapsed, ackP95MS: percentile(network.acks.map(entry => entry.microseconds / 1000), 0.95), appliedP95MS: percentile(network.applications.map(entry => entry.microseconds / 1000), 0.95), maxPredictionLeadFrames: Math.max(0, ...observation.frames.map(entry => entry.predictedFrame - entry.serverFrame)), maxPendingInputs: Math.max(0, ...observation.frames.map(entry => entry.pending ?? 0)), verifiedEffectSounds: audio.requirements.filter(resource => resource.playingPlayerIds.length > 0).map(resource => resource.id) } };
  if (mobileAudit) result.mobileAudit = mobileAudit;
  writeFileSync(outputFile, JSON.stringify(result));
  console.log(JSON.stringify({ output: outputFile, summary: result.summary }));
  assert.ok(observation.events.some((entry) => entry.trusted), "no trusted native events observed");
  assert.ok(observation.rendered, "canvas has no pixels");
  assert.ok(observation.frames.every(frame => frame.viewportWidth === 576 && frame.viewportHeight === 360 && frame.cameraWidth === 576 && frame.cameraHeight === 360), "rendered viewport/camera differs from shared 576x360 metadata");
  assert.ok(quickTap?.events.length > 0, "native short press/release lost its fire edge");
  assert.ok(quickTap.life !== quickTap.beforeLife || (quickTap.melee ? quickTap.afterAmmo === quickTap.beforeAmmo : quickTap.afterAmmo < quickTap.beforeAmmo), "native attack violated its authoritative ammunition rule");
  if (options.unarmed) {
    assert.ok(unarmed?.events.length > 0 && unarmed.weapon === "" && unarmed.ammo === 0, "native unarmed attack did not reach authority");
    assert.ok(observation.responses.some(entry => entry.kind === "attack"), "native unarmed attack did not immediately draw a punch arc");
  }
  assert.ok(responseTimes.length >= (options.minimumResponses ?? 8), `too few visible native responses: ${responseTimes.length}`);
  if (!options.touch) assert.ok(observation.responses.filter(entry => entry.kind === "key").length >= 4, "keyboard responses must be measured separately from pointer aim");
  assert.ok(cameraAimChecks.every(entry => entry.error <= 2), "moving camera changed a stationary mouse target");
  if (!options.touch) assert.ok(cameraAimChecks.length > 0, "no actionable moving-camera sample");
  assert.ok(result.summary.responseP95 <= (options.touch ? 50 : 33), JSON.stringify(result.summary));
  assert.ok(result.summary.frameP95 <= 20 && result.summary.frameP99 <= 33, JSON.stringify(result.summary));
  assert.ok(result.summary.visibleFPS >= 55, JSON.stringify(result.summary));
  if (mobileAudit) assert.ok(mobileAudit.fullEmpty.toolbar && !mobileAudit.fullEmpty.ammoToolbarOverlap, "fullscreen toolbar overlaps weapon/ammunition: " + JSON.stringify({ ammo: mobileAudit.fullEmpty.ammo, toolbar: mobileAudit.fullEmpty.toolbar }));
  if (requiredAudio.length) assert.ok(audio.validPlayerAttribution && media.captureBegan > 0, "native media attribution or capture clock is missing");
  assert.ok(audio.requirements.every(resource => resource.playingPlayerIds.length > 0), "required native effect never reached kPlaying: " + JSON.stringify(audio.requirements));
} finally {
  for (const request of requests.values()) clearTimeout(request.timer);
  socket.close();
}
