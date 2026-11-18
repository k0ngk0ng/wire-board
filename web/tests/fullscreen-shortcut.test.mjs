import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { join, resolve, dirname } from "node:path";
import { createHash, randomUUID } from "node:crypto";

test("F11 uses the shared game fullscreen lifecycle and retains game keyboard focus", { timeout: 120000 }, async () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const output = join(root, ".local/fullscreen-shortcut");
  const temporary = join(output, `temporary-${randomUUID().slice(0, 8)}`);
  const require = createRequire(new URL("../package.json", import.meta.url));
  const execute = promisify(execFile);
  const flags = ["--mute-audio", "--disable-breakpad", "--disable-crash-reporter"];
  if (process.env.GITHUB_ACTIONS === "true") flags.push("--no-sandbox");
  const common = (await execute("git", ["rev-parse", "--git-common-dir"], { cwd: root })).stdout.trim();
  const socketRoot = join(dirname(resolve(root, common)), ".local/ab");
  const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: socketRoot, AGENT_BROWSER_PROFILE: join(temporary, "profile"), AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: flags.join(",") };
  const report = { command: "node --test web/tests/fullscreen-shortcut.test.mjs", passed: false, checks: [] };
  await mkdir(temporary, { recursive: true });
  let server;
  let session;
  const browser = async (...args) => {
    let result;
    try {
      result = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 4 << 20 });
    } catch (error) { throw Error(`${error.message}\n${error.stdout ?? ""}\n${error.stderr ?? ""}`); }
    const value = JSON.parse(result.stdout);
    assert.equal(value.success, true, JSON.stringify(value));
    return value.data;
  };
  const evaluate = async code => (await browser("eval", code)).result;
  const wait = expression => browser("wait", "--fn", expression);
  const mode = () => evaluate("document.querySelector('[data-game-fullscreen-surface]').dataset.roomFullscreen || ''");
  try {
    await writeFile(join(temporary, "fixture.tsx"), `import {createRoot} from 'react-dom/client';import {useState,useRef} from 'react';import {GameFullscreen} from '${join(root, "web/src/ui/room-fullscreen.tsx")}';let mounts=0;window.fixture={mounts:()=>mounts,keys:[]};function Board(){useState(()=>++mounts);return <section tabIndex={-1} data-game-focus onKeyDown={e=>window.fixture.keys.push(e.key)} style={{height:400,background:'#cedfc0'}}>地图<button>部署</button></section>}function Fixture(){const [enabled,setEnabled]=useState(false);const [ready,setReady]=useState(false);const ref=useRef(null);return <><header>平台菜单<input data-outside /></header><button data-enable onClick={()=>setEnabled(v=>!v)}>就绪</button><button data-enter disabled={!ready} onClick={()=>ref.current.enter()}>全屏</button><GameFullscreen ref={ref} scope="shortcut-test" enabled={enabled} onReadyChange={setReady}><Board /></GameFullscreen></>}createRoot(document.getElementById('root')).render(<Fixture/>);`);
    await require("esbuild").build({ entryPoints: [join(temporary, "fixture.tsx")], outfile: join(temporary, "fixture.js"), bundle: true, jsx: "automatic", nodePaths: [join(root, "web/node_modules")], define: { "process.env.NODE_ENV": '"production"' }, legalComments: "none" });
    await writeFile(join(temporary, "index.html"), '<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="fixture.css"></head><body><div id="root"></div><script src="fixture.js"></script></body></html>');
    const files = new Map(await Promise.all(["index.html", "fixture.js", "fixture.css"].map(async name => [name, await readFile(join(temporary, name))])));
    server = createServer((request, response) => {
      const name = new URL(request.url, "http://127.0.0.1").pathname.slice(1) || "index.html";
      if (!files.has(name)) { response.writeHead(404); response.end(); return; }
      response.writeHead(200, { "Content-Type": name.endsWith(".js") ? "text/javascript" : name.endsWith(".css") ? "text/css" : "text/html" });
      response.end(files.get(name));
    });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "fs"], { cwd: root, env: environment })).stdout.trim();
    await browser("open", `http://127.0.0.1:${server.address().port}`);
    await browser("wait", "[data-game-focus]");
    await browser("focus", "[data-game-focus]");
    await browser("press", "F11");
    assert.equal(await mode(), "");
    report.checks.push("disabled");
    await browser("click", "[data-enable]");
    await browser("focus", "[data-outside]");
    await browser("press", "F11");
    assert.equal(await mode(), "");
    report.checks.push("outside focus");
    await browser("focus", "[data-game-focus]");
    await browser("press", "F11");
    await wait("document.querySelector('[data-room-fullscreen]') !== null");
    assert.equal(await mode(), "native");
    assert.equal(await evaluate("document.fullscreenElement.matches('[data-game-fullscreen-surface]') && !document.fullscreenElement.querySelector('header')"), true);
    assert.equal(await evaluate("document.activeElement.matches('[data-game-focus]')"), true);
    await browser("press", "Space");
    assert.equal(await evaluate("window.fixture.keys.includes(' ')"), true);
    assert.equal(await mode(), "native");
    await browser("screenshot", join(output, "native.png"));
    await browser("press", "F11");
    await wait("!document.querySelector('[data-room-fullscreen]')");
    assert.equal(await evaluate("!document.querySelector('header').inert && document.activeElement.matches('[data-game-focus]')"), true);
    report.checks.push("native game-only target", "keyboard focus and Space", "F11 native exit");
    await evaluate("window.nativeRequest=Element.prototype.requestFullscreen;window.fullscreenRequests=0;Element.prototype.requestFullscreen=function(){window.fullscreenRequests++;return window.nativeRequest.call(this).then(()=>new Promise(resolve=>window.finishFullscreen=resolve))};true");
    await browser("press", "F11");
    await wait("typeof window.finishFullscreen === 'function'");
    await browser("press", "F11");
    assert.equal(await mode(), "native");
    assert.equal(await evaluate("window.fullscreenRequests"), 1);
    await evaluate("window.finishFullscreen();true");
    await wait("!document.querySelector('[data-fullscreen-toggle]').disabled");
    assert.equal(await evaluate("(()=>{const key=new KeyboardEvent('keydown',{key:'F11',bubbles:true,cancelable:true,repeat:true});document.activeElement.dispatchEvent(key);return key.defaultPrevented})()"), true);
    assert.equal(await mode(), "native");
    await browser("press", "F11");
    await wait("!document.querySelector('[data-room-fullscreen]')");
    report.checks.push("pending request is not repeated", "synthetic repeated key is suppressed");
    await evaluate("Element.prototype.requestFullscreen=()=>Promise.reject(new Error('capability rejection'));true");
    await browser("press", "F11");
    await wait("document.querySelector('[data-room-fullscreen=page]') !== null");
    await browser("set", "viewport", "320", "740");
    await browser("screenshot", join(output, "fallback-320.png"));
    await browser("press", "F11");
    await wait("!document.querySelector('[data-room-fullscreen]')");
    await browser("press", "F11");
    await wait("document.querySelector('[data-room-fullscreen=page]') !== null");
    await browser("press", "Escape");
    await wait("!document.querySelector('[data-room-fullscreen]')");
    assert.equal(await evaluate("window.fixture.keys.includes('Escape')"), true);
    assert.equal(await evaluate("!document.querySelector('header').inert && document.documentElement.style.overflow === ''"), true);
    assert.equal(await evaluate("window.fixture.mounts()"), 1);
    report.checks.push("rejected native fallback", "F11 fallback exit", "Escape forwarded and cleanup", "board mount retained");
    report.passed = true;
  } catch (error) {
    report.error = error.stack ?? String(error);
    if (session) { try { await browser("screenshot", join(output, "failure.png")); } catch {} }
    throw error;
  } finally {
    if (session) { try { await browser("close"); } catch {} }
    if (server) await new Promise(resolve => server.close(resolve));
    await rm(temporary, { recursive: true, force: true });
    report.sourceSHA256 = createHash("sha256").update(await readFile(join(root, "web/src/ui/room-fullscreen.tsx"))).digest("hex");
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
