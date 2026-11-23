import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { createHash, randomUUID } from "node:crypto";

test("shared game protection covers native holds and menus while preserving controls and editors", { timeout: 180000 }, async () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const require = createRequire(new URL("../package.json", import.meta.url));
  const execute = promisify(execFile);
  const common = (await execute("git", ["rev-parse", "--path-format=absolute", "--git-common-dir"], { cwd: root })).stdout.trim();
  const workspace = path.dirname(common);
  const owned = path.join(workspace, ".local/hotlinemiami");
  const reportName = process.env.GAME_CONTROL_REPORT ?? "current";
  assert.match(reportName, /^[a-z0-9-]+$/);
  const out = path.join(owned, "mobile-selection/verification", reportName);
  const temporary = path.join(owned, "mobile-selection/tmp", randomUUID());
  const environment = { ...process.env, TMPDIR: path.join(owned, "tmp"), AGENT_BROWSER_SOCKET_DIR: path.join(owned, "s"), AGENT_BROWSER_PROFILE: path.join(temporary, "profile"), XDG_CACHE_HOME: path.join(temporary, "cache"), AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: ["--mute-audio", "--disable-breakpad", "--disable-crash-reporter", ...(process.env.GITHUB_ACTIONS === "true" ? ["--no-sandbox"] : [])].join(",") };
  for (const directory of [out, temporary, environment.TMPDIR, environment.AGENT_BROWSER_SOCKET_DIR, environment.AGENT_BROWSER_PROFILE, environment.XDG_CACHE_HOME]) await mkdir(directory, { recursive: true });
  const report = { passed: false, engine: "Chromium native mouse and simulated touch", checks: [], observations: {}, sources: {} };
  let server, session, socket, attached;
  const pending = new Map();
  let ordinal = 0;
  const browser = async (...args) => {
    const result = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 4 << 20 });
    const value = JSON.parse(result.stdout);
    assert.equal(value.success, true, JSON.stringify(value));
    return value.data;
  };
  const call = (method, params = {}, sessionId = attached) => new Promise((resolve, reject) => {
    const id = ++ordinal;
    const timer = setTimeout(() => { pending.delete(id); reject(Error("CDP timeout: " + method)); }, 10000);
    pending.set(id, { resolve, reject, timer });
    socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
  });
  const evaluate = async expression => {
    const response = await call("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
    assert.equal(response.exceptionDetails, undefined, JSON.stringify(response.exceptionDetails));
    return response.result.value;
  };
  const pause = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
  const rect = selector => evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});if(!element)throw Error('missing target: '+${JSON.stringify(selector)});element.scrollIntoView({block:'nearest',inline:'nearest'});const r=element.getBoundingClientRect(),s=getComputedStyle(element);if(Math.round(r.width)<=0||Math.round(r.height)<=0||s.visibility==='hidden'||s.display==='none')throw Error('hidden target: '+${JSON.stringify(selector)});const x=r.left+r.width/2,y=r.top+r.height/2,hit=document.elementFromPoint(x,y);if(!hit||hit!==element&&!element.contains(hit))throw Error('covered target: '+${JSON.stringify(selector)}+' by '+hit?.outerHTML.slice(0,180));return {x,y,width:r.width,height:r.height}})()`);
  const mouse = async (selector, button = "left", clickCount = 1) => {
    const point = await rect(selector);
    await call("Input.dispatchMouseEvent", { type: "mouseMoved", x: point.x, y: point.y });
    await call("Input.dispatchMouseEvent", { type: "mousePressed", x: point.x, y: point.y, button, buttons: button === "right" ? 2 : 1, clickCount });
    await call("Input.dispatchMouseEvent", { type: "mouseReleased", x: point.x, y: point.y, button, buttons: 0, clickCount });
    await pause(50);
  };
  const touch = (type, points) => call("Input.dispatchTouchEvent", { type, touchPoints: points });
  const check = (name, passed, detail) => report.checks.push({ name, passed: !!passed, detail });
  const eventBoundary = () => evaluate("({native:window.fixture.native.length,events:window.fixture.events.length})");
  const contactLifecycle = (records, kind, terminals, moved = true) => {
    const unique = [...new Map(records.filter(event => event.kind === kind).map(event => [event.serial, event])).values()];
    const active = new Map(), complete = [], errors = [];
    const start = kind === "touch" ? "touchstart" : "pointerdown";
    const movement = kind === "touch" ? "touchmove" : "pointermove";
    const ends = kind === "touch" ? ["touchend", "touchcancel"] : ["pointerup", "pointercancel"];
    for (const event of unique) {
      if (!event.trusted) errors.push("untrusted event");
      const contacts = kind === "touch" ? event.changed.filter(contact => contact.target) : [{ id: event.id, target: event.target }];
      for (const contact of contacts) {
        if (event.type === start) {
          if (active.has(contact.id)) errors.push("duplicate contact start: " + contact.id);
          active.set(contact.id, { id: contact.id, target: contact.target, types: [event.type] });
        } else {
          const current = active.get(contact.id);
          if (!current) { if (event.type !== movement) errors.push("contact without start: " + contact.id); continue; }
          if (current.target !== contact.target) errors.push("contact target changed: " + contact.id);
          current.types.push(event.type);
          if (ends.includes(event.type)) { active.delete(contact.id); complete.push(current); }
        }
      }
    }
    if (active.size) errors.push("contacts remain active");
    for (const [target, terminal] of Object.entries(terminals)) {
      const matches = complete.filter(contact => contact.target === target);
      if (matches.length !== 1 || matches[0].types.at(-1) !== terminal || moved && !matches[0].types.includes(movement)) errors.push("incomplete " + target + " contact");
    }
    return { passed: errors.length === 0, complete, active: [...active.keys()], errors };
  };
  const verifyNative = async (name, boundary, kind, terminals, moved = true) => {
    const observed = await evaluate(`({native:window.fixture.native.slice(${boundary.native}),events:window.fixture.events.slice(${boundary.events})})`);
    const lifecycle = contactLifecycle(observed.native, kind, terminals, moved);
    const recorded = new Map(observed.events.map(event => [event.serial, event]));
    const sameEvents = observed.native.every(event => event.trusted && JSON.stringify(event) === JSON.stringify(recorded.get(event.serial)));
    check(name, lifecycle.passed && sameEvents, { lifecycle, sameEvents, ...observed });
    const pointerTerminals = Object.fromEntries(Object.entries(terminals).map(([target, terminal]) => [target, terminal === "touchcancel" ? "pointercancel" : "pointerup"]));
    const pointerLifecycle = contactLifecycle(observed.events, "pointer", pointerTerminals, moved);
    check(name + ": raw pointer lifecycle", pointerLifecycle.passed, pointerLifecycle);
  };
  try {
    await require("esbuild").build({ entryPoints: [path.join(root, "web/tests/game-control-fixture.tsx")], outfile: path.join(temporary, "fixture.js"), bundle: true, jsx: "automatic", nodePaths: [path.join(root, "web/node_modules")], define: { "process.env.NODE_ENV": '"production"' }, legalComments: "none" });
    const postcss = require("postcss");
    const shared = postcss.parse(await readFile(path.join(root, "web/src/style.css"), "utf8")).nodes.filter(node => node.type === "rule" && node.selector.includes("[data-game-controls]")).map(node => node.toString()).join("\n");
    const html = '<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="fixture.css"><style>body{margin:0;padding:8px;font-family:system-ui;color:#293b35;background:#f6f5ef}.fixture-pictures{position:absolute;left:12px;right:12px;top:142px;display:flex;gap:12px;align-items:center}.fixture-pictures>*{max-width:70px;font-size:11px}input,textarea,select,[contenteditable]{display:block;max-width:260px;margin:5px}aside{padding:8px}</style><style>' + shared + '</style></head><body><div id="root"></div><script src="fixture.js"></script></body></html>';
    const files = new Map([["index.html", Buffer.from(html)], ["fixture.js", await readFile(path.join(temporary, "fixture.js"))], ["fixture.css", await readFile(path.join(temporary, "fixture.css"))], ["demo.svg", Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="60" height="40"><rect width="60" height="40" fill="#174e43"/><circle cx="30" cy="20" r="12" fill="#e6b953"/></svg>')]]);
    server = createServer((request, response) => {
      const name = new URL(request.url, "http://127.0.0.1").pathname.slice(1) || "index.html";
      if (!files.has(name)) { response.writeHead(404); response.end(); return; }
      response.writeHead(200, { "Content-Type": name.endsWith(".js") ? "text/javascript" : name.endsWith(".css") ? "text/css" : name.endsWith(".svg") ? "image/svg+xml" : "text/html" });
      response.end(files.get(name));
    });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "gc"], { cwd: root, env: environment })).stdout.trim();
    await browser("open", origin);
    await browser("wait", "[data-probe=hint]");
    await browser("set", "viewport", "320", "850");
    const endpoint = await browser("get", "cdp-url");
    socket = new WebSocket(typeof endpoint === "string" ? endpoint : endpoint.url ?? endpoint.cdpUrl ?? endpoint.value);
    socket.onmessage = ({ data }) => {
      const message = JSON.parse(data);
      const entry = pending.get(message.id);
      if (!entry) return;
      clearTimeout(entry.timer); pending.delete(message.id);
      if (message.error) entry.reject(Error(JSON.stringify(message.error))); else entry.resolve(message.result);
    };
    await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
    const targets = await call("Target.getTargets", {}, undefined);
    const target = targets.targetInfos.find(target => target.type === "page" && target.url.startsWith(origin));
    assert.ok(target);
    attached = (await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }, undefined)).sessionId;
    report.browser = await call("Browser.getVersion", {}, undefined);
    await call("Emulation.setDeviceMetricsOverride", { width: 320, height: 850, deviceScaleFactor: 1, mobile: true });
    await call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
    await evaluate("document.querySelector('[data-probe=input]').setAttribute('contenteditable','false');true");
    for (const name of ["heading", "hint", "image", "link", "canvas", "custom", "input", "textarea", "editable", "select", "outside", "outside-input", "overlay"]) await mouse(`[data-probe=${name}]`, "right");
    await mouse('[data-probe=ordinary]', 'right');
    check('ordinary button native menu blocked', await evaluate('window.fixture.menus.at(-1).prevented'));
    const ordinary = await rect('[data-probe=ordinary]');
    await touch('touchStart', [{ id: 8, x: ordinary.x, y: ordinary.y }]);
    await pause(1200);
    check('body-retargeted menu blocked during ordinary button hold', await evaluate("!document.body.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}))"));
    await touch('touchCancel', []);
    check('body menu restored after cancellation', await evaluate("document.body.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}))"));
    await touch('touchStart', [{ id: 9, x: ordinary.x, y: ordinary.y }]);
    await touch('touchEnd', []);
    await pause(100);
    check('ordinary touch tap retains compatibility click', await evaluate('window.fixture.clicks') === 1);
    report.observations.contextMenus = await evaluate("window.fixture.menus");
    const menus = report.observations.contextMenus;
    for (const name of ["heading", "hint", "image", "link", "canvas", "custom"]) check("protected menu: " + name, menus.some(menu => menu.target === name && menu.prevented && menu.trusted), menus.filter(menu => menu.target === name));
    for (const name of ["input", "textarea", "editable", "select", "outside", "outside-input", "overlay"]) check("native editor/outside menu: " + name, menus.some(menu => menu.target === name && !menu.prevented && menu.trusted));
    check("game custom menu retained", await evaluate("window.fixture.custom") === 1);
    await evaluate("document.activeElement.blur();getSelection().removeAllRanges();true");
    await mouse("[data-probe=hint]", "left", 3);
    report.observations.gameSelection = await evaluate("getSelection().toString()");
    check("game text cannot be selected", report.observations.gameSelection === "");
    await evaluate("(()=>{const r=document.createRange();r.selectNodeContents(document.querySelector('[data-probe=hint]'));getSelection().removeAllRanges();getSelection().addRange(r);return true})()");
    await pause(50);
    check("stale game selection clears", await evaluate("getSelection().toString()") === "");
    await mouse("[data-probe=outside]", "left", 3);
    check("outside text can be selected", (await evaluate("getSelection().toString()")).length > 0);
    await evaluate("getSelection().removeAllRanges();true");
    await call("Page.bringToFront");
    await call("Emulation.setFocusEmulationEnabled", { enabled: true });
    await browser("focus", "[data-probe=input]");
    report.observations.editorBefore = await evaluate("(()=>{const e=document.querySelector('[data-probe=input]');return {active:document.activeElement===e,focused:document.hasFocus(),start:e.selectionStart,end:e.selectionEnd,value:e.value}})()");
    await browser("press", process.platform === "darwin" ? "Meta+a" : "Control+a");
    await browser("wait", "--fn", "(()=>{const e=document.querySelector('[data-probe=input]');return e.selectionEnd>e.selectionStart})()");
    report.observations.editorAfter = await evaluate("(()=>{const e=document.querySelector('[data-probe=input]');return {active:document.activeElement===e,focused:document.hasFocus(),start:e.selectionStart,end:e.selectionEnd,value:e.value}})()");
    check("editor selection retained", await evaluate("(()=>{const e=document.querySelector('[data-probe=input]');return e.selectionEnd-e.selectionStart})()") > 0);
    await evaluate("document.activeElement.blur();getSelection().removeAllRanges();true");
    const hint = await rect("[data-probe=hint]");
    await touch("touchStart", [{ id: 3, x: hint.x, y: hint.y }]);
    await pause(1200);
    await touch("touchEnd", []);
    check("long-held hint creates no selection", await evaluate("getSelection().toString()") === "");
    await evaluate("getSelection().removeAllRanges();true");
    await browser("scrollintoview", "[data-hotlinemiami-stick=move]");
    const left = await rect("[data-hotlinemiami-stick=move]"), right = await rect("[data-hotlinemiami-stick=aim]");
    const heldBoundary = await eventBoundary();
    const points = [{ id: 1, x: left.x + 16, y: left.y - 12 }, { id: 2, x: right.x + 15, y: right.y - 10 }];
    await touch("touchStart", points);
    await pause(1200);
    await touch("touchMove", points.map(point => ({ ...point, x: point.x + 8 })));
    check("two long-held sticks remain active", await evaluate("['move','aim'].every(k=>{const v=window.fixture[k].at(-1);return v&&(v.x!==0||v.y!==0)})"));
    check("long hold creates no selection", await evaluate("getSelection().toString()") === "");
    await touch("touchEnd", [points[1]]);
    check("aim release preserves the held movement contact", await evaluate("(()=>{const move=window.fixture.move.at(-1),aim=window.fixture.aim.at(-1);return move&&(move.x!==0||move.y!==0)&&aim&&aim.x===0&&aim.y===0})()"));
    await touch("touchMove", [{ ...points[0], x: left.x + left.width }]);
    check("movement remains active after sliding outside the stick", await evaluate("window.fixture.move.at(-1)?.x>0"));
    await touch("touchEnd", [{ ...points[0], x: left.x + left.width }]);
    check("both sticks release", await evaluate("['move','aim'].every(k=>{const v=window.fixture[k].at(-1);return v&&v.x===0&&v.y===0})"));
    await verifyNative("native touch contacts retain stable IDs through independent release", heldBoundary, "touch", { move: "touchend", aim: "touchend" });
    await mouse("[data-probe=hint]", "left", 3);
    await browser("screenshot", path.join(out, "normal-320.png"));
    await browser("click", "[data-enter]");
    await browser("wait", "[data-room-fullscreen]");
    await mouse("[data-probe=hint]", "right");
    check("fullscreen menu protected", await evaluate("(()=>{const menu=window.fixture.menus.at(-1);return menu.target==='hint'&&menu.prevented&&menu.trusted})()"));
    await mouse("[data-probe=hint]", "left", 3);
    check("fullscreen selection protected", await evaluate("getSelection().toString()") === "");
    const fullHint = await rect("[data-probe=hint]");
    await touch("touchStart", [{ id: 3, x: fullHint.x, y: fullHint.y }]); await pause(1200); await touch("touchEnd", []);
    check("fullscreen long-held game text creates no selection", await evaluate("getSelection().toString()") === "");
    await mouse("[data-probe=toolbar]", "right");
    check("fullscreen toolbar protected", await evaluate("window.fixture.menus.at(-1).prevented"));
    await browser("screenshot", path.join(out, "fullscreen-320.png"));
    await browser("click", "[aria-label='退出全屏']");
    await browser("wait", "--fn", "!document.querySelector('[data-room-fullscreen]')");
    await evaluate("Element.prototype.requestFullscreen=()=>Promise.reject(new Error('fixture native unavailable'));true");
    await browser("click", "[data-enter]");
    await browser("wait", "[data-room-fullscreen=page]");
    await mouse("[data-probe=hint]", "right");
    check("fallback fullscreen menu protected", await evaluate("(()=>{const menu=window.fixture.menus.at(-1);return menu.target==='hint'&&menu.prevented&&menu.trusted})()"));
    await mouse("[data-probe=hint]", "left", 3);
    check("fallback fullscreen selection protected", await evaluate("getSelection().toString()") === "");
    const pageHint = await rect("[data-probe=hint]");
    await touch("touchStart", [{ id: 3, x: pageHint.x, y: pageHint.y }]); await pause(1200); await touch("touchEnd", []);
    check("fallback fullscreen long-held game text creates no selection", await evaluate("getSelection().toString()") === "");
    const cancelLeft = await rect("[data-hotlinemiami-stick=move]"), cancelRight = await rect("[data-hotlinemiami-stick=aim]");
    const cancelBoundary = await eventBoundary();
    const cancelPoints = [{ id: 1, x: cancelLeft.x + 12, y: cancelLeft.y }, { id: 2, x: cancelRight.x + 12, y: cancelRight.y }];
    await touch("touchStart", cancelPoints);
    await touch("touchMove", cancelPoints.map(point => ({ ...point, x: point.x + 8 })));
    check("fullscreen contacts are active before cancellation", await evaluate("['move','aim'].every(k=>{const v=window.fixture[k].at(-1);return v&&(v.x!==0||v.y!==0)})"));
    await touch("touchCancel", []);
    check("fullscreen touch cancellation releases both sticks", await evaluate("['move','aim'].every(k=>{const v=window.fixture[k].at(-1);return v&&v.x===0&&v.y===0})"));
    await verifyNative("native touch cancellation terminates both original contact IDs", cancelBoundary, "touch", { move: "touchcancel", aim: "touchcancel" });
    await browser("screenshot", path.join(out, "fallback-320.png"));
    await browser("click", "[aria-label='退出全屏']");
    await browser("wait", "--fn", "!document.querySelector('[data-room-fullscreen]')");
    await mouse("[data-probe=outside]", "left", 3);
    check("outside selection remains after fullscreen exit", (await evaluate("getSelection().toString()")).length > 0);
    report.observations.styles = await evaluate("[...document.querySelectorAll('[data-probe=hint],.hm-stick-name,[data-probe=input]')].map(e=>({text:e.textContent,select:getComputedStyle(e).userSelect,webkitSelect:getComputedStyle(e).webkitUserSelect,callout:getComputedStyle(e).getPropertyValue('-webkit-touch-callout')}))");
    report.observations.native = await evaluate("window.fixture.native");
    report.observations.events = await evaluate("window.fixture.events");
    report.passed = report.checks.every(check => check.passed);
    assert.equal(report.passed, true, JSON.stringify(report.checks.filter(check => !check.passed)));
  } catch (error) {
    report.error = error.stack ?? String(error);
    throw error;
  } finally {
    if (socket) socket.close();
    for (const entry of pending.values()) clearTimeout(entry.timer);
    if (session) { try { await browser("close"); } catch {} }
    if (server) await new Promise(resolve => server.close(resolve));
    for (const file of ["web/src/control-context-menu.ts", "web/src/ui/room-fullscreen.tsx", "web/src/ui/game-control-guard.ts", "web/src/ui/game-control-guard.css", "web/src/style.css", "web/tests/game-control-fixture.tsx", "web/tests/game-control-browser.test.mjs", "games/hotlinemiami/web/TouchControls.tsx", "games/hotlinemiami/web/style.css"]) {
      try { report.sources[file] = createHash("sha256").update(await readFile(path.join(root, file))).digest("hex"); } catch { report.sources[file] = null; }
    }
    await writeFile(path.join(out, "result.json"), JSON.stringify(report, null, 2) + "\n");
    await rm(temporary, { recursive: true, force: true });
  }
});
