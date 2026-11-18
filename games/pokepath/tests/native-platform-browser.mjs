import assert from "node:assert/strict";
import { execFileSync, spawn } from "node:child_process";
import { mkdirSync, writeFileSync, readFileSync, existsSync, rmSync, openSync, closeSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { randomUUID } from "node:crypto";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const owner = randomUUID().slice(0, 8);
const output = join(root, ".local/pokepath/native-browser", `run-${owner}`);
const standardWeb = process.argv.includes("--standard-web");
const webRoot = join(root, standardWeb ? "web/dist" : ".local/pokepath/native-web");
assert.ok(!standardWeb || !process.argv.includes("--build"), "Build the standard web with npm --prefix web run build before its browser acceptance");
const environment = { ...process.env, WIRE_BOARD_POKEPATH_BROWSER: "1", WIRE_BOARD_POKEPATH_SCOPED_WEB: standardWeb ? "0" : "1", WIRE_BOARD_POKEPATH_BROWSER_OUT: output, AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), TMPDIR: join(root, ".local/tmp"), GOCACHE: join(root, ".local/go-build"), GOMODCACHE: join(root, ".local/go-mod"), GOTMPDIR: join(root, ".local/tmp"), GOTOOLCHAIN: "local", npm_config_cache: join(root, ".local/npm"), AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_NO_VERSION_CHECK: "1" };
for (const path of [output, environment.AGENT_BROWSER_SOCKET_DIR, environment.TMPDIR, environment.GOCACHE, environment.GOMODCACHE, environment.npm_config_cache]) mkdirSync(path, { recursive: true });
const quietInit = join(output, "quiet-init.js");
writeFileSync(quietInit, `try{localStorage.setItem('wb_sound','off')}catch{}window.__nativeAudioAttempts=[];const originalPlay=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){window.__nativeAudioAttempts.push({source:this.currentSrc||this.src,platform:this.hasAttribute('data-platform-audio'),muted:this.muted,volume:this.volume});return originalPlay.call(this)}`);
environment.AGENT_BROWSER_INIT_SCRIPTS = quietInit;
const report = { command: ["node games/pokepath/tests/native-platform-browser.mjs", ...process.argv.slice(2)].join(" "), architecture: "independent Go/Wasm rules and native React/Canvas", run: owner, passed: false, checks: [], screenshots: [], accounts: 3, output };
const sessions = new Map();
const accounts = new Map();
const pause = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
let fixture;
let log;
let base;
let roomID;
let matchID;
let serve = process.argv.includes("--serve");
const verifyChallenge = process.argv.includes("--challenge");
const verifyBox = process.argv.includes("--box");
const verifyMaps = process.argv.includes("--maps");
const verifyRange = process.argv.includes("--range");
const verifyResult = process.argv.includes("--result-victory") ? "victory" : process.argv.includes("--result-defeat") ? "defeat" : undefined;

