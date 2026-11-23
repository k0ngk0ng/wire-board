import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, writeFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const base = process.env.SOLO_REMATCH_URL;
assert.match(base ?? "", /^http:\/\/127\.0\.0\.1:\d+\/?$/);
const out = path.join(root, ".local/gomoku/solo-rematch");
const temporary = path.join(out, "browser-temporary");
const env = { ...process.env, TMPDIR: path.join(root, ".local/gomoku/t"), AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/gomoku/s"), AGENT_BROWSER_NO_VERSION_CHECK: "1", XDG_CONFIG_HOME: path.join(temporary, "config"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CACHE_HOME: path.join(temporary, "cache") };
for (const folder of [out, temporary, env.TMPDIR, env.AGENT_BROWSER_SOCKET_DIR, env.XDG_CONFIG_HOME, env.XDG_DATA_HOME, env.XDG_CACHE_HOME]) await mkdir(folder, { recursive: true });
const execute = promisify(execFile);
const session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "qr"], { cwd: root, env })).stdout.trim();
const browser = async (...args) => { const result = await execute("agent-browser", ["--session", session, "--profile", path.join(temporary, "profile"), ...args, "--json"], { cwd: root, env, timeout: 30000 }); const value = JSON.parse(result.stdout); assert.ok(value.success, result.stdout); return value.data; };
const evaluate = async source => { const value = await browser("eval", source); return value.result ?? value; };
const api = (suffix, body) => evaluate(`(async()=>{const response=await fetch('/api'+${JSON.stringify(suffix)},{method:${body ? '"POST"' : '"GET"'},headers:{'Content-Type':'application/json'},body:${body ? JSON.stringify(JSON.stringify(body)) : 'undefined'}});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value})()`);
const command = async (type, extra = {}) => { const state = await api("/state"); return api("/rooms/" + state.room.id, { type, ...extra, version: state.room.version, nonce: crypto.randomUUID() }); };
const report = { passed: false, scope: "Actual application, authenticated Go rooms, one human plus bot, controlled Gomoku position with one legal winning move; native browser click restarts the same room with unchanged options" };
try {
  await browser("open", base);
  await api("/register", { name: "重开验证" + Date.now().toString().slice(-7), password: "solo-rematch-test-only", invite: "gamepad-browser" });
  await browser("reload");
  await api("/rooms", { kind: "gomoku", capacity: 2, name: "一键重开验收", options: { rules: "freestyle", difficulty: "easy", turnSeconds: 120 } });
  await command("add_bot");
  await command("ready");
  await command("start");
  await browser("wait", "--fn", "!!document.querySelector('[data-gomoku-cell=\"112\"]')");
  await browser("wait", "--fn", "!!document.querySelector('[data-gomoku-cell=\"112\"]:not(:disabled)')");
  await browser("click", '[data-gomoku-cell="112"]');
  await browser("wait", "--fn", "!!document.querySelector('[data-gomoku-confirm]:not(:disabled)')");
  await browser("click", "[data-gomoku-confirm]");
  await browser("wait", ".match-result-modal .result-restart");
  const before = (await api("/state")).room;
  await browser("screenshot", path.join(out, "result-before.png"));
  await evaluate("(()=>{window.quickRematchCommands=[];const original=window.fetch;window.fetch=(url,options)=>{if(options?.body){try{const type=JSON.parse(options.body).type;if(['rematch','ready','start'].includes(type))window.quickRematchCommands.push(type)}catch{}}return original(url,options)};return true})()");
  await browser("click", ".result-restart");
  await browser("wait", "--fn", "!document.querySelector('.match-result-modal')&&!!document.querySelector('[data-gomoku-cell]')");
  let after;
  for (let index = 0; index < 50; index++) {
    after = (await api("/state")).room;
    if (after.status === "playing" && !after.loading && after.matchId !== before.matchId) break;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  report.commands = await evaluate("window.quickRematchCommands");
  assert.deepEqual(report.commands, ["rematch", "ready", "start"]);
  assert.equal(after.id, before.id);
  assert.equal(after.status, "playing");
  assert.notEqual(after.matchId, before.matchId);
  assert.deepEqual(after.options, before.options);
  assert.equal(after.capacity, before.capacity);
  assert.deepEqual(after.seats.map(seat => seat.id), before.seats.map(seat => seat.id));
  report.room = { id: after.id, capacity: after.capacity, options: after.options, beforeMatch: before.matchId, nextMatch: after.matchId };
  await browser("screenshot", path.join(out, "new-game.png"));
  report.passed = true;
  await command("close");
  await command("leave");
} catch (error) {
  report.error = error.stack;
  await browser("screenshot", path.join(out, "failure.png")).catch(() => {});
  process.exitCode = 1;
} finally {
  await writeFile(path.join(out, "result.json"), JSON.stringify(report, null, 2) + "\n");
  await browser("close").catch(() => {});
  await rm(temporary, { recursive: true, force: true });
}
