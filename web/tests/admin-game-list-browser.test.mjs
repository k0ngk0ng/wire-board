import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import path from "node:path";

test(
  "admin cover cards sort inline and keep configuration in a dialog",
  { skip: !process.env.GAME_CONFIG_API_URL, timeout: 240000 },
  async () => {
    const root = fileURLToPath(new URL("../../", import.meta.url));
    const output = path.join(root, ".local/admin-game-config/list");
    const temporary = path.join(output, "temporary");
    const environment = {
      ...process.env,
      TMPDIR: path.join(temporary, "tmp"),
      XDG_CONFIG_HOME: path.join(temporary, "config"),
      XDG_DATA_HOME: path.join(temporary, "data"),
      XDG_CACHE_HOME: path.join(temporary, "cache"),
      AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
      AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/admin-game-config/s"),
      AGENT_BROWSER_NO_VERSION_CHECK: "1",
    };
    for (const directory of [
      output,
      temporary,
      environment.TMPDIR,
      environment.XDG_CONFIG_HOME,
      environment.XDG_DATA_HOME,
      environment.XDG_CACHE_HOME,
      environment.AGENT_BROWSER_PROFILE,
      environment.AGENT_BROWSER_SOCKET_DIR,
    ])
      await mkdir(directory, { recursive: true });
    const execute = promisify(execFile);
    const session = (
      await execute(
        "agent-browser",
        ["session", "id", "--scope", "worktree", "--prefix", "gl"],
        { cwd: root, env: environment },
      )
    ).stdout.trim();
    const browser = async (...args) => {
      const response = await execute(
        "agent-browser",
        [
          "--session",
          session,
          "--profile",
          environment.AGENT_BROWSER_PROFILE,
          ...args,
          "--json",
        ],
        { cwd: root, env: environment, timeout: 30000, maxBuffer: 4 << 20 },
      );
      const value = JSON.parse(response.stdout);
      assert.equal(value.success, true, response.stdout);
      return value.data;
    };
    const evaluate = async (expression) =>
      (await browser("eval", expression)).result;
    const wait = (expression) => browser("wait", "--fn", expression);
    const api = (endpoint, body) =>
      evaluate(
        `fetch(${JSON.stringify("/api" + endpoint)},${JSON.stringify(body === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) })}).then(async r=>{const data=await r.json();if(!r.ok)throw Error(JSON.stringify(data));return data})`,
      );
    const order = async () =>
      (await api("/admin")).games.map((game) => game.kind);
    const ids = () =>
      evaluate(
        "[...document.querySelectorAll('.admin-games [data-game-id]')].map(e=>e.dataset.gameId)",
      );
    const card = (id) => `.admin-games [data-game-id="${id}"]`;
    const require = createRequire(new URL("../package.json", import.meta.url));
    const config = JSON.parse(
      await readFile(path.join(root, ".local/business.json"), "utf8"),
    );
    const server = await require("vite").createServer({
      configFile: false,
      root: path.join(root, "web"),
      publicDir: false,
      cacheDir: path.join(temporary, "vite"),
      plugins: [require("@vitejs/plugin-react").default()],
      define: {
        "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(config.CDN_ORIGIN),
      },
      resolve: {
        alias: Object.fromEntries(
          ["react", "react-dom", "lucide-react", "fflate", "three"].map(
            (name) => [name, path.join(root, "web/node_modules", name)],
          ),
        ),
        dedupe: ["react", "react-dom", "lucide-react"],
      },
      server: {
        host: "127.0.0.1",
        port: 0,
        fs: { allow: [root] },
        proxy: {
          "/api": { target: process.env.GAME_CONFIG_API_URL, ws: true },
        },
      },
    });
    const report = { passed: false, checks: [], layouts: [] };
    const check = (name, passed) => {
      assert.ok(passed, name);
      report.checks.push(name);
    };
    let socket;
    try {
      await server.listen();
      const url = server.resolvedUrls.local[0];
      await browser("open", url);
      await api("/login", {
        name: "ConfigRoot",
        password: "config-test-password-123",
      });
      await browser("open", url + "?admin=1");
      await browser("wait", ".admin-tabs");
      await browser(
        "find",
        "role",
        "button",
        "click",
        "--name",
        "游戏管理",
        "--exact",
      );
      await wait("document.querySelectorAll('.admin-game-card').length>36");
      check(
        "covers in the actual list",
        await evaluate(
          "[...document.querySelectorAll('.admin-game-card')].every(card=>card.querySelector('.cover'))",
        ),
      );
      check(
        "one number palette across all games",
        await evaluate(
          "new Set([...document.querySelectorAll('.admin-game-number')].map(e=>getComputedStyle(e).backgroundColor)).size===1",
        ),
      );
      check(
        "configuration fields are not on cards",
        await evaluate(
          "!document.querySelector('.admin-game-card [role=switch]')",
        ),
      );
      check(
        "no separate sort action",
        await evaluate(
          "![...document.querySelectorAll('button')].some(e=>e.textContent.trim()==='游戏排序')",
        ),
      );
      await wait(
        "[...document.querySelectorAll('.admin-game-card .cover img')].length>=5&&[...document.querySelectorAll('.admin-game-card .cover img')].every(e=>e.complete&&e.naturalWidth>0)",
      );
      await browser("set", "viewport", "1280", "950");
      const initial = await order();
      await browser("scrollintoview", card(initial[0]));
      await browser("screenshot", path.join(output, "desktop.png"));
      await browser(
        "drag",
        card(initial[0]) + " .cover",
        card(initial[1]) + " .cover",
      );
      await wait(
        `document.querySelector('.admin-game-card').dataset.gameId===${JSON.stringify(initial[1])}`,
      );
      await wait(
        "!document.querySelector('.admin-games').getAttribute('aria-busy')||document.querySelector('.admin-games').getAttribute('aria-busy')==='false'",
      );
      let saved = await order();
      check(
        "desktop drag saves directly",
        saved[0] === initial[1] && saved[1] === initial[0],
      );
      check(
        "sort does not create a dialog",
        await evaluate("!document.querySelector('[role=dialog]')"),
      );
      check(
        "player order follows admin order",
        JSON.stringify((await api("/state")).availableGames) ===
          JSON.stringify(saved),
      );
      await browser(
        "fill",
        'input[aria-label="搜索管理游戏"]',
        "configfixture",
      );
      await wait("document.querySelectorAll('.admin-game-card').length===36");
      const filtered = await ids();
      await browser("focus", card(filtered[0]) + " .admin-sort-handle");
      await browser("press", "ArrowRight");
      await wait(
        `document.querySelector('.admin-game-card').dataset.gameId===${JSON.stringify(filtered[1])}`,
      );
      saved = await order();
      check(
        "keyboard sorts visible slots only",
        saved.slice(0, 5).join() ===
          initial
            .slice(0, 5)
            .map((id, index) =>
              index === 0 ? initial[1] : index === 1 ? initial[0] : id,
            )
            .join(),
      );
      await browser("fill", 'input[aria-label="搜索管理游戏"]', "铁路环游");
      await wait("document.querySelectorAll('.admin-game-card').length===1");
      await browser("click", card("rail") + " [data-game-config]");
      await browser("wait", ".admin-game-settings");
      check(
        "dialog identifies the selected game",
        await evaluate(
          "document.querySelector('.admin-game-settings').textContent.includes('铁路环游')",
        ),
      );
      await browser("click", '.admin-game-settings [role="switch"]');
      await wait(
        "document.querySelector('.admin-game-settings [role=switch]').getAttribute('aria-checked')==='true'&&!document.querySelector('.admin-game-settings [role=switch]').disabled",
      );
      check(
        "dialog persists base config",
        (await api("/state")).gameConfigs.rail.autoFullscreen,
      );
      for (const width of [1280, 320]) {
        await browser("set", "viewport", String(width), "900");
        check(
          "dialog has no overflow " + width,
          await evaluate("document.documentElement.scrollWidth<=innerWidth"),
        );
        await browser(
          "screenshot",
          path.join(output, "settings-" + width + ".png"),
        );
      }
      await browser("press", "Escape");
      await wait("!document.querySelector('.admin-game-settings')");
      check(
        "dialog restores focus",
        await evaluate("document.activeElement.matches('[data-game-config]')"),
      );
      await browser("fill", 'input[aria-label="搜索管理游戏"]', " ");
      await wait("document.querySelectorAll('.admin-game-card').length>36");
      await browser("screenshot", path.join(output, "mobile.png"));
      check(
        "mobile no overflow",
        await evaluate("document.documentElement.scrollWidth<=innerWidth"),
      );
      const endpoint = await browser("get", "cdp-url");
      socket = new WebSocket(
        typeof endpoint === "string" ? endpoint : endpoint.cdpUrl,
      );
      await new Promise((resolve, reject) => {
        socket.onopen = resolve;
        socket.onerror = reject;
      });
      let serial = 0;
      const pending = new Map();
      socket.onmessage = ({ data }) => {
        const value = JSON.parse(data);
        const request = pending.get(value.id);
        if (!request) return;
        pending.delete(value.id);
        value.error
          ? request.reject(Error(JSON.stringify(value.error)))
          : request.resolve(value.result);
      };
      const call = (method, params = {}, sessionId) =>
        new Promise((resolve, reject) => {
          const id = ++serial;
          pending.set(id, { resolve, reject });
          socket.send(JSON.stringify({ id, method, params, sessionId }));
        });
      const targets = await call("Target.getTargets");
      const target = targets.targetInfos.find(
        (value) => value.type === "page" && value.url.startsWith(url),
      );
      const { sessionId } = await call("Target.attachToTarget", {
        targetId: target.targetId,
        flatten: true,
      });
      await call(
        "Emulation.setTouchEmulationEnabled",
        { enabled: true, maxTouchPoints: 5 },
        sessionId,
      );
      await browser(
        "fill",
        'input[aria-label="搜索管理游戏"]',
        "configfixture",
      );
      await wait("document.querySelectorAll('.admin-game-card').length===36");
      const mobile = await ids();
      await browser("scrollintoview", card(mobile[0]));
      const points = await evaluate(
        `(()=>{const a=document.querySelector(${JSON.stringify(card(mobile[0]) + " .admin-sort-handle")}).getBoundingClientRect();const b=document.querySelector(${JSON.stringify(card(mobile[1]))}).getBoundingClientRect();return [{x:Math.round(a.x+a.width/2),y:Math.round(a.y+a.height/2)},{x:Math.round(b.x+b.width/2),y:Math.round(b.y+30)}]})()`,
      );
      await call(
        "Input.dispatchTouchEvent",
        { type: "touchStart", touchPoints: [{ ...points[0], id: 1 }] },
        sessionId,
      );
      await call(
        "Input.dispatchTouchEvent",
        { type: "touchMove", touchPoints: [{ ...points[1], id: 1 }] },
        sessionId,
      );
      await call(
        "Input.dispatchTouchEvent",
        { type: "touchEnd", touchPoints: [] },
        sessionId,
      );
      await wait(
        `document.querySelector('.admin-game-card').dataset.gameId===${JSON.stringify(mobile[1])}`,
      );
      check(
        "native touch sorting saves",
        (await order()).indexOf(mobile[0]) > (await order()).indexOf(mobile[1]),
      );
      await browser("set", "viewport", "1280", "950");
      await browser("fill", 'input[aria-label="搜索管理游戏"]', " ");
      await wait("document.querySelectorAll('.admin-game-card').length>36");
      const beforeFailure = await ids();
      await evaluate(
        "window.listFetch=window.fetch;window.fetch=(url,options)=>String(url)==='/api/admin/games/order'?Promise.resolve(new Response(JSON.stringify({error:'排序保存验收失败'}),{status:500,headers:{'Content-Type':'application/json'}})):window.listFetch(url,options);true",
      );
      await browser("scrollintoview", card(beforeFailure[0]));
      await browser("focus", card(beforeFailure[0]) + " .admin-sort-handle");
      await browser("press", "ArrowRight");
      await browser("wait", "--text", "排序保存验收失败");
      check(
        "save failure restores persisted order",
        JSON.stringify(await ids()) === JSON.stringify(beforeFailure),
      );
      await evaluate("window.fetch=window.listFetch;true");
      const concurrent = [...beforeFailure].reverse();
      await api("/admin/games/order", {
        order: concurrent,
        expectedOrder: beforeFailure,
      });
      await browser("focus", card(beforeFailure[0]) + " .admin-sort-handle");
      await browser("press", "ArrowRight");
      await wait(
        `document.querySelector('.admin-game-card').dataset.gameId===${JSON.stringify(concurrent[0])}`,
      );
      check(
        "concurrent editor wins without overwrite",
        JSON.stringify(await order()) === JSON.stringify(concurrent),
      );
      await browser("reload");
      await browser("wait", ".admin-tabs");
      await browser(
        "find",
        "role",
        "button",
        "click",
        "--name",
        "游戏管理",
        "--exact",
      );
      await wait("document.querySelectorAll('.admin-game-card').length>36");
      check(
        "reload keeps order",
        JSON.stringify(await ids()) === JSON.stringify(concurrent),
      );
      await browser(
        "fill",
        'input[aria-label="搜索管理游戏"]',
        "不存在的游戏xyz",
      );
      await browser("wait", ".game-search-empty");
      check("empty search", (await ids()).length === 0);
      report.passed = true;
    } catch (error) {
      report.error = error.stack;
      report.snapshot = await browser("snapshot").catch(() => null);
      await browser("screenshot", path.join(output, "failure.png")).catch(
        () => {},
      );
      throw error;
    } finally {
      await writeFile(
        path.join(output, "result.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      socket?.close();
      await browser("close").catch(() => {});
      await server.close();
      await rm(temporary, { recursive: true, force: true });
    }
  },
);