function browser(label, ...args) {
  let session = sessions.get(label);
  if (!session) {
    const name = execFileSync("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", `nb${owner}${label}`], { cwd: root, env: environment, encoding: "utf8" }).trim();
    session = { name, profile: join(output, `profile-${label}`) };
    sessions.set(label, session);
  }
  let raw;
  try {
    raw = execFileSync("agent-browser", ["--session", session.name, ...args, "--json"], { cwd: root, env: { ...environment, AGENT_BROWSER_PROFILE: session.profile }, encoding: "utf8", timeout: 45000, maxBuffer: 16 << 20 });
  } catch (error) { throw Error(`${label} ${args[0]}: ${error.stdout ?? error.stderr ?? error.message}`); }
  const value = JSON.parse(raw);
  if (!value.success) throw Error(JSON.stringify(value));
  return value.data;
}
function evaluate(label, source) { return browser(label, "eval", "-b", Buffer.from(source).toString("base64")).result; }
function request(label, path, body) {
  const options = body === undefined ? "{}" : `{method:"POST",headers:{"Content-Type":"application/json"},body:${JSON.stringify(JSON.stringify(body))}}`;
  return evaluate(label, `fetch(${JSON.stringify(path)},${options}).then(async response=>({status:response.status,body:await response.json()}))`);
}
function api(label, path, body) {
  const result = request(label, path, body);
  assert.ok(result.status >= 200 && result.status < 300, `${label} ${path}: ${JSON.stringify(result)}`);
  return result.body;
}
function room(label) { return api(label, "/api/state").room; }
function data(label) { return room(label)?.game?.data; }
function command(label, type, extra = {}, id) {
  for (let index = 0; index < 12; index++) {
    const current = room(label);
    const result = request(label, `/api/rooms/${id ?? current.id}`, { type, version: current?.version ?? 0, nonce: randomUUID(), ...extra });
    if (result.status === 409) continue;
    assert.equal(result.status, 200, JSON.stringify(result));
    return result.body;
  }
  throw Error(`Command did not settle: ${type}`);
}
async function waitFor(check, name, milliseconds = 30000) {
  const begin = Date.now();
  let last;
  while (Date.now() - begin < milliseconds) {
    last = check();
    if (last) return last;
    if (fixture?.exitCode !== null && fixture?.exitCode !== undefined) throw Error(`HTTP fixture exited during ${name}; see server.log`);
    await pause(100);
  }
  throw Error(`Timed out: ${name}; last=${JSON.stringify(last)}`);
}
async function waitCanvas(label) {
  return waitFor(() => {
    const status = evaluate(label, `({canvas:Boolean(document.querySelector('[data-pokepath-canvas]')),error:document.querySelector('[data-game-loading="pokepath"] [role="alert"] p')?.textContent})`);
    if (status.error) throw Error(`${label} native loading: ${status.error}`);
    return status.canvas;
  }, `${label} independent Canvas initialization`, 90000);
}
function click(label, selector) {
  if (verifyChallenge || verifyResult || verifyBox || verifyMaps || verifyRange) {
    execFileSync(process.execPath, [join(root, "scripts/browser_touch.mjs"), cdp(label), base, "click-selector", selector], { cwd: root, env: environment, timeout: 15000, encoding: "utf8" });
    return;
  }
  browser(label, "scrollintoview", selector);
  const box = evaluate(label, `(()=>{const node=document.querySelector(${JSON.stringify(selector)});if(!node)return null;const box=node.getBoundingClientRect();return {x:box.left+box.width/2,y:box.top+box.height/2,width:box.width,height:box.height,disabled:node.disabled}})()`);
  assert.ok(box?.width > 0 && box.height > 0 && !box.disabled, `Not an available native target: ${selector}`);
  browser(label, "mouse", "move", String(Math.round(box.x)), String(Math.round(box.y)));
  browser(label, "mouse", "down", "left");
  browser(label, "mouse", "up", "left");
}
function cdp(label) { return browser(label, "get", "cdp-url").cdpUrl; }
function touch(label, x, y, mode = "tap", toX = x, toY = y) {
  execFileSync(process.execPath, [join(root, "scripts/browser_touch.mjs"), cdp(label), base, mode, String(Math.round(x)), String(Math.round(y)), String(Math.round(toX)), String(Math.round(toY))], { cwd: root, env: environment, timeout: 15000, encoding: "utf8" });
}
function touchControl(label, selector) {
  browser(label, "scrollintoview", selector);
  const box = evaluate(label, `(()=>{const node=document.querySelector(${JSON.stringify(selector)});if(!node)return null;const box=node.getBoundingClientRect();return {x:box.left+box.width/2,y:box.top+box.height/2,width:box.width,height:box.height,disabled:node.disabled}})()`);
  assert.ok(box?.width > 0 && box.height > 0 && !box.disabled, `Not an available touch target: ${selector}`);
  touch(label, box.x, box.y);
}
function logicalPoint(label, x, y) {
  const point = evaluate(label, `(()=>{const canvas=document.querySelector('[data-pokepath-canvas]'),wrapper=canvas?.parentElement;if(!canvas||!wrapper)return null;const a=canvas.getBoundingClientRect(),b=wrapper.getBoundingClientRect();return {x:a.left+${Number(x)}*a.width/canvas.width,y:a.top+${Number(y)}*a.height/canvas.height,canvas:a.toJSON(),wrapper:b.toJSON(),width:innerWidth,height:innerHeight}})()`);
  assert.ok(point && point.x >= Math.max(0, point.wrapper.left) && point.x <= Math.min(point.width, point.wrapper.right) && point.y >= Math.max(0, point.wrapper.top) && point.y <= Math.min(point.height, point.wrapper.bottom), `Logical point is not drawn inside the visible battlefield: ${JSON.stringify(point)}`);
  return point;
}
function canvasClick(label, x, y, isTouch = false) {
  browser(label, "scrollintoview", "[data-pokepath-canvas]");
  const point = logicalPoint(label, x, y);
  if (isTouch) touch(label, point.x, point.y);
  else { browser(label, "mouse", "move", String(Math.round(point.x)), String(Math.round(point.y))); browser(label, "mouse", "down", "left"); browser(label, "mouse", "up", "left"); }
  return point;
}
function inspector(label) { return Number(evaluate(label, `document.querySelector('.pokepath-classic-card.selected[data-pokemon-id]')?.getAttribute('data-pokemon-id')`)); }
function choose(label, id, isTouch = false) { (isTouch ? touchControl : click)(label, `[data-pokemon-id="${id}"] .pokepath-card-portrait`); }
function fit(label, isTouch = false) { if (evaluate(label, `Boolean(document.querySelector('[data-map-fit]')?.getBoundingClientRect().width)`)) (isTouch ? touchControl : click)(label, "[data-map-fit]"); }
function sharedResources(label) {
  return evaluate(label, `(()=>{const width=innerWidth;const measure=node=>{const rect=node.getBoundingClientRect(),hit=document.elementFromPoint(rect.left+rect.width/2,rect.top+rect.height/2);return {text:node.textContent,rect:rect.toJSON(),occluded:!hit||!node.contains(hit),hit:hit?.tagName+'.'+hit?.className,panel:node.closest('.player-panel')?.getBoundingClientRect().toJSON()}};return {width,overflow:document.documentElement.scrollWidth>width+2,gold:[...document.querySelectorAll('[data-pokepath-gold]')].map(measure),health:[...document.querySelectorAll('[data-pokepath-health]')].map(measure)}})()`);
}
function checkpoint(label, name, resources = false) {
  if (resources) browser(label, "scrollintoview", "[data-pokepath-gold]");
  const measured = sharedResources(label);
  assert.equal(measured.overflow, false, `${name}: horizontal overflow`);
  if (resources) {
    assert.ok(measured.gold.length >= 2 && measured.health.length >= 2, `${name}: missing shared resources`);
    for (const item of [...measured.gold, ...measured.health]) assert.ok(item.rect.width > 0 && item.rect.x >= -1 && item.rect.right <= measured.width + 2 && !item.occluded, `${name}: shared resource clipped or covered: ${JSON.stringify(item)}`);
  }
  assert.equal(evaluate(label, "Boolean(document.querySelector('.pokepath-board iframe'))"), false, "embedded program appeared in native game");
  browser(label, "screenshot", join(output, `${name}.png`));
  assert.equal(evaluate(label, `document.querySelector('.toast.error')?.textContent?.trim() ?? ''`), "", `${name}: platform error toast`);
  writeFileSync(join(output, `${name}.snapshot.json`), JSON.stringify(browser(label, "snapshot", "-i"), null, 2));
  report.checks.push({ name, measurement: measured });
  report.screenshots.push(`${name}.png`);
  process.stdout.write(`PASS ${name}\n`);
}
async function startFixture(restart = false) {
  if (!restart && existsSync(join(output, "database"))) throw Error("Refusing to reuse another browser acceptance database");
  for (const name of ["url", "stop"]) rmSync(join(output, name), { force: true });
  if (log !== undefined) closeSync(log);
  log = openSync(join(output, restart ? "server-restart.log" : "server.log"), "w");
  fixture = spawn("go", ["test", "-count=1", "-timeout=45m", "-run", "^TestPokepathBrowserFixture$", "games/pokepath/tests/acceptance_fixture_test.go"], { cwd: root, env: environment, stdio: ["ignore", log, log] });
  await waitFor(() => existsSync(join(output, "url")), "HTTP and SQLite fixture", 120000);
  base = readFileSync(join(output, "url"), "utf8").trim();
}
async function stopFixture() {
  if (!fixture || fixture.exitCode !== null) return;
  writeFileSync(join(output, "stop"), "stop\n");
  const begin = Date.now();
  while (fixture.exitCode === null && Date.now() - begin < 30000) await pause(100);
  if (fixture.exitCode === null) { fixture.kill("SIGTERM"); await pause(300); }
  assert.ok(fixture.exitCode === 0 || fixture.signalCode === "SIGTERM", `HTTP fixture did not stop cleanly: ${fixture.exitCode}`);
}
function openAccount(label, account, login = false) {
  browser(label, "open", base);
  const auth = login ? { name: account.name, password: account.password } : { ...account, invite: "pokepath-e2e" };
  api(label, login ? "/api/login" : "/api/register", auth);
  browser(label, "reload");
  browser(label, "wait", ".room-list");
  assert.equal(evaluate(label, "localStorage.getItem('wb_sound')"), "off");
}
function closeSession(label) {
  if (!sessions.has(label)) return;
  browser(label, "close");
  rmSync(sessions.get(label).profile, { recursive: true, force: true });
  sessions.delete(label);
}
function physical(snapshot) { const value = structuredClone(snapshot); delete value.epoch; return value; }

function holdLoadReady(label, id) {
  assert.equal(evaluate(label, `(()=>{
    const path=${JSON.stringify(`/api/rooms/${id}`)}, original=window.fetch;
    let resolve;
    const held=new Promise(ready=>{resolve=ready});
    const gate={held:0,release(){window.fetch=original;resolve();}};
    window.__pokepathLoadReadyGate=gate;
    window.fetch=async(input,init)=>{
      if(input===path&&typeof init?.body==='string'&&JSON.parse(init.body).type==='load_ready'){gate.held++;await held;}
      return original.call(window,input,init);
    };
    return true;
  })()`), true);
}

