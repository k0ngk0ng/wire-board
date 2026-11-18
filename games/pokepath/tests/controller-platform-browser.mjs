import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const owned = JSON.parse(readFileSync(join(root, ".local/pokepath/native-browser/latest.json"), "utf8"));
process.kill(owned.pid, 0);
assert.equal(owned.mutedByBrowserFlag, true);
const output = join(root, ".local/pokepath/verification/controller-platform");
mkdirSync(output, { recursive: true });
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter" };
const browser = (...args) => {
  const result = JSON.parse(execFileSync("agent-browser", ["--session", owned.browsers.a.session, ...args, "--json"], { cwd: root, env: environment, encoding: "utf8", timeout: 45000, maxBuffer: 4 << 20 }));
  assert.equal(result.success, true, JSON.stringify(result));
  return result.data;
};
const evaluate = source => browser("eval", "-b", Buffer.from(source).toString("base64")).result;
const api = (path, body) => {
  const options = body === undefined ? "{}" : `{method:'POST',headers:{'Content-Type':'application/json'},body:${JSON.stringify(JSON.stringify(body))}}`;
  const result = evaluate(`fetch(${JSON.stringify(path)},${options}).then(async response=>({status:response.status,body:await response.json()}))`);
  assert.equal(result.status, 200, JSON.stringify(result));
  return result.body;
};
const data = () => api("/api/state").room.game.data;
const until = async (predicate, name) => {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw Error(`Timed out: ${name}`);
};
const frames = () => evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))");
const reload = async () => { browser("reload"); browser("wait", "[data-pokepath-canvas]"); await until(() => evaluate("Boolean(document.querySelector('[data-controller-navigation=pokepath]'))"), "actual navigation scheme attached"); };
const pad = async (index, pressed) => { evaluate(`window.pokepathVerificationPad.buttons[${index}]={pressed:${pressed},touched:${pressed},value:${pressed ? 1 : 0}};true`); frames(); };
const report = { command: "node games/pokepath/tests/controller-platform-browser.mjs", passed: false, gamepad: "Virtual standard Gamepad API; no physical hardware compatibility claim", fixture: owned.output, checks: [] };
let original;
let revision;
try {
  const settings = api("/api/controllers?game=pokepath");
  original = settings.profiles["game:pokepath"];
  const changed = structuredClone(original);
  changed.keyboard.profile = "KeyP";
  revision = api("/api/controllers/game:pokepath", changed).revision;
  await reload();
  assert.ok(data().health > 0);
  browser("focus", "[data-pokepath-canvas]");
  browser("press", "q");
  assert.equal(evaluate("Boolean(document.querySelector('.pokepath-classic-profile'))"), false);
  browser("press", "p");
  await until(() => evaluate("Boolean(document.querySelector('.pokepath-classic-profile'))"), "remapped native key opens the actual profile");
  browser("press", "Backspace");
  await until(() => evaluate("!document.querySelector('.pokepath-classic-profile')"), "mapped cancel closes the real game dialog");
  report.checks.push("HTTP persisted scheme loaded by real navigation", "original Q blocked after remap", "new P opens profile", "mapped cancel closes profile");
  browser("click", '[data-classic-section="settings"]');
  report.shortcutProfileKey = evaluate("document.querySelectorAll('.pokepath-settings-shortcuts kbd')[1]?.textContent");
  assert.equal(report.shortcutProfileKey, "P", "the actual saved mapping must also drive the visible shortcut hint");
  browser("screenshot", join(output, "remapped-hints.png"));
  browser("press", "Backspace");
  report.checks.push("visible shortcut hints reflect the active persisted scheme");
  browser("focus", "[data-pokepath-canvas]");
  evaluate("window.pokepathVerificationPad={id:'Pokepath virtual verification',index:0,mapping:'standard',connected:true,timestamp:performance.now(),axes:[0,0,0,0],buttons:Array.from({length:16},()=>({pressed:false,touched:false,value:0}))};Object.defineProperty(navigator,'getGamepads',{configurable:true,value:()=>[window.pokepathVerificationPad]});true");
  frames();
  await pad(15, true);
  await until(() => evaluate("document.querySelector('[data-pokepath-canvas]').dataset.controllerX!==undefined"), "virtual dpad moves a visible map cursor");
  await pad(15, false);
  const cursor = evaluate("(()=>{const canvas=document.querySelector('[data-pokepath-canvas]');return{x:Number(canvas.dataset.controllerX),y:Number(canvas.dataset.controllerY)}})()");
  await pad(8, true);
  await until(() => evaluate("Boolean(document.querySelector('.pokepath-classic-profile'))"), "virtual shortcut opens the profile through the shared action handler");
  await pad(8, false);
  await pad(1, true);
  await until(() => evaluate("!document.querySelector('.pokepath-classic-profile')"), "virtual cancel closes the profile");
  await pad(1, false);
  browser("focus", "[data-pokepath-canvas]");
  const before = data();
  await pad(7, true);
  await until(() => data().speed !== before.speed, "server accepts the actual virtual speed input");
  const after = data();
  frames();
  assert.equal(data().speed, after.speed);
  await pad(7, false);
  assert.equal(evaluate("window.__nativeAudioAttempts.length"), 0);
  browser("screenshot", join(output, "mapped-game.png"));
  report.checks.push("virtual dpad moves a cursor without deploying", "virtual standard shortcut and cancel", "virtual discrete speed reaches authority once", "global mute retained");
  report.cursor = cursor;
  report.authority = { beforeSpeed: before.speed, afterSpeed: after.speed, beforeSequence: before.sequence, afterSequence: after.sequence, towersUnchanged: before.towers.length === after.towers.length };
  assert.equal(report.authority.towersUnchanged, true);
  report.passed = true;
} catch (error) {
  report.error = error.stack ?? String(error);
  try { browser("screenshot", join(output, "failure.png")); report.snapshot = browser("snapshot", "-i"); } catch {}
  process.exitCode = 1;
} finally {
  if (original) {
    try { original.revision = revision; api("/api/controllers/game:pokepath", original); await reload(); }
    catch (error) { report.passed = false; report.cleanupError = String(error); process.exitCode = 1; }
  }
  writeFileSync(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  process.stdout.write(JSON.stringify({ passed: report.passed, checks: report.checks.length, error: report.error }) + "\n");
}
