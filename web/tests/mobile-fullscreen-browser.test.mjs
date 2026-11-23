import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";

test("mobile realtime play enters fullscreen during loading and respects a manual exit", { timeout: 120000 }, async () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const output = path.join(root, ".local/yuyuhakusho/mobile-fullscreen");
  const temporary = path.join(output, "temporary");
  const browserTemporary = path.join(root, ".local/yuyuhakusho/t", process.pid.toString(36));
  const sockets = path.join(root, ".local/yuyuhakusho/s");
  for (const folder of [output, temporary, browserTemporary, sockets, ...["profile", "cache", "config", "data"].map(name => path.join(temporary, name))]) await mkdir(folder, { recursive: true });
  const env = { ...process.env, TMPDIR: browserTemporary, XDG_CONFIG_HOME: path.join(temporary, "config"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CACHE_HOME: path.join(temporary, "cache"), AGENT_BROWSER_SOCKET_DIR: sockets, AGENT_BROWSER_PROFILE: path.join(temporary, "profile"), AGENT_BROWSER_NO_VERSION_CHECK: "1" };
  const execute = promisify(execFile);
  const require = createRequire(new URL("../package.json", import.meta.url));
  const session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "mf"], { cwd: root, env })).stdout.trim();
  const browser = async (...args) => {
    let stdout;
    try {
      ({ stdout } = await execute("agent-browser", ["--session", session, "--profile", path.join(temporary, "profile"), ...args, "--json"], { cwd: root, env, timeout: 30000 }));
    } catch (error) { throw Error(`${error.message}\n${error.stdout ?? ""}\n${error.stderr ?? ""}`); }
    const value = JSON.parse(stdout);
    assert.equal(value.success, true, stdout);
    return value.data;
  };
  const evaluate = async expression => (await browser("eval", expression)).result;
  const frames = () => evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))");
  const report = { passed: false, checks: [] };
  let server;
  let socket;
  let serial = 0;
  const pending = new Map();
  const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
    const id = ++serial;
    pending.set(id, { resolve, reject });
    socket.send(JSON.stringify({ id, method, params, sessionId }));
  });
  try {
    if (process.platform === "linux") assert.ok(Buffer.byteLength(path.join(browserTemporary, "com.google.Chrome.123456/SingletonSocket")) < 108, "Chromium singleton socket exceeds the Linux pathname limit");
    await writeFile(path.join(temporary, "fixture.tsx"), `import {StrictMode,useEffect,useState} from 'react';import {createRoot} from 'react-dom/client';import {GameFullscreen} from '${path.join(root, "web/src/ui/room-fullscreen.tsx")}';const query=new URLSearchParams(location.search);let mounts=0;window.mobileFullscreen={ready:false};function Board(){useEffect(()=>{mounts++;return()=>mounts--},[]);return <section data-game-focus tabIndex={-1} style={{height:'100%',background:'#14292c',color:'#fff'}}>游戏加载中<button data-action>攻击</button></section>}function Fixture(){const [scope,setScope]=useState('first');const [enabled,setEnabled]=useState(false);const [auto,setAuto]=useState(false);const [show,setShow]=useState(false);useEffect(()=>{Object.assign(window.mobileFullscreen,{ready:true,start:()=>{setAuto(true);setShow(true)},loaded:()=>setEnabled(true),next:()=>setScope('next'),close:()=>setShow(false),mounts:()=>mounts})},[]);return <><header data-platform>平台菜单<input /></header>{show&&<GameFullscreen scope={scope} enabled={enabled} autoEnter={auto} onReadyChange={()=>{}}><Board /></GameFullscreen>}</>}const fixture=<Fixture/>;createRoot(document.getElementById('root')).render(query.has('strict')?<StrictMode>{fixture}</StrictMode>:fixture);`);
    await require("esbuild").build({ entryPoints: [path.join(temporary, "fixture.tsx")], outfile: path.join(temporary, "fixture.js"), bundle: true, jsx: "automatic", nodePaths: [path.join(root, "web/node_modules")], define: { "process.env.NODE_ENV": '"development"' }, legalComments: "none" });
    await writeFile(path.join(temporary, "index.html"), '<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="fixture.css"><style>*{box-sizing:border-box}body{margin:0}</style><div id="root"></div><script src="fixture.js"></script>');
    const files = new Map(await Promise.all(["index.html", "fixture.js", "fixture.css"].map(async name => [name, await readFile(path.join(temporary, name))])));
    server = createServer((request, response) => {
      const name = new URL(request.url, "http://127.0.0.1").pathname.slice(1) || "index.html";
      if (!files.has(name)) { response.writeHead(404); response.end(); return; }
      response.writeHead(200, { "Content-Type": name.endsWith(".js") ? "text/javascript" : name.endsWith(".css") ? "text/css" : "text/html" });
      response.end(files.get(name));
    });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    const url = `http://127.0.0.1:${server.address().port}`;
    await browser("open", url);
    const endpoint = await browser("get", "cdp-url");
    socket = new WebSocket(typeof endpoint === "string" ? endpoint : endpoint.cdpUrl);
    await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
    socket.onmessage = ({ data }) => { const value = JSON.parse(data); const request = pending.get(value.id); if (!request) return; pending.delete(value.id); value.error ? request.reject(Error(JSON.stringify(value.error))) : request.resolve(value.result); };
    const { targetInfos } = await call("Target.getTargets");
    const target = targetInfos.find(value => value.type === "page" && value.url.startsWith(url));
    const { sessionId } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true });
    for (const [name, width, height, mobile, strict] of [["phone", 320, 760, true, false], ["pad", 1180, 820, true, true], ["narrow-desktop", 320, 760, false, false], ["desktop", 1280, 900, false, false]]) {
      await browser("open", url + (strict ? "/?strict=1" : "/"));
      await browser("set", "viewport", String(width), String(height));
      await call("Emulation.setTouchEmulationEnabled", { enabled: mobile, maxTouchPoints: mobile ? 5 : 1 }, sessionId);
      await browser("eval", "Element.prototype.requestFullscreen=()=>Promise.reject(Error('no user activation'));true");
      await browser("wait", "--fn", "window.mobileFullscreen?.ready===true");
      assert.equal(await evaluate("matchMedia('(pointer:coarse)').matches"), mobile);
      assert.equal(await evaluate("!!document.querySelector('[data-room-fullscreen]')"), false);
      await evaluate("window.mobileFullscreen.start();true");
      await frames();
      await evaluate("window.boardIdentity=document.querySelector('[data-game-focus]');true");
      const check = { name, width, height, mobile, strict, mode: await evaluate("document.querySelector('[data-game-fullscreen-surface]').dataset.roomFullscreen||''") };
      report.checks.push(check);
      assert.equal(check.mode, mobile ? "page" : "", name + " start fullscreen");
      if (!mobile) continue;
      const geometry = await evaluate("(()=>{const root=document.querySelector('[data-room-fullscreen]');return {root:root.getBoundingClientRect().toJSON(),inert:document.querySelector('[data-platform]').inert,mounts:window.mobileFullscreen.mounts(),focused:document.activeElement===window.boardIdentity}})()");
      check.geometry = geometry;
      assert.equal(Math.round(geometry.root.height), height);
      assert.equal(Math.round(geometry.root.width), width);
      assert.equal(geometry.inert, true);
      assert.equal(geometry.focused, true);
      assert.equal(geometry.mounts, 1);
      await browser("screenshot", path.join(output, name + ".png"));
      await browser("click", 'button[aria-label="退出全屏"]');
      await frames();
      await evaluate("window.mobileFullscreen.loaded();true");
      await frames();
      assert.equal(await evaluate("!!document.querySelector('[data-room-fullscreen]')"), false, "load completion re-enters after explicit exit");
      assert.equal(await evaluate("!document.querySelector('[data-platform]').inert&&document.documentElement.style.overflow===''&&window.boardIdentity===document.querySelector('[data-game-focus]')"), true);
      await evaluate("window.mobileFullscreen.next();true");
      await frames();
      assert.equal(await evaluate("!!document.querySelector('[data-room-fullscreen]')"), true, "new match must auto enter");
      await evaluate("window.mobileFullscreen.close();true");
      await frames();
      assert.equal(await evaluate("!document.querySelector('[data-platform]').inert&&document.documentElement.style.overflow===''&&window.mobileFullscreen.mounts()===0"), true, "unmount leaks fullscreen state");
    }
    report.passed = true;
  } catch (error) {
    report.error = error.stack;
    throw error;
  } finally {
    await writeFile(path.join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
    socket?.close();
    await browser("close").catch(() => {});
    await new Promise(resolve => server ? server.close(resolve) : resolve());
    await rm(temporary, { recursive: true, force: true });
    await rm(browserTemporary, { recursive: true, force: true });
  }
});