async function challengeAcceptance() {
  const players = ["a", "b", "c", "d"];
  for (const label of [...players, "s"]) {
    const account = { name: `挑战${label}${owner}`, password: "native-browser-test-password" };
    accounts.set(label, account);
    openAccount(label, account);
    browser(label, "set", "viewport", "1440", "900");
  }
  const created = api("a", "/api/rooms", { kind: "pokepath", capacity: 4, name: "四人挑战验收", options: { starter: "charmander" } });
  roomID = created.id;
  for (const label of players.slice(1)) {
    const joined = request(label, `/api/rooms/${roomID}`, { type: "join", version: room("a").version, nonce: randomUUID() });
    assert.equal(joined.status, 200, JSON.stringify(joined));
  }
  for (const label of players) { browser(label, "reload"); browser(label, "wait", ".waiting-cover"); command(label, "ready"); }
  writeFileSync(join(output, "challenge-ready"), "challenge\n");
  command("a", "start");
  for (const label of players) await waitCanvas(label);
  await waitFor(() => players.every(label => !room(label).loading && data(label).progression?.canChallenge), "four loaded challenge seats", 90000);
  matchID = room("a").matchId;
  assert.equal(room("a").seats.filter(seat => !seat.bot && !seat.left).length, 4);
  const open = '[data-classic-section="challenge"]';
  const level = '[data-challenge-rule="levelCap"]';
  const actual = (label, selector) => evaluate(label, `document.querySelector(${JSON.stringify(selector)})?.textContent`);
  const selected = label => actual(label, `${level} .pokepath-challenge-result`);
  const challenge = label => data(label).progression.challenge;
  const draft = label => data(label).progression.draft;
  const elapsed = label => data(label).progression.challengeElapsedMS ?? 0;
  const timer = label => actual(label, "[data-challenge-clock]");
  for (const label of players) {
    await waitFor(() => evaluate(label, "!document.querySelector('[data-game-loading]')"), `seat ${label} finished its loading overlay`);
    click(label, open); browser(label, "wait", level);
  }
  api("s", `/api/rooms/${roomID}/watch`, {});
  browser("s", "reload"); await waitCanvas("s");
  await waitFor(() => Boolean(evaluate("s", `(()=>{const element=document.querySelector('${open}'),r=element.getBoundingClientRect();return !document.querySelector('[data-game-loading]')&&element.contains(document.elementFromPoint(r.left+r.width/2,r.top+r.height/2))})()`)), "viewer challenge button receives pointer input");
  click("s", open);
  if (!evaluate("s", `Boolean(document.querySelector('${level}'))`)) touchControl("s", open);
  browser("s", "wait", level);
  const originally = selected("b");
  click("a", level); click("a", level);
  assert.match(selected("a"), /50/);
  assert.equal(selected("b"), originally);
  assert.equal(challenge("b").active, false);
  assert.equal(evaluate("s", `document.querySelector('${level}').getAttribute('aria-disabled')`), "true");
  assert.equal(evaluate("s", "document.querySelector('[data-challenge-submit]').disabled"), true);
  click("s", level);
  assert.equal(selected("s"), originally);
  assert.equal(evaluate("a", "[...document.querySelectorAll('[data-challenge-reward]')].map(node=>Number(node.dataset.challengeReward)).join(',')"), "3,0,1,2");
  const oldGold = data("a").gold;
  click("c", '[data-challenge-reward="3"]');
  await waitFor(() => data("d").progression.claims["0"]?.[3] && data("d").gold === oldGold + 500000, "correct reward granted once at all seats");
  assert.equal(evaluate("c", "document.querySelector('[data-challenge-reward=\"3\"]').getAttribute('aria-disabled')"), "true");
  click("a", '[data-challenge-rule="slotLimit"]'); click("a", '[data-challenge-rule="slotLimit"]');
  click("a", '[data-challenge-rule="draft"]'); click("a", '[data-challenge-rule="noItems"]');
  checkpoint("a", "challenge-plan-and-rewards");
  click("a", '.pokepath-classic-controls .wave');
  await waitFor(() => data("d").waveActive && data("d").enemies.length > 0, "old battle has enemies before the new challenge");
  const beforeChallengeEpoch = data("a").epoch;
  click("a", "[data-challenge-submit]");
  await waitFor(() => players.every(label => {
    const next = data(label);
    return next.epoch > beforeChallengeEpoch && !next.waveActive && next.enemies.length === 0 && next.wave === 1 && next.progression.challenge.active && next.progression.draft.active && next.progression.draft.rounds === 6;
  }), "all four peers replace the battle with a six-pick draft");
  await waitFor(() => players.every(label => timer(label) === "00:00:00.00"), "all seats see zero before the first challenge wave");
  const firstToken = draft("a").token;
  const draftSelector = `[data-draft-token="${firstToken}"]`;
  browser("b", "wait", draftSelector);
  const infoTarget = `${draftSelector} [data-draft-slot="0"] .pokepath-draft-info`;
  browser("b", "scrollintoview", infoTarget);
  const infoPoint = evaluate("b", `(()=>{const node=document.querySelector(${JSON.stringify(infoTarget)}),r=node.getBoundingClientRect(),x=Math.round(r.left+r.width/2),y=Math.round(r.top+r.height/2),target=document.elementFromPoint(x,y);return{x,y,hit:node.contains(target),target:target?.outerHTML.slice(0,180)}})()`);
  assert.ok(infoPoint.hit, JSON.stringify(infoPoint));
  evaluate("b", "window.draftInfoClicks=[];document.addEventListener('click',event=>window.draftInfoClicks.push({className:event.target.className,trusted:event.isTrusted}),{capture:true,once:true});true");
  click("b", infoTarget);
  const inspected = draft("b").offers[0];
  const inspectSelector = `[data-details-pokemon="${inspected}"] [inert]`;
  const detailsOpened = evaluate("b", `Boolean(document.querySelector(${JSON.stringify(inspectSelector)}))`);
  assert.equal(detailsOpened, true, JSON.stringify({ infoPoint, inspected, events: evaluate("b", "window.draftInfoClicks"), detail: evaluate("b", "document.querySelector('[data-details-pokemon]')?.outerHTML.slice(0,500)") }));
  browser("s", "wait", draftSelector);
  assert.equal(evaluate("s", "[...document.querySelectorAll('.pokepath-draft-choice,[data-draft-reroll],[data-draft-restart]')].every(node=>node.disabled)"), true);
  click("a", `${draftSelector} [data-draft-reroll]`);
  await waitFor(() => players.every(label => draft(label).token === firstToken + 1 && draft(label).round === 0), "reroll changes the same candidate receipt for every player");
  await waitFor(() => !evaluate("b", "Boolean(document.querySelector('[data-details-pokemon]'))"), "reroll closes an outdated details dialog on a different seat");
  const sendDraft = (label, type, token) => {
    const observed = room(label);
    return request(label, `/api/rooms/${roomID}`, { type: "action", version: observed.version, nonce: randomUUID(), action: { type, targetID: token, epoch: observed.game.data.epoch } });
  };
  const staleRestart = sendDraft("b", "draftRestart", firstToken);
  assert.ok(staleRestart.status >= 400, "a second player replayed an old restart");
  const preliminary = draft("a");
  click("a", `[data-draft-token="${preliminary.token}"] [data-draft-slot="0"] .pokepath-draft-choice`);
  await waitFor(() => data("d").team[0]?.id === preliminary.offers[0] && draft("d").round === 1, "preliminary pick is shared before restart");
  const restartToken = draft("d").token;
  click("d", `[data-draft-token="${restartToken}"] [data-draft-restart]`);
  await waitFor(() => players.every(label => draft(label).token === restartToken + 1 && draft(label).round === 0 && data(label).team.length === 0 && data(label).box.length === 30), "restart returns the prior pick to the shared pool");
  checkpoint("a", "shared-draft-after-restart");
  const chosen = [];
  for (const label of players.concat(["a", "b"])) {
    const previous = draft(label);
    browser(label, "wait", `[data-draft-token="${previous.token}"]`);
    const id = previous.offers[0];
    const pick = `[data-draft-token="${previous.token}"] [data-pokemon-id="${id}"] button:first-child`;
    if (label === "c") { browser(label, "set", "viewport", "320", "740"); touchControl(label, pick); }
    else click(label, pick);
    await waitFor(() => data("d").team.some(pokemon => pokemon.id === id) && draft("d").token !== previous.token, `seat ${label} selects shared candidate ${id}`);
    chosen.push({ label, id, token: previous.token });
  }
  await waitFor(() => players.every(label => data(label).team.length === 6 && !draft(label).active), "all six real picks are shared and unique");
  assert.equal(new Set(chosen.map(entry => entry.id)).size, 6);
  const observed = room("a");
  const stale = request("a", `/api/rooms/${roomID}`, { type: "action", version: observed.version, nonce: randomUUID(), action: { type: "draftPick", pick: 0, targetID: firstToken, epoch: data("a").epoch } });
  assert.ok(stale.status >= 400, "stale draft replay changed the team");
  for (const panel of ["routes", "box", "inventory"]) assert.equal(evaluate("d", `document.querySelector('[data-classic-section="${panel}"]').disabled`), true);
  browser("d", "focus", "[data-pokepath-canvas]"); browser("d", "press", "e");
  assert.equal(evaluate("d", "Boolean(document.querySelector('.pokepath-classic-box'))"), false, "draft allowed the box keyboard shortcut");
  if (!evaluate("d", "Boolean(document.querySelector('[data-challenge-submit]'))")) click("d", open);
  browser("d", "wait", level);
  assert.match(selected("d"), /50/);
  assert.equal(evaluate("d", `document.querySelector('${level}').getAttribute('aria-disabled')`), "true");
  checkpoint("d", "shared-challenge-after-draft");
  click("a", '.pokepath-classic-controls .wave');
  await waitFor(() => players.every(label => elapsed(label) >= 1000 && data(label).waveActive), "all seats inherit the first wave's challenge clock");
  click("a", '.pokepath-classic-controls .pause');
  await waitFor(() => players.every(label => data(label).paused && elapsed(label) === elapsed("a")), "all seats freeze at the same elapsed time");
  const frozenTime = elapsed("a");
  const wholeSeconds = Math.floor(frozenTime / 1000);
  const renderTime = [Math.floor(wholeSeconds / 3600), Math.floor(wholeSeconds / 60) % 60, wholeSeconds % 60].map(value => String(value).padStart(2, "0")).join(":") + "." + String(Math.floor(frozenTime / 10) % 100).padStart(2, "0");
  await waitFor(() => [...players, "s"].every(label => timer(label) === renderTime), "all visible clocks freeze including the spectator");
  assert.equal(evaluate("c", `(()=>{const list=['[data-challenge-status]','[data-challenge-clock]','.pokepath-terrain-counts'].map(selector=>document.querySelector(selector).getBoundingClientRect());return list.every(r=>r.x>=0&&r.right<=innerWidth)&&list[0].right<=list[1].left&&list[1].bottom<=list[2].top})()`), true);
  assert.equal(evaluate("d", "document.querySelector('[data-challenge-submit]').disabled"), true);
  assert.equal(evaluate("a", "Boolean(document.querySelector('.pokepath-wave-income'))"), false);
  checkpoint("a", "shared-frozen-challenge-clock");
  assert.equal(elapsed("b"), frozenTime);
  const activeWave = data("a").wave;
  const liveIDs = data("a").enemies.map(enemy => enemy.id);
  click("a", '.pokepath-classic-controls .pause');
  await waitFor(() => !data("d").paused, "partner can continue the old wave");
  click("d", "[data-challenge-submit]");
  await waitFor(() => players.every(label => {
    const current = data(label);
    return !current.progression.challenge.active && current.team.length === 0 && current.box.length === 30 && current.wave === activeWave && current.waveActive && current.enemies.some(enemy => liveIDs.includes(enemy.id));
  }), "surrender preserves the collection and already-spawned wave at every seat");
  await waitFor(() => [...players, "s"].every(label => timer(label) === null), "the finished challenge no longer displays a clock");
  for (const label of [...players, "s"]) assert.equal(evaluate(label, "window.__nativeAudioAttempts.length"), 0);
  report.accounts = 5;
  report.checks.push({ name: "four-authenticated-human-challenge-seats", players, localRuleChoice: true, rewardSlot: 3, rewardGold: 500000, picked: chosen, replacedRunningWave: true, rerolled: true, restarted: true, staleRestartStatus: staleRestart.status, staleStatus: stale.status, recoveredCollection: 30, frozenTime, displayedFrozenTime: renderTime, surrenderedWithEnemies: true, spectatorReadOnly: true, mutedByPlatform: true });
}

