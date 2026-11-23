import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, writeFile, readFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const base = process.env.CONTROLLER_ROOM_URL;
assert.match(base ?? "", /^http:\/\/127\.0\.0\.1:\d+\/?$/);
const temporary = path.join(root, ".local/yuyuhakusho/real-touch/browser-temporary");
for (const name of ["profile", "config", "data", "cache"]) await mkdir(path.join(temporary, name), { recursive: true });
const env = { ...process.env, TMPDIR: temporary, XDG_CONFIG_HOME: path.join(temporary, "config"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CACHE_HOME: path.join(temporary, "cache"), AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/yuyuhakusho/s"), AGENT_BROWSER_PROFILE: path.join(temporary, "profile"), AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const execute = promisify(execFile);
const session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "rr"], { cwd: root, env })).stdout.trim();
const browser = async (...args) => { const { stdout } = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env, timeout: 30000 }); const value = JSON.parse(stdout); assert.ok(value.success, stdout); return value.data; };
await browser("--profile", path.join(temporary, "profile"), "open", base);
const endpoint = await browser("get", "cdp-url");
const socket = new WebSocket(endpoint.cdpUrl);
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0;
const pending = new Map();
socket.onmessage = ({ data }) => { const value = JSON.parse(data); const request = pending.get(value.id); if (!request) return; pending.delete(value.id); value.error ? request.reject(Error(JSON.stringify(value.error))) : request.resolve(value.result); };
const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const id = ++serial; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, sessionId })); });
const { targetInfos } = await call("Target.getTargets");
const { sessionId } = await call("Target.attachToTarget", { targetId: targetInfos.find(value => value.type === "page" && value.url.startsWith(base)).targetId, flatten: true });
const evaluate = async expression => { const value = await call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }, sessionId); assert.ok(!value.exceptionDetails, JSON.stringify(value.exceptionDetails)); return value.result.value; };
const api = async (suffix, body) => evaluate(`(async()=>{const response=await fetch('/api'+${JSON.stringify(suffix)},{method:${body ? '"POST"' : '"GET"'},headers:{'Content-Type':'application/json'},body:${body ? JSON.stringify(JSON.stringify(body)) : 'undefined'}});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value})()`);
const command = async type => { const state = await api("/state"); return api("/rooms/" + state.room.id, { type, version: state.room.version, nonce: crypto.randomUUID() }); };
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
const wait = async (name, predicate, timeout = 90000) => { const deadline = Date.now() + timeout; while (Date.now() < deadline) { const value = await predicate(); if (value) return value; await delay(100); } throw Error("Timed out: " + name); };
const report = { passed: false, scope: "Normal built app with real Go authentication, SQLite rooms, WebSocket input, native game runtimes and game rendering", games: [] };
await call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 }, sessionId);
const viewport = async (width, height) => { await call("Emulation.setDeviceMetricsOverride", { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: true, screenOrientation: { type: width > height ? "landscapePrimary" : "portraitPrimary", angle: width > height ? 90 : 0 } }, sessionId); await delay(120); };
try {
  await api("/register", { name: "触控实测" + Date.now().toString().slice(-8), password: "controller-room-test-only", invite: "gamepad-browser" });
  await evaluate("localStorage.setItem('wb_sound','off');true");
  for (const kind of ["yuyuhakusho", "contra"]) {
    const directory = path.join(root, ".local", kind, "real-touch");
    await mkdir(directory, { recursive: true });
    const current = { kind, passed: false, checks: [], captures: [] };
    report.games.push(current);
    const previous = (await api("/state")).room;
    if (previous) {
      if (previous.status === "playing") await command("close");
      await command("leave");
    }
    await api("/rooms", { kind, capacity: kind === "contra" ? 1 : 2, options: kind === "contra" ? {} : { arena: 2, characters: [0, 1], teamBattle: false }, name: kind === "contra" ? "魂斗罗真实触控" : "幽游白书真实触控" });
    if (kind === "yuyuhakusho") await command("add_bot");
    await browser("reload");
    await viewport(844, 390);
    if ((await api("/state")).room.status !== "playing") {
      await browser("find", "role", "button", "click", "--name", "我准备好了", "--exact");
      await browser("wait", "--fn", "[...document.querySelectorAll('button')].some(button=>button.textContent.trim()==='开始游戏'&&!button.disabled)");
      await browser("find", "role", "button", "click", "--name", "开始游戏", "--exact");
    } else await browser("reload");
    await wait("real game ready", async () => {
      const state = await api("/state");
      const ready = await evaluate(`!!document.querySelector('${kind === "contra" ? "[data-contra-runtime]" : "[data-yuyu-board]"}')&&!!document.querySelector('[data-controller-button=a]:not(:disabled)')`);
      return ready && !state.room.loading && (kind === "contra" || state.room.game.data.phase === "playing");
    });
    current.room = (await api("/state")).room.id;
    if (kind === "contra") {
      const start = await evaluate("(()=>{const r=document.querySelector('[data-controller-button=start]').getBoundingClientRect();return {id:7,x:r.x+r.width/2,y:r.y+r.height/2}})()");
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [start] }, sessionId);
      await delay(100);
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] }, sessionId);
      await wait("Contra actual level playing", () => evaluate("document.querySelector('[data-contra-runtime]')?.dataset.phase==='playing'"));
      await delay(2800);
    }
    console.log("Real " + kind + " ready in room " + current.room);
    if (kind === "yuyuhakusho") {
      await viewport(844, 390);
      for (const [button, expected] of [["ac", 96], ["abc", 112]]) {
        const point = await evaluate(`(()=>{const r=document.querySelector('[data-controller-button=${button}]').getBoundingClientRect();return {id:3,x:r.x+r.width/2,y:r.y+r.height/2}})()`);
        await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [point] }, sessionId);
        const input = await wait("authoritative " + button, async () => { const room = (await api("/state")).room; return room.game.data.inputs[room.you].appliedButtons === expected ? room.game.data.inputs[room.you] : false; }, 10000);
        await browser("screenshot", path.join(directory, "phone-" + button + ".png"));
        current.checks.push({ combo: button, accepted: input });
        await call("Input.dispatchTouchEvent", { type: "touchCancel", touchPoints: [] }, sessionId);
      }
    }
    for (const [label, width, height] of [["phone", 844, 390], ["pad", 1180, 820], ["portrait", 390, 844]]) {
      await viewport(width, height);
      const geometry = await evaluate("(()=>{const box=e=>e.getBoundingClientRect().toJSON();const surface=document.querySelector('[data-room-fullscreen]');return {mode:surface?.dataset.roomFullscreen,game:box(document.querySelector('[data-game-focus]')),canvas:box(document.querySelector('[data-game-focus] canvas')),buttons:[...document.querySelectorAll('button[data-controller-button]')].map(e=>({name:e.dataset.controllerButton,box:box(e),disabled:e.disabled})),dpad:box(document.querySelector('[data-controller-dpad]'))}})()");
      assert.ok(geometry.mode, "automatic game fullscreen is missing");
      assert.equal(Math.round(geometry.canvas.top), 0, "fullscreen canvas is pushed down by a header row");
      const mainAction = geometry.buttons.find(value => value.name === "a").box;
      assert.ok(Math.abs(Math.round((geometry.dpad.top+geometry.dpad.height/2-mainAction.top-mainAction.height/2)*1000)) <= 1000, "actual direction and main action centers differ");
      assert.ok(geometry.buttons.every(value => !value.disabled), "game controls are disabled");
      assert.ok(geometry.buttons.filter(value => ["a", "b", "c", "x", "y", "z"].includes(value.name)).every(value => value.box.left >= width / 2), "action controls are not on the right side of the actual game");
      const chat = await evaluate("document.querySelector('.chat-toggle').getBoundingClientRect().toJSON()");
      assert.ok(geometry.buttons.every(({box}) => box.left >= chat.right || box.right <= chat.left || box.top >= chat.bottom || box.bottom <= chat.top), "chat covers an actual game control");
      const pixels = await evaluate("(()=>{const canvas=document.querySelector('[data-game-focus] canvas');const data=canvas.getContext('2d').getImageData(0,0,canvas.width,canvas.height).data;const colors=new Set();for(let i=0;i<data.length;i+=4)colors.add((data[i]<<16)|(data[i+1]<<8)|data[i+2]);return {width:canvas.width,height:canvas.height,colors:colors.size}})()");
      assert.ok(pixels.colors > 4, "real game canvas is blank");
      await browser("screenshot", path.join(directory, label + ".png"));
      const a = geometry.buttons.find(value => value.name === "a").box;
      const b = geometry.buttons.find(value => value.name === "b").box;
      const ab = { id: 2, x: (a.x + a.width / 2 + b.x + b.width / 2) / 2, y: (a.y + a.height / 2 + b.y + b.height / 2) / 2 };
      const d = geometry.dpad;
      const right = { id: 1, x: d.x + d.width * .8, y: d.y + d.height / 2 };
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [right, ab] }, sessionId);
      const expected = kind === "contra" ? 193 : 88;
      const accepted = await wait("authoritative AB plus right", async () => {
        const room = (await api("/state")).room;
        const input = room.game.data.inputs[room.you];
        return (input.held ?? input.appliedButtons) === expected ? { input, frame: room.game.data.frame ?? room.game.data.snapshot?.frame } : false;
      }, 10000);
      await browser("screenshot", path.join(directory, label + "-ab-held.png"));
      const highlit = await evaluate("[...document.querySelectorAll('button[data-controller-button][aria-pressed=true]')].map(e=>e.dataset.controllerButton)");
      assert.ok(highlit.includes("a") && highlit.includes("b"));
      await call("Input.dispatchTouchEvent", { type: "touchCancel", touchPoints: [] }, sessionId);
      await wait("authoritative release", async () => { const room = (await api("/state")).room; const input = room.game.data.inputs[room.you]; return (input.held ?? input.appliedButtons) === 0; }, 10000);
      current.captures.push({ label, width, height, geometry, pixels, accepted, highlit });
      console.log(kind + " " + label + " actual canvas and authoritative combo verified");
    }
    current.passed = true;
    await writeFile(path.join(directory, "result.json"), JSON.stringify(current, null, 2) + "\n");
  }
  report.passed = report.games.every(game => game.passed);
} catch (error) {
  report.error = error.stack;
  await browser("screenshot", path.join(root, ".local/yuyuhakusho/real-touch/failure.png")).catch(() => {});
  console.error(error);
  process.exitCode = 1;
} finally {
  report.sources = Object.fromEntries(await Promise.all(["web/src/controllers/FCController.tsx", "web/src/controllers/MDController.tsx", "web/src/controllers/pointer-controller.ts", "web/src/controllers/touch-overlay.css", "web/tests/controller-room-browser.mjs"].map(async name => [name, createHash("sha256").update(await readFile(path.join(root, name))).digest("hex")])));
  await writeFile(path.join(root, ".local/yuyuhakusho/real-touch/result.json"), JSON.stringify(report, null, 2) + "\n");
  socket.close();
  await browser("close").catch(() => {});
  await rm(temporary, { recursive: true, force: true });
}
