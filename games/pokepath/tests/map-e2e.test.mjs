import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { access, mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const execute = promisify(execFile);
const root = fileURLToPath(new URL("../../../", import.meta.url));
const helper = new URL("../web/map-viewport.js", import.meta.url);
const output = join(root, ".local/pokepath/verification/map");
const profile = join(root, ".local/pokepath/map-browser-profile");
const environment = { ...process.env, AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", TMPDIR: join(root, ".local/tmp") };
const report = { command: "node --test games/pokepath/tests/map-e2e.test.mjs", passed: false, checks: [] };
let session;

async function browser(...args) {
  const result = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 60000, maxBuffer: 8 << 20 });
  const value = JSON.parse(result.stdout);
  if (!value.success) throw Error(JSON.stringify(value));
  return value.data;
}

async function evaluate(code) {
  return (await browser("eval", "-b", Buffer.from(code).toString("base64"))).result;
}

async function connectCDP(base) {
  const endpoint = new URL((await browser("get", "cdp-url")).cdpUrl);
  const pages = await (await fetch(`http://127.0.0.1:${endpoint.port}/json/list`)).json();
  const page = pages.find(page => page.url.startsWith(base));
  assert.ok(page, "fixture browser page missing");
  const socket = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => { socket.addEventListener("open", resolve, { once: true }); socket.addEventListener("error", reject, { once: true }); });
  let sequence = 0;
  const pending = new Map();
  socket.addEventListener("message", event => {
    const value = JSON.parse(event.data);
    const entry = pending.get(value.id);
    if (!entry) return;
    pending.delete(value.id);
    if (value.error) entry.reject(Error(JSON.stringify(value.error)));
    else entry.resolve(value.result);
  });
  return {
    call(method, params = {}) {
      return new Promise((resolve, reject) => { const id = ++sequence; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params })); });
    },
    close() { socket.close(); }
  };
}