async function resultAcceptance(kind) {
  const seats = ["a", "b"];
  for (const label of [...seats, "s"]) {
    const account = { name: `结算${label}${owner}`, password: "native-browser-test-password" };
    accounts.set(label, account);
    openAccount(label, account);
    browser(label, "set", "viewport", "1440", "900");
  }
  roomID = api("a", "/api/rooms", { kind: "pokepath", capacity: 2, name: "路线结算验收", options: { starter: "charmander" } }).id;
  assert.equal(request("b", `/api/rooms/${roomID}`, { type: "join", version: room("a").version, nonce: randomUUID() }).status, 200);
  for (const label of seats) { browser(label, "reload"); browser(label, "wait", ".waiting-cover"); command(label, "ready"); }
  writeFileSync(join(output, `result-${kind}`), "result\n");
  command("a", "start");
  for (const label of seats) await waitCanvas(label);
  await waitFor(() => seats.every(label => !room(label).loading), "both original seats loaded the authoritative result");
  api("s", `/api/rooms/${roomID}/watch`, {});
  browser("s", "reload"); await waitCanvas("s");
  matchID = room("a").matchId;
  const action = kind === "victory" ? { type: "continueRoute" } : { type: "retry", mode: "retry" };
  const target = kind === "victory" ? "[data-result-continue]" : "[data-result-retry]";
  for (const label of [...seats, "s"]) browser(label, "wait", `[data-route-result="${kind}"]`);
  const spectator = room("s");
  const blocked = request("s", `/api/rooms/${roomID}`, { type: "action", version: spectator.version, nonce: randomUUID(), action: { ...action, epoch: spectator.game.data.epoch } });
  assert.ok(blocked.status >= 400);
  assert.equal(evaluate("s", `document.querySelector('${target}').disabled`), true);
  const from = data("a");
  assert.equal(from.towers.length, 1);
  checkpoint("a", `authoritative-${kind}-desktop`);
  if (kind === "defeat") {
    browser("b", "set", "viewport", "320", "740");
    assert.equal(evaluate("b", "(()=>{const r=document.querySelector('.pokepath-result').getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&document.documentElement.scrollWidth===innerWidth})()"), true);
    checkpoint("b", "defeat-checkpoint-mobile");
    touchControl("b", target);
  } else click("a", target);
  const nextWave = kind === "victory" ? 1 : 25;
  const nextHealth = kind === "victory" ? 14 : 7;
  await waitFor(() => seats.every(label => {
    const next = data(label);
    return next.phase === "prepare" && next.wave === nextWave && next.health === nextHealth && next.gold === from.gold && next.epoch > from.epoch && next.towers[0]?.id === from.towers[0].id;
  }), "one result action continued the route for every seat");
  await waitFor(() => [...seats, "s"].every(label => !evaluate(label, "Boolean(document.querySelector('[data-route-result]'))")), "the shared result closes for all observers");
  const after = room("b");
  const stale = request("b", `/api/rooms/${roomID}`, { type: "action", version: after.version, nonce: randomUUID(), action: { ...action, epoch: from.epoch } });
  assert.ok(stale.status >= 400, "stale confirmation was accepted");
  click("a", '.pokepath-classic-controls .wave');
  await waitFor(() => seats.every(label => data(label).waveActive && data(label).wave === nextWave), "the confirmed route started a fresh real wave");
  for (const label of [...seats, "s"]) assert.equal(evaluate(label, "window.__nativeAudioAttempts.length"), 0);
  report.checks.push({ name: "shared-route-result", kind, confirmedWith: kind === "victory" ? "mouse" : "touch", wave: nextWave, health: nextHealth, spectatorStatus: blocked.status, staleStatus: stale.status, tower: from.towers[0].id });
}

