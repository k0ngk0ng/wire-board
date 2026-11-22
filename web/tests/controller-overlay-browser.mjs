import assert from "node:assert/strict";
import { promisify } from "node:util";
import { execFile } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const out = path.join(root, ".local/contra/controller-overlay");
const temporary = path.join(out, "temporary");
const sockets = path.join(root, ".local/contra/s");
for (const folder of [out, temporary, sockets, ...["config", "data", "cache", "profile"].map(name => path.join(temporary, name))]) await mkdir(folder, { recursive: true });
const env = { ...process.env, TMPDIR: temporary, XDG_CONFIG_HOME: path.join(temporary, "config"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CACHE_HOME: path.join(temporary, "cache"), AGENT_BROWSER_PROFILE: path.join(temporary, "profile"), AGENT_BROWSER_SOCKET_DIR: sockets, AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const execute = promisify(execFile);
const session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "ov"], { cwd: root, env, timeout: 10000 })).stdout.trim();
const browser = async (...args) => { const { stdout } = await execute("agent-browser", ["--session", session, "--profile", path.join(temporary, "profile"), ...args, "--json"], { cwd: root, env, timeout: 30000 }); const value = JSON.parse(stdout); assert.ok(value.success, stdout); return value.data; };
const require = createRequire(new URL("../package.json", import.meta.url));
const { createServer } = require("vite");
const react = require("@vitejs/plugin-react").default;
await writeFile(path.join(temporary, "index.html"), `<meta name="viewport" content="width=device-width,initial-scale=1"><style>body{margin:0;padding:10px;box-sizing:border-box}*{box-sizing:border-box}</style><div id="root"></div><script type="module" src="/@fs/${path.join(root, "web/tests/ControllerOverlayFixture.tsx")}"></script>`);
const server = await createServer({ configFile: false, root: temporary, cacheDir: path.join(temporary, "vite-cache"), publicDir: false, optimizeDeps: { include: ["react", "react-dom/client", "react/jsx-dev-runtime"] }, plugins: [react()], resolve: { alias: { react: path.join(root, "web/node_modules/react"), "react-dom": path.join(root, "web/node_modules/react-dom"), "lucide-react": path.join(root, "web/node_modules/lucide-react") }, dedupe: ["react", "react-dom"] }, server: { host: "127.0.0.1", port: 0, fs: { allow: [root] } } });
await server.listen();
let socket;
const reports = [];
try {
  await browser("open", server.resolvedUrls.local[0] + "?kind=contra&protected=1");
  await browser("wait", "[data-controller-dpad]");
  const endpoint = await browser("get", "cdp-url");
  socket = new WebSocket(typeof endpoint === "string" ? endpoint : endpoint.cdpUrl);
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  let serial = 0;
  const pending = new Map();
  socket.onmessage = ({ data }) => { const body = JSON.parse(data); const request = pending.get(body.id); if (!request) return; pending.delete(body.id); body.error ? request.reject(Error(JSON.stringify(body.error))) : request.resolve(body.result); };
  const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const id = ++serial; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, sessionId })); });
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(value => value.type === "page" && value.url.startsWith(server.resolvedUrls.local[0]));
  let { sessionId } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true });
  const evaluate = async expression => { const result = await call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }, sessionId); assert.ok(!result.exceptionDetails, JSON.stringify(result.exceptionDetails)); return result.result.value; };
  await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
  const desktopHud = await evaluate("(()=>{const hud=document.querySelector('[data-chat-protected]').getBoundingClientRect();const chat=document.querySelector('.chat-toggle').getBoundingClientRect();return {coarse:matchMedia('(pointer:coarse)').matches,hud:hud.toJSON(),chat:chat.toJSON(),overlap:hud.left<chat.right&&hud.right>chat.left&&hud.top<chat.bottom&&hud.bottom>chat.top}})()");
  assert.equal(await evaluate('document.querySelector(".chat-toggle").textContent.trim()'), '', 'chat bubble contains a visible text label');
  const desktopImage = await call("Page.captureScreenshot", { format: "png", captureBeyondViewport: false }, sessionId);
  await writeFile(path.join(out, "normal-desktop-hud.png"), Buffer.from(desktopImage.data, "base64"));
  assert.equal(desktopHud.coarse, false);
  assert.equal(desktopHud.overlap, false, "normal desktop chat covers protected game HUD: " + JSON.stringify(desktopHud));
  for (const kind of ["contra", "basketball", "yuyuhakusho", "battlecity"]) {
    const directory = path.join(root, ".local", kind, "controller-overlay");
    await mkdir(directory, { recursive: true });
    const pageURL = server.resolvedUrls.local[0] + "?kind=" + kind;
    if (kind !== "contra") {
      socket.close();
      await browser("close");
      await browser("open", pageURL);
      await browser("wait", kind === "battlecity" ? "[data-battlecity-dpad]" : "[data-controller-dpad]");
      const nextEndpoint = await browser("get", "cdp-url");
      socket = new WebSocket(typeof nextEndpoint === "string" ? nextEndpoint : nextEndpoint.cdpUrl);
      await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
      socket.onmessage = ({ data }) => { const body = JSON.parse(data); const request = pending.get(body.id); if (!request) return; pending.delete(body.id); body.error ? request.reject(Error(JSON.stringify(body.error))) : request.resolve(body.result); };
      const nextTargets = await call("Target.getTargets");
      const nextTarget = nextTargets.targetInfos.find(value => value.type === "page" && value.url === pageURL);
      assert.ok(nextTarget);
      ({ sessionId } = await call("Target.attachToTarget", { targetId: nextTarget.targetId, flatten: true }));
    }
    await call("Page.navigate", { url: pageURL }, sessionId);
    let ready = false;
    for (let attempt = 0; attempt < 100; attempt++) {
      try { ready = await evaluate(`location.href===${JSON.stringify(pageURL)}&&window.controllerOverlayReady===true&&!!document.querySelector('[data-controller-dpad],[data-battlecity-dpad]')`); } catch {}
      if (ready) break;
      await new Promise(resolve => setTimeout(resolve, 50));
    }
    assert.ok(ready, "controller fixture did not become ready");
    await call("Target.detachFromTarget", { sessionId });
    const currentTargets = await call("Target.getTargets");
    const currentTarget = currentTargets.targetInfos.find(value => value.type === "page" && value.url === server.resolvedUrls.local[0] + "?kind=" + kind);
    assert.ok(currentTarget, "current controller fixture target is missing");
    ({ sessionId } = await call("Target.attachToTarget", { targetId: currentTarget.targetId, flatten: true }));
    const report = { kind, passed: false, scope: "actual shared controller and game CSS in a UI fixture; native touch, not a complete rule match or physical handset", layouts: [] };
    reports.push(report);
    for (const [width, height] of [[320, 760], [844, 390], [466, 678], [678, 466], [626, 890], [890, 626], [744, 1133], [1133, 744], [820, 1180], [1180, 820], [1024, 1366], [1366, 1024]]) {
      await call("Emulation.setDeviceMetricsOverride", { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: true, screenOrientation: { type: width > height ? "landscapePrimary" : "portraitPrimary", angle: width > height ? 90 : 0 } }, sessionId);
      await call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 }, sessionId);
      await call("Page.bringToFront", {}, sessionId);
      await call("Input.dispatchTouchEvent", { type: "touchCancel", touchPoints: [] }, sessionId).catch(error => { if (!error.message.includes("Must send a TouchStart first")) throw error; });
      await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))");
      const geometry = await evaluate(`(()=>{const game=document.querySelector('[data-overlay-stage]');const panel=document.querySelector('.touch-controller-overlay,.bc-controls');const screen=document.querySelector('.bc-field-wrap,.contra-runtime-screen,.basketball-screen,.yuyu-screen');const targets=[...panel.querySelectorAll('[data-controller-button],.bc-dpad,.bc-fire')].map(element=>({name:element.dataset.controllerButton??element.className,rect:element.getBoundingClientRect().toJSON(),background:getComputedStyle(element).backgroundColor}));return {game:game.getBoundingClientRect().toJSON(),screen:screen.getBoundingClientRect().toJSON(),panel:panel.getBoundingClientRect().toJSON(),position:getComputedStyle(panel).position,gridArea:getComputedStyle(panel).gridArea,targets}})()`);
      const layout = { width, height, geometry };
      report.layouts.push(layout);
      assert.ok(await evaluate('document.documentElement.scrollWidth<=innerWidth'), kind + ' normal chat placement created horizontal overflow');
      layout.normalChat = await evaluate(`(()=>{const chat=document.querySelector('.chat-toggle').getBoundingClientRect();return [...document.querySelectorAll('[data-controller-button],.bc-dpad,.bc-fire')].map(element=>{const rect=element.getBoundingClientRect();return {button:element.dataset.controllerButton??element.className,covered:rect.left<chat.right&&rect.right>chat.left&&rect.top<chat.bottom&&rect.bottom>chat.top}})})()`);
      assert.ok(layout.normalChat.every(button => !button.covered), kind + ' normal room chat covers a touch action: ' + JSON.stringify(layout.normalChat));
      if (kind !== "battlecity" && width > height && geometry.game.width >= 760) {
        layout.picture = await evaluate("document.querySelector('[data-overlay-stage] canvas').getBoundingClientRect().toJSON()");
        assert.ok(layout.picture.width >= 320, kind + " side controls shrank the picture too far");
        assert.ok(geometry.targets.every(value => value.rect.right <= layout.picture.left || value.rect.left >= layout.picture.right), kind + " side controls cover the game picture");
      }
      assert.ok(kind === "battlecity" ? geometry.gridArea.includes("field") : geometry.position === "absolute", kind + " still reserves an external controller row");
      assert.ok(geometry.targets.every(value => value.rect.width >= 44 && value.rect.height >= 44), kind + " undersized touch target");
      assert.ok(geometry.targets.every(value => value.rect.left >= geometry.game.left && value.rect.right <= geometry.game.right && value.rect.top >= geometry.game.top && value.rect.bottom <= geometry.game.bottom), kind + " touch target escaped game area");
      assert.ok(geometry.targets.every(value => value.background.startsWith("rgba(")), kind + " opaque control background");
      const beforeImage = await call("Page.captureScreenshot", { format: "png", captureBeyondViewport: false, clip: { x: 0, y: 0, width, height, scale: 1 } }, sessionId);
      await writeFile(path.join(directory, width + "x" + height + "-before.png"), Buffer.from(beforeImage.data, "base64"));
      const selectors = kind === "battlecity" ? ["[data-battlecity-dpad]", "[data-battlecity-fire]"] : ["[data-controller-dpad]", "[data-controller-button=a]"];
      const boxes = await evaluate(`${JSON.stringify(selectors)}.map(selector=>document.querySelector(selector).getBoundingClientRect().toJSON())`);
      const points = boxes.map((box, id) => ({ id, x: box.x + box.width * (id ? 0.5 : 0.2), y: box.y + box.height / 2, radiusX: 3, radiusY: 3, force: 1 }));
      layout.hitTargets = await evaluate(`${JSON.stringify(points)}.map(point=>{const hit=document.elementFromPoint(point.x,point.y);return {x:point.x,y:point.y,tag:hit?.tagName,className:hit?.className,button:hit?.closest('[data-controller-button]')?.getAttribute('data-controller-button'),dpad:!!hit?.closest('[data-controller-dpad],[data-battlecity-dpad]'),viewport:{width:innerWidth,height:innerHeight,visualHeight:visualViewport.height,scale:visualViewport.scale}}})`);
      assert.ok(layout.hitTargets[0].dpad && (kind === "battlecity" ? layout.hitTargets[1].className.includes("bc-fire") : layout.hitTargets[1].button === "a"), kind + " touch target covered");
      await evaluate("window.controllerOverlayEvents=[];for(const type of ['touchstart','touchmove','touchend','pointerdown'])document.addEventListener(type,event=>window.controllerOverlayEvents.push({type:event.type,target:event.target.className,trusted:event.isTrusted,changed:event.changedTouches?[...event.changedTouches].map(touch=>touch.identifier):null,active:event.touches?[...event.touches].map(touch=>touch.identifier):null}),{capture:true,once:true});true");
      await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: points }, sessionId);
      await evaluate("new Promise(resolve=>requestAnimationFrame(resolve))");
      const combined = await evaluate("({...window.controllerOverlayProbe})");
      layout.combined = combined;
      layout.events = await evaluate("window.controllerOverlayEvents");
      assert.ok(kind === "battlecity" ? combined.move === "left" && combined.fire : combined.actions.includes("left") && combined.actions.includes("a"), kind + " combined input failed");
      if (kind !== "battlecity") {
        const nextBox = await evaluate("document.querySelector('[data-controller-button=b]').getBoundingClientRect().toJSON()");
        points[1] = { ...points[1], x: nextBox.x + nextBox.width / 2, y: nextBox.y + nextBox.height / 2 };
        await call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: points }, sessionId);
        await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
        layout.slid = await evaluate("({...window.controllerOverlayProbe})");
        assert.ok(layout.slid.actions.includes("left") && layout.slid.actions.includes("b") && !layout.slid.actions.includes("a"), kind + " sliding A to B retained the old action");
      }
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [points[1]] }, sessionId);
      await evaluate("new Promise(resolve=>requestAnimationFrame(resolve))");
      const one = await evaluate("({...window.controllerOverlayProbe})");
      layout.one = one;
      assert.ok(kind === "battlecity" ? one.move === "left" && !one.fire : one.actions.length === 1 && one.actions.includes("left"), kind + " single release cleared other finger");
      await call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [points[0]] }, sessionId);
      await evaluate("new Promise(resolve=>requestAnimationFrame(resolve))");
      const released = await evaluate("({...window.controllerOverlayProbe})");
      layout.released = released;
      assert.ok(kind === "battlecity" ? released.move === "none" && !released.fire : released.actions.length === 0, kind + " retained input");
      if (width === 626 && height === 890) {
        await call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: points }, sessionId);
        await evaluate("new Promise(resolve=>requestAnimationFrame(resolve))");
        await call("Emulation.setDeviceMetricsOverride", { width: 890, height: 626, screenWidth: 890, screenHeight: 626, deviceScaleFactor: 1, mobile: true, screenOrientation: { type: "landscapePrimary", angle: 90 } }, sessionId);
        await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))");
        layout.foldReleased = await evaluate("({...window.controllerOverlayProbe})");
        assert.ok(kind === "battlecity" ? layout.foldReleased.move === "none" && !layout.foldReleased.fire : layout.foldReleased.actions.length === 0, kind + " fold retained an old touch input");
        await call("Input.dispatchTouchEvent", { type: "touchCancel", touchPoints: [] }, sessionId).catch(error => { if (!error.message.includes("Must send a TouchStart first")) throw error; });
        await call("Emulation.setDeviceMetricsOverride", { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: true, screenOrientation: { type: "portraitPrimary", angle: 0 } }, sessionId);
        await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
      }
      const image = await call("Page.captureScreenshot", { format: "png", captureBeyondViewport: false, clip: { x: 0, y: 0, width, height, scale: 1 } }, sessionId);
      await writeFile(path.join(directory, width + "x" + height + ".png"), Buffer.from(image.data, "base64"));
      const mouseClick = async selector => {
        const box = await evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});const box=element.getBoundingClientRect();const hit=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);if(hit!==element&&!element.contains(hit))throw Error('fullscreen action is covered');return box.toJSON()})()`);
        const mouse = { x: box.x + box.width / 2, y: box.y + box.height / 2, button: "left", clickCount: 1 };
        await call("Input.dispatchMouseEvent", { type: "mousePressed", ...mouse, buttons: 1 }, sessionId);
        await call("Input.dispatchMouseEvent", { type: "mouseReleased", ...mouse, buttons: 0 }, sessionId);
      };
      await mouseClick("#fixture-fullscreen");
      let fullscreenReady = false;
      for (let attempt = 0; attempt < 100; attempt++) {
        fullscreenReady = await evaluate("!!document.querySelector('[data-room-fullscreen] .chat-toggle')");
        if (fullscreenReady) break;
        await new Promise(resolve => setTimeout(resolve, 20));
      }
      assert.ok(fullscreenReady, kind + " did not enter the actual fullscreen shell");
      await call("Emulation.setDeviceMetricsOverride", { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: true, screenOrientation: { type: width > height ? "landscapePrimary" : "portraitPrimary", angle: width > height ? 90 : 0 } }, sessionId);
      await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))");
      const full = await evaluate(`(()=>{const rect=element=>element.getBoundingClientRect().toJSON();const root=document.querySelector('[data-overlay-stage]');const chat=document.querySelector('.chat-toggle');const targets=[...root.querySelectorAll('[data-controller-button],.bc-dpad,.bc-fire')].map(element=>({name:element.dataset.controllerButton??element.className,rect:rect(element)}));return {mode:document.querySelector('[data-room-fullscreen]').dataset.roomFullscreen,root:rect(root),chat:rect(chat),targets,canvas:rect(root.querySelector('canvas')),resources:[...root.querySelectorAll('.bc-player')].map(element=>({text:element.textContent,rect:rect(element)})),width:innerWidth,height:innerHeight}})()`);
      layout.fullscreen = full;
      assert.ok(full.root.top >= 0 && full.root.bottom <= height && full.root.left >= 0 && full.root.right <= width, kind + " fullscreen game escaped the available viewport");
      const overlapping = (a, b) => a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
      assert.ok(full.targets.every(value => !overlapping(value.rect, full.chat)), kind + " fullscreen chat covers a touch action");
      assert.ok(full.targets.every(value => value.rect.left >= 0 && value.rect.right <= width && value.rect.top >= 0 && value.rect.bottom <= height), kind + " fullscreen controls escaped the viewport");
      assert.ok(full.resources.every(value => value.rect.top >= 0 && value.rect.bottom <= height && !overlapping(value.rect, full.chat)), kind + " team resources are clipped or covered by chat");
      if (kind === "battlecity") assert.equal(Math.round(full.canvas.width), Math.round(full.canvas.height), "tank field lost its square aspect");
      const fullImage = await call("Page.captureScreenshot", { format: "png", captureBeyondViewport: false, clip: { x: 0, y: 0, width, height, scale: 1 } }, sessionId);
      await writeFile(path.join(directory, width + "x" + height + "-fullscreen.png"), Buffer.from(fullImage.data, "base64"));
      await mouseClick('button[aria-label="退出全屏"]');
      await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
    }
    report.passed = true;
    await writeFile(path.join(directory, "report.json"), JSON.stringify(report, null, 2) + "\n");
  }
} finally {
  await writeFile(path.join(out, "all-games.json"), JSON.stringify({ passed: reports.length === 4 && reports.every(report => report.passed), reports }, null, 2) + "\n");
  socket?.close();
  await browser("close").catch(() => {});
  await server.close();
}
