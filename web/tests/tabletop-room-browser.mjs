import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, writeFile, readFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";
import path from "node:path";
import { createRequire } from "node:module";

const root = fileURLToPath(new URL("../../", import.meta.url));
let base = process.env.TABLETOP_ROOM_URL;
assert.match(base ?? "", /^http:\/\/127\.0\.0\.1:\d+\/?$/);
const reportName = process.env.TABLETOP_LAYOUT_REPORT ?? "";
assert.match(reportName, /^(?:[a-z0-9]+(?:-[a-z0-9]+)*)?$/);
const out = path.join(root, ".local/splendor/fullscreen", reportName);
const temporary = path.join(out, "browser-temporary");
await mkdir(path.join(root,".local/splendor/t"),{recursive:true});
await mkdir(path.join(root,".local/splendor/s"),{recursive:true});
for (const name of ["profile", "config", "data", "cache"]) await mkdir(path.join(temporary, name), { recursive: true });
const env = { ...process.env, TMPDIR: path.join(root, ".local/splendor/t"), XDG_CONFIG_HOME: path.join(temporary, "config"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CACHE_HOME: path.join(temporary, "cache"), AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/splendor/s"), AGENT_BROWSER_PROFILE: path.join(temporary, "profile"), AGENT_BROWSER_NO_VERSION_CHECK: "1" };
let preview;
if (process.env.TABLETOP_SOURCE === "1") {
  const require = createRequire(new URL("../package.json", import.meta.url));
  const { createServer } = require("vite");
  const manifest = JSON.parse(await readFile(path.join(root, "web/dist/game-load-manifest.json"), "utf8"));
  const loadingManifest = { name: "tabletop-source-manifest", configureServer(server) {
    server.middlewares.use("/api/version", (request, response) => { response.setHeader("Content-Type", "application/json"); response.end(JSON.stringify({ version: "dev", assets: [] })); });
    server.middlewares.use("/game-load-manifest.json", (request, response) => { response.setHeader("Content-Type", "application/json"); response.end(JSON.stringify({ ...manifest, entry: "/src/main.tsx", games: Object.fromEntries(Object.keys(manifest.games).map(kind => [kind, []])) })); });
  } };
  preview = await createServer({ configFile: false, root: path.join(root, "web"), cacheDir: path.join(temporary, "vite-cache"), publicDir: false, plugins: [require("@vitejs/plugin-react").default({ include: /\.[jt]sx$/ }), loadingManifest], resolve: { dedupe: ["react", "react-dom", "lucide-react", "three", "fflate"] }, server: { host: "127.0.0.1", port: 0, proxy: { "/api": { target: base, ws: true } }, fs: { allow: [root] } } });
  await preview.listen();
  base = preview.resolvedUrls.local[0];
}
const execute = promisify(execFile);
const session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "tt"], { cwd: root, env })).stdout.trim();
const browser = async (...args) => { const { stdout } = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env, timeout: 30000 }); const value = JSON.parse(stdout); assert.ok(value.success, stdout); return value.data; };
await browser("--profile", path.join(temporary, "profile"), "open", base);
const endpoint = await browser("get", "cdp-url");
const socket = new WebSocket(endpoint.cdpUrl);
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
let serial = 0;
const pending = new Map();
const mediaRequests = [];
socket.onmessage = async ({ data }) => {
  const value = JSON.parse(data);
  if (value.method === "Fetch.requestPaused") {
    const suffix = new URL(value.params.request.url).pathname.replace(/^\/media\//, "");
    const allowed = /^splendor\/(?:[a-z0-9-]+\/)*[a-z0-9_-]+\.webp$/.test(suffix);
    const bytes = allowed ? await readFile(path.join(root, ".local/splendor/fullscreen/media", suffix)).catch(() => undefined) : undefined;
    mediaRequests.push({ url: value.params.request.url, bytes: bytes?.length ?? 0 });
    await call("Fetch.fulfillRequest", { requestId: value.params.requestId, responseCode: bytes ? 200 : 404, responseHeaders: [{ name: "Content-Type", value: "image/webp" }, { name: "Access-Control-Allow-Origin", value: "*" }], body: bytes?.toString("base64") ?? "" }, value.sessionId).catch(() => {});
    return;
  }
  const request = pending.get(value.id); if (!request) return; pending.delete(value.id); clearTimeout(request.timer); value.error ? request.reject(Error(JSON.stringify(value.error))) : request.resolve(value.result);
};
const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const id = ++serial; const timer=setTimeout(()=>{pending.delete(id);reject(Error("CDP timeout "+method));},15000); pending.set(id, { resolve, reject, timer }); socket.send(JSON.stringify({ id, method, params, sessionId })); });
const { targetInfos } = await call("Target.getTargets");
const { sessionId } = await call("Target.attachToTarget", { targetId: targetInfos.find(value => value.type === "page" && value.url.startsWith(base)).targetId, flatten: true });
const evaluate = async (expression, selected = sessionId) => { const value = await call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }, selected); assert.ok(!value.exceptionDetails, JSON.stringify(value.exceptionDetails)); return value.result.value; };
const api = async (suffix, body, selected = sessionId) => evaluate(`(async()=>{const response=await fetch('/api'+${JSON.stringify(suffix)},{method:${body ? '"POST"' : '"GET"'},headers:{'Content-Type':'application/json'},body:${body ? JSON.stringify(JSON.stringify(body)) : 'undefined'}});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value})()`, selected);
const command = async (type, extra = {}, selected = sessionId, roomId) => { const state = await api("/state", undefined, selected); return api("/rooms/" + (roomId ?? state.room.id), { type, ...extra, version: (roomId ? state.rooms.find(room => room.id === roomId) : state.room)?.version, nonce: crypto.randomUUID() }, selected); };
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
const wait = async (name, predicate, timeout = 90000) => { const deadline = Date.now() + timeout; while (Date.now() < deadline) { const value = await predicate(); if (value) return value; await delay(100); } throw Error("Timed out: " + name); };
const report = { passed: false, scope: `${process.env.TABLETOP_SOURCE === "1" ? "Current source preview" : "Compiled application"} with real Go rooms, authenticated clients, public resources, card proportions and playable actions; ${process.env.TABLETOP_SPLENDOR_ONLY === "1" ? "Splendor layout and Snooker capacity only" : "also includes Xiangqi repetition warning"}`, checks: [] };
await mkdir(out, {recursive:true});
await call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 }, sessionId);
const viewport = async (width, height) => { await call("Emulation.setDeviceMetricsOverride", { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: true, screenOrientation: { type: width > height ? "landscapePrimary" : "portraitPrimary", angle: width > height ? 90 : 0 } }, sessionId); await delay(120); };