async function boxAcceptance() {
  const players = ["a", "b"];
  for (const label of [...players, "s"]) {
    openAccount(label, { name: `仓库${label}${owner}`, password: "native-browser-test-password" });
    browser(label, "set", "viewport", "1440", "900");
  }
  roomID = api("a", "/api/rooms", { kind: "pokepath", capacity: 2, name: "仓库联机换位验收" }).id;
  assert.equal(request("b", `/api/rooms/${roomID}`, { type: "join", version: room("a").version, nonce: randomUUID() }).status, 200);
  for (const label of players) { browser(label, "reload"); browser(label, "wait", ".waiting-cover"); command(label, "ready"); }
  command("a", "start");
  for (const label of players) await waitCanvas(label);
  await waitFor(() => players.every(label => !room(label).loading), "both warehouse seats have loaded");
  const initial = data("a");
  const first = initial.team[0].id;
  const send = (label, action) => command(label, "action", { action: { ...action, epoch: data(label).epoch } });
  send("a", { type: "egg", quote: initial.progression.remainingEggs });
  const second = data("a").team.find(pokemon => pokemon.id !== first).id;
  send("a", { type: "team", pokemonID: second, mode: "box" });
  send("a", { type: "deploy", pokemonID: first, tileID: initial.tiles.find(tile => tile.terrain === 1).id });
  api("s", `/api/rooms/${roomID}/watch`, {});
  browser("s", "reload"); await waitCanvas("s");
  const open = '[data-classic-section="box"]';
  const card = id => `[data-box-pokemon="${id}"]`;
  const slot = '[data-party-slot="0"]';
  for (const label of ["b", "s"]) {
    await waitFor(() => !evaluate(label, "Boolean(document.querySelector('[data-game-loading]'))"), "ready warehouse observer");
    click(label, open);
    await waitFor(() => evaluate(label, `Boolean(document.querySelector('${card(second)}'))`), "warehouse includes actual reserve and roster");
  }
  const ordering = label => evaluate(label, "document.querySelector('[data-box-sort]')?.textContent");
  const initialSort = ordering("b");
  click("b", "[data-box-next]");
  const changedSort = ordering("b");
  assert.notEqual(changedSort, initialSort);
  assert.equal(ordering("s"), initialSort);
  click("b", open); click("b", open);
  assert.equal(ordering("b"), changedSort, "closing the box loses the same player's sort preference");
  click("b", "[data-box-previous]");
  click("b", card(second));
  checkpoint("b", "box-desktop-before-slot-replacement");
  const point = evaluate("b", `(()=>{const locate=selector=>{const r=document.querySelector(selector).getBoundingClientRect();return[Math.round(r.left+r.width/2),Math.round(r.top+r.height/2)]};return{origin:locate('${card(second)}'),target:locate('${slot}')}})()`);
  browser("b", "mouse", "move", ...point.origin.map(String));
  browser("b", "mouse", "down", "left");
  browser("b", "mouse", "move", String(point.origin[0] - 12), String(point.origin[1]));
  await waitFor(() => evaluate("b", "Boolean(document.querySelector('.pokepath-box-ghost'))"), "warehouse portrait follows the real drag");
  browser("b", "mouse", "move", ...point.target.map(String));
  browser("b", "mouse", "up", "left");
  await waitFor(() => [...players, "s"].every(label => data(label).team[0]?.id === second && data(label).box.some(pokemon => pokemon.id === first) && data(label).towers.length === 0), "real pointer drop replaces one shared slot and retires its old tower");
  click("b", card(second));
  click("b", "[data-box-name]");
  browser("b", "fill", "[data-box-rename-input]", "潮汐");
  click("b", "[data-box-rename-accept]");
  await waitFor(() => [...players, "s"].every(label => data(label).team[0].alias === "潮汐"), "warehouse nickname update reaches both seats and the viewer");
  const now = room("a");
  const stale = request("a", `/api/rooms/${roomID}`, { type: "action", version: now.version, nonce: randomUUID(), action: { type: "teamSlot", epoch: now.game.data.epoch, pokemonID: first, slot: 0, targetID: first } });
  assert.match(JSON.stringify(stale.body), /位置已经变化/);
  assert.equal(evaluate("s", `document.querySelector('[data-box-command="box"]').disabled`), true);
  const readonly = room("s");
  assert.ok(request("s", `/api/rooms/${roomID}`, { type: "action", version: readonly.version, nonce: randomUUID(), action: { type: "teamSlot", epoch: readonly.game.data.epoch, pokemonID: first, slot: 0, targetID: second } }).status >= 400);
  browser("b", "set", "viewport", "320", "740");
  checkpoint("b", "box-mobile-shared-roster");
  touchControl("b", card(second));
  touchControl("b", '[data-box-command="box"]');
  await waitFor(() => data("a").team.length === 0, "real mobile touch removes the selected partner");
  touchControl("b", card(first));
  touchControl("b", '[data-box-command="team"]');
  await waitFor(() => players.every(label => data(label).team[0]?.id === first && data(label).box.some(pokemon => pokemon.id === second)), "real mobile touch restores the other partner for both seats");
  assert.equal(data("a").gold, data("b").gold);
  for (const label of [...players, "s"]) assert.equal(evaluate(label, "window.__nativeAudioAttempts.length"), 0);
  checkpoint("b", "box-mobile-touch-transfer");
  report.checks.push({ name: "shared-box-slot-and-mobile-transfer", first, second, staleStatus: stale.status, deployedPartnerRetired: true, spectatorReadOnly: true, touchWidth: 320 });
}

async function mapAcceptance() {
  const players = ["a", "b"];
  for (const label of players) {
    openAccount(label, { name: `地图${label}${owner}`, password: "native-browser-test-password" });
    browser(label, "set", "viewport", "1440", "900");
  }
  roomID = api("a", "/api/rooms", { kind: "pokepath", capacity: 2, name: "合作地图机关验收" }).id;
  assert.equal(request("b", `/api/rooms/${roomID}`, { type: "join", version: room("a").version, nonce: randomUUID() }).status, 200);
  for (const label of players) { browser(label, "reload"); browser(label, "wait", ".waiting-cover"); command(label, "ready"); }
  writeFileSync(join(output, "map-ready"), "map\n");
  command("a", "start");
  for (const label of players) await waitCanvas(label);
  await waitFor(() => players.every(label => !room(label).loading && data(label).route === 6), "two seats see the route before revealing the secret");
  const digest = label => evaluate(label, `(async()=>{const canvas=document.querySelector('[data-pokepath-canvas]'),pixels=canvas.getContext('2d').getImageData(0,0,canvas.width,canvas.height).data,sha=new Uint8Array(await crypto.subtle.digest('SHA-256',pixels));return Array.from(sha).map(byte=>byte.toString(16).padStart(2,'0')).join('')})()`);
  const requested = label => evaluate(label, "performance.getEntriesByType('resource').some(resource=>resource.name.endsWith('/maps/3-1bis.png'))");
  const compare = label => evaluate(label, `(async()=>{
    const canvas=document.querySelector('[data-pokepath-canvas]'),{width,height}=canvas;
    const decode=async name=>{const resource=performance.getEntriesByType('resource').find(entry=>entry.name.endsWith('/maps/'+name));if(!resource)return null;const image=new Image();image.src=resource.name;await image.decode();const sample=document.createElement('canvas');sample.width=width;sample.height=height;const ctx=sample.getContext('2d',{willReadFrequently:true});ctx.drawImage(image,0,0,width,height);return ctx.getImageData(0,0,width,height).data};
    const [initial,revealed]=await Promise.all(['3-1.png','3-1bis.png'].map(decode));
    if(!initial||!revealed)return null;
    const actual=canvas.getContext('2d').getImageData(0,0,width,height).data;
    let distinct=0,before=0,after=0;
    for(let index=0;index<actual.length;index++){if(Math.abs(initial[index]-revealed[index])<8)continue;distinct++;before+=Math.abs(initial[index]-actual[index]);after+=Math.abs(revealed[index]-actual[index])}
    return{distinct,baseMAE:before/distinct,revealedMAE:after/distinct};
  })()`);
  const drawsRevealedMap = value => Boolean(value && value.distinct > 0 && value.baseMAE > 8 && value.revealedMAE < 1);
  const before = digest("a");
  assert.equal(digest("b"), before);
  for (const label of players) assert.equal(requested(label), false);
  const hotspot = data("a").progression.hotspots.find(value => value.key === "gengarite");
  checkpoint("a", "shared-map-before-discovery");
  canvasClick("a", hotspot.x + hotspot.width / 2, hotspot.y + hotspot.height / 2);
  const after = await waitFor(() => {
    if (!players.every(label => data(label).progression.secretItems.gengarite && requested(label))) return false;
    const samples = players.map(compare);
    return samples.every(drawsRevealedMap) ? samples : false;
  }, "both seats draw the revealed map after the authoritative pickup");
  checkpoint("b", "shared-map-after-discovery");
  const route = id => command("b", "action", { action: { type: "route", route: id, epoch: data("b").epoch } });
  const drawnRoute = label => Number(evaluate(label, "document.querySelector('[data-pokepath-board]').dataset.route"));
  route(0);
  await waitFor(() => players.every(label => data(label).route === 0 && drawnRoute(label) === 0 && !drawsRevealedMap(compare(label))), "both browser engines load the previously unseen route before drawing it");
  checkpoint("b", "shared-map-new-route-installed");
  route(6);
  const revisited = await waitFor(() => {
    if (!players.every(label => data(label).route === 6 && drawnRoute(label) === 6)) return false;
    const samples = players.map(compare);
    return samples.every(drawsRevealedMap) ? samples : false;
  }, "revealed art survives returning to the route");
  assert.ok(data("b").items.some(item => item.id === "gengarite"));
  for (const label of players) assert.equal(evaluate(label, "document.querySelector('.pokepath-action-error')?.textContent ?? ''"), "");
  report.checks.push({ name: "shared-secret-map-resource-and-revisit", route: 6, players, initial: before, revealed: after, revisited, lazyLoaded: true });
}

