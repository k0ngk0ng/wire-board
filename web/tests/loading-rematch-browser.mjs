import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, writeFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const base = process.env.LOADING_REMATCH_URL;
assert.match(base ?? "", /^http:\/\/127\.0\.0\.1:\d+\/?$/);
const out = path.join(root, ".local/yuyuhakusho/loading-rematch");
const temporary = path.join(out, "temporary");
const env = { ...process.env, TMPDIR: path.join(root, ".local/yuyuhakusho/t/lr"), AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/yuyuhakusho/s"), AGENT_BROWSER_NO_VERSION_CHECK: "1", XDG_CONFIG_HOME: path.join(temporary, "config"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CACHE_HOME: path.join(temporary, "cache") };
for (const folder of [out, temporary, env.TMPDIR, env.AGENT_BROWSER_SOCKET_DIR, env.XDG_CONFIG_HOME, env.XDG_DATA_HOME, env.XDG_CACHE_HOME]) await mkdir(folder, { recursive: true });
const execute = promisify(execFile);
const session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "lr"], { cwd: root, env })).stdout.trim();
const browser = async (...args) => {
  const result = await execute("agent-browser", ["--session", session, "--profile", path.join(temporary, "profile"), ...args, "--json"], { cwd: root, env, timeout: 30000 });
  const value = JSON.parse(result.stdout);
  assert.ok(value.success, result.stdout);
  return value.data;
};
const evaluate = async source => (await browser("eval", source)).result;
const api = (suffix, body) => evaluate(`(async()=>{const response=await fetch('/api'+${JSON.stringify(suffix)},{method:${body ? '"POST"' : '"GET"'},headers:{'Content-Type':'application/json'},body:${body ? JSON.stringify(JSON.stringify(body)) : 'undefined'}});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value})()`);
const command = async type => { const { room } = await api("/state"); return api("/rooms/" + room.id, { type, version: room.version, nonce: crypto.randomUUID() }); };
const report = { passed: false, strictMode: true, fixture: "Real terminal checkpoint, original game timer and rules, authenticated Go loading barrier and native rematch button", checks: [] };
let socket;
let serial = 0;
const pending = new Map();
const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const id = ++serial; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, sessionId })); });
try {
  await browser("open", base);
  const endpoint = await browser("get", "cdp-url");
  socket = new WebSocket(typeof endpoint === "string" ? endpoint : endpoint.cdpUrl);
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  socket.onmessage = ({ data }) => { const value = JSON.parse(data); const request = pending.get(value.id); if (!request) return; pending.delete(value.id); value.error ? request.reject(Error(JSON.stringify(value.error))) : request.resolve(value.result); };
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(value => value.type === "page" && value.url.startsWith(base));
  const { sessionId } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true });
  await call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 }, sessionId);
  await browser("set", "viewport", "1174", "744");
  await api("/register", { name: "重开回归" + Date.now().toString().slice(-7), password: "loading-rematch-local", invite: "gamepad-browser" });
  await browser("reload");
  await evaluate(`(()=>{Element.prototype.requestFullscreen=()=>Promise.reject(Error('fixture page fullscreen'));window.loadingTrace={begins:[],ready:[],reordered:false,rejectNext:false};const original=window.fetch.bind(window);let first;window.fetch=async(url,options)=>{let body;try{body=JSON.parse(options?.body)}catch{}if(body?.type==='load_begin'){loadingTrace.begins.push(body.loadClient);if(!loadingTrace.reordered){if(!first)return new Promise((resolve,reject)=>{first={url,options,resolve,reject}});loadingTrace.reordered=true;const response=await original(url,options);try{first.resolve(await original(first.url,first.options))}catch(error){first.reject(error)}return response}}if(body?.type==='load_ready'){loadingTrace.ready.push(body.loadClient);if(loadingTrace.rejectNext){loadingTrace.rejectNext=false;return new Response(JSON.stringify({error:'回归测试：加载确认失败'}),{status:409,headers:{'Content-Type':'application/json'}})}}return original(url,options)};return true})()`);
  await api("/rooms", { kind: "yuyuhakusho", capacity: 4, name: "加载重开回归", options: { arena: 0, characters: [7, 0, 3, 10] } });
  for (let seat = 1; seat < 4; seat++) await command("add_bot");
  await command("ready");
  await command("start");
  await browser("wait", "--fn", "window.loadingTrace.reordered&&window.loadingTrace.ready.length>0");
  await browser("wait", "--fn", "!document.querySelector('[data-game-loading]')||!!document.querySelector('[data-game-loading] [role=alert]')");
  report.race = await evaluate("structuredClone(loadingTrace)");
  await browser("screenshot", path.join(out, "after-reordered-start.png"));
  assert.equal(await evaluate("document.querySelector('[data-game-loading] [role=alert]')?.textContent??''"), "", "the same page replaced its own loading session");
  assert.equal(new Set(report.race.begins).size, 1);
  const before = (await api("/state")).room;
  assert.ok(!before.loading);
  for (let round = 0; round < 3; round++) {
    await browser("wait", ".result-restart");
    if (round < 2) await evaluate("loadingTrace.rejectNext=true;true");
    await browser("click", ".result-restart");
    if (round < 2) {
      await browser("wait", "[data-game-loading] [role=alert]");
      const cover = await evaluate("(()=>{const cover=document.querySelector('[data-game-loading]');return {fullscreen:!!cover.closest('[data-room-fullscreen]'),inert:!!cover.closest('[inert]'),visible:getComputedStyle(cover).visibility,buttons:[...cover.querySelectorAll('button')].map(button=>({text:button.textContent,visible:button.getClientRects().length>0}))}})()");
      report.errorCover = cover;
      await browser("screenshot", path.join(out, "recoverable-error.png"));
      assert.equal(cover.fullscreen, true);
      assert.equal(cover.inert, false);
      assert.equal(cover.visible, "visible");
      if (round === 1) {
        await browser("click", 'button[aria-label="退出全屏"]');
        assert.equal(await evaluate("!!document.querySelector('[data-game-loading]').closest('[inert]')||!!document.querySelector('[data-room-fullscreen]')"), false);
      }
      await browser("click", '[data-game-loading] button.primary');
    }
    await browser("wait", "--fn", "!document.querySelector('[data-game-loading]')&&!document.querySelector('.match-result-modal')");
    const room = (await api("/state")).room;
    assert.equal(room.id, before.id);
    assert.ok(["playing", "finished"].includes(room.status));
    assert.ok(!room.loading);
    assert.notEqual(room.matchId, before.matchId);
    assert.deepEqual(room.options, before.options);
    assert.deepEqual(room.seats.map(seat => seat.id), before.seats.map(seat => seat.id));
    report.checks.push({ round, match: room.matchId, loaded: true });
  }
  await browser("wait", ".result-review");
  await browser("click", ".result-review");
  await browser("click", 'button[aria-label="退出全屏"]');
  await browser("wait", "--fn", "!document.querySelector('[data-room-fullscreen]')");
  report.afterExit = await evaluate("({fullscreen:!!document.querySelector('[data-room-fullscreen]'),inert:[...document.querySelectorAll('[inert]')].map(element=>({tag:element.tagName,class:element.className,text:element.textContent.slice(0,80)}))})");
  assert.equal(await evaluate("!!document.querySelector('[data-room-fullscreen]')||!!document.querySelector('header[inert]')"), false);
  await browser("screenshot", path.join(out, "three-rematches.png"));
  report.passed = true;
} catch (error) {
  report.error = error.stack;
  await browser("screenshot", path.join(out, "failure.png")).catch(() => {});
  throw error;
} finally {
  await writeFile(path.join(out, "result.json"), JSON.stringify(report, null, 2) + "\n");
  socket?.close();
  await browser("close").catch(() => {});
  await rm(temporary, { recursive: true, force: true });
  await rm(env.TMPDIR, { recursive: true, force: true });
}
