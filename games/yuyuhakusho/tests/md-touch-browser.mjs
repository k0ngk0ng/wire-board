import assert from "node:assert/strict";
import { promisify } from "node:util";
import { execFile } from "node:child_process";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const out = path.join(root, ".local/yuyuhakusho/mobile-controls");
const temporary = path.join(out, "temporary");
const sockets = path.join(root, ".local/yuyuhakusho/s");
for (const folder of [out, temporary, sockets]) await mkdir(folder, { recursive: true });
const env = { ...process.env, TMPDIR: temporary, AGENT_BROWSER_PROFILE: path.join(temporary, "profile"), AGENT_BROWSER_SOCKET_DIR: sockets, AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const execute = promisify(execFile);
const browser = async (...args) => {
  const { stdout } = await execute("agent-browser", ["--session", "ym", ...args, "--json"], { cwd: root, env, timeout: 30000 });
  const value = JSON.parse(stdout);
  assert.equal(value.success, true, stdout);
  return value.data;
};
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const { createServer } = require("vite");
const react = require("@vitejs/plugin-react").default;
await writeFile(path.join(temporary, "index.html"), `<meta name="viewport" content="width=device-width,initial-scale=1"><div id="root"></div><script type="module" src="/@fs/${path.join(root, "games/yuyuhakusho/tests/MDTouchFixture.tsx")}"></script>`);
const server = await createServer({ configFile: false, root: temporary, cacheDir: path.join(temporary, "vite-cache"), publicDir: false, optimizeDeps: { include: ["react", "react-dom/client", "react/jsx-dev-runtime"] }, plugins: [react()], resolve: { alias: { react: path.join(root, "web/node_modules/react"), "react-dom": path.join(root, "web/node_modules/react-dom") }, dedupe: ["react", "react-dom"] }, server: { host: "127.0.0.1", port: 0, fs: { allow: [root] } } });
await server.listen();
const url = server.resolvedUrls.local[0];
const result = { passed: false, checks: [], input: "native CDP touch events through the shared MD controller and the game's MDInput" };
let socket;
try {
  await browser("open", url);
  await browser("wait", "[data-controller-dpad]");
  const endpoint = (await browser("get", "cdp-url")).cdpUrl;
  socket = new WebSocket(endpoint);
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  let serial = 0;
  const pending = new Map();
  socket.onmessage = ({ data }) => { const value = JSON.parse(data); const request = pending.get(value.id); if (!request) return; pending.delete(value.id); value.error ? request.reject(Error(JSON.stringify(value.error))) : request.resolve(value.result); };
  const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const id = ++serial; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, sessionId })); });
  const { targetInfos } = await call("Target.getTargets");
  const target = targetInfos.find(value => value.type === "page" && value.url === url);
  const { sessionId } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true });
  const evaluate = async expression => {
    const value = await call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }, sessionId);
    if (value.exceptionDetails) throw Error(JSON.stringify(value.exceptionDetails));
    return value.result.value;
  };
  await call("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 }, sessionId);
  for (const [width, height] of [[320, 640], [844, 390]]) {
    await call("Emulation.setDeviceMetricsOverride", { width, height, deviceScaleFactor: 1, mobile: true }, sessionId);
    await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
    const rects = await evaluate(`Object.fromEntries(['[data-controller-dpad]','[data-controller-button=a]','[data-controller-button=b]','[data-controller-button=c]','[data-controller-button=x]','[data-controller-button=y]'].map(selector=>[selector,document.querySelector(selector).getBoundingClientRect().toJSON()]))`);
    const contact = (id, selector, fraction = 0.5) => { const rect = rects[selector]; return { id, x: rect.x + rect.width * fraction, y: rect.y + rect.height / 2, radiusX: 3, radiusY: 3, force: 1 }; };
    const direction = contact(0, "[data-controller-dpad]", 0.2);
    const a = contact(7, "[data-controller-button=a]");
    const b = contact(9, "[data-controller-button=b]");
    const c = contact(7, "[data-controller-button=c]");
    const x = contact(11, "[data-controller-button=x]");
    const y = contact(13, "[data-controller-button=y]");
    const events = [];
    const touch = async (type, touchPoints, expected) => {
      await call("Input.dispatchTouchEvent", { type, touchPoints }, sessionId);
      await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
      const mask = await evaluate("window.mdTouchProbe.mask");
      events.push({ type, touchPoints, expected, mask });
      result.checks.push({ width, height, ...events.at(-1) });
      assert.equal(mask, expected, JSON.stringify(events.at(-1)));
    };
    await touch("touchStart", [direction], 4);
    await touch("touchStart", [direction, a], 68);
    await touch("touchStart", [direction, a, b], 84);
    await touch("touchMove", [direction, c, b], 52);
    await touch("touchEnd", [direction], 48);
    await touch("touchEnd", [c], 16);
    await touch("touchCancel", [], 0);
    await touch("touchStart", [direction, a, b, x, y], 1620);
    await touch("touchEnd", [x], 596);
    await touch("touchCancel", [], 0);
    await touch("touchStart", [direction, a], 68);
    await evaluate("document.getElementById('profile').click()");
    await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
    assert.equal(await evaluate("window.mdTouchProbe.mask"), 0);
    await touch("touchEnd", [], 0);
    await call("Input.dispatchKeyEvent", { type: "keyDown", key: "j", code: "KeyJ" }, sessionId);
    await touch("touchStart", [direction], 68);
    await touch("touchEnd", [direction], 64);
    await call("Input.dispatchKeyEvent", { type: "keyUp", key: "j", code: "KeyJ" }, sessionId);
    await touch("touchStart", [direction, a], 68);
    await evaluate("document.getElementById('disable').click()");
    await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))");
    assert.equal(await evaluate("window.mdTouchProbe.mask"), 0);
    await touch("touchEnd", [], 0);
    await evaluate("document.getElementById('disable').click()");
    await browser("screenshot", path.join(out, `${width}.png`));
  }
  const events = await evaluate("window.mdTouchProbe.events");
  assert.ok(events.length > 0 && events.every(value => value.trusted));
  result.trustedEvents = events;
  result.passed = true;
} catch (error) {
  process.stderr.write(`${error.stack}\n`);
  try { process.stderr.write(JSON.stringify(await browser("errors")) + "\n"); process.stderr.write(JSON.stringify(await browser("get", "text", "body")) + "\n"); } catch {}
  throw error;
} finally {
  socket?.close();
  await writeFile(path.join(out, "result.json"), `${JSON.stringify(result, null, 2)}\n`);
  try { await browser("close"); } finally { await server.close(); await rm(temporary, { recursive: true, force: true }); }
}