async function rangeAcceptance() {
  const players = ["a", "b"];
  for (const label of players) {
    openAccount(label, { name: `射程${label}${owner}`, password: "native-browser-test-password" });
    browser(label, "set", "viewport", "1440", "900");
  }
  roomID = api("a", "/api/rooms", { kind: "pokepath", capacity: 2, name: "合作候选射程验收" }).id;
  assert.equal(request("b", `/api/rooms/${roomID}`, { type: "join", version: room("a").version, nonce: randomUUID() }).status, 200);
  for (const label of players) { browser(label, "reload"); browser(label, "wait", ".waiting-cover"); command(label, "ready"); }
  writeFileSync(join(output, "range-ready"), "range\n");
  command("a", "start");
  for (const label of players) await waitCanvas(label);
  await waitFor(() => players.every(label => !room(label).loading && data(label).team.length === 3 && data(label).towers.length === 1), "both seats have loaded the deterministic range room");
  const scout = data("b").team.find(pokemon => pokemon.species === "pidgey");
  const light = data("b").team.find(pokemon => pokemon.species === "patrat");
  const field = data("b").tiles.find(tile => tile.id === 14);
  const sample = { x: field.x + 12, y: field.y + 12 - 135 };
  const rgb = () => evaluate("b", `Array.from(document.querySelector('[data-pokepath-canvas]').getContext('2d').getImageData(${sample.x},${sample.y},1,1).data)`);
  const initial = rgb();
  choose("b", scout.id); fit("b");
  const position = logicalPoint("b", field.x + 12, field.y + 12);
  browser("b", "mouse", "move", String(Math.round(position.x)), String(Math.round(position.y)));
  let ruleRange = null;
  const supported = await waitFor(() => {
    const colors = rgb();
    ruleRange = data("b").team.find(pokemon => pokemon.id === scout.id).combatStats.range;
    return colors[0] < initial[0] && colors[2] > initial[2] ? colors : false;
  }, "Patrat light extends the hypothetical tile beyond 125 pixels");
  assert.equal(ruleRange, 125);
  checkpoint("b", "prospective-range-with-shared-light");
  click("a", `[data-pokemon-id="${light.id}"] .deploy`);
  await waitFor(() => data("b").towers.length === 0 && JSON.stringify(rgb()) === JSON.stringify(initial), "partner removes support and the pending preview shortens without moving the pointer");
  checkpoint("b", "prospective-range-after-peer-retires-light");
  canvasClick("b", field.x + 12, field.y + 12);
  await waitFor(() => players.every(label => data(label).towers.some(tower => tower.pokemonID === scout.id && tower.tileID === field.id) && data(label).team.find(pokemon => pokemon.id === scout.id).combatStats.range === 125), "real deployment uses the unlit range and both seats agree");
  report.checks.push({ name: "destination-preview-updates-after-peer-support-change", players, distance: 135, baseRange: 125, supported, initial, footprintDroppedOnPeerRetire: true, deployedRange: 125 });
}

