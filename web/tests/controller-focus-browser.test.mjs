import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { randomUUID, createHash } from "node:crypto";

test(
  "FC and MD keyboard control survives background clicks while editors and dialogs retain focus",
  { timeout: 180000 },
  async () => {
    const root = fileURLToPath(new URL("../../", import.meta.url)),
      require = createRequire(new URL("../package.json", import.meta.url)),
      execute = promisify(execFile);
    const workspace = path.dirname(
      (
        await execute(
          "git",
          ["rev-parse", "--path-format=absolute", "--git-common-dir"],
          { cwd: root },
        )
      ).stdout.trim(),
    );
    const out = path.join(
      root,
      ".local/room-tools-followup/verification",
      process.env.CONTROLLER_FOCUS_REPORT ?? "focus",
    );
    const temporary = path.join(
      root,
      ".local/room-tools-followup/tmp",
      randomUUID(),
    );
    const environment = {
      ...process.env,
      TMPDIR: path.join(workspace, ".local/cft"),
      XDG_CACHE_HOME: path.join(temporary, "cache"),
      AGENT_BROWSER_SOCKET_DIR: path.join(workspace, ".local/room-flow/s"),
      AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
      AGENT_BROWSER_NO_VERSION_CHECK: "1",
      AGENT_BROWSER_ARGS: [
        "--mute-audio",
        "--disable-breakpad",
        "--disable-crash-reporter",
        ...(process.env.GITHUB_ACTIONS === "true" ? ["--no-sandbox"] : []),
      ].join(","),
    };
    for (const directory of [
      out,
      temporary,
      environment.TMPDIR,
      environment.XDG_CACHE_HOME,
      environment.AGENT_BROWSER_SOCKET_DIR,
      environment.AGENT_BROWSER_PROFILE,
    ])
      await mkdir(directory, { recursive: true });
    const report = {
      passed: false,
      scope:
        "Native browser events across actual FC, MD, basketball and common input adapters inside the production GameFullscreen; isolated from physics and server rules",
      checks: [],
      observations: [],
    };
    let server,
      session,
      socket,
      attached,
      ordinal = 0;
    const pending = new Map();
    const browser = async (...args) => {
      const result = await execute(
        "agent-browser",
        ["--session", session, ...args, "--json"],
        { cwd: root, env: environment, timeout: 35000, maxBuffer: 2 << 20 },
      );
      const value = JSON.parse(result.stdout);
      assert.equal(value.success, true, JSON.stringify(value));
      return value.data;
    };
    const evaluate = async (code) =>
      (await browser("eval", "-b", Buffer.from(code).toString("base64")))
        .result;
    const call = (method, params = {}, sessionId = attached) =>
      new Promise((resolve, reject) => {
        const id = ++ordinal;
        const timer = setTimeout(() => {
          pending.delete(id);
          reject(Error(method + " timeout"));
        }, 10000);
        pending.set(id, { resolve, reject, timer });
        socket.send(
          JSON.stringify({
            id,
            method,
            params,
            ...(sessionId ? { sessionId } : {}),
          }),
        );
      });
    const key = (type, code = "KeyD") =>
      call("Input.dispatchKeyEvent", {
        type,
        code,
        key: code === "Space" ? " " : code.slice(3).toLowerCase(),
        windowsVirtualKeyCode: code === "Space" ? 32 : 68,
      });
    const check = (name, value) => {
      report.checks.push({ name, passed: !!value });
      assert.ok(value, name);
    };
    const wait = (condition) => browser("wait", "--fn", condition);
    const observe = async (adapter, stage) => {
      report.observations.push({
        adapter,
        stage,
        state: await evaluate("window.fixture.inspect()"),
      });
    };
    try {
      const source = `import {createRoot} from 'react-dom/client';import {useEffect,useRef,useState} from 'react';import {GameFullscreen} from ${JSON.stringify(path.join(root, "web/src/ui/room-fullscreen.tsx"))};import {RuntimeInput} from ${JSON.stringify(path.join(root, "games/contra/web/runtime-input.ts"))};import {BasketballInput} from ${JSON.stringify(path.join(root, "games/basketball/web/input.ts"))};import {MDInput,mdBits} from ${JSON.stringify(path.join(root, "games/yuyuhakusho/web/input.ts"))};import {GameController} from ${JSON.stringify(path.join(root, "web/src/controllers/game-controller.ts"))};const mode=new URLSearchParams(location.search).get('adapter')||'contra';const state=window.fixture={x:20,mask:0,cancels:0,clicks:0,enabled:true,frames:0,events:[]};state.inspect=()=>({enabled:state.enabled,committedEnabled:document.querySelector('#game')?.dataset.enabled,ready:document.querySelector('#game')?.dataset.ready,fullscreenDisabled:document.querySelector('#fullscreen')?.disabled,active:document.activeElement?.id,focused:document.hasFocus(),hidden:document.hidden,visibility:document.visibilityState,mask:state.mask,x:state.x,frames:state.frames,events:state.events});for(const type of ['keydown','keyup','click','focusin','focusout','visibilitychange','blur','focus'])window.addEventListener(type,event=>queueMicrotask(()=>{state.events.push({type,trusted:event.isTrusted,target:event.target?.id||event.target?.nodeName||'',active:document.activeElement?.id,focused:document.hasFocus(),hidden:document.hidden,enabled:state.enabled,committedEnabled:document.querySelector('#game')?.dataset.enabled,ready:document.querySelector('#game')?.dataset.ready,code:event.code,prevented:event.defaultPrevented});if(state.events.length>200)state.events.shift()}),true);let controller;const profile={revision:1,turboHz:10,keyboard:{right:'KeyD',start:'Space'}};function App(){const game=useRef(null),fullscreen=useRef(null);const [enabled,setEnabled]=useState(true);const [ready,setReady]=useState(false);useEffect(()=>{const cancel=()=>{state.mask=0;state.cancels++};if(mode==='md'){controller=new MDInput(game.current,{...profile,keyboard:Object.fromEntries(Object.keys(mdBits).map(name=>[name,name==='right'?'KeyD':name==='start'?'Space':'']))},cancel);controller.setEnabled(true)}else if(mode==='basketball')controller=new BasketballInput(profile,game.current,cancel);else if(mode==='common')controller=new GameController(game.current,profile,{id:'fixture',name:'fixture',buttons:[{id:'right',label:'右'},{id:'start',label:'开始'}],defaults:profile},{enabled:()=>state.enabled,changed:()=>{}});else controller=new RuntimeInput(profile,game.current,cancel);state.read=()=>{const sampled=controller.sample(performance.now());return typeof sampled==='number'?sampled:(sampled.has('right')?1:0)|(sampled.has('start')?16:0)};state.setEnabled=value=>{state.enabled=value;controller.setEnabled?.(value);setEnabled(value)};state.dispose=()=>controller.dispose();let frame;const draw=()=>{state.frames++;state.mask=state.read();if(state.mask&(mode==='md'?8:1))state.x=Math.min(260,state.x+2);const canvas=game.current.querySelector('canvas'),ctx=canvas.getContext('2d');ctx.fillStyle='#e8ede5';ctx.fillRect(0,0,320,100);ctx.fillStyle='#176247';ctx.fillRect(state.x,35,24,24);frame=requestAnimationFrame(draw)};draw();return()=>{cancelAnimationFrame(frame);controller.dispose()}},[]);return <><div id='blank' tabIndex={-1}>房间空白与普通文字</div><input id='chat' placeholder='聊天'/><textarea id='textarea'/><div id='editable' contentEditable>编辑内容</div><button id='outside-button' onClick={()=>state.clicks++}>房间操作</button><a id='outside-link' href='#outside'>房间链接</a><button id='open-modal' onClick={()=>document.querySelector('dialog').showModal()}>打开设置</button><button id='fullscreen' disabled={!ready} onClick={()=>fullscreen.current.enter()}>全屏测试</button><dialog><input id='dialog-input'/><button id='close-modal' onClick={()=>document.querySelector('dialog').close()}>关闭设置</button></dialog><GameFullscreen ref={fullscreen} scope='focus-fixture' enabled={enabled} onReadyChange={setReady}><div id='padding' tabIndex={-1}>游戏区边缘</div><section ref={game} id='game' data-enabled={enabled} data-ready={ready} data-game-focus tabIndex={0}><canvas width='320' height='100'/></section></GameFullscreen></>};createRoot(document.getElementById('root')).render(<App/>);`;
      await writeFile(path.join(temporary, "fixture.tsx"), source);
      await require("esbuild").build({
        entryPoints: [path.join(temporary, "fixture.tsx")],
        outfile: path.join(temporary, "fixture.js"),
        bundle: true,
        jsx: "automatic",
        nodePaths: [path.join(root, "web/node_modules")],
        define: { "process.env.NODE_ENV": '"production"' },
        legalComments: "none",
      });
      const html = Buffer.from(
        '<!doctype html><html lang="zh-CN"><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="fixture.css"><style>body{margin:16px;font-family:system-ui;color:#293b35;background:#faf9f1}#blank,#padding{padding:16px;background:#e6ece2;margin:8px 0}input,textarea,[contenteditable],button,a{margin:5px}#game{padding:12px}canvas{max-width:100%}dialog{background:#faf9f1;color:#293b35}</style></head><body><div id="root"></div><script src="fixture.js"></script></body></html>',
      );
      const files = new Map([
        ["/", [html, "text/html"]],
        [
          "/fixture.js",
          [
            await readFile(path.join(temporary, "fixture.js")),
            "text/javascript",
          ],
        ],
        [
          "/fixture.css",
          [await readFile(path.join(temporary, "fixture.css")), "text/css"],
        ],
        [
          "/idle.html",
          [Buffer.from("<!doctype html><title>Other tab</title>"), "text/html"],
        ],
      ]);
      server = createServer((request, response) => {
        const entry = files.get(
          new URL(request.url, "http://localhost").pathname,
        );
        response.writeHead(entry ? 200 : 404, {
          "Content-Type": entry?.[1] ?? "text/plain",
        });
        response.end(entry?.[0] ?? "");
      });
      await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
      const origin = "http://127.0.0.1:" + server.address().port;
      session = (
        await execute(
          "agent-browser",
          ["session", "id", "--scope", "worktree", "--prefix", "cf"],
          { cwd: root, env: environment },
        )
      ).stdout.trim();
      await browser("open", origin);
      await browser("set", "viewport", "1100", "800");
      const endpoint = await browser("get", "cdp-url");
      socket = new WebSocket(
        typeof endpoint === "string"
          ? endpoint
          : (endpoint.url ?? endpoint.cdpUrl ?? endpoint.value),
      );
      socket.onmessage = ({ data }) => {
        const message = JSON.parse(data),
          entry = pending.get(message.id);
        if (!entry) return;
        clearTimeout(entry.timer);
        pending.delete(message.id);
        if (message.error) entry.reject(Error(JSON.stringify(message.error)));
        else entry.resolve(message.result);
      };
      await new Promise((resolve, reject) => {
        socket.onopen = resolve;
        socket.onerror = reject;
      });
      const targets = await call("Target.getTargets", {}, undefined),
        target = targets.targetInfos.find(
          (target) => target.type === "page" && target.url.startsWith(origin),
        );
      attached = (
        await call(
          "Target.attachToTarget",
          { targetId: target.targetId, flatten: true },
          undefined,
        )
      ).sessionId;
      for (const adapter of ["contra", "md", "basketball", "common"]) {
        await browser("open", origin + "/?adapter=" + adapter);
        await browser("wait", "#game");
        await wait("typeof window.fixture.read==='function'");
        await wait("document.querySelector('#game').dataset.enabled==='true'&&document.querySelector('#game').dataset.ready==='true'&&!document.querySelector('#fullscreen').disabled");
        await browser("click", "#game");
        await key("keyDown");
        await wait("window.fixture.mask>0");
        await key("keyUp");
        await wait("window.fixture.mask===0");
        const before = await evaluate("window.fixture.x");
        await browser("click", "#blank");
        const scroll = await evaluate("scrollY");
        await key("keyDown");
        await wait("window.fixture.mask>0");
        await key("keyUp");
        await wait("window.fixture.mask===0");
        check(
          adapter + " moves visibly after background click",
          (await evaluate("window.fixture.x")) > before,
        );
        check(
          adapter + " restores game focus without scrolling",
          await evaluate(
            "document.activeElement.id==='game'&&scrollY===" + scroll,
          ),
        );
        for (const selector of [
          "#chat",
          "#textarea",
          "#editable",
          "#outside-button",
          "#outside-link",
        ]) {
          await browser("click", selector);
          await key("keyDown");
          check(
            adapter + " protects " + selector,
            await evaluate(
              'window.fixture.read()===0&&document.activeElement.id!=="game"',
            ),
          );
          await key("keyUp");
        }
        await browser("click", "#outside-button");
        const clicks = await evaluate("window.fixture.clicks");
        await browser("press", "Space");
        check(
          adapter + " preserves native button keyboard action",
          (await evaluate("window.fixture.clicks")) === clicks + 1,
        );
        await browser("click", "#open-modal");
        await key("keyDown");
        check(
          adapter + " protects modal",
          await evaluate(
            "window.fixture.read()===0&&document.activeElement.id==='dialog-input'",
          ),
        );
        await key("keyUp");
        await browser("click", "#close-modal");
        for (const fallback of [false, true]) {
          if (fallback)
            await evaluate(
              "Element.prototype.requestFullscreen=()=>Promise.reject(Error('native unavailable'));true",
            );
          await browser("click", "#fullscreen");
          await wait("!!document.querySelector('[data-room-fullscreen]')");
          await browser("click", "#padding");
          await key("keyDown");
          await wait("window.fixture.mask>0");
          await key("keyUp");
          check(
            adapter + " fullscreen background restores inner root " + fallback,
            await evaluate("document.activeElement.id==='game'"),
          );
          await browser("click", "[aria-label=退出全屏]");
          await wait("!document.querySelector('[data-room-fullscreen]')");
        }
        await browser("click", "#blank");
        await key("keyDown");
        await wait("window.fixture.mask>0");
        const tabs = await browser("tab"),
          original = tabs.tabs.find((tab) => tab.active).tabId;
        await browser("tab", "new", origin + "/idle.html");
        const hidden = await call("Runtime.evaluate", {
          expression: "window.fixture.read()",
          returnByValue: true,
        });
        check(
          adapter + " releases keys on tab loss",
          hidden.result.value === 0,
        );
        await browser("tab", original);
        await key("keyUp");
        check(
          adapter + " old press does not resume",
          await evaluate("window.fixture.read()===0"),
        );
        await evaluate("window.fixture.setEnabled(false);true");
        await wait("document.querySelector('#game').dataset.enabled==='false'&&document.querySelector('#game').dataset.ready==='false'&&document.querySelector('#fullscreen').disabled");
        await browser("click", "#blank");
        await key("keyDown");
        check(
          adapter + " disabled game does not take focus",
          await evaluate(
            "window.fixture.read()===0&&document.activeElement.id==='blank'",
          ),
        );
        await key("keyUp");
        await evaluate("window.fixture.setEnabled(true);true");
        await wait("document.querySelector('#game').dataset.enabled==='true'&&document.querySelector('#game').dataset.ready==='true'&&!document.querySelector('#fullscreen').disabled&&document.hasFocus()&&!document.hidden");
        await browser("click", "#blank");
        check(
          adapter + " re-enabled background restores game focus",
          await evaluate("document.activeElement.id==='game'"),
        );
        await key("keyDown");
        await wait("window.fixture.mask>0");
        await key("keyUp");
        await wait("window.fixture.mask===0");
        await observe(adapter, "enabled-input-recovered");
        await browser("screenshot", path.join(out, adapter + ".png"));
        await writeFile(
          path.join(out, adapter + ".snapshot.json"),
          JSON.stringify(await browser("snapshot", "-i"), null, 2),
        );
        await evaluate("window.fixture.dispose();true");
        await key("keyDown");
        check(
          adapter + " disposed input stays released",
          await evaluate("window.fixture.read()===0"),
        );
        await key("keyUp");
      }
      check(
        "no browser runtime errors",
        !(await browser("errors")).errors.length,
      );
      report.passed = true;
    } catch (error) {
      report.error = error.stack;
      if (error.stdout || error.stderr)
        report.browserFailure = { stdout: error.stdout, stderr: error.stderr };
      if (session) {
        try {
          report.page = {
            state: await evaluate("window.fixture.inspect()"),
            url: await browser("get", "url"),
            tabs: await browser("tab"),
            snapshot: await browser("snapshot", "-i"),
          };
          await browser("screenshot", path.join(out, "failure.png"));
        } catch {}
      }
      throw error;
    } finally {
      if (socket) socket.close();
      for (const entry of pending.values()) clearTimeout(entry.timer);
      if (session) {
        try {
          await browser("close");
        } catch {}
      }
      if (server) await new Promise((resolve) => server.close(resolve));
      report.sources = {};
      for (const file of [
        "web/src/ui/game-focus.ts",
        "web/src/ui/room-fullscreen.tsx",
        "games/contra/web/runtime-input.ts",
        "games/basketball/web/input.ts",
        "games/yuyuhakusho/web/input.ts",
        "web/src/controllers/game-controller.ts",
        "web/tests/controller-focus-browser.test.mjs",
      ])
        report.sources[file] = createHash("sha256")
          .update(await readFile(path.join(root, file)))
          .digest("hex");
      await writeFile(
        path.join(out, "result.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      await rm(temporary, { recursive: true, force: true });
      console.log("Controller focus evidence:", path.join(out, "result.json"));
    }
  },
);
