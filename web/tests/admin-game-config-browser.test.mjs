import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { createHash } from "node:crypto";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";

test(
  "admin base config controls real room fullscreen on desktop and phone",
  { timeout: 240000, skip: !process.env.GAME_CONFIG_API_URL },
  async () => {
    const root = fileURLToPath(new URL("../../", import.meta.url));
    const output = path.join(root, ".local/admin-game-config");
    const temporary = path.join(output, "browser-temporary");
    const environment = {
      ...process.env,
      TMPDIR: path.join(temporary, "tmp"),
      XDG_CONFIG_HOME: path.join(temporary, "config"),
      XDG_DATA_HOME: path.join(temporary, "data"),
      XDG_CACHE_HOME: path.join(temporary, "cache"),
      AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
      AGENT_BROWSER_SOCKET_DIR: path.join(output, "s"),
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
        ["session", "id", "--scope", "worktree", "--prefix", "gc"],
        { cwd: root, env: environment },
      )
    ).stdout.trim();
    const browser = async (...args) => {
      let response;
      try {
        response = await execute(
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
      } catch (error) {
        throw Error(
          `${error.message}\n${error.stdout ?? ""}\n${error.stderr ?? ""}`,
        );
      }
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
    const command = (type) =>
      evaluate(
        `(async()=>{const state=await fetch('/api/state').then(r=>r.json());const r=await fetch('/api/rooms/'+state.room.id,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:${JSON.stringify(type)},version:state.room.version,nonce:crypto.randomUUID()})});const data=await r.json();if(!r.ok)throw Error(JSON.stringify(data));return data})()`,
      );
    const require = createRequire(new URL("../package.json", import.meta.url));
    const publishedConfig = JSON.parse(
      await readFile(path.join(root, ".local/business.json"), "utf8"),
    );
    const publishedImages = JSON.parse(await readFile(path.join(root, "web/src/ui/published-images.json"), "utf8"));
    const coverURL = publishedConfig.CDN_ORIGIN + "/" + publishedImages["/game-covers/rail.png"];
    const coverHead = await fetch(coverURL, { method: "HEAD" });
    assert.equal(coverHead.ok, true);
    const coverBytes = Number(coverHead.headers.get("content-length"));
    assert.ok(Number.isSafeInteger(coverBytes) && coverBytes > 0);
    const { createServer } = require("vite");
    const react = require("@vitejs/plugin-react").default;
    const server = await createServer({
      configFile: false,
      define: {
        "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(
          publishedConfig.CDN_ORIGIN,
        ),
      },
      root: path.join(root, "web"),
      cacheDir: path.join(temporary, "vite-cache"),
      publicDir: false,
      plugins: [
        react(),
        {
          name: "config-development-manifest",
          configureServer(server) {
            server.middlewares.use(
              "/game-load-manifest.json",
              (request, response) => {
                response.setHeader("Content-Type", "application/json");
                response.end(
                  JSON.stringify({
                    build: "",
                    assetBase: publishedConfig.CDN_ORIGIN + "/bga/",
                    home: { files: [{ url: coverURL, bytes: coverBytes }] },
                    entry: "/src/main.tsx",
                    games: { rail: [] },
                    media: {},
                  }),
                );
              },
            );
          },
        },
      ],
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
        headers: { "Service-Worker-Allowed": "/" },
        fs: { allow: [root] },
        proxy: {
          "/api": { target: process.env.GAME_CONFIG_API_URL, ws: true },
        },
      },
    });
    const report = {
      passed: false,
      scope:
        "Actual platform React pages, authenticated HTTP, SQLite and Rail; local development manifest without bundled media downloads; Chromium desktop and 320px touch emulation",
      sources: Object.fromEntries(
        await Promise.all(
          [
            "web/src/admin.tsx",
            "web/src/admin.css",
            "web/src/main.tsx",
            "web/src/game-api/loading.tsx",
            "web/src/ui/room-fullscreen.tsx",
            "internal/server/game_config.go",
            "web/tests/admin-game-config-browser.test.mjs",
          ].map(async (file) => [
            file,
            createHash("sha256")
              .update(await readFile(path.join(root, file)))
              .digest("hex"),
          ]),
        ),
      ),
      checks: [],
      layouts: [],
    };
    const check = (name, condition) => {
      assert.ok(condition, name);
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
      await browser("fill", 'input[aria-label="搜索管理游戏"]', "铁路环游");
      await browser("wait", '[data-game-id="rail"]');
      const openSettings = async () => {
        await browser("click", '[data-game-id="rail"] [data-game-config]');
        await browser("wait", ".admin-game-settings");
      };
      await openSettings();
      const toggle = '.admin-game-settings [role="switch"]';
      check(
        "new game defaults off",
        await evaluate(
          `document.querySelector(${JSON.stringify(toggle)}).getAttribute('aria-checked')==='false'`,
        ),
      );
      await browser("click", toggle);
      await wait(
        `document.querySelector(${JSON.stringify(toggle)}).getAttribute('aria-checked')==='true'&&!document.querySelector(${JSON.stringify(toggle)}).disabled`,
      );
      check(
        "saved configuration delivered to players",
        (await api("/state")).gameConfigs.rail.autoFullscreen,
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
      await browser("fill", 'input[aria-label="搜索管理游戏"]', "铁路环游");
      await openSettings();
      await wait(
        `document.querySelector(${JSON.stringify(toggle)})?.getAttribute('aria-checked')==='true'`,
      );
      check("reload preserves saved flag", true);
      await evaluate(
        "window.configFetch=window.fetch;window.fetch=(url,options)=>String(url).endsWith('/rail/config')?new Promise(resolve=>window.configSave=()=>resolve(new Response(JSON.stringify({error:'配置保存验收失败'}),{status:500,headers:{'Content-Type':'application/json'}}))):window.configFetch(url,options);true",
      );
      await browser("click", toggle);
      check(
        "pending save disables the switch",
        await evaluate(
          `document.querySelector(${JSON.stringify(toggle)}).disabled`,
        ),
      );
      await evaluate("window.configSave();true");
      await browser("wait", "--text", "配置保存验收失败");
      check(
        "failed save retains previous flag",
        await evaluate(
          `document.querySelector(${JSON.stringify(toggle)}).getAttribute('aria-checked')==='true'`,
        ),
      );
      await evaluate("window.fetch=window.configFetch;true");
      await browser("click", toggle);
      await wait(
        `document.querySelector(${JSON.stringify(toggle)}).getAttribute('aria-checked')==='false'&&!document.querySelector(${JSON.stringify(toggle)}).disabled`,
      );
      await browser("click", toggle);
      await wait(
        `document.querySelector(${JSON.stringify(toggle)}).getAttribute('aria-checked')==='true'&&!document.querySelector(${JSON.stringify(toggle)}).disabled`,
      );
      check("retry saves successfully after failure", true);
      for (const width of [1280, 320]) {
        await browser("set", "viewport", String(width), "850");
        if (await evaluate("!!document.querySelector('.admin-game-settings')"))
          await browser("press", "Escape");
        await browser("fill", 'input[aria-label="搜索管理游戏"]', " ");
        await wait("document.querySelectorAll('.admin-game-card').length>36");
        check(
          "game list no overflow " + width,
          await evaluate("document.documentElement.scrollWidth<=innerWidth"),
        );
        check(
          "large list and long names " + width,
          await evaluate(
            "document.querySelectorAll('.admin-game-card').length>36&&document.querySelector('.admin-games').textContent.includes('基础配置长名称游戏与游戏列表换行验收')",
          ),
        );
        await browser("scrollintoview", '[data-game-id="configfixture0"]');
        await browser(
          "screenshot",
          path.join(output, "admin-list-" + width + ".png"),
        );
        await browser("fill", 'input[aria-label="搜索管理游戏"]', "铁路环游");
        await openSettings();
        await browser("scrollintoview", toggle);
        const layout = await evaluate(
          `(()=>{const button=document.querySelector(${JSON.stringify(toggle)});const r=button.getBoundingClientRect();const style=getComputedStyle(button);return {width:Math.round(r.width),height:Math.round(r.height),color:style.color,background:style.backgroundColor}})()`,
        );
        report.layouts.push({ viewport: width, ...layout });
        const luminance = (color) => {
          const channels = color
            .match(/\d+/g)
            .slice(0, 3)
            .map(Number)
            .map((channel) => channel / 255)
            .map((channel) =>
              channel <= 0.04045
                ? channel / 12.92
                : ((channel + 0.055) / 1.055) ** 2.4,
            );
          return (
            channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722
          );
        };
        const foreground = luminance(layout.color);
        const background = luminance(layout.background);
        layout.contrastMilli = Math.round(
          ((Math.max(foreground, background) + 0.05) /
            (Math.min(foreground, background) + 0.05)) *
            1000,
        );
        check("switch text contrast " + width, layout.contrastMilli >= 4500);
        check(
          "touch target " + width,
          layout.width >= 44 && layout.height >= 44,
        );
        await browser(
          "screenshot",
          path.join(output, "admin-" + width + ".png"),
        );
        await browser("press", "Escape");
        await browser(
          "fill",
          'input[aria-label="搜索管理游戏"]',
          "不存在的游戏xyz",
        );
        await browser("wait", ".game-search-empty");
        check(
          "empty search " + width,
          await evaluate(
            "document.querySelectorAll('.admin-game-card').length===0",
          ),
        );
      }
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
      for (const [name, width, touch, automatic] of [
        ["desktop", 1280, false, true],
        ["phone", 320, true, true],
        ["off-phone", 320, true, false],
      ]) {
        await api("/admin/games/rail/config", { autoFullscreen: automatic });
        await browser("set", "viewport", String(width), "850");
        await call(
          "Emulation.setTouchEmulationEnabled",
          { enabled: touch, maxTouchPoints: 5 },
          sessionId,
        );
        await browser("open", url);
        await browser("wait", ".profile-nav");
        await evaluate(
          "Element.prototype.requestFullscreen=()=>Promise.reject(Error('no user activation'));true",
        );
        await evaluate(
          "window.configFetch=window.fetch;window.fetch=(url,options)=>String(url)==='/game-load-manifest.json'?new Promise(resolve=>window.releaseConfigManifest=()=>resolve(window.configFetch(url,options))):window.configFetch(url,options);true",
        );
        await api("/rooms", {
          kind: "rail",
          capacity: 2,
          name: "全屏配置验收",
        });
        await browser("wait", ".waiting-seat");
        check(
          "waiting stays outside fullscreen " + name,
          await evaluate(
            "!document.querySelector('[data-room-fullscreen]')&&!document.querySelector('[data-fullscreen-enter]')",
          ),
        );
        await command("add_bot");
        await command("ready");
        await command("start");
        await wait(
          "!!document.querySelector('[data-game-fullscreen-surface]')",
        );
        await wait("typeof window.releaseConfigManifest==='function'");
        if (automatic) {
          await browser(
            "wait",
            "[data-room-fullscreen=page] [data-game-loading]",
          );
          check(
            "fullscreen before resources are ready " + name,
            await evaluate(
              "!!document.querySelector('[data-room-fullscreen] [data-game-loading]')&&!document.querySelector('[data-room-fullscreen] svg.rail-map')",
            ),
          );
        } else {
          check(
            "disabled during loading",
            await evaluate("!document.querySelector('[data-room-fullscreen]')"),
          );
        }
        await evaluate(
          "window.releaseConfigManifest();window.fetch=window.configFetch;true",
        );
        await wait(
          "!!document.querySelector('[data-fullscreen-enter]')&&!document.querySelector('[data-fullscreen-enter]').disabled",
        );
        if (automatic) {
          await browser("wait", "[data-room-fullscreen=page]");
          check(
            "game focus after loading " + name,
            await evaluate(
              "document.querySelector('[data-room-fullscreen]').contains(document.activeElement)",
            ),
          );
          check(
            "automatic page fallback " + name,
            await evaluate(
              "document.querySelector('[data-room-fullscreen]').getBoundingClientRect().width===innerWidth",
            ),
          );
          await browser(
            "screenshot",
            path.join(output, "playing-" + name + ".png"),
          );
          await browser("press", "Escape");
          await wait("!document.querySelector('[data-room-fullscreen]')");
          await evaluate(
            "window.configBoard=document.querySelector('[data-fullscreen-board]').firstElementChild;true",
          );
          await browser(
            "find",
            "role",
            "button",
            "click",
            "--name",
            "全屏",
            "--exact",
          );
          await browser("wait", "[data-room-fullscreen]");
          await browser("click", 'button[aria-label="退出全屏"]');
          await wait("!document.querySelector('[data-room-fullscreen]')");
          check(
            "exit restores state and board " + name,
            await evaluate(
              "document.body.style.position===''&&![...document.querySelectorAll('[inert]')].some(e=>e.inert)&&window.configBoard===document.querySelector('[data-fullscreen-board]').firstElementChild",
            ),
          );
          await api("/admin/games/rail/config", { autoFullscreen: false });
          await api("/admin/games/rail/config", { autoFullscreen: true });
          await browser("wait", ".profile-nav");
          check(
            "config refresh does not reenter exited match " + name,
            await evaluate("!document.querySelector('[data-room-fullscreen]')"),
          );
        } else {
          check(
            "disabled config never auto enters",
            await evaluate("!document.querySelector('[data-room-fullscreen]')"),
          );
        }
        const state = await api("/state");
        if (name === "desktop") {
          await api("/logout", {});
          await api("/register", {
            name: "ConfigViewer",
            password: "config-viewer-password",
            invite: "test-invite",
          });
          await api("/rooms/" + state.room.id + "/watch", {});
          await browser("open", url);
          await wait(
            "!!document.querySelector('[data-fullscreen-enter]')&&!document.querySelector('[data-fullscreen-enter]').disabled",
          );
          check(
            "spectators stay outside fullscreen",
            await evaluate("!document.querySelector('[data-room-fullscreen]')"),
          );
          await api("/logout", {});
          await api("/login", {
            name: "ConfigRoot",
            password: "config-test-password-123",
          });
          await browser("open", url);
          await wait(
            "!!document.querySelector('[data-game-fullscreen-surface]')",
          );
        }
        await api("/admin/rooms/" + state.room.id + "/close", {});
        await command("leave");
      }
      report.passed = true;
      await rm(path.join(output, "failure.png"), { force: true });
    } catch (error) {
      report.error = error.stack;
      report.images = await evaluate(
        "[...document.images].map(image=>({src:image.src,current:image.currentSrc,complete:image.complete,width:image.naturalWidth}))",
      ).catch(() => null);
      report.snapshot = await browser("snapshot").catch(() => null);
      report.state = await api("/state")
        .then((state) => ({ room: state.room, config: state.gameConfigs }))
        .catch(() => null);
      await browser("screenshot", path.join(output, "failure.png")).catch(
        () => {},
      );
      throw error;
    } finally {
      await writeFile(
        path.join(output, "browser-result.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      socket?.close();
      await browser("close").catch(() => {});
      await server.close();
      await rm(temporary, { recursive: true, force: true });
    }
  },
);