try {
  if (process.argv.includes("--build")) execFileSync(process.execPath, [join(root, "web/node_modules/vite/bin/vite.js"), "build", "--configLoader", "runner", "--config", "games/pokepath/build/vite.mjs"], { cwd: root, env: environment, stdio: "inherit", timeout: 180000 });
  assert.ok(existsSync(join(webRoot, "index.html")), "Build native Vite delivery first or pass --build");
  report.webRoot = webRoot;
  const manifest = readFileSync(join(webRoot, "pokepath/v1/native/manifest.json"), "utf8");
  writeFileSync(join(output, "native-manifest.json"), manifest);
  const runtimeManifest = JSON.parse(manifest);
  report.runtime = { engine: runtimeManifest.engine, abi: runtimeManifest.abi, sourceSHA256: runtimeManifest.sourceSHA256, buildSHA256: runtimeManifest.buildSHA256, goVersion: runtimeManifest.goVersion };
  await startFixture();
  if (verifyChallenge) await challengeAcceptance();
  else if (verifyResult) await resultAcceptance(verifyResult);
  else if (verifyBox) await boxAcceptance();
  else if (verifyMaps) await mapAcceptance();
  else if (verifyRange) await rangeAcceptance();
  else {
  for (const label of ["a", "b", "s"]) {
    const account = { name: `原生${label}${owner}`, password: "native-browser-test-password" };
    accounts.set(label, account);
    openAccount(label, account);
  }
  const created = api("a", "/api/rooms", { kind: "pokepath", capacity: 2, name: "原生巡径合作验收", options: { starter: "charmander" } });
  roomID = created.id;
  const joined = request("b", `/api/rooms/${roomID}`, { type: "join", version: room("a").version, nonce: randomUUID() });
  assert.equal(joined.status, 200, JSON.stringify(joined));
  for (const label of ["a", "b"]) { browser(label, "reload"); browser(label, "wait", ".waiting-cover"); }
  assert.equal(evaluate("a", "Boolean(document.querySelector('[data-fullscreen-toggle]'))"), false);
  command("a", "ready"); command("b", "ready"); command("a", "start");
  for (const label of ["a", "b"]) await waitCanvas(label);
  await waitFor(() => !room("a").loading && data("a").gold === 50, "both real native clients load Go/Wasm", 90000);
  matchID = room("a").matchId;
  for (const label of ["a", "b"]) assert.equal(evaluate(label, "window.__nativeAudioAttempts.length"), 0, "global mute was bypassed");
  checkpoint("a", "desktop-native-cooperative-start", true);
  api("s", `/api/rooms/${roomID}/watch`, {}); browser("s", "reload"); await waitCanvas("s");
  const observed = room("s");
  const denied = request("s", `/api/rooms/${roomID}`, { type: "action", version: observed.version, nonce: randomUUID(), action: { epoch: observed.game.data.epoch, type: "speed" } });
  assert.ok(denied.status >= 400, "spectator changed the shared native world");
  assert.equal(evaluate("s", `document.querySelector('.pokepath-board .upgrade')?.disabled`), true);
  report.checks.push({ name: "spectator-native-authority-rejected", status: denied.status });
  const settingsMenu = '[data-classic-section="settings"]';
  const nextLanguage = '[data-setting-next="language"]';
  const dictionary = JSON.parse(readFileSync(join(webRoot, "pokepath/v1/ui-text.json"), "utf8"));
  const settings = label => data(label).progression.settings;
  for (const label of ["a", "b"]) click(label, settingsMenu);
  const languages = [];
  for (let offset = 1; offset <= 10; offset++) {
    const language = (8 + offset) % 10;
    click("a", nextLanguage);
    const title = dictionary.text.menu.settings.title[language];
    await waitFor(() => settings("b").language === language && evaluate("b", "document.querySelector('[data-settings-section=settings] h2')?.textContent") === title, `peer sees language ${language}`);
    languages.push({ language, title });
  }
  click("s", settingsMenu);
  await waitFor(() => evaluate("s", `document.querySelector(${JSON.stringify(nextLanguage)})?.disabled === true`), "ready spectator can view settings without changing them");
  const musicSlider = "#pokepath-volume-music";
  browser("a", "scrollintoview", musicSlider);
  browser("a", "focus", musicSlider);
  browser("a", "press", "Home");
  await waitFor(() => settings("b").audio.music === 0 && evaluate("b", `document.querySelector('${musicSlider}')?.parentElement.querySelector('output').textContent`) === "0", "peer sees actual music gain zero");
  browser("a", "press", "End");
  await waitFor(() => settings("b").audio.music === 20, "peer sees actual maximum music gain");
  for (const label of ["a", "b", "s"]) {
    assert.equal(evaluate(label, "window.__nativeAudioAttempts.length"), 0);
    click(label, settingsMenu);
  }
  report.checks.push({ name: "shared-native-settings-and-spectator-read-only", languages, musicGain: [0, 20], mutedByPlatform: true });
  const initial = data("a");
  const firstID = initial.team[0].id;
  click("a", `[data-pokemon-id="${firstID}"] .upgrade`);
  await waitFor(() => data("a").team[0].level === initial.team[0].level + 1, "trusted native mouse upgrade");
  await waitFor(() => data("b").gold === data("a").gold, "shared upgrade cost");
  const priorEggs = data("a").progression.remainingEggs;
  click("b", '.pokepath-classic-menu [data-classic-section="shop"]'); click("b", '.pokepath-shop-egg');
  await waitFor(() => data("a").team.length >= 2, "normal native egg purchase");
  const shared = data("a");
  const secondID = shared.team.find(pokemon => pokemon.id !== firstID).id;
  await waitFor(() => evaluate("b", `Boolean(document.querySelector('[data-shop-hatch="${secondID}"]'))`), "server receipt opens the actual hatched partner");
  assert.equal(shared.progression.remainingEggs, priorEggs - 1);
  assert.equal(evaluate("a", "Boolean(document.querySelector('[data-shop-hatch]'))"), false);
  const afterPurchase = room("a");
  const staleEgg = request("a", `/api/rooms/${roomID}`, { type: "action", version: afterPurchase.version, nonce: randomUUID(), action: { type: "egg", quote: priorEggs, epoch: shared.epoch } });
  assert.ok(staleEgg.status >= 400 && staleEgg.status !== 409, JSON.stringify(staleEgg));
  assert.match(JSON.stringify(staleEgg.body), /蛋池已经变化/);
  assert.equal(data("a").gold, shared.gold);
  checkpoint("b", "shop-authority-hatch-buyer-only");
  click("b", "[data-hatch-close]");
  await waitFor(() => !evaluate("b", "Boolean(document.querySelector('[data-shop-hatch]'))"), "hatch closes before the next action");
  click("b", '.pokepath-classic-menu [data-classic-section="shop"]');
  report.checks.push({ name: "shared-shop-authority", priorEggs, remainingEggs: shared.progression.remainingEggs, buyer: "b", partner: secondID, staleStatus: staleEgg.status });
  choose("a", firstID); choose("b", secondID);
  assert.equal(inspector("a"), firstID); assert.equal(inspector("b"), secondID);
  assert.equal(data("a").gold, data("b").gold);
  const catalog = JSON.parse(readFileSync(join(webRoot, "pokepath/v1/catalog.json"), "utf8"));
  const deploy = async (label, id, alreadyChoosing = false) => {
    if (!alreadyChoosing) { choose(label, id); fit(label); }
    const value = data(label), pokemon = value.team.find(pokemon => pokemon.id === id), definition = catalog.species[pokemon.species];
    const entry = value.paths[0][0];
    const tiles = value.tiles.filter(tile => definition.terrains.includes(tile.terrain) && !value.towers.some(tower => tower.tileID === tile.id)).sort((a, b) => Math.abs(Math.hypot(a.x + 12 - entry.x, a.y + 12 - entry.y) - 90) - Math.abs(Math.hypot(b.x + 12 - entry.x, b.y + 12 - entry.y) - 90));
    assert.ok(tiles.length, "no legitimate deployment site");
    const chosen = tiles[0];
    const point = canvasClick(label, chosen.x + 12, chosen.y + 12);
    await waitFor(() => data(label).towers.some(tower => tower.pokemonID === id && tower.tileID === chosen.id), "native Canvas deployment");
    return { chosen, point };
  };
  const firstPlacement = await deploy("a", firstID);
  await waitFor(() => data("b").towers.some(tower => tower.tileID === firstPlacement.chosen.id), "second seat receives the already occupied tile");
  choose("b", secondID); fit("b");
  const occupiedTile = firstPlacement.chosen;
  canvasClick("b", occupiedTile.x + 12, occupiedTile.y + 12);
  assert.equal(data("b").towers.some(tower => tower.pokemonID === secondID), false);
  assert.equal(evaluate("b", "document.querySelector('.toast.error')?.textContent?.trim() ?? ''"), "");
  const secondPlacement = await deploy("b", secondID, true);
  report.checks.push({ name: "deployment-preview-observes-other-seat", occupied: occupiedTile.id, subsequentValid: secondPlacement.chosen.id, retainedSelection: true });
  choose("a", secondID);
  browser("a", "press", "Escape");
  const firstTower = data("a").towers.find(tower => tower.pokemonID === firstID);
  canvasClick("a", firstTower.x, firstTower.y - 4);
  assert.equal(inspector("a"), firstID, "the drawn tower body did not open its management state");
  assert.equal(evaluate("a", `Boolean(document.querySelector('[data-details-pokemon="${firstID}"]'))`), true);
  assert.equal(inspector("b"), secondID, "one player's local selection replaced the other player's selection");
  report.checks.push({ name: "trusted-mouse-deploy-and-independent-selection", firstPlacement, secondPlacement, selected: { a: inspector("a"), b: inspector("b") }, world: data("a").snapshot });
  click("a", '[aria-label="关闭单位详情"]');
  browser("b", "set", "viewport", "320", "740");
  checkpoint("b", "mobile-native-shared-resources", true);
  choose("b", firstID, true); browser("b", "press", "Escape"); fit("b", true);
  const secondTower = data("b").towers.find(tower => tower.pokemonID === secondID);
  const actualTowerTouch = canvasClick("b", secondTower.x, secondTower.y - 4, true);
  assert.equal(inspector("b"), secondID, "mobile touch missed the actual drawn tower");
  assert.equal(evaluate("b", `Boolean(document.querySelector('[data-details-pokemon="${secondID}"]'))`), true);
  assert.equal(inspector("a"), firstID);
  touchControl("b", '[aria-label="关闭单位详情"]');
  touchControl("b", `[data-pokemon-id="${secondID}"] .deploy`);
  await waitFor(() => !data("b").towers.some(tower => tower.pokemonID === secondID), "trusted touch retire");
  const current = data("b"), chosen = current.tiles.find(tile => tile.id === secondPlacement.chosen.id);
  choose("b", secondID, true); fit("b", true); canvasClick("b", chosen.x + 12, chosen.y + 12, true);
  await waitFor(() => data("b").towers.some(tower => tower.pokemonID === secondID), "trusted touch redeploy");
  report.checks.push({ name: "mobile-drawn-tower-touch-management", actualTowerTouch, pokemonID: secondID });
  browser("b", "scrollintoview", "[data-fullscreen-toggle]");
  click("b", "[data-fullscreen-toggle]");
  await waitFor(() => evaluate("b", "Boolean(document.fullscreenElement||document.querySelector('[data-room-fullscreen]'))"), "uniform game fullscreen");
  const fullscreen = evaluate("b", `(()=>{const node=document.fullscreenElement;return {native:Boolean(node),root:node?.getAttribute('data-game-fullscreen-surface'),isPage:node===document.body||node===document.documentElement,platformMenuInside:Boolean(node?.querySelector('.site-header,.room-tools')),canvasInside:Boolean(node?.querySelector('[data-pokepath-canvas]'))}})()`);
  assert.equal(fullscreen.isPage, false); assert.equal(fullscreen.platformMenuInside, false); assert.equal(fullscreen.canvasInside, true);
  checkpoint("b", "mobile-native-uniform-fullscreen");
  browser("b", "press", "Escape");
  await waitFor(() => !evaluate("b", "Boolean(document.fullscreenElement||document.querySelector('[data-room-fullscreen]'))"), "fullscreen ESC exit");
  report.checks.push({ name: "native-fullscreen-scope-and-escape", fullscreen });
  click("b", '.pokepath-board [aria-label="下一波"]');
  await waitFor(() => data("a").waveActive && data("b").waveActive, "shared native wave starts");
  click("a", "[data-save-game]");
  await waitFor(() => data("b").saving && room("b").game.paused, "first native save consent freezes cooperation");
  const frozen = data("a").snapshot;
  await pause(200);
  assert.deepEqual(data("b").snapshot, frozen, "native world advanced after the save freeze");
  click("b", "[data-save-game]");
  await waitFor(() => !room("a") && !room("b"), "both original seats are released");
  assert.ok(api("a", "/api/saves").saves.some(saved => saved.id === roomID));
  report.checks.push({ name: "native-two-human-save-consent", frozen });
  const solo = api("b", "/api/rooms", { kind: "pokepath", capacity: 1, name: "触摸升级正式规则验收", options: { starter: "treecko" } });
  command("b", "ready"); command("b", "start");
  await waitCanvas("b"); await waitFor(() => !room("b").loading && data("b").gold === 50, "new normal room for touch upgrade");
  const soloBefore = data("b").team[0];
  touchControl("b", `[data-pokemon-id="${soloBefore.id}"] .upgrade`);
  await waitFor(() => data("b").team[0].level === soloBefore.level + 1, "trusted mobile touch upgrade");
  checkpoint("b", "mobile-native-touch-upgrade");
  report.checks.push({ name: "touch-upgrade-under-normal-new-room-rules", roomID: solo.id, gold: data("b").gold, level: data("b").team[0].level });
  command("b", "close");
  command("b", "leave");
  await waitFor(() => !room("b"), "solo input acceptance cleanup");
  for (const label of ["a", "b", "s"]) closeSession(label);
  await stopFixture(); await startFixture(true);
  for (const label of ["a", "b"]) openAccount(label, accounts.get(label), true);
  click("a", `[data-resume-save="${roomID}"]`);
  await waitFor(() => room("a")?.status === "restoring", "first original account waits for partner");
  assert.ok(!room("b"));
  holdLoadReady("b", roomID);
  click("b", `[data-resume-save="${roomID}"]`);
  for (const label of ["a", "b"]) await waitCanvas(label);
  await waitFor(() => evaluate("b", "window.__pokepathLoadReadyGate.held === 1"), "second client's real load acknowledgement is held", 90000);
  const readyRoom = await waitFor(() => { const value = room("a"); return value?.status === "playing" && value.loading?.ready?.filter(Boolean).length === 1 && value; }, "first client ready and restored world still frozen", 90000);
  const restored = readyRoom.game.data.snapshot;
  assert.equal(readyRoom.matchId, matchID); assert.equal(readyRoom.id, roomID);
  assert.ok(restored.epoch > frozen.epoch);
  assert.deepEqual(physical(restored), physical(frozen), "cold continuation changed the saved world before either side could advance it");
  assert.deepEqual(data("b").snapshot, restored, "the two viewers received different restored states");
  evaluate("b", "window.__pokepathLoadReadyGate.release()");
  const resumedRoom = await waitFor(() => { const value = room("a"); return value?.status === "playing" && !value.loading && value.game.data.snapshot.clock > restored.clock && value; }, "live rule ticks continue after the real ready request reaches the server", 90000);
  const resumed = resumedRoom.game.data.snapshot;
  assert.equal(resumedRoom.matchId, matchID); assert.equal(resumedRoom.id, roomID);
  assert.equal(resumed.epoch, restored.epoch);
  assert.ok(data("a").towers.length === frozen.towers.length && data("b").wave === frozen.wave);
  for (const label of ["a", "b"]) assert.equal(evaluate(label, "window.__nativeAudioAttempts.length"), 0);
  checkpoint("a", "desktop-native-cold-original-roster-continuation", true);
  report.checks.push({ name: "cold-original-roster-native-continuation", roomID, matchID, before: frozen, atLoadBarrier: restored, after: resumed, exactAtLoadBarrier: true, resourcesPreserved: true, offlineRulesCatchup: restored.clock - frozen.clock, elapsedAfterReady: resumed.clock - restored.clock });
  }
  report.passed = true;
  if (serve) {
    const owned = { pid: process.pid, fixturePID: fixture.pid, base, roomID, matchID, output, mutedByBrowserFlag: true, accounts: Object.fromEntries([...accounts].map(([label, account]) => [label, { name: account.name }])), browsers: Object.fromEntries([...sessions].map(([label, session]) => [label, { session: session.name, profile: session.profile, cdp: cdp(label) }])) };
    writeFileSync(join(output, "owned.json"), JSON.stringify(owned, null, 2));
    writeFileSync(join(root, ".local/pokepath/native-browser/latest.json"), JSON.stringify(owned, null, 2));
    process.stdout.write(`OWNED ${JSON.stringify(owned)}\n`);
    writeFileSync(join(output, "result.json"), JSON.stringify(report, null, 2));
    await new Promise(resolve => { process.once("SIGTERM", resolve); process.once("SIGINT", resolve); const timer = setInterval(() => { if (existsSync(join(output, "end"))) { clearInterval(timer); resolve(); } }, 500); timer.unref(); });
  }
} catch (error) {
  serve = false;
  report.error = error.stack ?? String(error);
  report.diagnostics = {};
  for (const label of sessions.keys()) {
    try { report.diagnostics[label] = { state: api(label, "/api/state"), errors: browser(label, "errors"), text: evaluate(label, "document.body.innerText"), snapshot: browser(label, "snapshot", "-i") }; browser(label, "screenshot", join(output, `failure-${label}.png`), "--full"); } catch (cause) { report.diagnostics[label] = String(cause); }
  }
  process.stderr.write(`${report.error}\n`);
  process.exitCode = 1;
} finally {
  for (const label of [...sessions.keys()]) { try { closeSession(label); } catch (error) { report.cleanupError = String(error); } }
  try { await stopFixture(); } catch (error) { report.cleanupError = String(error); }
  if (log !== undefined) closeSync(log);
  writeFileSync(join(output, "result.json"), JSON.stringify(report, null, 2));
  process.stdout.write(`RESULT ${join(output, "result.json")}\n`);
}
