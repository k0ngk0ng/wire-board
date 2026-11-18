import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { randomUUID } from "node:crypto";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const owned = JSON.parse(readFileSync(join(root, ".local/pokepath/native-browser/latest.json"), "utf8"));
assert.equal(owned.mutedByBrowserFlag, true);
assert.equal(new URL(owned.base).hostname, "127.0.0.1");
const identity = randomUUID().slice(0, 8);
const output = join(root, ".local/pokepath/verification/network", identity);
mkdirSync(output, { recursive: true });
const environment = { ...process.env, AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", TMPDIR: join(root, ".local/tmp") };
const sleep = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const report = { command: "node games/pokepath/tests/presentation-network.mjs", identity, fixture: owned.output, passed: false, checks: [], presentations: {} };
const proxy = spawn(process.execPath, [join(root, "games/pokepath/tests/presentation-proxy.mjs"), owned.base], { cwd: root, env: environment, stdio: ["pipe", "pipe", "pipe"] });
const lines = readline.createInterface({ input: proxy.stdout })[Symbol.asyncIterator]();
const next = async () => {
  const line = await lines.next();
  assert.equal(line.done, false, "owned proxy exited");
  return JSON.parse(line.value);
};
const configure = async value => { proxy.stdin.write(JSON.stringify(value) + "\n"); return next(); };
function browser(label, ...args) {
  const result = JSON.parse(execFileSync("agent-browser", ["--session", owned.browsers[label].session, ...args, "--json"], { cwd: root, env: environment, encoding: "utf8", timeout: 45000, maxBuffer: 16 << 20 }));
  assert.equal(result.success, true, JSON.stringify(result));
  return result.data;
}
const evaluate = (label, expression) => browser(label, "eval", "-b", Buffer.from(expression).toString("base64")).result;
const state = label => evaluate(label, "fetch('/api/state').then(async r=>{if(!r.ok)throw Error('authenticated state unavailable');return r.json()})");
const data = label => state(label).room?.game?.data;
function post(label, path, body, allowConflict = false) {
  const result = evaluate(label, `fetch(${JSON.stringify(path)},{method:'POST',headers:{'Content-Type':'application/json'},body:${JSON.stringify(JSON.stringify(body))}}).then(async response=>({status:response.status,body:await response.json()}))`);
  if (allowConflict && result.status === 409) return undefined;
  assert.ok(result.status >= 200 && result.status < 300, JSON.stringify(result));
  return result.body;
}
function roomCommand(label, type) {
  for (let attempt = 0; attempt < 12; attempt++) {
    const room = state(label).room;
    const result = post(label, `/api/rooms/${room.id}`, { type, version: room.version, nonce: randomUUID() }, true);
    if (result !== undefined) return result;
  }
  throw Error(`Room command did not settle: ${type}`);
}
async function until(predicate, name, duration = 30000) {
  const deadline = Date.now() + duration;
  while (Date.now() < deadline) {
    const result = predicate();
    if (result) return result;
    await sleep(100);
  }
  throw Error(`Timed out: ${name}`);
}
async function running() {
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    const current = data("a");
    if (current.health <= 0) {
      browser("a", "click", ".pokepath-defeat button");
      await until(() => data("a").health > 0, "the real defeat retry is acknowledged");
    } else if (current.paused) {
      browser("a", "click", ".pokepath-classic-controls .pause");
      await until(() => !data("a").paused, "the real resume input is acknowledged");
    } else if (!current.waveActive) {
      browser("a", "click", ".pokepath-classic-controls .wave");
      await until(() => data("a").waveActive || data("a").health <= 0, "the real wave starts before measuring motion");
    } else if (current.enemies.some(enemy => {
      if (!enemy.entered || enemy.dead || enemy.escaped || enemy.health <= 0) return false;
      const path = current.paths?.[enemy.pathIndex ?? 0];
      if (!path?.length) return false;
      const next = Math.min(path.length - 1, (enemy.pathOriginWaypoint ?? 0) + (enemy.waypoint ?? 0));
      let distance = Math.hypot(path[next].x - enemy.x, path[next].y - enemy.y);
      for (let index = next + 1; index < path.length; index++) distance += Math.hypot(path[index].x - path[index - 1].x, path[index].y - path[index - 1].y);
      return distance / (Math.max(1, enemy.baseSpeed ?? 60) * current.speed) >= 10;
    })) return;
    await sleep(100);
  }
  throw Error("No real enemy entered the visible battlefield within the fixed setup deadline");
}
async function measure(label, url, name, selector = ".pokepath-classic-controls .auto", milliseconds = 8000) {
  const tag = `network-${identity}-${name}`;
  const args = [join(root, "games/pokepath/tests/presentation-browser.mjs"), owned.browsers[label].cdp, url, tag, String(milliseconds), "0", selector];
  const child = spawn(process.execPath, args, { cwd: root, env: environment, stdio: ["ignore", "pipe", "pipe"] });
  let diagnostic = "";
  child.stdout.on("data", bytes => { diagnostic += bytes; });
  child.stderr.on("data", bytes => { diagnostic += bytes; });
  const [code] = await once(child, "exit");
  writeFileSync(join(output, `${name}.log`), diagnostic);
  assert.equal(code, 0, diagnostic);
  const filename = join(root, `.local/pokepath/verification/presentation-${tag}.json`);
  const result = JSON.parse(readFileSync(filename, "utf8"));
  report.presentations[name] = filename;
  return result;
}
function responsive(result, name, motion = true) {
  assert.ok(result.nativeRAF.p95 <= 25, `${name}: RAF p95 ${result.nativeRAF.p95}`);
  assert.ok(result.pixelResponse.firstChangedCaptureMS?.upper <= 100, `${name}: actual input pixels exceeded 100 ms`);
  assert.ok(result.inputACK.count > 0 && !result.inputACK.timedOut, `${name}: input acknowledgment unavailable`);
  if (motion) {
    assert.ok(result.actualCanvasChanges.moving.milliseconds >= 3000, `${name}: insufficient active visible motion`);
    assert.ok(result.actualCanvasChanges.moving.perSecond >= 55, `${name}: actual moving pixels ${result.actualCanvasChanges.moving.perSecond}/s`);
  }
  assert.ok(result.authorityFrames.length > 0);
  assert.ok(result.authorityFrames.every(frame => !frame.snapshot), `${name}: ordinary stream resent full states`);
  assert.ok(result.authorityToPresentationMS.count > 0, `${name}: authority presentation timing unavailable`);
}