const check = (name, passed, detail) => { report.checks.push({name, passed:!!passed, detail}); };
const capture = async (name, selected = sessionId) => { const image = await call("Page.captureScreenshot", {format:"png",captureBeyondViewport:false}, selected); await writeFile(path.join(out,name+".png"),Buffer.from(image.data,"base64")); };
let other;
try {
  const {browserContextId} = await call("Target.createBrowserContext");
  const {targetId} = await call("Target.createTarget", {url:base,browserContextId});
  other = (await call("Target.attachToTarget", {targetId,flatten:true})).sessionId;
  for (const selected of [sessionId, other]) await call("Fetch.enable", { patterns: [{ urlPattern: "https://assets.example.test/media/*" }] }, selected);
  await wait("other page", () => evaluate("location.origin!=='null'", other));
  for (const [index, selected] of [sessionId,other].entries()) {
    await api("/register", {name:["青石","白露"][index]+Date.now().toString().slice(-6),password:"tabletop-test-only",invite:"gamepad-browser"}, selected);
    await evaluate("localStorage.setItem('wb_sound','off');true", selected);
    await call("Page.reload", {}, selected);
  }
  const start = async kind => {
    await api("/rooms", {kind,capacity:2,name:kind==="splendor"?"宝石全屏验收":"象棋警告验收",options:kind==="xiangqi"?{difficulty:"standard",turnSeconds:120}:{} });
    const id = (await api("/state")).room.id;
    await command("join",{},other,id);
    for (const selected of [sessionId,other]) await command("ready",{},selected);
    await wait("ready to start", async () => (await api("/state")).room.seats.every(seat=>seat.ready));
    await command("start");
    for (const selected of [other, sessionId]) {
      await call("Page.reload", {}, selected);
      await wait("reload complete", () => evaluate("document.readyState === 'complete'", selected));
    }
    await wait("two real clients finish loading", async () => { const error = await evaluate("document.querySelector('.game-loading [role=alert],.game-load-error')?.textContent"); assert.ok(!error, error); const state=await api("/state"); return state.room.status==="playing"&&!state.room.loading; });
    await wait("actual board", () => evaluate(`!!document.querySelector('.${kind}-board')`));
  };
  const click = async selector => {
    await evaluate("[...document.querySelectorAll('button')].find(button=>button.textContent.trim()==='稍后')?.click();true");
    const p=await evaluate(`(()=>{const e=document.querySelector(${JSON.stringify(selector)});e.scrollIntoView({block:'nearest'});const r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()`);
    await call("Input.dispatchMouseEvent", {type:"mousePressed",...p,button:"left",buttons:1,clickCount:1},sessionId);
    await call("Input.dispatchMouseEvent", {type:"mouseReleased",...p,button:"left",buttons:0,clickCount:1},sessionId);
  };
  await api("/rooms", { kind: "snooker", capacity: 4, name: "双人单打人数验收", options: { bestOf: 1, aimAssist: "line", teams: [0,1,0,1] } });
  await command("add_bot");
  await command("ready");
  await wait("snooker capacity selector", () => evaluate("!!document.querySelector('select[aria-label=\"房间人数\"]')"));
  await call("Page.bringToFront", {}, sessionId);
  await evaluate("(()=>{const select=document.querySelector('select[aria-label=\"房间人数\"]');select.value='2';select.dispatchEvent(new Event('change',{bubbles:true}));return true})()");
  await delay(1000);
  const resized = (await api("/state")).room;
  check("snooker two occupied seats can switch from doubles to singles", resized.capacity === 2 && JSON.stringify(resized.options.teams) === "[0,1]", {capacity:resized.capacity, options:resized.options});
  await command("leave");
  await start("splendor");
  check("normal turns hide the pass action", await evaluate("!document.querySelector('.splendor-board .pass-button')"));
  check("production artwork configuration active",await evaluate("!!document.querySelector('.with-artwork')"));
  check("normal mode has no duplicate mini players", await evaluate("getComputedStyle(document.querySelector('.splendor-fullscreen-players')).display==='none'"));
  const active=(await api("/state")).room.game.turn===0?sessionId:other;
  if(active===sessionId) await command("action",{action:{type:"take",tokens:[1,1,1,0,0,0]}});
  await command("action",{action:{type:"reserve",tier:1}},other);
  await wait("reserved count propagated", async () => (await api("/state")).room.game.splendor.players.some(p=>(p.reserved?.length??p.reservedCount??0)>0));
  const state=await api("/state");
  report.publicState=state.room.game.splendor.players;
  check("opponent hidden cards remain hidden in API", !state.room.game.splendor.players[1].reserved?.length);
  const inspectLayout = async (label, fullscreen, width, height) => {
    const detail = await evaluate(`(()=>{const box=e=>{const r=e.getBoundingClientRect();return Object.fromEntries(['left','top','right','bottom','width','height'].map(k=>[k,Math.round(r[k]*1000)]))};const visible=e=>e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden';const overlap=(a,b)=>a.left<b.right&&a.right>b.left&&a.top<b.bottom&&a.bottom>b.top;const cards=[...document.querySelectorAll('.market-row .dev-card,.market-row .deck')].filter(visible);const counts=[...document.querySelectorAll('.wallet-counts,.splendor-public-resources > div')].filter(visible).map(e=>({parent:box(e),children:[...e.children].filter(visible).map(box)}));const protectedElements=[...document.querySelectorAll('.market-row .dev-card,.market-row .deck,.gem-bank button,.splendor-fullscreen-player,.wallet-counts')].filter(visible);const overlays=[...document.querySelectorAll('.game-fullscreen-toolbar,.game-fullscreen-status,.chat-toggle')].filter(visible);return {cards:cards.map(box),costs:cards.flatMap(e=>[...e.querySelectorAll('.cost')].map(c=>({card:box(e),cost:box(c)}))),counts,bank:[...document.querySelectorAll('.gem-bank .token-button')].map(box),overlaps:overlays.flatMap(e=>protectedElements.filter(p=>overlap(box(e),box(p))).map(p=>({overlay:e.className,element:p.className}))),overflow:document.documentElement.scrollWidth>innerWidth}})()`);
    check(label+" cards preserve artwork proportions",detail.cards.length===15&&detail.cards.every(r=>Math.abs(Math.round(r.width/r.height*1000)-714)<=1),detail.cards);
    check(label+" costs fit within cards",detail.costs.every(({card,cost})=>cost.left>=card.left&&cost.right<=card.right&&cost.top>=card.top&&cost.bottom<=card.bottom),detail.costs);
    check(label+" resource counts fit their columns",detail.counts.every(({parent,children})=>children.every(c=>c.left>=parent.left&&c.right<=parent.right)),detail.counts);
    check(label+" gem targets support touch",detail.bank.every(r=>r.width>=44000&&r.height>=44000),detail.bank);
    check(label+" no horizontal page overflow",!detail.overflow);
    if (fullscreen) check(label+" overlays leave cards and resources clear",detail.overlaps.length===0,detail.overlaps);
    if (fullscreen) check(label+" exit stays in top right",await evaluate("(()=>{const r=document.querySelector('[aria-label=\"退出全屏\"]').getBoundingClientRect();return Math.round(r.top)<=12&&Math.round(innerWidth-r.right)<=12})()"));
    await capture(label);
  };
  for (const [label,width,height] of [["phone",844,390],["short-phone",852,300],["pad",1180,744],["pad-portrait",820,1080],["portrait",390,664],["narrow",320,568],["desktop",1440,900]]) {
    await viewport(width,height);
    if(await evaluate("!!document.querySelector('[data-room-fullscreen]')")) await click('[aria-label="退出全屏"]');
    await wait("normal mode",()=>evaluate("!document.querySelector('[data-room-fullscreen]')"));
    await evaluate("document.querySelector('.splendor-board').scrollIntoView({block:'start'});true");
    await delay(200);
    await inspectLayout(label+"-normal",false,width,height);
    if(!await evaluate("!!document.querySelector('[data-room-fullscreen]')")) await click('[aria-label="全屏"]');
    await wait("fullscreen",()=>evaluate("!!document.querySelector('[data-room-fullscreen]')"));
    await delay(300);
    await inspectLayout(label+"-fullscreen",true,width,height);
    const geometry=await evaluate("(()=>{const box=e=>{const r=e.getBoundingClientRect();return Object.fromEntries(['top','left','right','bottom','width','height'].map(k=>[k,Math.round(r[k]*1000)]))};return {cards:[...document.querySelectorAll('.market-row .dev-card,.market-row .deck')].filter(e=>e.getBoundingClientRect().width).map(box),players:[...document.querySelectorAll('.splendor-fullscreen-player')].map(box),market:box(document.querySelector('.market')),sidebar:box(document.querySelector('.splendor-sidebar')),overflow:document.documentElement.scrollWidth>innerWidth}})()");
    check(label+" development cards retain 5:7",geometry.cards.length===15&&geometry.cards.every(r=>Math.abs(Math.round(r.width*7/r.height)-5)===0&&Math.abs(Math.round(r.width/r.height*1000)-714)<=1),geometry);
    check(label+" all player information visible",geometry.players.length===2&&geometry.players.every(r=>r.height>0&&r.top>=0&&r.bottom<=height*1000&&r.left>=0&&r.right<=width*1000),geometry.players);
    check(label+" no horizontal overflow",!geometry.overflow);
    if(width<height) {
      check(label+" compact top spacing",geometry.cards[0].top-Math.max(...geometry.players.map(p=>p.bottom))<=62000,geometry);
      check(label+" controls follow cards",geometry.sidebar.top-Math.max(...geometry.cards.map(p=>p.bottom))<=16000,geometry);
    }
    check(label+" market stays in viewport",geometry.cards.every(r=>r.top>=0&&r.bottom<=height*1000&&r.right<=width*1000),geometry.cards);
    check(label+" no hidden faces in public players",await evaluate("!document.querySelector('.splendor-fullscreen-players [data-card-id]')"));
    for(const [seat,p] of state.room.game.splendor.players.entries()) {
      const actual=await evaluate(`(()=>{const e=document.querySelector('[data-splendor-public-player="${seat}"]');return {tokens:[...e.querySelectorAll('[data-public-token]')].map(e=>Number(e.textContent)),bonus:[...e.querySelectorAll('[data-public-bonus]')].map(e=>Number(e.textContent)),reserved:Number(e.querySelector('[data-public-reserved]').dataset.publicReserved)}})()`);
      check(label+" player "+seat+" resources match authoritative state",JSON.stringify(actual.tokens)===JSON.stringify(p.tokens)&&JSON.stringify(actual.bonus)===JSON.stringify(p.bonus)&&actual.reserved===(p.reserved?.length??p.reservedCount??0),actual);
    }
    await delay(700);
    await capture(label);
  }
  await click('.market-row .dev-card');
  check("card purchase dialog opens in fullscreen",await wait("purchase",()=>evaluate("!!document.querySelector('[role=dialog]')")));
  await capture("purchase");
  await command("close");
  for(const selected of [other,sessionId]) await command("leave",{},selected);
  if (process.env.TABLETOP_SPLENDOR_ONLY !== "1") {
  await start("xiangqi");
  const sq=name=>(9-Number(name[1]))*9+name.charCodeAt(0)-97;
  const moves=[["b2","b4"],["g6","g5"],["a0","a2"],["i6","i5"],["a2","d2"],["g5","g4"],["d2","d9"],["e9","e8"],["d9","d8"],["e8","e9"],["d8","d9"],["e9","e8"],["d9","d8"],["e8","e9"],["d8","d9"],["e9","e8"],["d9","d8"],["e8","e9"],["d8","d9"]];
  for (const [from,to] of moves) {
    const state=await api("/state");
    if(state.room.game.data.warning)break;
    const selected=state.room.game.data.turn===state.room.you?sessionId:other;
    const view=await wait("move allowed",async()=>{const r=(await api("/state",undefined,selected)).room;return r.game.data.canMove?r:false});
    const data=view.game.data;
    assert.ok(data.legal.some(move=>move.from===sq(from)&&move.to===sq(to)),"illegal setup "+from+to);
    await command("action",{action:{type:"move",round:data.round,ply:data.ply,from:sq(from),to:sq(to)}},selected);
  }
  const warning=(await api("/state")).room.game.data.warning;
  assert.ok(warning,"legal sequence must produce a repetition warning");
  report.warning=warning;
  await wait("warning rendered",()=>evaluate("!!document.querySelector('.xiangqi-warning')"));
  for(const [label,width,height] of [["xiangqi-phone",844,390],["xiangqi-portrait",390,844],["xiangqi-pad",1180,820]]) {
    await viewport(width,height);
    if(!await evaluate("!!document.querySelector('[data-room-fullscreen]')"))await click('[aria-label="全屏"]');
    await delay(200);
    const r=await evaluate("(()=>{const w=document.querySelector('.xiangqi-warning').getBoundingClientRect();const b=document.querySelector('.xiangqi-map').getBoundingClientRect();return {warning:Math.round(w.height*1000),bottom:Math.round(b.bottom*1000),height:Math.round(b.height*1000)}})()");
    check(label+" warning compact and board visible",r.warning>0&&r.warning<=64000&&r.bottom<=height*1000&&r.height>=100000,r);
    await delay(700);
    await capture(label);
  }
  await command("close");
  for(const selected of [other,sessionId]) await command("leave",{},selected);
  }
  const extra=[];
  for(const name of ["墨玉","赤金"]) {
    const {browserContextId}=await call("Target.createBrowserContext");
    const {targetId}=await call("Target.createTarget",{url:base,browserContextId});
    const selected=(await call("Target.attachToTarget",{targetId,flatten:true})).sessionId;
    await call("Fetch.enable", { patterns: [{ urlPattern: "https://assets.example.test/media/*" }] }, selected);
    await wait("extra client page",()=>evaluate("location.origin!=='null'",selected));
    await api("/register",{name:name+Date.now().toString().slice(-6),password:"tabletop-test-only",invite:"gamepad-browser"},selected);
    await call("Page.reload", {}, selected);
    extra.push(selected);
  }
  await api("/rooms",{kind:"splendor",capacity:4,name:"四人扩展牌市验收",splendorOptions:{orient:true,cities:true,tradingPosts:true,strongholds:true}});
  const roomId=(await api("/state")).room.id;
  for(const selected of [other,...extra]) await command("join",{},selected,roomId);
  for(const selected of [sessionId,other,...extra]) await command("ready",{},selected);
  await command("start");
  for (const selected of [other, ...extra, sessionId]) {
    await call("Page.reload", {}, selected);
    await wait("extra page reload", () => evaluate("document.readyState === 'complete'", selected));
  }
  await call("Page.bringToFront",{},sessionId);
  await wait("four real clients ready",async()=>!(await api("/state")).room.loading);
  for(const [label,width,height] of [["four-phone",844,390],["four-portrait",390,844],["four-pad",1180,820]]) {
    await viewport(width,height);
    if(!await evaluate("!!document.querySelector('[data-room-fullscreen]')")) {
      await wait("four player fullscreen ready",()=>evaluate("!!document.querySelector('[aria-label=\"全屏\"]:not(:disabled)')"));
      await click('[aria-label="全屏"]');
      await wait("four player fullscreen active",()=>evaluate("!!document.querySelector('[data-room-fullscreen]')"));
    }
    for(const page of ["base","orient"]) {
      await click('.gem-market-switch button:'+ (page==="base"?"first":"last")+'-child');
      await delay(300);
      const boxes=await evaluate("(()=>{const b=e=>{const r=e.getBoundingClientRect();return {top:Math.round(r.top*1000),bottom:Math.round(r.bottom*1000),left:Math.round(r.left*1000),right:Math.round(r.right*1000),width:Math.round(r.width*1000),height:Math.round(r.height*1000)}};return {players:[...document.querySelectorAll('.splendor-fullscreen-player')].map(b),counts:[...document.querySelectorAll('.splendor-public-resources > div')].map(e=>({parent:b(e),children:[...e.children].map(b)})),cards:[...document.querySelectorAll('.market-row .dev-card,.market-row .deck')].filter(e=>e.getBoundingClientRect().width).map(b)}})()");
      check(label+page+" all four players visible",boxes.players.length===4&&boxes.players.every(r=>r.top>=0&&r.bottom<=height*1000&&r.left>=0&&r.right<=width*1000),boxes.players);
      check(label+page+" public counts do not overlap",boxes.counts.every(({parent,children})=>children.every(c=>c.left>=parent.left&&c.right<=parent.right)),boxes.counts);
      check(label+page+" cards retain ratio",boxes.cards.length===(page==="base"?15:9)&&boxes.cards.every(r=>Math.abs(Math.round(r.width/r.height*1000)-714)<=1&&r.bottom<=height*1000),boxes.cards);
      await capture(label+"-"+page);
    }
  }
  report.passed=report.checks.every(c=>c.passed);
  await writeFile(path.join(out,"result.json"),JSON.stringify(report,null,2)+"\n");
  assert.ok(report.passed,JSON.stringify(report.checks.filter(c=>!c.passed)));
} catch(error) { report.error=error.stack; report.lastRoom = await api("/state").then(value => value.room).catch(() => undefined); await capture("failure").catch(()=>{});console.error(error);process.exitCode=1; }
finally {
  report.mediaRequests = mediaRequests;
  report.sources=Object.fromEntries(await Promise.all(["games/splendor/web/board.tsx","games/splendor/web/splendor-layout.css","games/splendor/web/splendor-fullscreen.css","games/splendor/web/splendor-fullscreen-players.tsx","games/splendor/web/splendor-cities.tsx","games/splendor/web/types.ts","games/splendor/server/state.go","web/src/game-api/load-assets.ts","web/src/game-api/loading.tsx","web/src/ui/room-fullscreen.tsx","web/src/main.tsx","web/tests/tabletop-room-browser.mjs"].map(async file=>[file,createHash("sha256").update(await readFile(path.join(root,file))).digest("hex")])));
  await writeFile(path.join(out,"result.json"),JSON.stringify(report,null,2)+"\n");
  for(const request of pending.values())clearTimeout(request.timer);
  socket.close();await browser("close").catch(()=>{});await preview?.close();await rm(temporary,{recursive:true,force:true});
}
