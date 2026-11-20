import assert from "node:assert/strict";
import { execFileSync, spawn } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync, existsSync, rmSync, openSync, closeSync } from "node:fs";
import { randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const out = path.join(root, ".local/pokepath/independent-trainers/browser");
const temporary = path.join(out, "temporary");
const sockets = path.join(root, ".local/pokepath/s");
for (const folder of [out, temporary, sockets, path.join(root, ".local/tmp"), path.join(root, ".local/go-build"), path.join(root, ".local/go-mod")]) mkdirSync(folder, { recursive: true });
const standardWeb = process.argv.includes("--standard-web");
assert.ok(!standardWeb || !process.argv.includes("--build"));
const env = { ...process.env, TMPDIR: path.join(root, ".local/tmp"), GOTMPDIR: path.join(root, ".local/tmp"), GOCACHE: path.join(root, ".local/go-build"), GOMODCACHE: path.join(root, ".local/go-mod"), GOTOOLCHAIN: "local", WIRE_BOARD_POKEPATH_BROWSER: "1", WIRE_BOARD_POKEPATH_SCOPED_WEB: standardWeb ? "0" : "1", WIRE_BOARD_POKEPATH_BROWSER_OUT: temporary, AGENT_BROWSER_SOCKET_DIR: sockets, AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const init = path.join(temporary, "init.js");
writeFileSync(init, "localStorage.setItem('wb_sound','off')");
env.AGENT_BROWSER_INIT_SCRIPTS = init;
const sessions = new Set();
const browser = (label, ...args) => {
  sessions.add(label);
  const raw = execFileSync("agent-browser", ["--session", `pt${label}`, ...args, "--json"], { cwd: root, env: { ...env, AGENT_BROWSER_PROFILE: path.join(temporary, `profile-${label}`) }, encoding: "utf8", timeout: 30000, maxBuffer: 16 << 20 });
  const result = JSON.parse(raw);
  assert.equal(result.success, true, raw);
  return result.data;
};
const evaluate = (label, source) => browser(label, "eval", "-b", Buffer.from(source).toString("base64")).result;
const request = (label, resource, body) => evaluate(label, `fetch(${JSON.stringify(resource)},${body === undefined ? "{}" : `{method:'POST',headers:{'Content-Type':'application/json'},body:${JSON.stringify(JSON.stringify(body))}}`}).then(async response=>({status:response.status,body:await response.json()}))`);
const api = (label, resource, body) => { const result = request(label, resource, body); assert.ok(result.status >= 200 && result.status < 300, JSON.stringify(result)); return result.body; };
const room = label => api(label, "/api/state").room;
const data = label => room(label)?.game?.data;
const command = (label, type, extra = {}) => {
  for (let attempt = 0; attempt < 10; attempt++) {
    const current = room(label);
    const result = request(label, `/api/rooms/${current.id}`, { type, version: current.version, nonce: randomUUID(), ...extra });
    if (result.status === 409) continue;
    assert.equal(result.status, 200, JSON.stringify(result));
    return result.body;
  }
  throw Error(`Command did not settle: ${type}`);
};
const pause = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const waitFor = async (check, name, timeout = 30000) => { const start = Date.now(); while (Date.now() - start < timeout) { if (check()) return; await pause(100); } throw Error(`Timed out: ${name}`); };
const cardIDs = label => evaluate(label, "[...document.querySelectorAll('.pokepath-classic-card[data-pokemon-id]')].map(node=>Number(node.dataset.pokemonId))");
const selected = label => evaluate(label, "document.querySelector('.pokepath-classic-card.selected')?.dataset.pokemonId");
const click = (label, selector) => { browser(label, "scrollintoview", selector); browser(label, "click", selector); };
const checkpoint = (label, name) => { browser(label, "screenshot", path.join(out, `${name}.png`)); writeFileSync(path.join(out, `${name}.json`), JSON.stringify({ room: room(label), cards: cardIDs(label), ui: browser(label, "snapshot", "-i") }, null, 2)); };
const deploy = (label, pokemon, tile) => {
  click(label, `[data-pokemon-id="${pokemon}"] .pokepath-card-portrait`);
  browser(label, "scrollintoview", "[data-pokepath-canvas]");
  const point = evaluate(label, `(()=>{const canvas=document.querySelector('[data-pokepath-canvas]'),rect=canvas.getBoundingClientRect();return {x:rect.x+${tile.x + 12}*rect.width/canvas.width,y:rect.y+${tile.y + 12}*rect.height/canvas.height}})()`);
  browser(label, "mouse", "move", String(Math.round(point.x)), String(Math.round(point.y)));
  browser(label, "mouse", "down", "left");
  browser(label, "mouse", "up", "left");
};
if (process.argv.includes("--build")) execFileSync(process.execPath, [path.join(root, "web/node_modules/vite/bin/vite.js"), "build", "--configLoader", "runner", "--config", "games/pokepath/build/vite.mjs"], { cwd: root, env, stdio: "inherit", timeout: 180000 });
const result = { passed: false, command: ["node games/pokepath/tests/native-trainers-browser.mjs", ...process.argv.slice(2)].join(" "), checks: [], rules: JSON.parse(readFileSync(path.join(root, "games/pokepath/data/rules-revision.json"), "utf8")).rulesVersion };
const log = openSync(path.join(out, "server.log"), "w");
const fixture = spawn("go", ["test", "-count=1", "-timeout=12m", "-run", "^TestPokepathBrowserFixture$", "games/pokepath/tests/acceptance_fixture_test.go"], { cwd: root, env, stdio: ["ignore", log, log] });
try {
  await waitFor(() => existsSync(path.join(temporary, "url")), "HTTP fixture", 90000);
  const base = readFileSync(path.join(temporary, "url"), "utf8");
  const suffix = randomUUID().slice(0, 8);
  for (const label of ["a", "b"]) {
    browser(label, "open", base);
    browser(label, "set", "viewport", "1280", "900");
    api(label, "/api/register", { name: `训练家${label}${suffix}`, password: "trainer-browser-only", invite: "pokepath-e2e" });
  }
  const created = api("a", "/api/rooms", { kind: "pokepath", capacity: 2, name: "独立训练家验收", options: {} });
  api("b", `/api/rooms/${created.id}`, { type: "join", version: room("a").version, nonce: randomUUID() });
  command("a", "ready"); command("b", "ready"); command("a", "start");
  for (const label of ["a", "b"]) browser(label, "reload");
  for (const label of ["a", "b"]) browser(label, "wait", "[data-pokepath-canvas]");
  await waitFor(() => !room("a").loading && cardIDs("a").length === 3 && cardIDs("b").length === 3, "independent initial interfaces", 90000);
  const first = data("a").team[0].id;
  const second = data("b").team[0].id;
  assert.equal(cardIDs("a").some(id => cardIDs("b").includes(id)), false);
  assert.deepEqual(data("a").team.map(value => value.species), ["charmander", "treecko", "froaki"]);
  const selection = selected("a");
  click("b", `[data-pokemon-id="${data("b").team[2].id}"] .pokepath-card-portrait`);
  assert.equal(selected("a"), selection);
  click("a", `[data-pokemon-id="${first}"] .upgrade`);
  await waitFor(() => data("a").gold === 10 && data("b").gold === 50, "personal upgrade and wallet");
  for (const label of ["a", "b"]) click(label, '[data-classic-section="shop"]');
  click("b", ".pokepath-shop-egg");
  await waitFor(() => data("b").gold === 40 && data("b").team.length === 4 && evaluate("b", "Boolean(document.querySelector('[data-shop-hatch]'))"), "personal purchase and hatch");
  assert.equal(evaluate("a", "Boolean(document.querySelector('[data-shop-hatch]'))"), false);
  assert.equal(data("a").progression.eggPrice, 10);
  assert.equal(data("a").team.length, 3);
  assert.equal(data("b").progression.eggPrice, 47);
  checkpoint("a", "owner-shop"); checkpoint("b", "partner-hatch");
  click("b", "[data-hatch-close]");
  for (const label of ["a", "b"]) click(label, '[data-classic-section="shop"]');
  await waitFor(() => cardIDs("a").length === 3 && cardIDs("b").length === 4, "separate rendered rosters");
  const current = room("b");
  const denial = request("b", `/api/rooms/${current.id}`, { type: "action", version: current.version, nonce: randomUUID(), action: { type: "upgrade", epoch: data("b").epoch, pokemonID: first } });
  assert.ok(denial.status >= 400);
  const tiles = data("a").tiles.filter(tile => tile.terrain === 1);
  deploy("a", first, tiles[0]);
  await waitFor(() => data("a").towers.length === 1, "owner deployment");
  deploy("b", second, tiles[1]);
  await waitFor(() => data("a").towers.length === 2 && data("b").towers.length === 2, "shared battle with separate towers");
  assert.equal(evaluate("a", `Boolean(document.querySelector('[data-pokemon-id="${second}"]'))`), false);
  assert.equal(evaluate("b", `Boolean(document.querySelector('[data-pokemon-id="${first}"]'))`), false);
  checkpoint("a", "desktop-owner"); checkpoint("b", "desktop-partner");
  browser("a", "set", "viewport", "320", "780");
  await waitFor(() => evaluate("a", "document.querySelector('[data-pokepath-board]').getBoundingClientRect().width <= 320"), "phone layout");
  checkpoint("a", "phone-owner");
  const summaries = evaluate("a", "[...document.querySelectorAll('[data-pokepath-gold]')].map(node=>({text:node.textContent,rect:node.getBoundingClientRect().toJSON()}))");
  assert.deepEqual(summaries.map(value => value.text.match(/\d+/)?.[0]), ["10", "40"]);
  assert.ok(summaries.every(value => value.rect.x >= 0 && value.rect.right <= 320));
  result.checks.push({ name: "personal-rosters-shops-selection-and-hatches", first, second, ownerGold: 10, partnerGold: 40, cards: [3, 4], denial: denial.status });
  result.checks.push({ name: "common-battle-and-mobile-balances", towers: 2, summaries });
  result.passed = true;
} catch (error) {
  process.stderr.write(`${error.stack}\n`);
  for (const label of sessions) { try { browser(label, "screenshot", path.join(out, `failure-${label}.png`)); process.stderr.write(JSON.stringify({ label, errors: browser(label, "errors"), body: browser(label, "get", "text", "body") }) + "\n"); } catch {} }
  throw error;
} finally {
  for (const label of sessions) { try { browser(label, "close"); } catch {} }
  writeFileSync(path.join(temporary, "stop"), "stop");
  await waitFor(() => fixture.exitCode !== null, "fixture shutdown", 30000).catch(() => fixture.kill("SIGTERM"));
  closeSync(log);
  await pause(100);
  rmSync(temporary, { recursive: true, force: true });
  writeFileSync(path.join(out, "result.json"), `${JSON.stringify(result, null, 2)}\n`);
}