let proxyURL;
try {
  proxyURL = (await next()).url;
  browser("b", "open", proxyURL);
  browser("b", "set", "viewport", "1280", "720");
  browser("b", "wait", "[data-pokepath-canvas]");
  await until(() => !state("a").room.loading && !state("b").room.loading, "both native clients ready");
  for (const label of ["a", "b"]) {
    assert.equal(state(label).room.id, owned.roomID);
    assert.equal(evaluate(label, "localStorage.getItem('wb_sound')"), "off");
  }
  await running();
  const normal = await measure("a", owned.base, "normal");
  responsive(normal, "normal");
  report.checks.push({ name: "normal-authenticated-two-player", actualMovingPixelsPerSecond: normal.actualCanvasChanges.moving.perSecond });
  await configure({ delayMS: 120, reset: true });
  await running();
  const [fast, slow] = await Promise.all([measure("a", owned.base, "fast"), measure("b", proxyURL, "slow", ".pokepath-classic-controls .pause")]);
  responsive(fast, "fast");
  responsive(slow, "slow");
  assert.ok(slow.transportRTT.p50 >= 200, "the actual slow transport RTT was not observed");
  assert.ok(slow.inputACK.p50 >= 200);
  assert.ok(slow.pixelResponse.firstChangedCaptureMS.upper < slow.inputACK.p50, "slow peer waited for the server before drawing its input");
  report.checks.push({ name: "independent-fast-and-slow-transports", fastRTT: fast.transportRTT, slowRTT: slow.transportRTT, slowInputACK: slow.inputACK, slowPixelResponse: slow.pixelResponse });
  const beforeRetirement = data("a");
  for (const tower of beforeRetirement.towers) {
    browser("a", "click", `[data-pokemon-id="${tower.pokemonID}"] .deploy`);
    await until(() => !data("a").towers.some(value => value.pokemonID === tower.pokemonID), "the real retirement input is acknowledged");
  }
  const afterRetirement = data("a");
  assert.equal(afterRetirement.gold, beforeRetirement.gold);
  assert.deepEqual(afterRetirement.team.map(pokemon => [pokemon.id, pokemon.level]), beforeRetirement.team.map(pokemon => [pokemon.id, pokemon.level]));
  report.outagePreparation = { scope: "Legal native tower retirement isolates transport continuity from normal enemy kills; the preceding two-player battle and subsequent dense practice retain real deployed towers", towers: beforeRetirement.towers, beforeGold: beforeRetirement.gold, afterGold: afterRetirement.gold, teamLevels: afterRetirement.team.map(pokemon => [pokemon.id, pokemon.level]), remainingTowers: afterRetirement.towers.length };
  await running();
  const before = data("a");
  await configure({ pauseUp: true, pauseDown: true });
  const outage = await measure("a", owned.base, "fast-during-outage", ".pokepath-classic-controls .auto", 1500);
  const after = data("a");
  assert.ok(after.clock - before.clock >= 1000, "a stalled peer stopped the shared server clock");
  responsive(outage, "fast-during-outage", false);
  assert.ok(outage.actualCanvasChanges.moving.milliseconds >= 1000, "outage sample has insufficient actual visible motion");
  assert.ok(outage.actualCanvasChanges.moving.perSecond >= 55, "the stalled peer stopped fast visible updates");
  await configure({ pauseUp: false, pauseDown: false });
  await until(() => state("b").room?.id === owned.roomID, "slow connection recovery");
  assert.ok(state("a").room.seats.every(seat => !seat.autoPlay), "an actual human was automatically delegated to a bot");
  report.checks.push({ name: "one-peer-outage-keeps-fast-player-running", outageMS: 1500, ruleAdvanceMS: after.clock - before.clock, actualFastPixelsPerSecond: outage.actualCanvasChanges.moving.perSecond });
  await configure({ disconnect: true });
  await sleep(1200);
  await until(() => state("b").room?.id === owned.roomID, "TCP reconnect with the original seat");
  const original = data("a");
  const oldInputs = new Set(original.acceptedInputs);
  for (let index = 0; index < 6; index++) browser("a", "click", ".pokepath-classic-controls .auto");
  await until(() => data("a").acceptedInputs.filter(id => !oldInputs.has(id)).length >= 6, "six distinct native input receipts");
  const final = data("a");
  assert.equal(final.autoWave, original.autoWave);
  assert.equal(final.epoch, original.epoch);
  for (const label of ["a", "b"]) {
    assert.equal(evaluate(label, "window.__nativeAudioAttempts.length"), 0);
    assert.equal(evaluate(label, "Boolean(document.querySelector('.pokepath-error'))"), false);
  }
  report.checks.push({ name: "native-rapid-input-and-reconnect", distinctClicks: 6, epoch: final.epoch, roomID: owned.roomID });
  report.proxy = await configure({});
  roomCommand("a", "close");
  roomCommand("a", "leave");
  roomCommand("b", "leave");
  writeFileSync(join(owned.output, "editor-owner"), "Prepare an editor-owned account without changing initial gold or level\n");
  const denseRoom = post("a", "/api/rooms", { kind: "pokepath", capacity: 1, name: "巡径密集呈现验收", options: { starter: "charmander" } });
  roomCommand("a", "ready");
  roomCommand("a", "start");
  browser("a", "reload");
  browser("a", "wait", "[data-pokepath-canvas]");
  await until(() => !state("a").room.loading, "dense native room initialized");
  assert.equal(data("a").gold, 50);
  assert.equal(data("a").team[0].level, 1);
  assert.ok(data("a").items.some(item => item.id === "revysBook"));
  browser("a", "click", '.pokepath-classic-menu [data-classic-section="routes"]');
  browser("a", "click", ".pokepath-classic-routes nav > button:nth-child(2)");
  browser("a", "fill", '[aria-label="第 1 组数量"]', "200");
  browser("a", "click", ".pokepath-editor-footer > button.primary");
  await until(() => data("a").progression.custom, "a legal 200-enemy editor practice");
  const pokemon = data("a").team[0];
  browser("a", "click", `[data-pokemon-id="${pokemon.id}"] .pokepath-card-portrait`);
  if (evaluate("a", "Boolean(document.querySelector('[data-map-fit]')?.getBoundingClientRect().width)")) browser("a", "click", "[data-map-fit]");
  const point = evaluate("a", "(()=>{const canvas=document.querySelector('[data-pokepath-canvas]'),rect=canvas.getBoundingClientRect();return{x:rect.x+252/canvas.width*rect.width,y:rect.y+132/canvas.height*rect.height}})()");
  browser("a", "mouse", "move", String(Math.round(point.x)), String(Math.round(point.y)));
  browser("a", "mouse", "down", "left");
  browser("a", "mouse", "up", "left");
  await until(() => data("a").towers.length === 1, "native mouse deployment in dense practice");
  browser("a", "click", ".pokepath-classic-controls .wave");
  await until(() => data("a").enemies.length === 200, "the complete legal dense enemy sequence");
  await running();
  const dense = await measure("a", owned.base, "dense", ".pokepath-classic-controls .pause");
  responsive(dense, "dense");
  assert.ok(data("a").enemies.length >= 180, "dense practice lost the intended rule workload before measurement completed");
  report.checks.push({ name: "formal-native-dense-editor-practice", scope: "A prepared editor-book owner with 50 gold and a level-one partner enters a 200-enemy practice using real UI and mouse controls; not a natural campaign claim", setupReceipt: join(owned.output, "editor-owner-receipt.json"), roomID: denseRoom.id, initialEnemies: 200, remainingEnemies: data("a").enemies.length, actualMovingPixelsPerSecond: dense.actualCanvasChanges.moving.perSecond });
  report.passed = true;
} catch (error) {
  report.error = error.stack ?? String(error);
  process.exitCode = 1;
} finally {
  try { await configure({ pauseUp: false, pauseDown: false, delayMS: 0 }); } catch {}
  try { if (data("a")?.paused === false) browser("a", "click", ".pokepath-classic-controls .pause"); } catch {}
  try { browser("b", "open", owned.base); browser("b", "wait", state("b").room?.status === "playing" ? "[data-pokepath-canvas]" : ".room-list"); } catch {}
  if (proxy.exitCode === null) { proxy.kill("SIGTERM"); await once(proxy, "exit"); }
  writeFileSync(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  process.stdout.write(JSON.stringify({ passed: report.passed, result: join(output, "result.json"), error: report.error }) + "\n");
}