const fixture = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>body{margin:0;background:#ecede4;color:#24352b;font:16px sans-serif;min-height:1600px}#controls{display:flex;gap:8px;padding:12px}button{padding:10px}#wrapper{position:relative;margin:10px 20px;width:720px;height:500px;border:2px solid #213b2a;overflow:auto;touch-action:none}canvas{display:block;background:#cee2b8}#outside{position:absolute;left:850px;top:650px;width:180px;height:100px;background:#dde6ed}#status{margin:15px} @media(max-width:500px){#outside{display:none}#wrapper{width:calc(100vw - 24px);height:400px;margin:10px}#controls{gap:3px;padding:8px}button{padding:8px;font-size:12px}}</style></head><body><div id="controls" data-local-control><button data-map-zoom-in>放大</button><button data-map-zoom-out>缩小</button><button data-map-fit>适应全图</button><button data-map-reset>复位</button></div><div id="wrapper"><canvas width="1440" height="1248"></canvas></div><div id="status">拖动和缩放地图，点击橙色塔身。</div><div id="outside">地图外滚轮区域</div><script type="module">import{attachMapViewport}from"/map-viewport.js";const canvas=document.querySelector('canvas'),wrapper=document.querySelector('#wrapper'),context=canvas.getContext('2d');context.fillStyle='#cee2b8';context.fillRect(0,0,1440,1248);context.strokeStyle='#9bbf80';for(let x=0;x<1440;x+=24){context.beginPath();context.moveTo(x,0);context.lineTo(x,1248);context.stroke()}for(let y=0;y<1248;y+=24){context.beginPath();context.moveTo(0,y);context.lineTo(1440,y);context.stroke()}context.fillStyle='#9d6241';context.fillRect(0,188,1440,32);context.fillRect(580,0,32,1248);context.fillStyle='#e17f32';context.beginPath();context.arc(300,280,16,0,Math.PI*2);context.fill();window.clicks=[];window.gestures=[];window.viewport=attachMapViewport({canvas,wrapper,onChange:state=>window.gestures.push(state)});for(const type of ['click','wheel','pointerdown','pointermove','pointerup','pointercancel','mousedown','mouseup','mousemove'])document.addEventListener(type,event=>{if(window.viewport.handleEvent(event))return}, {capture:true,passive:false});wrapper.addEventListener('click',event=>window.clicks.push({...window.viewport.logicalPoint(event.clientX,event.clientY),trusted:event.isTrusted}));window.mapReady=true;</script></body></html>`;

test("real mouse, wheel and touch preserve map coordinates and suppress drag clicks", { timeout: 120000 }, async () => {
  await access(helper);
  await mkdir(output, { recursive: true });
  await mkdir(environment.AGENT_BROWSER_SOCKET_DIR, { recursive: true });
  await mkdir(environment.TMPDIR, { recursive: true });
  const identity = await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "pm"], { cwd: root, env: environment, encoding: "utf8" });
  session = identity.stdout.trim();
  const server = createServer(async (request, response) => {
    if (request.url === "/map-viewport.js") { response.writeHead(200, { "Content-Type": "text/javascript" }); response.end(await readFile(helper)); }
    else { response.writeHead(200, { "Content-Type": "text/html" }); response.end(fixture); }
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const base = `http://127.0.0.1:${server.address().port}/`;
  let cdp;
  try {
    await browser("open", base);
    await browser("set", "viewport", "1080", "820");
    await browser("wait", "--fn", "window.mapReady===true");
    cdp = await connectCDP(base);
    const initial = await evaluate("window.viewport.state");
    assert.equal(initial.scale, 1);
    assert.equal(await evaluate("document.querySelector('canvas').parentElement.id"), "wrapper");
    await evaluate("document.querySelector('#wrapper').scrollTo(180,120)");
    const point = await evaluate("(()=>{const r=document.querySelector('#wrapper').getBoundingClientRect();return{x:r.left+350,y:r.top+260}})()");
    const anchor = await evaluate(`window.viewport.logicalPoint(${point.x},${point.y})`);
    await cdp.call("Input.dispatchMouseEvent", { type: "mouseWheel", x: point.x, y: point.y, deltaX: 0, deltaY: -240 });
    await browser("wait", "--fn", "window.viewport.state.scale>1.2");
    const zoomed = await evaluate(`({state:window.viewport.state,point:window.viewport.logicalPoint(${point.x},${point.y})})`);
    assert.ok(Math.abs(anchor.x - zoomed.point.x) < 1 && Math.abs(anchor.y - zoomed.point.y) < 1, JSON.stringify({ anchor, zoomed }));
    report.checks.push({ name: "native-wheel-anchor", anchor, zoomed });
    const scaleBeforeOutside = zoomed.state.scale;
    await cdp.call("Input.dispatchMouseEvent", { type: "mouseWheel", x: 950, y: 700, deltaX: 0, deltaY: 240 });
    await browser("wait", "--fn", "scrollY>0");
    assert.equal((await evaluate("window.viewport.state")).scale, scaleBeforeOutside);
    await evaluate("scrollTo(0,0)");
    const beforeDrag = await evaluate("window.viewport.state");
    await cdp.call("Input.dispatchMouseEvent", { type: "mouseMoved", x: point.x, y: point.y });
    await cdp.call("Input.dispatchMouseEvent", { type: "mousePressed", x: point.x, y: point.y, button: "left", buttons: 1, clickCount: 1 });
    for (let step = 1; step <= 4; step++) await cdp.call("Input.dispatchMouseEvent", { type: "mouseMoved", x: point.x - step * 25, y: point.y - step * 12, button: "left", buttons: 1 });
    await cdp.call("Input.dispatchMouseEvent", { type: "mouseReleased", x: point.x - 100, y: point.y - 48, button: "left", buttons: 0, clickCount: 1 });
    const afterDrag = await evaluate("({state:window.viewport.state,clicks:window.clicks})");
    assert.ok(afterDrag.state.scrollLeft > beforeDrag.scrollLeft + 80, JSON.stringify({ beforeDrag, afterDrag }));
    assert.equal(afterDrag.clicks.length, 0, "drag triggered a game click");
    report.checks.push({ name: "native-drag-no-click", beforeDrag, afterDrag });
    const target = await evaluate("(()=>{const r=document.querySelector('#wrapper').getBoundingClientRect();const x=r.left+200,y=r.top+180;return{x,y,logical:window.viewport.logicalPoint(x,y)}})()");
    await cdp.call("Input.dispatchMouseEvent", { type: "mousePressed", x: target.x, y: target.y, button: "left", buttons: 1, clickCount: 1 });
    await cdp.call("Input.dispatchMouseEvent", { type: "mouseReleased", x: target.x, y: target.y, button: "left", buttons: 0, clickCount: 1 });
    const clicked = await evaluate("window.clicks");
    assert.equal(clicked.length, 1);
    assert.equal(clicked[0].trusted, true);
    assert.ok(Math.abs(clicked[0].x - target.logical.x) < 0.1 && Math.abs(clicked[0].y - target.logical.y) < 0.1);
    await browser("screenshot", join(output, "desktop.png"));
    await browser("click", "[data-map-fit]");
    const fitted = await evaluate("window.viewport.state");
    assert.ok(fitted.scale < 1 && fitted.maxScrollLeft < 2 && fitted.maxScrollTop < 2, JSON.stringify(fitted));
    const fitCenters = await evaluate("(()=>{const a=document.querySelector('canvas').getBoundingClientRect(),b=document.querySelector('#wrapper').getBoundingClientRect();return{canvasX:Math.round((a.left+a.width/2)*1000),canvasY:Math.round((a.top+a.height/2)*1000),wrapperX:Math.round((b.left+b.width/2)*1000),wrapperY:Math.round((b.top+b.height/2)*1000)}})()");
    assert.ok(Math.abs(fitCenters.canvasX-fitCenters.wrapperX)<=2000 && Math.abs(fitCenters.canvasY-fitCenters.wrapperY)<=2000, `fitted map is not centered: ${JSON.stringify(fitCenters)}`);
    for (let count=0;count<6;count++) await browser("click", "[data-map-zoom-out]");
    assert.equal(Math.round((await evaluate("window.viewport.state")).scale*1000000),Math.round(fitted.scale*1000000),"zoom-out shrinks below a useful whole-map scale");
    report.checks.push({name:"fit-centered-and-minimum-useful-scale",fitCenters});
    await browser("click", "[data-map-reset]");
    assert.equal((await evaluate("window.viewport.state")).scale, 1);
    await evaluate("document.querySelector('#wrapper').style.scale='0.8';document.querySelector('#wrapper').scrollTo(180,120)");
    const scaledPoint = await evaluate("(()=>{const r=document.querySelector('#wrapper').getBoundingClientRect();return{x:r.left+r.width/2,y:r.top+r.height/2}})()");
    const scaledBefore = await evaluate(`window.viewport.logicalPoint(${scaledPoint.x},${scaledPoint.y})`);
    await cdp.call("Input.dispatchMouseEvent", { type: "mouseWheel", ...scaledPoint, deltaX: 0, deltaY: -100 });
    const scaledAfter = await evaluate(`window.viewport.logicalPoint(${scaledPoint.x},${scaledPoint.y})`);
    assert.ok(Math.abs(scaledBefore.x - scaledAfter.x) < 1 && Math.abs(scaledBefore.y - scaledAfter.y) < 1, JSON.stringify({ scaledBefore, scaledAfter }));
    report.checks.push({ name: "scaled-parent-anchor", scaledBefore, scaledAfter });
    await browser("set", "viewport", "320", "740");
    await cdp.call("Emulation.setDeviceMetricsOverride", { width: 320, height: 740, deviceScaleFactor: 1, mobile: true });
    await cdp.call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 2 });
    await browser("reload");
    await browser("wait", "--fn", "window.mapReady===true");
    assert.equal((await evaluate("window.viewport.state")).scale, 1, "mobile did not start at native size");
    const mobilePoint = await evaluate("(()=>{const r=document.querySelector('#wrapper').getBoundingClientRect();return{x:r.left+160,y:r.top+180}})()");
    await cdp.call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ ...mobilePoint, id: 1 }] });
    for (let step = 1; step <= 4; step++) await cdp.call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: mobilePoint.x - step * 24, y: mobilePoint.y - step * 15, id: 1 }] });
    await cdp.call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    const touchDrag = await evaluate("({state:window.viewport.state,clicks:window.clicks})");
    assert.ok(touchDrag.state.scrollLeft > 70 && touchDrag.state.scrollTop > 40, JSON.stringify(touchDrag));
    assert.equal(touchDrag.clicks.length, 0);
    const pinchBefore = touchDrag.state.scale;
    await cdp.call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: 100, y: 200, id: 1 }, { x: 220, y: 260, id: 2 }] });
    await cdp.call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: 70, y: 180, id: 1 }, { x: 250, y: 280, id: 2 }] });
    await cdp.call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    const pinched = await evaluate("({state:window.viewport.state,clicks:window.clicks})");
    assert.ok(pinched.state.scale > pinchBefore * 1.25, JSON.stringify(pinched));
    assert.equal(pinched.clicks.length, 0);
    report.checks.push({ name: "native-touch-drag-pinch", touchDrag, pinched });
    const tower = await evaluate("window.viewport.pointToClient(300,280)");
    await cdp.call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ ...tower, id: 1 }] });
    await cdp.call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await browser("wait", "--fn", "window.clicks.length===1");
    const towerClick = await evaluate("({click:window.clicks[0],width:innerWidth,canvas:document.querySelector('canvas').getBoundingClientRect().toJSON()})");
    assert.equal(towerClick.width, 320);
    assert.equal(towerClick.click.trusted, true);
    assert.ok(Math.abs(towerClick.click.x - 300) < 1 && Math.abs(towerClick.click.y - 280) < 1, JSON.stringify(towerClick));
    await browser("screenshot", join(output, "mobile-tower.png"));
    report.checks.push({ name: "native-touch-rendered-tower", tower, towerClick });
    await cdp.call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: 150, y: 250, id: 3 }] });
    await evaluate("dispatchEvent(new Event('blur'))");
    assert.equal((await evaluate("window.viewport.state")).pointers, 0);
    await cdp.call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    assert.equal((await evaluate("window.clicks")).length, 1, "old touch committed a click after blur");
    report.checks.push({ name: "blur-cancels-old-touch", state: await evaluate("window.viewport.state"), clicks: await evaluate("window.clicks.length") });
    await cdp.call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: 150, y: 250, id: 3 }] });
    await cdp.call("Input.dispatchTouchEvent", { type: "touchCancel", touchPoints: [] });
    assert.equal((await evaluate("window.viewport.state")).pointers, 0);
    report.checks.push({ name: "native-pointer-cancel", state: await evaluate("window.viewport.state") });
    await evaluate("document.querySelector('canvas').width=2160;document.querySelector('canvas').height=1872;window.viewport.update()");
    const large = await evaluate("window.viewport.state");
    assert.equal(large.width, 2160);
    assert.equal(large.height, 1872);
    assert.equal(large.scale, 1);
    await browser("click", "[data-map-fit]");
    const mobileFit = await evaluate("({state:window.viewport.state,overflow:document.documentElement.scrollWidth>innerWidth})");
    assert.equal(mobileFit.overflow, false);
    assert.ok(mobileFit.state.maxScrollLeft < 2 && mobileFit.state.maxScrollTop < 2);
    await browser("screenshot", join(output, "mobile.png"));
    report.checks.push({ name: "mobile-large-map-fit", large, mobileFit });
    await browser("set", "viewport", "480", "320");
    await cdp.call("Emulation.setDeviceMetricsOverride", { width: 480, height: 320, deviceScaleFactor: 1, mobile: true });
    await browser("wait", "--fn", "innerWidth===480&&window.viewport.state.scale>0.2");
    const rotated = await evaluate("window.viewport.state");
    assert.equal(rotated.pointers, 0);
    assert.ok(rotated.maxScrollLeft < 2 && rotated.maxScrollTop < 2, JSON.stringify(rotated));
    report.checks.push({ name: "resized-viewport-remains-fitted", rotated });
    await evaluate("window.viewport.destroy()");
    assert.equal(await evaluate("document.querySelector('canvas').parentElement.id"), "wrapper");
    assert.equal(await evaluate("document.querySelector('canvas').style.getPropertyValue('width')"), "");
    assert.equal(await evaluate("document.querySelector('#wrapper').style.getPropertyValue('--scroll-pct')"), "");
    report.checks.push({ name: "destroy-restores-owned-styles", styles: await evaluate("({canvas:document.querySelector('canvas').getAttribute('style'),wrapper:document.querySelector('#wrapper').getAttribute('style')})") });
    assert.deepEqual((await browser("errors")).errors, []);
    report.passed = true;
  } finally {
    if (cdp) cdp.close();
    try { await browser("close"); } catch {}
    await new Promise(resolve => server.close(resolve));
    await rm(profile, { recursive: true, force: true });
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
